package recu

import (
	"fmt"
	"os"
	"recurbate/playlist"
	"recurbate/tools"
	"regexp"
	"strings"
	"sync"
	"time"
)

// returns token from given recu html
func regexTokenMatch(html string, videoid string) (string, error) {
	term := fmt.Sprintf(`%s"[\n\s]*data-token="([^"]*)"`, videoid)
	regexToken := regexp.MustCompile(term)
	matches := regexToken.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1], nil
	}
	return "", fmt.Errorf("video token not found")
}

var (
	regexVideoID      *regexp.Regexp
	regexVideoIDMutex sync.Mutex
)

// return video ID from given video url
func regexVideoIDMatch(text string) (string, error) {
	regexVideoIDMutex.Lock()
	if regexVideoID == nil {
		regexVideoID = regexp.MustCompile(`([\d]*)/play`)
	}
	regexVideoIDMutex.Unlock()
	matches := regexVideoID.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1], nil
	}
	return "", fmt.Errorf("video id match not found")
}

func parseDownloadLoop(url string, timeout int, header map[string]string) (data []byte, err error) {
	ch := make(chan error)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		data, _, err = tools.RequestRetry(url, timeout, ch, []int{200}, 30, time.Millisecond*200, timeout, header, nil, "GET")
	}()
	for range ch {
		fmt.Printf("Failed Retrying...\033[18D")
	}
	wg.Wait()
	return
}

// Takes recurbate video URL and returns playlist raw data and returns file name {ts-urls, filename, "done", error}
func Parse(siteUrl string, header map[string]string, jsonLoc, maxRes int) (playList playlist.Playlist, errorType string, err error) {
	// getting webpage
	fmt.Printf("\rDownloading HTML: ")
	htmldata, err := parseDownloadLoop(siteUrl, 10, tools.FormatedHeader(header, "", 1))
	if err != nil {
		errorType = "cloudflare"
		return
	}
	html := string(htmldata)
	fmt.Printf("\r\033[2KDownloading HTML: Complete\n")
	// determine video ID
	id, err := regexVideoIDMatch(siteUrl)
	if err != nil {
		errorType = "panic"
		return
	}
	// determine unique page token
	token, err := regexTokenMatch(html, id)
	if err != nil {
		errorType = "panic"
		return
	}
	// parse api url
	apiUrl := strings.Join(strings.Split(siteUrl, "/")[:3], "/") + "/api/video/" + id + "?token=" + token
	// request api
	fmt.Printf("\rGetting Link to Playlist: ")
	apidata, err := parseDownloadLoop(apiUrl, 10, tools.FormatedHeader(header, siteUrl, 2))
	if err != nil {
		errorType = "panic"
		return
	}
	api := string(apidata)
	// continue based on response from api
	fmt.Printf("\r\033[2KGetting Link to Playlist: Complete\n")
	switch api {
	case "shall_subscribe":
		errorType = "wait"
		return
	case "shall_signin":
		errorType = "cookie"
		return
	case "wrong_token":
		errorType = "panic"
		err = fmt.Errorf("wrong token")
		return
	}
	// search for m3u8 link from api response
	playlistUrl, err := tools.SearchString(api, `<source src="`, `"`)
	if err != nil {
		errorType = "panic"
		return
	}
	playlistUrl = strings.ReplaceAll(playlistUrl, "amp;", "")
	fmt.Printf("\rDownloading Playlists: ")
	// get m3u8 playlist
	playlistData, err := parseDownloadLoop(playlistUrl, 10, tools.FormatedHeader(header, "", 0))
	if err != nil {
		errorType = "panic"
		return
	}
	fmt.Printf("\r\033[2KDownloading Playlists: Complete\n")
	// determine url prefix for playlist entries
	prefix := playlistUrl[:strings.LastIndex(playlistUrl, "/")+1]
	// if playlist contains resolution selection
	playlistData, err = resolution(playlistData, prefix, header, maxRes)
	if err != nil {
		errorType = "panic"
		return
	}
	playlistRef := string(playlistData)
	// added prefix to playlist
	playlistLines := strings.Split(playlistRef, "\n")
	for i, line := range playlistLines {
		if len(line) < 2 || line[0] == '#' {
			continue
		}
		if !strings.Contains(line, prefix) {
			playlistLines[i] = prefix + line
		}
	}
	playList, err = playlist.New([]byte(strings.Join(playlistLines, "\n")), playlistUrl, jsonLoc)
	if err != nil {
		errorType = "panic"
	}
	return
}

