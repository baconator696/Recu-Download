package state

import (
	"encoding/json"
	"fmt"
	"os"
	"recurbate/playlist"
	"recurbate/tools"
)

type Config struct {
	Wd           string
	JsonFilename string
	Json_ref     Json
	Videos       VideoSlice
	maxRes       int
	ErrCh        chan error
	MsgCh        chan string
}
type Video struct {
	Url            string
	Header         map[string]string
	Section        [2]float32
	section_ref    []any
	Offset         int
	importedOffset bool
	State          state
	Playlist       playlist.Playlist
	MaxRes         int
}
type state struct {
	Stage           stage
	DownloadSpeed   float32
	Eta             float32
	ProgressPercent float32
	complete        bool
	fail            bool
}
type stage int

const (
	HTML stage = iota
	PLAYLISTURL
	PLAYLISTS
	PLAYLIST
	DOWNLOAD
	COMPLETE
)

func New(workingDir string, jsonFilename string) (conf Config, err error) {
	var jsonConfig Json
	_, err = os.Stat(jsonFilename)
	if err != nil {
		jsonConfig = defaultJson()
		err = jsonConfig.SaveJson(workingDir, jsonFilename, false)
		if err != nil {
			return
		}
	} else {
		jsonData, err := os.ReadFile(jsonFilename)
		if err != nil {
			err = fmt.Errorf("error reading json file %s: %v", jsonFilename, err)
			return conf, err
		}
		err = json.Unmarshal(jsonData, &jsonConfig)
		if err != nil {
			err = fmt.Errorf("error: parsing json: %v", err)
			return conf, err
		}
	}
	if jsonConfig.isEmptyJson() {
		err = fmt.Errorf("Please fill in the %v with the \n\tURLs to Download\n\tCookies\n\tUser-Agent\n", jsonFilename)
		return
	}
	conf.JsonFilename = jsonFilename
	conf.Json_ref = jsonConfig
	conf.Wd = workingDir
	conf.maxRes = jsonConfig.parseMaxRes()
	conf.ErrCh = make(chan error)
	conf.MsgCh = make(chan string)
	for _, urlObject := range jsonConfig.Urls {
		var v Video
		v.Url, v.Section, v.section_ref, v.Offset, err, v.State.Stage = parseUrl(urlObject)
		if err != nil {
			return
		}
		if v.Offset != 0 {
			v.importedOffset = true
		}
		v.Header = jsonConfig.Header
		conf.Videos = append(conf.Videos, v)
		v.MaxRes = conf.maxRes
	}
	return
}

// create json object from config object
func (self *Config) CreateJson(disableMutex bool) (js Json) {
	if !disableMutex {
		JsonMutex.Lock()
	}
	js.Header = self.Json_ref.Header
	js.Options = self.Json_ref.Options
	urls := make([]any, 0)
	for _, video := range self.Videos {
		if video.section_ref == nil {
			if !video.State.complete && !video.State.fail {
				urls = append(urls, video.Url)
			} else if video.State.complete {
				urls = append(urls, []any{urls, video.Url, "COMPLETE"})
			} else {
				urls = append(urls, []any{urls, video.Url, video.Offset})
			}
		} else {
			if !video.State.complete && !video.State.fail {
				urls = append(urls, []any{urls, video.Url, video.section_ref[0], video.section_ref[1], video.section_ref[2]})
			} else if video.State.complete {
				urls = append(urls, []any{urls, video.Url, video.section_ref[0], video.section_ref[1], video.section_ref[2], "COMPLETE"})
			} else {
				urls = append(urls, []any{urls, video.Url, video.section_ref[0], video.section_ref[1], video.section_ref[2], video.Offset})
			}
		}
	}
	if !disableMutex {
		JsonMutex.Unlock()
	}

	return
}

// Parse URL object from json
func parseUrl(urlObject any) (urlString string, section [2]float32, section_ref []any, offset int, err error, stage stage) {
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("GetVideo: urls are in wrong format, error: %v", r)
		}
	}()
	switch urlObjectT := urlObject.(type) {
	case string:
		urlString = urlObjectT
	case []any:
		switch len(urlObjectT) {
		case 1:
			urlString = urlObjectT[0].(string)
		case 2:
			urlString = urlObjectT[0].(string)
			str, ok := urlObjectT[1].(string)
			if ok {
				if str == "COMPLETE" {
					stage = COMPLETE
				}
			} else {
				offset = int(urlObjectT[1].(float64))
			}
		case 4:
			urlString = urlObjectT[0].(string)
			section_ref = urlObjectT[1:]
			section = tools.PercentPrase(section_ref)
		case 5:
			urlString = urlObjectT[0].(string)
			section_ref = urlObjectT[1:4]
			section = tools.PercentPrase(section_ref)
			str, ok := urlObjectT[4].(string)
			if ok {
				if str == "COMPLETE" {
					stage = COMPLETE
				}
			} else {
				offset = int(urlObjectT[4].(float32))
			}

		default:
			err = fmt.Errorf("incorrect length of url array")
		}
	default:
		err = fmt.Errorf("url is incorrect type")
	}
	return
}

type VideoSlice []Video

func (p VideoSlice) Len() int {
	return len(p)
}
func (p VideoSlice) Less(prior, latter int) bool {
	return len(p[prior].Playlist.List) < len(p[latter].Playlist.List)
}
func (p VideoSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
