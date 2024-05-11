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
)

func parallelService(links []string, header map[string]string, num int, duration []float64) {
	playlists := make([][]byte, len(links))
	filenames := make([]string, len(links))
	for i, link := range links {
		data, filename, status := tools.RecurbateParser(link, header)
		if status == "cloudflare" {
			fmt.Printf("Cloudflare Blocked: Failed on url: %v\n", link)
		}
		if status == "cookie" {
			fmt.Printf("Cookie Expired: Failed on url: %v\n", link)
		}
		if status == "wait" {
			fmt.Printf("Daily View Used: Failed on url: %v\n", link)
		}
		if status == "panic" {
			fmt.Printf("Panic: Failed on url: %v\n", link)
		}
		if status == "done" {
			playlists[i] = data
			filenames[i] = filename
		}
	}
	var wg sync.WaitGroup
	for i, data := range playlists {
		if data == nil {
			continue
		}
		wg.Add(1)
		go func(data []byte, i int) {
			defer wg.Done()
			fail := 0
			fail = tools.MuxPlaylist(data, filenames[i], header, num, duration, fail)
			if fail == 0 {
				fmt.Printf("Completed: %v:%v\n", filenames[i], links[i])
				return
			}
			fmt.Printf("Download Failed at line: %v\n", fail)
			err := saveJson(links, header, fail*-1, duration)
			if err != nil {
				fmt.Println(err)
			}
			err = os.WriteFile(filenames[i]+".m3u8", data, 0666)
			if err != nil {
				fmt.Println(data)
				fmt.Printf("Failed to write playlist data: %v\n", err)
			}
		}(data,i)
	}
	wg.Wait()
}
func serialService(links []string, header map[string]string, num int, duration []float64) {
	playlists := make([][]byte, len(links))
	filenames := make([]string, len(links))
	for i, link := range links {
		data, filename, status := tools.RecurbateParser(link, header)
		if status == "cloudflare" {
			fmt.Printf("Cloudflare Blocked: Failed on url: %v\n", link)
		}
		if status == "cookie" {
			fmt.Printf("Cookie Expired: Failed on url: %v\n", link)
		}
		if status == "wait" {
			fmt.Printf("Daily View Used: Failed on url: %v\n", link)
		}
		if status == "panic" {
			fmt.Printf("Panic: Failed on url: %v\n", link)
		}
		if status == "done" {
			playlists[i] = data
			filenames[i] = filename
		}
	}
	for i, data := range playlists {
		if data == nil {
			continue
		}
		fail := 0
		fmt.Printf("%v/%v",i,len(playlists))
		fail = tools.MuxPlaylist(data, filenames[i], header, num, duration, fail)
		if fail == 0 {
			fmt.Printf("Completed: %v:%v\n", filenames[i], links[i])
			continue
		}
		fmt.Printf("Download Failed at line: %v\n", fail)
		err := saveJson(links, header, fail*-1, duration)
		if err != nil {
			fmt.Println(err)
		}
		err = os.WriteFile(filenames[i]+".m3u8", data, 0666)
		if err != nil {
			fmt.Println(data)
			fmt.Printf("Failed to write playlist data: %v\n", err)
		}
	}
}
func downloadPlaylist(links []string, header map[string]string) {
	i := 0
	data, filename, status := tools.RecurbateParser(links[i], header)
	if status == "cloudflare" {
		fmt.Println("Cloudflare Blocked")
		os.Exit(3)
	}
	if status == "cookie" {
		fmt.Println("Cookie Expired")
		os.Exit(3)
	}
	if status == "wait" {
		fmt.Println("Daily View Used")
		os.Exit(3)
	}
	if status == "done" {
		err := os.WriteFile(filename+".m3u8", data, 0666)
		if err != nil {
			fmt.Println(data)
			fmt.Printf("Failed to write playlist data: %v\n", err)
			os.Exit(2)
		}
		fmt.Printf("Completed: %v:%v\n", filename, links[i])
	}
	if status == "panic" {
		os.Exit(69)
	}
}
func downloadConent(links []string, header map[string]string, num int, duration []float64) {
	data, err := os.ReadFile(tools.Argparser(3))
	if err != nil {
		fmt.Printf("Failed to read playlist: %v\n", err)
		os.Exit(2)
	}
	filename := tools.Argparser(3)
	filename = strings.ReplaceAll(filename, ".m3u8", "")
	fail := 0
	fail = tools.MuxPlaylist(data, filename, header, num, duration, fail)
	if fail == 0 {
		fmt.Printf("Completed: %v:%v\n", filename, links[0])
		return
	}
	fmt.Printf("Download Failed at line: %v\n", fail)
	err = saveJson(links, header, fail*-1, duration)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	os.Exit(1)
}
func saveJson(links []string, header map[string]string, num int, duration []float64) (err error){
	jsonData, err := json.MarshalIndent(tools.Templet{
		Urls:     links,
		Header:   header,
		Num:      num,
		Duration: duration,
	}, "", "\t")
	if err != nil {
		return fmt.Errorf("error: Parsing Json%v",err)
	}
	jsonLocation := "config.json"
	if tools.Argparser(1) != "" {
		jsonLocation = tools.Argparser(1)
	}
	err = os.WriteFile(jsonLocation, jsonData, 0666)
	if err != nil {
		err = fmt.Errorf("error: Saving Json:%v",err)
		return
	}
	return
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
	if tools.Argparser(1) == "--help" {
		fmt.Println("Recurbate:")
		fmt.Println("If ran for the first time, json configuration will be generated in the working directory")
		fmt.Println("Fill in the json's URL, Cookie and User-Agent to allow the program to run")
		fmt.Println("\nUsage: recurbate <json location> playlist/series <playlist.m3u8>")
		fmt.Println("\nif \"playlist\" is used, only the .m3u8 playlist file will be downloaded,specifiying the playlist location will download the contents of the playlist")
		fmt.Println("\nif \"series\" is used, the program will download both the playlists and videos in series, all the playlists will be downloaded first")
		fmt.Println("\njson parameter definitions:\n\tDuration: start and stop percentage for the video download\n\tnum: helps you get a preview of a hidden video, you can put anything above 10, putting 20 will give 20 preview clips in the video. 3-10 will create much larger previews")
		fmt.Println("\nif download fails and a playlist is saved, you can specifiy what line it failed at using the 'num' parameter using a negative number, this will allow the program to resume where it left off. Only use this if a partial file was downloaded")
		return
	}
	json_location := "config.json"
	if tools.Argparser(1) != "" {
		json_location = tools.Argparser(1)
	}
	_, err := os.Stat(json_location) // Check if json exists
	if err != nil {
		defaultConfig := tools.TempletJSON()
		saveJson(defaultConfig.Urls, defaultConfig.Header, defaultConfig.Num, defaultConfig.Duration)
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
		fmt.Println("Error: Reading Json")
		fmt.Println(err)
		os.Exit(4)
	}
	if config.Urls[0] == "" || config.Header["Cookie"] == tools.TempletJSON().Header["Cookie"] || config.Header["User-Agent"] == tools.TempletJSON().Header["User-Agent"] {
		fmt.Println("please modify config.json")
		return
	}
	if tools.Argparser(3) != "" { // Checks if playlist file is passed and if it exists
		_, err := os.Stat(tools.Argparser(3))
		if err != nil {
			fmt.Println(err)
			os.Exit(4)
		}
		downloadConent(config.Urls, config.Header, config.Num, config.Duration)
		return
	} else if tools.Argparser(2) == "playlist" { // Checks if playlist and downloads it
		downloadPlaylist(config.Urls, config.Header)
		return
	} else if tools.Argparser(2) == "series" { // Checks if series and downloads it
		serialService(config.Urls, config.Header, config.Num, config.Duration)
		return
	} else {
		parallelService(config.Urls, config.Header, config.Num, config.Duration)
		return
	}
}
