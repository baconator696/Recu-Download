package resolution

import (
	"fmt"
	"regexp"
	"strconv"
	"sync"

	"github.com/google/btree"
)

type kv struct {
	i int
	s string
}

func (self kv) less(other kv) bool {
	return self.i < other.i
}

var (
	regexResolution      *regexp.Regexp
	regexResolutionMutex sync.Mutex
)

type playlists struct {
	tree *btree.BTreeG[kv]
}

// given a raw playist of resolutions, packages playlists into usable functions
func New(playlistsRaw, prefix string) (p playlists, err error) {
	resolutions := btree.NewG(6, kv.less)
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
			return p, fmt.Errorf("Atoi error in recurbate/playlist/resolution.New(): %v", err)
		}
		resolutions.ReplaceOrInsert(kv{resH, link})
	}
	p.tree = resolutions
	return
}
func (self playlists) Max(maxRes int) (playlistUrl string) {
	self.tree.DescendLessOrEqual(kv{maxRes, ""}, func(item kv) bool {
		playlistUrl = item.s
		return false
	})
	if playlistUrl == "" {
		min, _ := self.tree.Min()
		playlistUrl = min.s
	}
	return
}
