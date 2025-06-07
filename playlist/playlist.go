package playlist

import (
	"fmt"
	"strings"
)

type Playlist struct {
	JsonLoc  int
	M3u8     []byte
	List     []string
	Filename string
}

func New(raw_m3u8 []byte, url string, jsonLoc int) (playList Playlist, err error) {
	filename, err := parsePlaylistUrl(url)
	if err != nil {
		return playList, err
	}
	playList = NewFromFilename(raw_m3u8, filename, jsonLoc)
	return
}
func NewFromFilename(raw_m3u8 []byte, filename string, jsonLoc int) (playList Playlist) {
	playlistLines := strings.Split(string(raw_m3u8), "\n")
	list := make([]string, 0, len(playlistLines)/2)
	for _, line := range playlistLines {
		if len(line) < 2 || line[0] == '#' {
			continue
		}
		list = append(list, line)
	}
	if len(list) > 0 {
		list = list[1 : len(list)-1]
	}
	playList = Playlist{
		JsonLoc:  jsonLoc,
		M3u8:     raw_m3u8,
		List:     list,
		Filename: filename,
	}
	return
}
func (p *Playlist) Len() int {
	return len(p.List)
}
func (p *Playlist) IsNil() bool {
	return p.M3u8 == nil
}

// returns playlists domain name
func (p *Playlist) PlaylistOrigin() (domain string, err error) {
	if len(p.List) == 0 {
		err = fmt.Errorf("playlist contains no data")
		return
	}
	var second int
	last := 0
	for x := range [3]int{} {
		temp := strings.Index(p.List[0][last:], "/") + 1
		if temp == 0 {
			panic("playlist doesn't contain urls")
		}
		last += temp
		if x == 1 {
			second = last
		}
	}
	domain = p.List[0][second : last-1]
	return
}

// creates the filename from a given m3u8 url
func parsePlaylistUrl(url string) (filename string, err error) {
	urlSplit := strings.Split(url, "/")
	if len(urlSplit) < 6 {
		return "", fmt.Errorf("wrong url format")
	}
	// parse username and date
	username := urlSplit[4]
	date := strings.ReplaceAll(urlSplit[5], ",", "-")
	dateSplit := strings.Split(date, "-")
	if len(dateSplit) < 5 {
		return "", fmt.Errorf("wrong date format")
	}
	if len(dateSplit[0]) == 4 {
		dateSplit[0] = dateSplit[0][2:]
	}
	filename = fmt.Sprintf("CB_%s_%s-%s-%s_%s-%s", username, dateSplit[0], dateSplit[1], dateSplit[2], dateSplit[3], dateSplit[4])
	return
}
