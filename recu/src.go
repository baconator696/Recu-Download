package recu

import (
	"fmt"
	"os"
	"recurbate/playlist"
	"recurbate/tools"
	"strings"
	"time"
)

// Takes recurbate video URL and returns playlist raw data and returns file name {ts-urls, filename, "done", error}
func Parse(siteUrl string, header map[string]string) (playList playlist.Playlist, errorType string, err error) {
	// http request
	downloadLoop := func(url string, timeout int, header map[string]string) (data []byte, err error) {
		retry := 0
		for {
			var status int
			data, status, err = tools.Request(url, timeout, header, nil, "GET")
			if err == nil && status == 200 {
				break
			}
			fmt.Printf("Failed Retrying...\033[18D")
			if retry > 5 {
				if err == nil {
					err = fmt.Errorf("%s, status code: %d", tools.ANSIColor(string(data), 2), status)
				}
				return
			}
			retry++
			timeout += 30
			time.Sleep(time.Millisecond * 200)
		}
		return
	}
	// getting webpage
	fmt.Printf("\rDownloading HTML: ")
	htmldata, err := downloadLoop(siteUrl, 10, tools.FormatedHeader(header, "", 1))
	if err != nil {
		errorType = "cloudflare"
		return
	}
	html := string(htmldata)
	fmt.Printf("\r\033[2KDownloading HTML: Complete\n")
	// determine unique page token
	token, err := tools.SearchString(html, `data-token="`, `"`)
	if err != nil {
		errorType = "panic"
		return
	}
	// determine video token
	id, err := tools.SearchString(html[strings.Index(html, token):], `data-video-id="`, `"`)
	if err != nil {
		errorType = "panic"
		return
	}
	// parse api url
	apiUrl := strings.Join(strings.Split(siteUrl, "/")[:3], "/") + "/api/video/" + id + "?token=" + token
	// request api
	fmt.Printf("\rGetting Link to Playlist: ")
	apidata, err := downloadLoop(apiUrl, 10, tools.FormatedHeader(header, apiUrl, 2))
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
	playlistData, err := downloadLoop(playlistUrl, 10, tools.FormatedHeader(header, "", 0))
	if err != nil {
		errorType = "panic"
	}
	playlistRef := string(playlistData)
	playlistLines := strings.Split(playlistRef, "\n")
	fmt.Printf("\r\033[2KDownloading Playlists: Complete\n")
	// determine url prefix for playlist entries
	prefix := playlistUrl[:strings.LastIndex(playlistUrl, "/")+1]
	// if playlist contains resolution selection
	if strings.Contains(playlistRef, "EXT-X-STREAM-INF") {
		for i := 0; i < len(playlistLines)-1; i++ {
			if strings.Contains(playlistLines[i], "NAME=max") {
				playlistUrl = playlistLines[i+1]
				if !strings.Contains(playlistUrl, prefix) {
					playlistUrl = prefix + playlistUrl
				}
			}
		}
		fmt.Printf("\rDownloading Playlist: ")
		playlistData, err = downloadLoop(playlistUrl, 10, tools.FormatedHeader(header, "", 0))
		if err != nil {
			errorType = "panic"
			return
		}
		playlistLines = strings.Split(string(playlistData), "\n")
		fmt.Printf("\r\033[2KDownloading Playlist: Complete\n")
	}
	// added prefix to playlist
	for i, line := range playlistLines {
		if len(line) < 2 || line[0] == '#' {
			continue
		}
		if !strings.Contains(line, prefix) {
			playlistLines[i] = prefix + line
		}
	}
	playList, err = playlist.New([]byte(strings.Join(playlistLines, "\n")), playlistUrl)
	if err != nil {
		errorType = "panic"
	}
	return
}

// Muxes the transport streams and saves it to a file
func Mux(playList playlist.Playlist, header map[string]string, restartIndex int, durationPercent []float64) int {
	var data []byte
	var err error
	var file *os.File
	var avgdur, avgsize tools.AvgBuffer
	restarted := false
	if restartIndex != 0 {
		restarted = true
	}
	if durationPercent[0] > 100 || durationPercent[1] <= durationPercent[0] {
		return 0
	}
	if durationPercent[0] < 0 {
		durationPercent[0] = 0
	}
	if durationPercent[1] > 100 {
		durationPercent[1] = 100
	}
	// checks if continuation of previous run
	if restarted {
		file, err = os.OpenFile(playList.Filename+".ts", os.O_APPEND|os.O_WRONLY, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "oringal file not found, creating new one: %v", err)
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
			fmt.Fprintf(os.Stderr, "can not create file: %v", err)
			return restartIndex
		}
	}
	defer file.Close()
	// muxing loop //
	var startIndex, endIndex int
	if restarted {
		startIndex = restartIndex
	} else {
		startIndex = int(float64(playList.Len()) * durationPercent[0] / 100)
	}
	endIndex = int(float64(playList.Len()) * durationPercent[1] / 100)
	for i, tsLink := range playList.List[startIndex:endIndex] {
		i := i + startIndex
		startTime := time.Now()
		err := downloadLoop(&data,tsLink, header, 10, 5)
		if err != nil {
			fmt.Println()
			fmt.Fprintf(os.Stderr, "Error: %v\n", tools.ANSIColor(err, 2))
			fmt.Fprintf(os.Stderr, "Failed at %.2f%%\n", float32(i)/float32(playList.Len())*100)
			return i
		}
		endDur := time.Since(startTime).Minutes()
		_, err = file.Write(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "can not write file: %v", err)
			return i
		}
		// Calculate User Interface Timings
		avgsize.Add(float64(len(data)))
		avgdur.Add(endDur)
		getavgdur := avgdur.Average()
		speedSecs := avgsize.Average() / (getavgdur * 60)
		eta := getavgdur * ((float64(playList.Len()) * durationPercent[1] / 100) - float64(i))
		percent := float64(i) / float64(playList.Len()) * 100
		fmt.Printf("\n\033[A\033[2KDownloading: %s\tRemaining: %s\t%s", tools.ANSIColor(fmt.Sprintf("%.1f%%", percent), 33), tools.FormatMinutes(eta), tools.FormatBytesPerSecond(speedSecs))
		if tools.Abort {
			fmt.Println("\naborting...")
			return i
		}
	}
	fmt.Println()
	return 0
}

// download retry loop for Mux()
func downloadLoop(data *[]byte, url string, header map[string]string, timeout, maxRetry int) (err error) {
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
