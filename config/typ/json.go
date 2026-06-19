package typ

import (
	"encoding/json"
	"fmt"
	"os"
	"recurbate/tools"
	"strconv"
	"sync"
)

var (
	jsonMutex sync.Mutex
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
func (jsonConf *Json) SaveJson(jsonLoc string) (err error) {
	jsonMutex.Lock()
	var jsonData []byte
	jsonData, err = json.MarshalIndent(jsonConf, "", "\t")
	if err != nil {
		return fmt.Errorf("error: Parsing Json%v", err)
	}
	if tools.Argparser(1) != "" {
		jsonLoc = tools.Argparser(1)
	}
	err = os.WriteFile(jsonLoc, jsonData, 0666)
	if err != nil {
		err = fmt.Errorf("error: Saving Json:%v", err)
		return
	}
	jsonMutex.Unlock()
	return
}

func (jsonConf *Json) isEmptyJson() bool {
	return (len(jsonConf.Urls) < 1 || jsonConf.Urls[0] == "" || jsonConf.Header["Cookie"] == "" || jsonConf.Header["User-Agent"] == "")

}

// parse maximum resolution from json to an integer
func (jsonConf Json) parseMaxRes() int {
	maxResString := jsonConf.Options["Maximum Resolution (Height)"]
	i, err := strconv.Atoi(maxResString)
	if err != nil {
		i = 6969
	}
	return i
}
