package config

import (
	"fmt"
	"os"
	"recurbate/playlist"
	"recurbate/recu"
	"sort"
	"sync"
	"time"
)

func (self Config) HybridService(errCh chan error, statusCh chan recu.Status) {
	// get playlists
	playlists := make([]playlist.Playlist, len(self.Urls))
	for i, link := range self.Urls {
		playlists[i] = self.GetPlaylist(errCh, statusCh, link, i)
	}
	serversMap := make(map[string]playlist.PlaylistSlice)
	// organize playlist by server
	for _, playList := range playlists {
		domainName, err := playList.PlaylistOrigin()
		if err != nil {
			errCh <- err
			continue
		}
		if serversMap[domainName] == nil {
			serversMap[domainName] = make(playlist.PlaylistSlice, 0)
		}
		serversMap[domainName] = append(serversMap[domainName], playList)
	}
	// makes shortest playlists go first
	for _, playlists := range serversMap {
		sort.Sort(playlists)
	}
	var wg sync.WaitGroup
	for _, playlists := range serversMap {
		wg.Add(1)
		go func(playlists []playlist.Playlist) {
			defer wg.Done()
			for _, playList := range playlists {
				if playList.IsNil() {
					continue
				}
				if self.GetVideo(playList) == nil {
					continue
				}
				err := os.WriteFile(playList.Filename+".m3u8", playList.M3u8, 0666)
				if err != nil {
					fmt.Println(string(playList.M3u8))
					errCh <- fmt.Errorf("Failed to write playlist data: %v\n", err)
				}
			}
		}(playlists)
		time.Sleep(time.Second)
	}
	wg.Wait()
}

func (self Config) SerialService(errCh chan error, statusCh chan recu.Status) {
	playlists := make(playlist.PlaylistSlice, len(self.Urls))
	for i, link := range self.Urls {
		playlists[i] = self.GetPlaylist(errCh, statusCh, link, i)
	}
	sort.Sort(playlists)
	for i, playList := range playlists {
		if playList.IsNil() {
			continue
		}
		fmt.Printf("%d/%d:\n", i+1, len(playlists))
		if self.GetVideo(playList) == nil {
			continue
		}
		err := os.WriteFile(playList.Filename+".m3u8", playList.M3u8, 0666)
		if err != nil {
			fmt.Println(string(playList.M3u8))
			errCh <- fmt.Errorf("Failed to write playlist data: %v\n", err)
		}
	}
}

func (self Config) DownloadPlaylist(errCh chan error, statusCh chan recu.Status) {
	for i, v := range self.Urls {
		playList := self.GetPlaylist(errCh, statusCh, v, i)
		if playList.IsNil() {
			continue
		}
		err := os.WriteFile(playList.Filename+".m3u8", playList.M3u8, 0666)
		if err != nil {
			fmt.Println(string(playList.M3u8))
			errCh <- fmt.Errorf("Failed to write playlist data: %v\n", err)
			continue
		}
		fmt.Printf("Completed: %v:%v\n", playList.Filename, v)
	}
}
