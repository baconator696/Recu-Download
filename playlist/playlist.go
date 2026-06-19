package playlist

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

type Playlist struct {
	JsonLoc  int
	M3u8     []byte
	List     []string
	Filename string
}
type PlaylistSlice []Playlist

func (p PlaylistSlice) Len() int {
	return len(p)
}
func (p PlaylistSlice) Less(prior, latter int) bool {
	return len(p[prior].List) < len(p[latter].List)
}
func (p PlaylistSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func New(errCh chan error, raw_m3u8 []byte, url string, jsonLoc int) (playList Playlist, err error) {
	filename, err := createFilename(url)
	if err != nil {
		return
	}
	playlistLines := strings.Split(string(raw_m3u8), "\n")
	list := make([]string, 0, len(playlistLines)/2)
	for _, line := range playlistLines {
		if len(line) < 2 || line[0] == '#' {
			continue
		}
		appendedCheck, err := appendCheck(line)
		if err != nil {
			errCh <- fmt.Errorf("Issue with the TS Fragmanet check parameter creator:%v", err)
			list = append(list, line)
		} else {
			list = append(list, appendedCheck)
		}

	}
	if len(list) >= 2 {
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
		loc := strings.Index(p.List[0][last:], "/") + 1
		if loc == 0 {
			err = fmt.Errorf("playlist doesn't contain urls")
			return
		}
		last += loc
		if x == 1 {
			second = last
		}
	}
	domain = p.List[0][second : last-1]
	return
}

// creates the filename from a given m3u8 url
func createFilename(url string) (filename string, err error) {
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

// regex global vars
var (
	RegexUID              *regexp.Regexp
	RegexExpires          *regexp.Regexp
	RegexRequest          *regexp.Regexp
	RexegtsFragCheckMutex sync.Mutex
)

// determine check parameter for playlist fragment link
func appendCheck(url string) (appended string, err error) {
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("appendCheck Panic:\nurl:%s\n%v", appended, r)
		}
	}()
	RexegtsFragCheckMutex.Lock()
	if RegexUID == nil {
		RegexUID = regexp.MustCompile("uid=([^&]*)")
	}
	if RegexExpires == nil {
		RegexExpires = regexp.MustCompile("expires=([^&]*)")
	}
	if RegexRequest == nil {
		RegexRequest = regexp.MustCompile("request_id=([^&]*)")
	}
	RexegtsFragCheckMutex.Unlock()
	uidMatches := RegexUID.FindStringSubmatch(url)
	if len(uidMatches) < 2 {
		return url, fmt.Errorf("uid not found")
	}
	uidMatch := uidMatches[1]
	expiresMatches := RegexExpires.FindStringSubmatch(url)
	if len(expiresMatches) < 2 {
		return url, fmt.Errorf("uid not found")
	}
	expiresMatch := expiresMatches[1]
	requestMatches := RegexRequest.FindStringSubmatch(url)
	if len(requestMatches) < 2 {
		return url, fmt.Errorf("uid not found")
	}
	requestMatch := requestMatches[1]
	expiredSeg := reverseString(reverseString(expiresMatch)[0:4])
	appended = fmt.Sprintf("%s&check=%s%s%s", url, requestMatch[0:4], uidMatch[2:6], expiredSeg)
	return
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
