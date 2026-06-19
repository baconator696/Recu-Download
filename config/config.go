package config

import (
	"encoding/json"
	"fmt"
	"os"
	"recurbate/playlist"
	"recurbate/recu"
	"recurbate/tools"
	"strconv"
	"strings"
	"sync"
)

var (
	mtx sync.Mutex
)

// Defines the JSON used
type Config struct {
	Urls    []any             `json:"urls"`
	Header  map[string]string `json:"header"`
	Options map[string]string `json:"options"`
}

// Gets Playlist
func (config Config) GetPlaylist(errCh chan error, statusCh chan recu.Status, urlAny any, jsonLoc int) (playList playlist.Playlist) {
	url, _, _, err, complete := parseUrl(urlAny)
	if err != nil {
		errCh <- fmt.Errorf("GetPlaylist: urls are in wrong format, error: %v\n", err)
	}
	if complete { // url already downloaded
		return
	}
	playList, status, err := recu.Parse(errCh, statusCh, url, config.Header, jsonLoc, config.parseMaxRes())
	if err != nil {
		switch status {
		case recu.CLOUDFLARE:
			errCh <- fmt.Errorf("%s\nCloudflare Blocked: Failed on url: %v\n", err.Error(), url)
		case recu.COOKIE:
			errCh <- fmt.Errorf("Please Log in: Failed on url: %s\n", url)
		case recu.WAIT:
			errCh <- fmt.Errorf("Daily View Used: Failed on url: %s\n", url)
		case recu.OTHER:
			errCh <- fmt.Errorf("Error: %s\nFailed on url: %s\n", err.Error(), url)
		}
	}
	return
}

// Parse URL object from json
func parseUrl(url any) (urlString string, duration []float64, startIndex int, err error, complete bool) {
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("GetVideo: urls are in wrong format, error: %v", r)
		}
	}()
	switch t := url.(type) {
	case string:
		urlString = t
	case []any:
		switch len(t) {
		case 1:
			urlString = t[0].(string)
		case 2:
			urlString = t[0].(string)
			str, ok := t[1].(string)
			if ok {
				if str == "COMPLETE" {
					complete = true
				}
			} else {
				startIndex = int(t[1].(float64))
			}
		case 4:
			urlString = t[0].(string)
			duration = tools.PercentPrase(t[1:])
		case 5:
			urlString = t[0].(string)
			duration = tools.PercentPrase(t[1:4])
			str, ok := t[4].(string)
			if ok {
				if str == "COMPLETE" {
					complete = true
				}
			} else {
				startIndex = int(t[4].(float64))
			}

		default:
			err = fmt.Errorf("incorrect length of url array")
		}
	default:
		err = fmt.Errorf("url is incorrect type")
	}
	if duration == nil {
		duration = []float64{0, 100}
	}
	return
}

// parse maximum resolution from json to an integer
func (config Config) parseMaxRes() int {
	maxResString := config.Options["Maximum Resolution (Height)"]
	i, err := strconv.Atoi(maxResString)
	if err != nil {
		i = 6969
	}
	return i
}

// Saves video to working directory
func (config *Config) GetVideo(playList playlist.Playlist) error {
	url, duration, startIndex, err, _ := parseUrl(config.Urls[playList.JsonLoc])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	// download and mux playlist
	lastIndex, err := recu.Mux(playList, tools.FormatedHeader(config.Header, "", 0), startIndex, duration)
	println()
	if err == nil {
		modifyUrl(&config.Urls[playList.JsonLoc], "COMPLETE")
		fmt.Printf("Completed: %v:%v\n", playList.Filename, url)
	} else {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintf(os.Stderr, "Download Failed at line: %v\n", lastIndex)
		modifyUrl(&config.Urls[playList.JsonLoc], lastIndex)
	}
	// save state to json
	err2 := config.Save()
	if err2 != nil {
		fmt.Println(err2)
	}
	return err
}

// Modify URL object from json
func modifyUrl(url *any, lastIndex any) {
	switch t := (*url).(type) {
	case string:
		*url = []any{t, lastIndex}
	case []any:
		switch len(t) {
		case 1:
			t = append(t, lastIndex)
			*url = t
		case 2:
			t[1] = lastIndex
			*url = t
		case 4:
			t = append(t, lastIndex)
			*url = t
		case 5:
			t[4] = lastIndex
			*url = t
		}
	}
}

// Returns default templet
func Default() Config {
	var jsonTemplet Config
	jsonTemplet.Header = map[string]string{
		"Cookie":     "",
		"User-Agent": "",
	}
	jsonTemplet.Urls = []any{""}
	jsonTemplet.Options = map[string]string{
		"Maximum Resolution (Height)": "",
	}
	return jsonTemplet
}

// Saves Json
func (config *Config) Save() (err error) {
	mtx.Lock()
	var jsonData []byte
	jsonData, err = json.MarshalIndent(config, "", "\t")
	if err != nil {
		return fmt.Errorf("error: Parsing Json%v", err)
	}
	jsonLocation := "config.json"
	if tools.Argparser(1) != "" {
		jsonLocation = tools.Argparser(1)
	}
	err = os.WriteFile(jsonLocation, jsonData, 0666)
	if err != nil {
		err = fmt.Errorf("error: Saving Json:%v", err)
		return
	}
	mtx.Unlock()
	return
}

func (config *Config) Empty() bool {
	return (len(config.Urls) < 1 || config.Urls[0] == "" || config.Header["Cookie"] == "" || config.Header["User-Agent"] == "")

}

// Parse Urls from HTML
func (config Config) ParseHtml(url string) (err error) {
	fmt.Println("Downloading HTML")
	resp, code, err := tools.Request(url, 10, tools.FormatedHeader(config.Header, "", 1), nil, "GET")
	if code != 200 || err != nil {
		if err == nil {
			err = fmt.Errorf("response: %s, status code: %d, cloudflare blocked", tools.ANSIColor(string(resp), 2), code)
		}
		return
	}
	fmt.Println("Searching for Links")
	urlSplit := strings.Split(url, "/")
	name := urlSplit[4]
	prefix := strings.Join(urlSplit[:3], "/")
	urls := config.Urls
	lines := strings.Split(string(resp), "\n")
	for _, v := range lines {
		code, err := tools.SearchString(v, fmt.Sprintf(`href="/%s/video/`, name), `/play"`)
		if err != nil {
			continue
		}
		urls = append(urls, fmt.Sprintf("%s/%s/video/%s/play", prefix, name, code))
	}
	config.Urls = urls
	err = config.Save()
	return
}
