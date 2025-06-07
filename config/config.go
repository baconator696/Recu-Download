package config

import (
	"encoding/json"
	"fmt"
	"os"
	"recurbate/playlist"
	"recurbate/recu"
	"recurbate/tools"
	"sync"
)

// mutex
var (
	mtx sync.Mutex
)

// Defines the JSON used
type Config struct {
	Urls   []any             `json:"urls"`
	Header map[string]string `json:"header"`
}

// Gets Playlist
func (config Config) GetPlaylist(urlAny any, jsonLoc int) (playList playlist.Playlist) {
	defer func() {
		r := recover()
		if r != nil {
			fmt.Fprintf(os.Stderr, "urls are in wrong format, error: %v\n", r)
		}
	}()
	var url string
	switch t := urlAny.(type) {
	case string:
		url = t
	case []any:
		if len(t) > 0 {
			url = t[0].(string)
		} else {
			panic("no url")
		}
	default:
		panic("url is incorrect type")
	}
	playList, status, err := recu.Parse(url, config.Header, jsonLoc)
	switch status {
	case "cloudflare":
		fmt.Fprintf(os.Stderr, "%s\nCloudflare Blocked: Failed on url: %v\n", err.Error(), url)
	case "cookie":
		fmt.Fprintf(os.Stderr, "Please Log in: Failed on url: %v\n", url)
	case "wait":
		fmt.Fprintf(os.Stderr, "Daily View Used: Failed on url: %v\n", url)
	case "panic":
		fmt.Fprintf(os.Stderr, "Error: %s\nFailed on url: %v\n", err.Error(), url)
	}
	return
}

// Saves video to working directory
func (config *Config) GetVideo(playList playlist.Playlist) (fail int) {
	defer func() {
		r := recover()
		if r != nil {
			fmt.Fprintf(os.Stderr, "urls are in wrong format, error: %v\n", r)
			fail = 1
		}
	}()
	var url string
	var duration []float64 = nil
	var num int = 0
	// parse list of urls in json
	switch t := config.Urls[playList.JsonLoc].(type) {
	case string:
		url = t
	case []any:
		switch len(t) {
		case 1:
			url = t[0].(string)
		case 2:
			url = t[0].(string)
			num = int(t[1].(float64))
		case 4:
			url = t[0].(string)
			duration = tools.PercentPrase(t[1:])
		case 5:
			url = t[0].(string)
			duration = tools.PercentPrase(t[1:4])
			num = int(t[4].(float64))
		default:
			panic("incorrect length of url array")
		}
	default:
		panic("url is incorrect type")
	}
	if duration == nil {
		duration = []float64{0, 100}
	}
	// download and mux playlist
	fail = recu.Mux(playList, tools.FormatedHeader(config.Header, "", 0), num, duration)
	if fail == 0 {
		fmt.Printf("Completed: %v:%v\n", playList.Filename, url)
		return
	}
	// if fail, save state to json
	fmt.Fprintf(os.Stderr, "Download Failed at line: %v\n", fail)
	switch t := config.Urls[playList.JsonLoc].(type) {
	case string:
		config.Urls[playList.JsonLoc] = []any{t, fail}
	case []any:
		switch len(t) {
		case 1:
			t = append(t, fail)
			config.Urls[playList.JsonLoc] = t
		case 2:
			t[1] = fail
			config.Urls[playList.JsonLoc] = t
		case 4:
			t = append(t, fail)
			config.Urls[playList.JsonLoc] = t
		case 5:
			t[4] = fail
			config.Urls[playList.JsonLoc] = t
		}
	}
	err := config.Save()
	if err != nil {
		fmt.Println(err)
	}
	return
}

// Returns default templet
func Default() Config {
	var jsonTemplet Config
	jsonTemplet.Header = map[string]string{
		"Cookie":     "",
		"User-Agent": "",
	}
	jsonTemplet.Urls = []any{""}
	return jsonTemplet
}

// Saves Json
func (config *Config) Save() (err error) {
	mtx.Lock()
	var jsonData []byte
	jsonData, err = json.MarshalIndent(struct {
		Urls   []any             `json:"urls"`
		Header map[string]string `json:"header"`
	}{
		Urls:   config.Urls,
		Header: config.Header,
	}, "", "\t")
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
//func (config Config) ParseHtml(url string) (err error) {
//	fmt.Println("Downloading HTML")
//	resp, code, err := request(url, 10, formatedHeader(config.Header, "", 1), nil, "GET")
//	if code != 200 || err != nil {
//		if err == nil {
//			err = fmt.Errorf("response: %s, status code: %d, cloudflare blocked", ANSIColor(string(resp), 2), code)
//		}
//		return
//	}
//	fmt.Println("Searching for Links")
//	urlSplit := strings.Split(url, "/")
//	name := urlSplit[4]
//	prefix := strings.Join(urlSplit[:3], "/") + fmt.Sprintf("/%s/video/", name)
//	suffix := "/play"
//	var urls []any
//	lines := strings.Split(string(resp), "\n")
//	for _, v := range lines {
//		code, err := searchString(v, fmt.Sprintf(`href="/%s/video/`, name), `/play"`)
//		if err != nil {
//			continue
//		}
//		urls = append(urls, prefix+code+suffix)
//	}
//	config.Urls = urls
//	err = SaveJson(config)
//	return
//}
