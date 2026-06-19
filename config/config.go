package config

import (
	"fmt"
	"recurbate/config/typ"
	"recurbate/recu"
)

// Gets Playlist
func GetPlaylist(conf *typ.Video) {
	if conf.State.Stage == typ.COMPLETE	 { // url already downloaded
		return
	}
	errT, err := recu.Parse(conf)
	if err != nil {
		switch errT {
		case recu.CLOUDFLARE:
			conf.ErrCh <- fmt.Errorf("%s\nCloudflare Blocked: Failed on url: %v\n", err.Error(), conf.Url)
		case recu.COOKIE:
			conf.ErrCh <- fmt.Errorf("Please Log in: Failed on url: %s\n", conf.Url)
		case recu.WAIT:
			conf.ErrCh <- fmt.Errorf("Daily View Used: Failed on url: %s\n", conf.Url)
		case recu.OTHER:
			conf.ErrCh <- fmt.Errorf("Error: %s\nFailed on url: %s\n", err.Error(), conf.Url)
		}
	}
	return
}

// Saves video to working directory
func GetVideo(video *typ.Video, conf *typ.Config) error {
	// download and mux playlist
	err := recu.Mux(video, conf)
	if err == nil {
		conf.MsgCh <- fmt.Sprintf("\nCompleted: %v:%v\n", video.Playlist.Filename, video.Url)
	} else {
		conf.ErrCh <- err
		conf.ErrCh <- fmt.Errorf("Download Failed at line: %v\n", video.Offset)
	}
	return err
}


// Parse Urls from HTML
//func (jsonConf Json) ParseHtml(url string) (err error) {
//	fmt.Println("Downloading HTML")
//	resp, code, err := tools.Request(url, 10, tools.FormatedHeader(jsonConf.Header, "", 1), nil, "GET")
//	if code != 200 || err != nil {
//		if err == nil {
//			err = fmt.Errorf("response: %s, status code: %d, cloudflare blocked", tools.ANSIColor(string(resp), 2), code)
//		}
//		return
//	}
//	fmt.Println("Searching for Links")
//	urlSplit := strings.Split(url, "/")
//	name := urlSplit[4]
//	prefix := strings.Join(urlSplit[:3], "/")
//	urls := jsonConf.Urls
//	lines := strings.Split(string(resp), "\n")
//	for _, v := range lines {
//		code, err := tools.SearchString(v, fmt.Sprintf(`href="/%s/video/`, name), `/play"`)
//		if err != nil {
//			continue
//		}
//		urls = append(urls, fmt.Sprintf("%s/%s/video/%s/play", prefix, name, code))
//	}
//	jsonConf.Urls = urls
//	err = jsonConf.SaveJson()
//	return
//}
