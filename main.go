package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"recurbate/tools"
	"strings"
	"sync"
	"syscall"
	"time"
)

var tag string

func parallelService(config tools.Templet) {
	playlists := make([][]byte, len(config.Urls))
	filenames := make([]string, len(config.Urls))
	for i, link := range config.Urls {
		playlists[i], filenames[i] = config.GetPlaylist(link)
	}
	var wg sync.WaitGroup
	for i, data := range playlists {
		if data == nil {
			continue
		}
		wg.Add(1)
		go func(data []byte, i int) {
			defer wg.Done()
			if config.GetVideo(data, filenames[i], i) == 0 {
				return
			}
			err := os.WriteFile(filenames[i]+".m3u8", data, 0666)
			if err != nil {
				fmt.Println(data)
				fmt.Fprintf(os.Stderr, "Failed to write playlist data: %v\n", err)
			}
		}(data, i)
		time.Sleep(time.Second)
	}
	wg.Wait()
}
func serialService(config tools.Templet) {
	playlists := make([][]byte, len(config.Urls))
	filenames := make([]string, len(config.Urls))
	for i, link := range config.Urls {
		playlists[i], filenames[i] = config.GetPlaylist(link)
	}
	for i, data := range playlists {
		if data == nil {
			continue
		}
		fmt.Printf("%d/%d:\n", i+1, len(playlists))
		if config.GetVideo(data, filenames[i], i) == 0 {
			continue
		}
		err := os.WriteFile(filenames[i]+".m3u8", data, 0666)
		if err != nil {
			fmt.Println(data)
			fmt.Fprintf(os.Stderr, "Failed to write playlist data: %v\n", err)
		}
	}
}
func downloadPlaylist(config tools.Templet) {
	for _, v := range config.Urls {
		data, filename := config.GetPlaylist(v)
		if data == nil {
			continue
		}
		err := os.WriteFile(filename+".m3u8", data, 0666)
		if err != nil {
			fmt.Println(data)
			fmt.Fprintf(os.Stderr, "Failed to write playlist data: %v\n", err)
			continue
		}
		fmt.Printf("Completed: %v:%v\n", filename, v)
	}
}
func downloadConent(config tools.Templet) {
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
	config.GetVideo(data, filename, 0)
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
		defaultConfig := tools.TempletJSON()
		tools.SaveJson(defaultConfig)
		fmt.Printf("%v created in working directory\nPlease fill in the %v with the \n\tURLs to Download\n\tCookies\n\tUser-Agent\n", json_location, json_location)
		return
	}
	jsonData, err := os.ReadFile(json_location)
	if err != nil {
		fmt.Println(err)
		os.Exit(4)
	}
	var config tools.Templet
	err = json.Unmarshal(jsonData, &config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Reading Json: %v", err)
		os.Exit(4)
	}
	if len(config.Urls) < 1 || config.Urls[0] == "" || config.Header["Cookie"] == "" || config.Header["User-Agent"] == "" {
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
			downloadConent(config)
		} else {
			downloadPlaylist(config)
		}
	case "series":
		serialService(config)
	case "parse":
		err := config.ParseHtml(tools.Argparser(3))
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("Parsed HTML Successfully")
		}
	default:
		parallelService(config)
	}
}
