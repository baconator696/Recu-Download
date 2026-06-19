package recu

import (
	"fmt"
	"recurbate/config/typ"
	"recurbate/playlist"
	"recurbate/playlist/resolution"
	"recurbate/tools"
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

func parseDownloadLoop(status chan string, url string, timeout int, header map[string]string) (data []byte, err error) {
	retryCh := make(chan error)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(retryCh)
		data, _, err = tools.RequestRetry(url, timeout, retryCh, []int{200}, 30, time.Millisecond*200, timeout, header, nil, "GET")
	}()
	for range retryCh {
		status <- "Failed Retrying...\n"
	}
	wg.Wait()
	return
}

// Takes recurbate video URL and returns playlist raw data and returns file name {ts-urls, filename, "done", error}
func Parse(video *typ.Video) (errT errorType, err error) {
	// getting webpage
	video.MsgCh <- "\rDownloading HTML: "
	htmldata, err := parseDownloadLoop(video.MsgCh, video.Url, 10, tools.FormatedHeader(video.Header, "", 1))
	if err != nil {
		errT = CLOUDFLARE
		return
	}
	html := string(htmldata)
	video.MsgCh <- "Complete\n"
	// determine video ID
	id, err := regexVideoIDMatch(video.Url)
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
	apiUrl := strings.Join(strings.Split(video.Url, "/")[:3], "/") + "/api/video/" + id + "?token=" + token
	// request api
	video.MsgCh <- "\rGetting Link to Playlist: "
	apidata, err := parseDownloadLoop(video.MsgCh, apiUrl, 10, tools.FormatedHeader(video.Header, video.Url, 2))
	if err != nil {
		errT = OTHER
		return
	}
	api := string(apidata)
	// continue based on response from api
	video.MsgCh <- "Complete\n"
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
	video.MsgCh <- "\rDownloading Playlists: "
	// get m3u8 playlist
	playlistData, err := parseDownloadLoop(video.MsgCh, playlistUrl, 10, tools.FormatedHeader(video.Header, "", 0))
	if err != nil {
		errT = OTHER
		return
	}
	// determine url prefix for playlist entries
	prefix := playlistUrl[:strings.LastIndex(playlistUrl, "/")+1]
	// if playlist contains resolution selection
	playlistData, err = get_resolution_playlist(video.MsgCh, playlistData, prefix, video.Header, video.MaxRes)
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
	video.Playlist, err = playlist.New(video.ErrCh, []byte(strings.Join(playlistLines, "\n")), playlistUrl)
	if err != nil {
		errT = OTHER
	}
	video.MsgCh <- "Complete\n"
	return
}

// If playlist contains list of resolutions, return the maximum Resolution playlist
func get_resolution_playlist(statusCh chan string, playlistData []byte, prefix string, header map[string]string, maxRes int) ([]byte, error) {
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
