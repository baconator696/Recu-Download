package recu

import (
	"fmt"
	"os"
	"recurbate/config/typ"
	"recurbate/tools"
	"recurbate/tools/avgBuffer"
	"time"
)

// Muxes the transport streams and saves it to a file
func Mux(video *typ.Video, conf *typ.Config) (err error) {
	var data []byte
	var file *os.File
	avgdur := avgBuffer.New(25)
	avgsize := avgBuffer.New(25)
	if video.Offset < 0 {
		video.Offset = 0
	}
	if tools.Abort {
		return fmt.Errorf("aborting")
	}
	if video.Section[0] > 100 || video.Section[1] <= video.Section[0] {
		return fmt.Errorf("duration format error")
	}
	if video.Section[0] < 0 {
		video.Section[0] = 0
	}
	if video.Section[1] > 100 {
		video.Section[1] = 100
	}
	// checks if continuation of previous run
	if video.Offset != 0 {
		file, err = os.OpenFile(conf.Wd+video.Playlist.Filename+".ts", os.O_APPEND|os.O_WRONLY, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "original file not found, creating new one: %v", err)
		}
	}
	// creates file
	if file == nil {
		// checks for filename collisions
		_, err = os.Stat(conf.Wd + video.Playlist.Filename + ".ts")
		if err == nil {
			for i := 1; i > 0; i++ {
				new := fmt.Sprintf("%s(%d)", video.Playlist.Filename, i)
				_, err := os.Stat(conf.Wd + new + ".ts")
				if err != nil {
					video.Playlist.Filename = new
					break
				}
			}
		}
		file, err = os.OpenFile(conf.Wd+video.Playlist.Filename+".ts", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			return fmt.Errorf("can not create file: %v", err)
		}
	}
	defer file.Close()
	// muxing loop //
	if video.Offset == 0 {
		video.Offset = int(float32(video.Playlist.Len()) * video.Section[0] / 100)
	}
	endIndex := int(float32(video.Playlist.Len()) * video.Section[1] / 100)
	for i, tsLink := range video.Playlist.List[video.Offset:endIndex] {
		i := i + video.Offset
		if tools.Abort {
			fmt.Println()
			return fmt.Errorf("aborting")
		}
		startTime := time.Now()
		err := muxDownloadLoop(&data, tsLink, video.Header, 10, 5)
		if err != nil {
			fmt.Println()
			err = fmt.Errorf("error: %v\nFailed at %.2f%%", tools.ANSIColor(err, 2), float32(i)/float32(video.Playlist.Len())*100)
			return err
		}
		endDur := float32(time.Since(startTime).Minutes())
		_, err = file.Write(data)
		if err != nil {
			err = fmt.Errorf("can not write file: %v", err)
			return err
		}
		// Calculate User Interface Timings
		avgsize.Add(float32(len(data)))
		avgdur.Add(endDur)
		getavgdur := avgdur.Average()
		video.State.DownloadSpeed = avgsize.Average() / (getavgdur * 60)
		video.State.Eta = getavgdur * ((float32(video.Playlist.Len()) * video.Section[1] / 100) - float32(i))
		video.State.ProgressPercent = float32(i) / float32(video.Playlist.Len()) * 100
		fmt.Printf("\n\033[A\033[2KDownloading: %s\tRemaining: %s\t%s",
			tools.ANSIColor(fmt.Sprintf("%.1f%%", video.State.ProgressPercent), 33),
			tools.FormatMinutes(video.State.Eta),
			tools.FormatBytesPerSecond(video.State.DownloadSpeed),
		)
	}
	return nil
}

// download retry loop for Mux()
func muxDownloadLoop(data *[]byte, url string, header map[string]string, timeout, maxRetry int) (err error) {
	retry := 0
	for {
		var status int
		*data, status, err = tools.Request(url, timeout, header, nil, "GET")
		if err == nil && status == 200 {
			break
		}
		if status == 429 {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if status == 410 {
			fmt.Fprintln(os.Stderr, "\nDownload Expired")
			retry = maxRetry
		}
		retry++
		if err == nil {
			err = fmt.Errorf("status Code: %d, %s ", status, string(*data))
		} else {
			timeout += 30
		}
		if retry > maxRetry {
			return
		}
		fmt.Fprintf(os.Stderr, "\n\033[2A\033[2KError: %v, Retrying...\n", tools.ANSIColor(tools.ShortenString(err, 40), 2))
		time.Sleep(time.Second)
	}
	return
}
