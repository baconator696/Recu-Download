package config

import (
	"fmt"
	"os"
	"recurbate/config/typ"
	"sort"
	"sync"
	"time"
)

func HybridService(conf *typ.Config) {
	// get playlists
	for _, video := range conf.Videos {
		GetPlaylist(&video)
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
	for originServer, _ := range serversSet {
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
				// SAVE STATE TO JSON
				if err == nil {
					continue
				}
				err = os.WriteFile(conf.Wd+video.Playlist.Filename+".m3u8", video.Playlist.M3u8, 0666)
				if err != nil {
					conf.MsgCh <- string(video.Playlist.M3u8) + "\n"
					conf.ErrCh <- fmt.Errorf("Failed to write playlist data: %v\n", err)
				}
			}
		}(originServer)
		time.Sleep(time.Second)
	}
	wg.Wait()
}

//func (jsonConf Json) SerialService(errCh chan error, statusCh chan recu.Status) {
//	playlists := make(playlist.PlaylistSlice, len(jsonConf.Urls))
//	for i, link := range jsonConf.Urls {
//		playlists[i] = jsonConf.GetPlaylist(errCh, statusCh, link, i)
//	}
//	sort.Sort(playlists)
//	for i, playList := range playlists {
//		if playList.IsNil() {
//			continue
//		}
//		fmt.Printf("%d/%d:\n", i+1, len(playlists))
//		if jsonConf.GetVideo(playList) == nil {
//			continue
//		}
//		err := os.WriteFile(playList.Filename+".m3u8", playList.M3u8, 0666)
//		if err != nil {
//			fmt.Println(string(playList.M3u8))
//			errCh <- fmt.Errorf("Failed to write playlist data: %v\n", err)
//		}
//	}
//}
//
//func (jsonConf Json) DownloadPlaylist(errCh chan error, statusCh chan recu.Status) {
//	for i, v := range jsonConf.Urls {
//		playList := jsonConf.GetPlaylist(errCh, statusCh, v, i)
//		if playList.IsNil() {
//			continue
//		}
//		err := os.WriteFile(playList.Filename+".m3u8", playList.M3u8, 0666)
//		if err != nil {
//			fmt.Println(string(playList.M3u8))
//			errCh <- fmt.Errorf("Failed to write playlist data: %v\n", err)
//			continue
//		}
//		fmt.Printf("Completed: %v:%v\n", playList.Filename, v)
//	}
//}
