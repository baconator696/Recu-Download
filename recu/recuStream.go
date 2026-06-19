package recu

import (
	"fmt"
	"os"
	"recurbate/playlist"
	"recurbate/tools"
	"recurbate/tools/avgBuffer"
	"time"
)

// Muxes the transport streams and saves it to a file
func Mux(playList playlist.Playlist, header map[string]string, startIndex int, durationPercent []float64) (failIndex int, err error) {
	var data []byte
	var file *os.File
	avgdur := avgBuffer.New(25)
	avgsize := avgBuffer.New(25)
	if startIndex < 0 {
		startIndex = 0
	}
	if tools.Abort {
		return startIndex, fmt.Errorf("aborting")
	}
	if durationPercent[0] > 100 || durationPercent[1] <= durationPercent[0] {
		return startIndex, fmt.Errorf("duration format error")
	}
	if durationPercent[0] < 0 {
		durationPercent[0] = 0
	}
	if durationPercent[1] > 100 {
		durationPercent[1] = 100
	}
	// checks if continuation of previous run
	if startIndex != 0 {
		file, err = os.OpenFile(playList.Filename+".ts", os.O_APPEND|os.O_WRONLY, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "original file not found, creating new one: %v", err)
		}
	}
	// creates file
	if file == nil {
		// checks for filename collisions
		_, err = os.Stat(playList.Filename + ".ts")
		if err == nil {
			for i := 1; i > 0; i++ {
				new := fmt.Sprintf("%s(%d)", playList.Filename, i)
				_, err := os.Stat(new + ".ts")
				if err != nil {
					playList.Filename = new
					break
				}
			}
		}
		file, err = os.OpenFile(playList.Filename+".ts", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			return startIndex, fmt.Errorf("can not create file: %v", err)
		}
	}
	defer file.Close()
	// muxing loop //
	if startIndex == 0 {
		startIndex = int(float64(playList.Len()) * durationPercent[0] / 100)
	}
	endIndex := int(float64(playList.Len()) * durationPercent[1] / 100)
	for i, tsLink := range playList.List[startIndex:endIndex] {
		i := i + startIndex
		if tools.Abort {
			fmt.Println()
			return i, fmt.Errorf("aborting")
		}
		startTime := time.Now()
		err := muxDownloadLoop(&data, tsLink, header, 10, 5)
		if err != nil {
			fmt.Println()
			err = fmt.Errorf("error: %v\nFailed at %.2f%%", tools.ANSIColor(err, 2), float32(i)/float32(playList.Len())*100)
			return i, err
		}
		endDur := time.Since(startTime).Minutes()
		_, err = file.Write(data)
		if err != nil {
			err = fmt.Errorf("can not write file: %v", err)
			return i, err
		}
		// Calculate User Interface Timings
		avgsize.Add(float64(len(data)))
		avgdur.Add(endDur)
		getavgdur := avgdur.Average()
		speedSecs := avgsize.Average() / (getavgdur * 60)
		eta := getavgdur * ((float64(playList.Len()) * durationPercent[1] / 100) - float64(i))
		percent := float64(i) / float64(playList.Len()) * 100
		fmt.Printf("\n\033[A\033[2KDownloading: %s\tRemaining: %s\t%s", tools.ANSIColor(fmt.Sprintf("%.1f%%", percent), 33), tools.FormatMinutes(eta), tools.FormatBytesPerSecond(speedSecs))
	}
	return 0, nil
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
