package typ

import (
	"encoding/json"
	"fmt"
	"os"
	"recurbate/playlist"
	"recurbate/tools"
)

type Config struct {
	Wd      string
	jsonLoc string
	Videos  VideoSlice
	maxRes  int
	ErrCh   chan error
	MsgCh   chan string
}
type Video struct {
	Url      string
	Header   map[string]string
	Section  []float32
	Offset   int
	importedOffset bool
	State    state
	Playlist playlist.Playlist
	MaxRes   int
	ErrCh    chan error
	MsgCh    chan string
}
type state struct {
	Stage           stage
	DownloadSpeed   float32
	Eta             float32
	ProgressPercent float32
	complete bool
	fail bool
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

func New(workingDir string, jsonLoc string) (conf Config, err error) {
	var jsonConfig Json
	_, err = os.Stat(jsonLoc)
	if err != nil {
		jsonConfig = defaultJson()
		err = jsonConfig.SaveJson(jsonLoc)
		if err != nil {
			return
		}
	} else {
		jsonData, err := os.ReadFile(jsonLoc)
		if err != nil {
			err = fmt.Errorf("error reading json file %s: %v", jsonLoc, err)
			return conf, err
		}
		err = json.Unmarshal(jsonData, &jsonConfig)
		if err != nil {
			err = fmt.Errorf("error: parsing json: %v", err)
			return conf, err
		}
	}
	if jsonConfig.isEmptyJson() {
		err = fmt.Errorf("Please fill in the %v with the \n\tURLs to Download\n\tCookies\n\tUser-Agent\n", jsonLoc)
		return
	}
	conf.jsonLoc = jsonLoc
	conf.Wd = workingDir
	conf.maxRes = jsonConfig.parseMaxRes()
	conf.ErrCh = make(chan error)
	conf.MsgCh = make(chan string)
	for _, urlObject := range jsonConfig.Urls {
		var v Video
		v.Url, v.Section, v.Offset, err, v.State.Stage = parseUrl(urlObject)
		if err != nil {
			return
		}
		if v.Offset != 0 {
			v.importedOffset = true
		}
		v.Header = jsonConfig.Header
		conf.Videos = append(conf.Videos, v)
		v.MaxRes = conf.maxRes
		v.ErrCh = conf.ErrCh
		v.MsgCh = conf.MsgCh
	}
	return
}

// Parse URL object from json
func parseUrl(urlObject any) (urlString string, duration []float32, startIndex int, err error, stage stage) {
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("GetVideo: urls are in wrong format, error: %v", r)
		}
	}()
	switch t := urlObject.(type) {
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
					stage = COMPLETE
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
					stage = COMPLETE
				}
			} else {
				startIndex = int(t[4].(float32))
			}

		default:
			err = fmt.Errorf("incorrect length of url array")
		}
	default:
		err = fmt.Errorf("url is incorrect type")
	}
	if duration == nil {
		duration = []float32{0, 100}
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
