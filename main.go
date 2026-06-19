package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"recurbate/config"
	"recurbate/maintenance"
	"recurbate/recu"
	"recurbate/tools"
	"strings"
	"sync"
	"syscall"
)

var tag string

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
	downloaded, if you wanted to download with external tool
if "series" is used, the program will download all the videos
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
	maintenance.CheckUpdate(tag)
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
		if tools.Argparser(2) != "parse" {
			return
		}
	}
	var switchFunc func(self config.Config, ch chan error, statusCh chan recu.Status)
	switch tools.Argparser(2) {
	case "playlist":
		switchFunc = config.Config.DownloadPlaylist
		return
	case "series":
		switchFunc = config.Config.SerialService
	case "parse":
		err := cfg.ParseHtml(tools.Argparser(3))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			fmt.Println("Parsed HTML Successfully")
		}
		return
	default:
		switchFunc = config.Config.HybridService
	}
	errCh := make(chan error)
	statusCh := make(chan recu.Status)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer close(errCh)
		defer close(statusCh)
		switchFunc(cfg, errCh, statusCh)
	}()
	// std out
	go func() {
		defer wg.Done()
		recuCLI(statusCh)
	}()
	for msg := range errCh {
		fmt.Fprintln(os.Stderr, msg)
	}
	wg.Wait()
}

func recuCLI(statusCh chan recu.Status) {
	for msg := range statusCh {
		switch msg {
		case recu.FailRetry:
			fmt.Printf("Failed Retrying...\n")
		case recu.DownloadHTML:
			fmt.Printf("\rDownloading HTML: ")
		case recu.GetPlaylist:
			fmt.Printf("\rDownloading Playlists: ")
		case recu.GetPlaylistUrl:
			fmt.Printf("\rGetting Link to Playlist: ")
		case recu.CompleteLastAction:
			fmt.Printf("Complete\n")
		}
	}
}
