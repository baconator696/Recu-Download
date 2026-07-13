package main

import (
	"fmt"
	"os"
	"os/signal"
	"recurbate/config"
	"recurbate/config/state"
	"recurbate/maintenance"
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
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	conf, err := state.New(wd, json_location)
	if err != nil {
		if tools.Argparser(2) != "parse" {
			panic(err)
		}
	}

	var switchFunc func(self *state.Config)
	switch tools.Argparser(2) {
	case "playlist":
		switchFunc = config.DownloadPlaylist
	case "series":
		switchFunc = config.SerialService
	case "parse": // CLI ONLY
		err := conf.Json_ref.ParseHtml(tools.Argparser(3), wd, json_location)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			fmt.Println("Parsed HTML Successfully")
		}
		return
	default:
		switchFunc = config.HybridService
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer close(conf.ErrCh)
		defer close(conf.MsgCh)
		switchFunc(&conf)
	}()
	// std out
	go func() {
		defer wg.Done()
		for msg := range conf.MsgCh {
			fmt.Print(msg)
		}
	}()
	for msg := range conf.ErrCh {
		fmt.Fprintln(os.Stderr, msg)
	}
	wg.Wait()
}
