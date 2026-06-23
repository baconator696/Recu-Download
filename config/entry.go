package config

import (
	"fmt"
	"os"
	"recurbate/config/state"
	"sort"
	"sync"
	"time"
)

func HybridService(conf *state.Config) {
	// get playlists
	for _, video := range conf.Videos {
		GetPlaylist(&video, conf)
	}
	// makes shortest playlists go first
	sort.Sort(conf.Videos)
	serversSet := make(map[string]bool)
	// NEED to organize playlist by server
	for _, video := range conf.Videos {
		domainName, err := video.Playlist.PlaylistOrigin()
		if err != nil {
			conf.ErrCh <- err
			continue
		}
		serversSet[domainName] = true
	}
	var wg sync.WaitGroup
	for originServer := range serversSet {
		wg.Add(1)
		go func(originServer string) {
			defer wg.Done()
			for _, video := range conf.Videos {
				compare, err := video.Playlist.PlaylistOrigin()
				if err != nil {
					conf.ErrCh <- fmt.Errorf("compare, err := video.Playlist.PlaylistOrigin(): %v\n", err)
					continue
				}
				if originServer != compare {
					continue
				}
				if video.Playlist.IsNil() {
					continue
				}
				err = GetVideo(&video, conf)
				state.JsonMutex.Lock()
				js := conf.CreateJson(true)
				err2 := js.SaveJson(conf.Wd, conf.JsonFilename, true)
				if err2 != nil {
					conf.ErrCh <- err2
				}
				state.JsonMutex.Unlock()
				if err == nil {
					continue
				}
				conf.MsgCh <- string(video.Playlist.M3u8) + "\n"
				conf.ErrCh <- fmt.Errorf("Failed to write playlist data: %v\n", err)
			}
		}(originServer)
		time.Sleep(time.Second)
	}
	wg.Wait()
}

func SerialService(conf *state.Config) {
	for _, video := range conf.Videos {
		GetPlaylist(&video, conf)
	}
	sort.Sort(conf.Videos)
	for _, video := range conf.Videos {
		if video.Playlist.IsNil() {
			continue
		}
		err := GetVideo(&video, conf)
		js := conf.CreateJson(true)
		err2 := js.SaveJson(conf.Wd, conf.JsonFilename, true)
		if err2 != nil {
			conf.ErrCh <- err2
		}
		if err == nil {
			continue
		}
		conf.MsgCh <- string(video.Playlist.M3u8) + "\n"
		conf.ErrCh <- fmt.Errorf("Failed to write playlist data: %v\n", err)
	}
}

func DownloadPlaylist(conf *state.Config) {
	for _, video := range conf.Videos {
		GetPlaylist(&video, conf)
		if video.Playlist.IsNil() {
			continue
		}
		err := os.WriteFile(conf.Wd+video.Playlist.Filename+".m3u8", video.Playlist.M3u8, 0666)
		if err != nil {
			conf.MsgCh <- string(video.Playlist.M3u8) + "\n"
			conf.ErrCh <- fmt.Errorf("Failed to write playlist data: %v\n", err)
			continue
		}
		conf.MsgCh <- fmt.Sprintf("Completed: %v:%v\n", video.Playlist.Filename, video.Url)
	}
}
