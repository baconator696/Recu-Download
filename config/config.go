package config

import (
	"fmt"
	"recurbate/config/state"
	"recurbate/recu"
	"time"
)

// Gets Playlist
func GetPlaylist(video *state.Video, conf *state.Config) {
	if video.State.Complete { // url already downloaded
		return
	}
	errT, err := recu.Parse(video, conf)
	if err != nil {
		switch errT {
		case recu.CLOUDFLARE:
			conf.ErrCh <- fmt.Errorf("%s\nCloudflare Blocked: Failed on url: %v", err.Error(), video.Url)
		case recu.COOKIE:
			conf.ErrCh <- fmt.Errorf("Please Log in: Failed on url: %s", video.Url)
		case recu.WAIT:
			conf.ErrCh <- fmt.Errorf("Daily View Used: Failed on url: %s", video.Url)
		case recu.OTHER:
			conf.ErrCh <- fmt.Errorf("Error: %s\nFailed on url: %s", err.Error(), video.Url)
		}
	}
}

// Saves video to working directory
func GetVideo(video *state.Video, conf *state.Config) {
	// download and mux playlist
	err := recu.Mux(video, conf)
	if err == nil {
		conf.MsgCh <- fmt.Sprintf("\nCompleted: %v:%v\n", video.Playlist.Filename, video.Url)
	} else {
		time.Sleep(time.Second)
		conf.ErrCh <- fmt.Errorf("\n%v",err)
		conf.ErrCh <- fmt.Errorf("Download Failed at line: %v", video.Offset)
		video.State.Fail = true
	}
}