// If playlist contains list of resolutions, return the maximum Resolution playlist
func resolution(playlistData []byte, prefix string, header map[string]string, maxRes int) ([]byte, error) {
	playlistRef := string(playlistData)
	if strings.Contains(playlistRef, "EXT-X-STREAM-INF") {
		TEMP := playlist.ParseResolutionPlaylistLinks(playlistRef, prefix)
		var playlistUrl string
		TEMP.DescendLessOrEqual(playlist.IntStrN(maxRes), func(item playlist.IntStr) bool {
			playlistUrl = item.Get()
			return false
		})
		if playlistUrl == "" {
			println("The given Max Resolution isn't available, using maximum")
			max, _ := TEMP.Max()
			playlistUrl = max.Get()
		}
		fmt.Printf("\rDownloading Playlist: ")
		var err error
		playlistData, err = parseDownloadLoop(playlistUrl, 10, tools.FormatedHeader(header, "", 0))
		if err != nil {
			return nil, err
		}
		fmt.Printf("\r\033[2KDownloading Playlist: Complete\n")
	}
	return playlistData, nil
}

// Muxes the transport streams and saves it to a file
func Mux(playList playlist.Playlist, header map[string]string, startIndex int, durationPercent []float64) (failIndex int, err error) {
	var data []byte
	var file *os.File
	var avgdur, avgsize tools.AvgBuffer
	if startIndex < 0 {
		startIndex = 0
	}
	if tools.Abort {
		return startIndex, fmt.Errorf("aborting")
	}
	if durationPercent[0] > 100 || durationPercent[1] <= durationPercent[0] {
		return startIndex, fmt.Errorf("duration format error")
	}
	if durationPercent[0] < 0 {
		durationPercent[0] = 0
	}
	if durationPercent[1] > 100 {
		durationPercent[1] = 100
	}
	// checks if continuation of previous run
	if startIndex != 0 {
		file, err = os.OpenFile(playList.Filename+".ts", os.O_APPEND|os.O_WRONLY, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "original file not found, creating new one: %v", err)
		}
	}
	// creates file
	if file == nil {
		// checks for filename collisions
		_, err = os.Stat(playList.Filename + ".ts")
		if err == nil {
			for i := 1; i > 0; i++ {
				new := fmt.Sprintf("%s(%d)", playList.Filename, i)
				_, err := os.Stat(new + ".ts")
				if err != nil {
					playList.Filename = new
					break
				}
			}
		}
		file, err = os.OpenFile(playList.Filename+".ts", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			return startIndex, fmt.Errorf("can not create file: %v", err)
		}
	}
	defer file.Close()
	// muxing loop //
	if startIndex == 0 {
		startIndex = int(float64(playList.Len()) * durationPercent[0] / 100)
	}
	endIndex := int(float64(playList.Len()) * durationPercent[1] / 100)
	for i, tsLink := range playList.List[startIndex:endIndex] {
		i := i + startIndex
		if tools.Abort {
			fmt.Println()
			return i, fmt.Errorf("aborting")
		}
		startTime := time.Now()
		err := muxDownloadLoop(&data, tsLink, header, 10, 5)
		if err != nil {
			fmt.Println()
			err = fmt.Errorf("error: %v\nFailed at %.2f%%", tools.ANSIColor(err, 2), float32(i)/float32(playList.Len())*100)
			return i, err
		}
		endDur := time.Since(startTime).Minutes()
		_, err = file.Write(data)
		if err != nil {
			err = fmt.Errorf("can not write file: %v", err)
			return i, err
		}
		// Calculate User Interface Timings
		avgsize.Add(float64(len(data)))
		avgdur.Add(endDur)
		getavgdur := avgdur.Average()
		speedSecs := avgsize.Average() / (getavgdur * 60)
		eta := getavgdur * ((float64(playList.Len()) * durationPercent[1] / 100) - float64(i))
		percent := float64(i) / float64(playList.Len()) * 100
		fmt.Printf("\n\033[A\033[2KDownloading: %s\tRemaining: %s\t%s", tools.ANSIColor(fmt.Sprintf("%.1f%%", percent), 33), tools.FormatMinutes(eta), tools.FormatBytesPerSecond(speedSecs))
	}
	return 0, nil
}

// download retry loop for Mux()
func muxDownloadLoop(data *[]byte, url string, header map[string]string, timeout, maxRetry int) (err error) {
	retry := 0
	for {
		var status int
		*data, status, err = tools.Request(url, timeout, header, nil, "GET")
		if err == nil && status == 200 {
			break
		}
		if status == 429 {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if status == 410 {
			fmt.Fprintln(os.Stderr, "\nDownload Expired")
			retry = maxRetry
		}
		retry++
		if err == nil {
			err = fmt.Errorf("status Code: %d, %s ", status, string(*data))
		} else {
			timeout += 30
		}
		if retry > maxRetry {
			return
		}
		fmt.Fprintf(os.Stderr, "\n\033[2A\033[2KError: %v, Retrying...\n", tools.ANSIColor(tools.ShortenString(err, 40), 2))
		time.Sleep(time.Second)
	}
	return
}
