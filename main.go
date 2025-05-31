package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"recurbate/config"
	"recurbate/playlist"
	"recurbate/tools"
	"strings"
	"sync"
	"syscall"
	"time"
)

var tag string

func parallelService(cfg config.Config) {
	playlists := make([]playlist.Playlist, len(cfg.Urls))
	for i, link := range cfg.Urls {
		playlists[i] = cfg.GetPlaylist(link)
	}
	var wg sync.WaitGroup
	for i, playList := range playlists {
		if playList.IsNil() {
			continue
		}
		wg.Add(1)
		go func(playList playlist.Playlist, i int) {
			defer wg.Done()
			if cfg.GetVideo(playList, i) == 0 {
				return
			}
			err := os.WriteFile(playList.Filename+".m3u8", playList.M3u8, 0666)
			if err != nil {
				fmt.Println(playList.M3u8)
				fmt.Fprintf(os.Stderr, "Failed to write playlist data: %v\n", err)
			}
		}(playList, i)
		time.Sleep(time.Second)
	}
	wg.Wait()
}
func serialService(cfg config.Config) {
	playlists := make([]playlist.Playlist, len(cfg.Urls))
	for i, link := range cfg.Urls {
		playlists[i] = cfg.GetPlaylist(link)
	}
	for i, playList := range playlists {
		if playList.IsNil() {
			continue
		}
		fmt.Printf("%d/%d:\n", i+1, len(playlists))
		if cfg.GetVideo(playList, i) == 0 {
			continue
		}
		err := os.WriteFile(playList.Filename+".m3u8", playList.M3u8, 0666)
		if err != nil {
			fmt.Println(playList.M3u8)
			fmt.Fprintf(os.Stderr, "Failed to write playlist data: %v\n", err)
		}
	}
}
func downloadPlaylist(cfg config.Config) {
	for _, v := range cfg.Urls {
		playList := cfg.GetPlaylist(v)
		if playList.IsNil() {
			continue
		}
		err := os.WriteFile(playList.Filename+".m3u8", playList.M3u8, 0666)
		if err != nil {
			fmt.Println(playList.M3u8)
			fmt.Fprintf(os.Stderr, "Failed to write playlist data: %v\n", err)
			continue
		}
		fmt.Printf("Completed: %v:%v\n", playList.Filename, v)
	}
}
func downloadConent(cfg config.Config) {
	playlistPath := tools.Argparser(3)
	data, err := os.ReadFile(playlistPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read playlist: %v\n", err)
		return
	}
	filename := playlistPath
	if strings.Contains(filename, string(os.PathSeparator)) {
		tempSplit := strings.Split(filename, string(os.PathSeparator))
		filename = tempSplit[len(tempSplit)-1]
	}
	filename = strings.ReplaceAll(filename, ".m3u8", "")
	playList := playlist.NewFromUsername(data, filename)
	cfg.GetVideo(playList, 0)
}
func readme() string {
	path := tools.Argparser(0)
	if strings.Contains(path, string(os.PathSeparator)) {
		split := strings.Split(path, string(os.PathSeparator))
		path = split[len(split)-1]
	}
	string1 := `Recurbate:
If ran for the first time, json configuration will be generated
	in the working directory
Fill in the json's URL, Cookie and User-Agent to allow the
	program to run

Usage: `
	string2 := ` <json location> playlist|series <playlist.m3u8>

if "playlist" is used, only the .m3u8 playlist file will be
	downloaded, specifiying the playlist location will
	download the contents of the playlist
if "series" is used, the program will download both the videos
	in series`
	return string1 + path + string2
}
func init() {
	go func() {
		inter := make(chan os.Signal, 1)
		signal.Notify(inter, os.Interrupt, syscall.SIGTERM)
		<-inter
		tools.Abort = true
		force := make(chan os.Signal, 1)
		signal.Notify(force, os.Interrupt, syscall.SIGTERM)
		<-force
		os.Exit(0)
	}()
}
func main() {
	fmt.Printf("Recu %v\n", tag)
	tools.CheckUpdate(tag)
	if tools.Argparser(1) == "--help" {
		fmt.Println(readme())
		return
	}
	json_location := "config.json"
	if tools.Argparser(1) != "" {
		json_location = tools.Argparser(1)
	}
	_, err := os.Stat(json_location)
	if err != nil {
		defaultConfig := config.Default()
		defaultConfig.Save()
		fmt.Printf("%v created in working directory\nPlease fill in the %v with the \n\tURLs to Download\n\tCookies\n\tUser-Agent\n", json_location, json_location)
		return
	}
	jsonData, err := os.ReadFile(json_location)
	if err != nil {
		fmt.Println(err)
		os.Exit(4)
	}
	var cfg config.Config
	err = json.Unmarshal(jsonData, &cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Reading Json: %v", err)
		os.Exit(4)
	}
	if cfg.Empty() {
		fmt.Println("please modify config.json")
		return
	}
	switch tools.Argparser(2) {
	case "playlist":
		if tools.Argparser(3) != "" {
			_, err := os.Stat(tools.Argparser(3))
			if err != nil {
				fmt.Println(err)
				os.Exit(4)
			}
			downloadConent(cfg)
		} else {
			downloadPlaylist(cfg)
		}
	case "series":
		serialService(cfg)
	//case "parse":
	//	err := config.ParseHtml(tools.Argparser(3))
	//	if err != nil {
	//		fmt.Println(err)
	//	} else {
	//		fmt.Println("Parsed HTML Successfully")
	//	}
	default:
		parallelService(cfg)
	}
}
