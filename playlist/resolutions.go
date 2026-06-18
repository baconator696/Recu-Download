package playlist

import (
	"fmt"
	"github.com/google/btree"
	"os"
	"regexp"
	"strconv"
	"sync"
)

type IntStr struct {
	i int
	s string
}

func (self IntStr) less(other IntStr) bool {
	return self.i < other.i
}
func IntStrNew(i int, s string) IntStr {
	return IntStr{i, s}
}
func IntStrN(i int) IntStr {
	var s string
	return IntStr{i, s}
}
func (self IntStr) Get() string {
	return self.s
}

var (
	regexResolution      *regexp.Regexp
	regexResolutionMutex sync.Mutex
)

// given a raw playist of resolutions, returns sorted map of playlist urls, by frame height
func ParseResolutionPlaylistLinks(playlistsRaw, prefix string) (resolutions *btree.BTreeG[IntStr]) {
	resolutions = btree.NewG(6, IntStr.less)

	regexResolutionMutex.Lock()
	if regexResolution == nil {
		regexResolution = regexp.MustCompile(`RESOLUTION=\d+x(\d+).+\n(.+)`)
	}
	regexResolutionMutex.Unlock()
	matches := regexResolution.FindAllStringSubmatch(playlistsRaw, -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		resH, err := strconv.Atoi(match[1])
		link := prefix + match[2]
		if err != nil {
			fmt.Fprintf(os.Stderr, "Atoi error in parseResolutionPlaylistLinks(): %v", err)
			continue
		}
		resolutions.ReplaceOrInsert(IntStrNew(resH, link))
	}
	return
}
