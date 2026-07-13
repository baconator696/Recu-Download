package state

import (
	"encoding/json"
	"fmt"
	"os"
	"recurbate/tools"
	"strconv"
	"strings"
	"sync"
)

var (
	JsonMutex sync.Mutex
)

type Json struct {
	Urls    []any             `json:"urls"`
	Header  map[string]string `json:"header"`
	Options map[string]string `json:"options"`
}

// Returns default templet
func defaultJson() Json {
	var jsonTemplet Json
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
func (self *Json) SaveJson(wd string, jsonFilename string, disableMutex bool) (err error) {
	if !disableMutex {
		JsonMutex.Lock()
	}
	var jsonData []byte
	jsonData, err = json.MarshalIndent(self, "", "\t")
	if err != nil {
		return fmt.Errorf("error: Parsing Json%v", err)
	}
	err = os.WriteFile(wd+"/"+jsonFilename, jsonData, 0666)
	if err != nil {
		err = fmt.Errorf("error: Saving Json:%v", err)
		return
	}
	if !disableMutex {
		JsonMutex.Unlock()
	}
	return
}

func (self *Json) isEmptyJson() bool {
	return (len(self.Urls) < 1 || self.Urls[0] == "" || self.Header["Cookie"] == "" || self.Header["User-Agent"] == "")

}

// parse maximum resolution from json to an integer
func (self Json) parseMaxRes() int {
	maxResString := self.Options["Maximum Resolution (Height)"]
	i, err := strconv.Atoi(maxResString)
	if err != nil {
		i = 6969
	}
	return i
}

// Parse Urls from HTML    // CLI ONLY
func (self Json) ParseHtml(url string, wd string, jsonFilename string) (err error) {
	fmt.Println("Downloading HTML")
	resp, code, err := tools.Request(url, 10, tools.FormatedHeader(self.Header, "", 1), nil, "GET")
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
	urls := self.Urls
	lines := strings.Split(string(resp), "\n")
	for _, v := range lines {
		code, err := tools.SearchString(v, fmt.Sprintf(`href="/%s/video/`, name), `/play"`)
		if err != nil {
			continue
		}
		urls = append(urls, fmt.Sprintf("%s/%s/video/%s/play", prefix, name, code))
	}
	self.Urls = urls
	err = self.SaveJson(wd, jsonFilename, true)
	return
}
