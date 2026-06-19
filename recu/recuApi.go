package recu

import (
	"fmt"
	"os"
	"recurbate/playlist"
	"recurbate/playlist/resolution"
	"recurbate/tools"
	"recurbate/tools/avgBuffer"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Status int

const (
	FailRetry Status = iota
	DownloadHTML
	GetPlaylistUrl
	GetPlaylist
	CompleteLastAction
)

type errorType int

const (
	OTHER errorType = iota
	COOKIE
	WAIT
	CLOUDFLARE
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

func parseDownloadLoop(status chan Status, url string, timeout int, header map[string]string) (data []byte, err error) {
	retryCh := make(chan error)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(retryCh)
		data, _, err = tools.RequestRetry(url, timeout, retryCh, []int{200}, 30, time.Millisecond*200, timeout, header, nil, "GET")
	}()
	for range retryCh {
		status <- FailRetry
	}
	wg.Wait()
	return
}

// Takes recurbate video URL and returns playlist raw data and returns file name {ts-urls, filename, "done", error}
func Parse(errCh chan error, statusCh chan Status, siteUrl string, header map[string]string, jsonLoc, maxRes int) (playList playlist.Playlist, errT errorType, err error) {
	// getting webpage
	statusCh <- DownloadHTML
	htmldata, err := parseDownloadLoop(statusCh, siteUrl, 10, tools.FormatedHeader(header, "", 1))
	if err != nil {
		errT = CLOUDFLARE
		return
	}
	html := string(htmldata)
	statusCh <- CompleteLastAction
	// determine video ID
	id, err := regexVideoIDMatch(siteUrl)
	if err != nil {
		errT = OTHER
		return
	}
	// determine unique page token
	token, err := regexTokenMatch(html, id)
	if err != nil {
		errT = OTHER
		return
	}
	// parse api url
	apiUrl := strings.Join(strings.Split(siteUrl, "/")[:3], "/") + "/api/video/" + id + "?token=" + token
	// request api
	statusCh <- GetPlaylistUrl
	apidata, err := parseDownloadLoop(statusCh, apiUrl, 10, tools.FormatedHeader(header, siteUrl, 2))
	if err != nil {
		errT = OTHER
		return
	}
	api := string(apidata)
	// continue based on response from api
	statusCh <- CompleteLastAction
	switch api {
	case "shall_subscribe":
		errT = WAIT
		return
	case "shall_signin":
		errT = COOKIE
		return
	case "wrong_token":
		errT = OTHER
		err = fmt.Errorf("wrong token")
		return
	}
	// search for m3u8 link from api response
	playlistUrl, err := tools.SearchString(api, `<source src="`, `"`)
	if err != nil {
		errT = OTHER
		return
	}
	playlistUrl = strings.ReplaceAll(playlistUrl, "amp;", "")
	statusCh <- GetPlaylist
	// get m3u8 playlist
	playlistData, err := parseDownloadLoop(statusCh, playlistUrl, 10, tools.FormatedHeader(header, "", 0))
	if err != nil {
		errT = OTHER
		return
	}
	// determine url prefix for playlist entries
	prefix := playlistUrl[:strings.LastIndex(playlistUrl, "/")+1]
	// if playlist contains resolution selection
	playlistData, err = get_resolution_playlist(statusCh, playlistData, prefix, header, maxRes)
	if err != nil {
		errT = OTHER
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
	playList, err = playlist.New(errCh, []byte(strings.Join(playlistLines, "\n")), playlistUrl, jsonLoc)
	if err != nil {
		errT = OTHER
	}
	statusCh <- CompleteLastAction
	return
}

// If playlist contains list of resolutions, return the maximum Resolution playlist
func get_resolution_playlist(statusCh chan Status, playlistData []byte, prefix string, header map[string]string, maxRes int) ([]byte, error) {
	playlistRef := string(playlistData)
	if strings.Contains(playlistRef, "EXT-X-STREAM-INF") {
		playlists, err := resolution.New(playlistRef, prefix)
		if err != nil {
			return nil, err
		}
		playlistUrl := playlists.Max(maxRes)
		playlistData, err = parseDownloadLoop(statusCh, playlistUrl, 10, tools.FormatedHeader(header, "", 0))
		if err != nil {
			return nil, err
		}
	}
	return playlistData, nil
}

// Muxes the transport streams and saves it to a file
func Mux(playList playlist.Playlist, header map[string]string, startIndex int, durationPercent []float64) (failIndex int, err error) {
	var data []byte
	var file *os.File
	avgdur := avgBuffer.New(25)
	avgsize := avgBuffer.New(25)
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
