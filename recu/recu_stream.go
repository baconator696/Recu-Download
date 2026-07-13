package recu

import (
	"fmt"
	"os"
	"recurbate/config/state"
	"recurbate/tools"
	"recurbate/tools/avgBuffer"
	"time"
)

// Muxes the transport streams and saves it to a file
func Mux(video *state.Video, conf *state.Config) (err error) {
	video.State.Stage = state.DOWNLOAD
	var data []byte
	var file *os.File
	avgdur := avgBuffer.New(25)
	avgsize := avgBuffer.New(25)
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
	if video.ImportedOffset {
		file, err = os.OpenFile(conf.Wd+"/"+video.Playlist.Filename+".ts", os.O_APPEND|os.O_WRONLY, 0666)
		if err != nil {
			conf.ErrCh <- fmt.Errorf("original file not found, creating new one: %v", err)
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
		file, err = os.OpenFile(conf.Wd+"/"+video.Playlist.Filename+".ts", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			return fmt.Errorf("can not create file: %v", err)
		}
	}
	defer file.Close()
	// muxing loop //
	if !video.ImportedOffset {
		video.Offset = int(float32(video.Playlist.Len()) * video.Section[0] / 100)
	}
	endIndex := int(float32(video.Playlist.Len()) * video.Section[1] / 100)
	startOffset := video.Offset
	for i, tsLink := range video.Playlist.List[startOffset:endIndex] {
		video.Offset = i + startOffset
		if tools.Abort {
			return fmt.Errorf("aborting")
		}
		startTime := time.Now()
		err := muxDownloadLoop(&data, tsLink, video.Header, 10, 5, conf.ErrCh)
		if err != nil {
			return fmt.Errorf("error: %v\nFailed at %.2f%%", tools.ANSIColor(err, 2), float32(i)/float32(video.Playlist.Len())*100)
		}
		endDur := float32(time.Since(startTime).Minutes())
		_, err = file.Write(data)
		if err != nil {
			return fmt.Errorf("can not write file: %v", err)
		}
		// Calculate User Interface Timings
		avgsize.Add(float32(len(data)))
		avgdur.Add(endDur)
		getavgdur := avgdur.Average()
		video.State.DownloadSpeed = avgsize.Average() / (getavgdur * 60)
		video.State.Eta = getavgdur * ((float32(video.Playlist.Len()) * video.Section[1] / 100) - float32(i))
		video.State.ProgressPercent = float32(i) / float32(video.Playlist.Len()) * 100
		conf.MsgCh <- fmt.Sprintf("\n\033[A\033[2KDownloading: %s\tRemaining: %s\t%s",
			tools.ANSIColor(fmt.Sprintf("%.1f%%", video.State.ProgressPercent), 33),
			tools.FormatMinutes(video.State.Eta),
			tools.FormatBytesPerSecond(video.State.DownloadSpeed),
		)
	}
	video.State.Stage = state.COMPLETE
	video.State.Complete = true
	return nil
}

// download retry loop for Mux()
func muxDownloadLoop(data *[]byte, url string, header map[string]string, timeout, maxRetry int, errCh chan error) (err error) {
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
			errCh <- fmt.Errorf("\nDownload Expired")
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
		errCh <- fmt.Errorf("\nError: %v, Retrying...", tools.ShortenString(err, 40))
		time.Sleep(time.Second)
	}
	return
}
