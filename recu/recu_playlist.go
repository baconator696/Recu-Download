package recu

import (
	"fmt"
	"recurbate/config/state"
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

var (
	// video ID
	regexVideoId      *regexp.Regexp
	regexVideoIdMutex sync.Mutex
	// TOKEN
	regexToken      *regexp.Regexp
	regexTokenMutex sync.Mutex
	// requestId
	regexRequestId      *regexp.Regexp
	regexRequestIdMutex sync.Mutex
)

// return video ID from given video url
func regexVideoIDMatch(text string) (string, error) {
	regexVideoIdMutex.Lock()
	if regexVideoId == nil {
		regexVideoId = regexp.MustCompile(`([\d]*)/play`)
	}
	regexVideoIdMutex.Unlock()
	matches := regexVideoId.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1], nil
	}
	return "", fmt.Errorf("video id match not found")
}

// return video TOKEN from given video url
func regexTokenMatch(text string) (string, error) {
	regexTokenMutex.Lock()
	if regexToken == nil {
		regexToken = regexp.MustCompile(`window\.token = "([^"]+)"`)
	}
	regexTokenMutex.Unlock()
	matches := regexToken.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1], nil
	}
	return "", fmt.Errorf("video TOKEN not found")
}

// return requestId from given video url
func regexRequestIdMatch(text string) (string, error) {
	regexRequestIdMutex.Lock()
	if regexRequestId == nil {
		regexRequestId = regexp.MustCompile(`window\.requestId = "([^"]+)"`)
	}
	regexRequestIdMutex.Unlock()
	matches := regexRequestId.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1], nil
	}
	return "", fmt.Errorf("requestId match not found")
}

func parseDownloadLoop(status chan string, url string, retry int, timeout int, header map[string]string) (data []byte, err error) {
	retryCh := make(chan error)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(retryCh)
		data, _, err = tools.RequestRetry(url, retry, retryCh, []int{200}, 30, time.Millisecond*200, timeout, header, nil, "GET")
	}()
	for range retryCh {
		status <- "Failed Retrying...\n"
	}
	wg.Wait()
	return
}

// Takes recurbate video URL and returns playlist raw data and returns file name {ts-urls, filename, "done", error}
func Parse(video *state.Video, conf *state.Config) (errT errorType, err error) {
	// getting webpage
	video.State.Stage = state.HTML
	conf.MsgCh <- "\rDownloading HTML: "
	htmldata, err := parseDownloadLoop(conf.MsgCh, video.Url, 3, 10, tools.FormatedHeader(video.Header, "", tools.HTML_HEADER, nil))
	if err != nil {
		errT = CLOUDFLARE
		return
	}
	html := string(htmldata)
	conf.MsgCh <- "Complete\n"
	// determine video ID
	id, err := regexVideoIDMatch(video.Url)
	if err != nil {
		errT = OTHER
		return
	}
	// determine unique page token
	token, err := regexTokenMatch(html)
	if err != nil {
		errT = OTHER
		return
	}
	// determine unique request ID
	requestId, err := regexRequestIdMatch(html)
	if err != nil {
		errT = OTHER
		return
	}
	// parse api url
	apiUrl := strings.Join(strings.Split(video.Url, "/")[:3], "/") + "/api/video/" + id + "?parent-request=" + requestId
	// request api
	video.State.Stage = state.PLAYLISTURL
	conf.MsgCh <- "\rGetting Link to Playlist: "
	apidata, err := parseDownloadLoop(conf.MsgCh, apiUrl, 3, 10, tools.FormatedHeader(video.Header, video.Url, tools.API_PLAYLIST_HEADER, &token))
	if err != nil {
		errT = OTHER
		return
	}
	api := string(apidata)
	// continue based on response from api
	conf.MsgCh <- "Complete\n"
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
	video.State.Stage = state.PLAYLIST
	conf.MsgCh <- "\rDownloading Playlists: "
	// get m3u8 playlist
	playlistData, err := parseDownloadLoop(conf.MsgCh, playlistUrl, 3, 10, tools.FormatedHeader(video.Header, "", tools.PLAYLIST_HEADER, &token))
	if err != nil {
		errT = OTHER
		return
	}
	// determine url prefix for playlist entries
	prefix := playlistUrl[:strings.LastIndex(playlistUrl, "/")+1]
	// if playlist contains resolution selection
	playlistData, err = get_resolution_playlist(conf.MsgCh, playlistData, prefix, video.Header, video.MaxRes)
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
	video.Playlist, err = playlist.New(conf.ErrCh, []byte(strings.Join(playlistLines, "\n")), playlistUrl)
	if err != nil {
		errT = OTHER
	}
	conf.MsgCh <- "Complete\n"
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
		playlistData, err = parseDownloadLoop(statusCh, playlistUrl, 3, 10, tools.FormatedHeader(header, "", tools.PLAYLIST_HEADER, nil))
		if err != nil {
			return nil, err
		}
	}
	return playlistData, nil
}
