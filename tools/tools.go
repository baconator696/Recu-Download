package tools

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	Abort bool
)

// Defines the JSON used
type Templet struct {
	Urls     []string          `json:"urls"`
	Header   map[string]string `json:"header"`
	Num      int               `json:"num"`
	Duration []float64         `json:"duration"`
}

// Defines the Average Buffer
type AvgBuffer struct {
	data []float64
	pos  int
	size int
}

// Returns the Average of the buffer
func (buff AvgBuffer) average() (avg float64) {
	for _, value := range buff.data {
		avg += value
	}
	avg /= float64(len(buff.data))
	return
}

// Adds a number to the average buffer
func (buff *AvgBuffer) add(add float64) {
	if buff.size <= 0 {
		buff.size = 25
	}
	if buff.pos < 0 || buff.pos >= buff.size {
		buff.pos = 0
	}
	for buff.pos >= len(buff.data) {
		buff.data = append(buff.data, add)
	}
	buff.data[buff.pos] = add
	buff.pos++
}

// Parses executatables arguments to prevent runtime errors
func Argparser(n int) string {
	if len(os.Args) > n {
		return os.Args[n]
	}
	return ""
}

// Looks for the first occurence of start and end and returns the string in between
func searchString(str string, start string, end string) (string, error) {
	index1 := strings.Index(str, start)
	index2 := strings.Index(str[index1+len(start):], end)
	if index1 == -1 || index2 == -1 {
		return "", fmt.Errorf("could not find {%v} and/or {%v} in {%v}", start, end, str)
	}
	return str[index1+len(start) : index1+len(start)+index2], nil
}

// Returns the raw data from the URL
func request(url string, timeout int, header map[string]string, body []byte, Type string) ([]byte, int, error) {
	req, err := http.NewRequest(Type, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, 0, fmt.Errorf("http.NewRequest:%v", err)
	}
	for key, value := range header {
		req.Header.Set(key, value)
	}
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}
	data, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("client.Do:%v", err)
	}
	defer data.Body.Close()
	databytes, err := io.ReadAll(data.Body)
	if err != nil {
		return nil, data.StatusCode, fmt.Errorf("io.ReadAll:%v", err)
	}
	return databytes, data.StatusCode, nil
}

// Converts int in Seconds to a formated string
func formatMinutes(num float64) string {
	var unit string
	switch true {
	case num < 1:
		num *= 60
		unit = "secs"
	case num > 1440:
		num /= 1440
		unit = "days"
	case num > 60:
		num /= 60
		unit = "hours"
	default:
		unit = "mins"
	}
	return fmt.Sprintf("%.1f %s", num, unit)
}

// Converts Number of Bytes per second to a formated string
func formatBytesPerSecond(num float64) string {
	var unit string
	switch true {
	case num < 1000:
		unit = "B/s"
	case num >= 1000000:
		num /= 1000000
		unit = "MB/s"
	case num >= 1000:
		num /= 1000
		unit = "KB/s"
	}
	return fmt.Sprintf("%.1f %s", num, unit)
}

// Muxes the transport streams and saves it to a file
func MuxPlaylist(indexdata []byte, filename string, header map[string]string, num int, duration []float64, restart int) int {
	var data []byte
	var err error
	var file *os.File
	var avgdur AvgBuffer
	var dur float64
	var eta float64
	var percent float64
	var speed float64
	var avgsize AvgBuffer
	var retry int
	var start time.Time
	var getavgdur float64
	indexlist := strings.Split(string(indexdata), "\n")
	length := len(indexlist)
	if num == 0 || num > length/2 {
		num = 1
	}
	if num%2 == 0 {
		num--
	}
	if num < 0 {
		restart = num * -1
		num = 1
	}
	if duration[0] > 100 || duration[1] <= duration[0] {
		return 0
	}
	if duration[0] < 0 {
		duration[0] = 0
	}
	if duration[1] > 100 {
		duration[1] = 100
	}
	step := int(float64(length) * duration[0] / 100)
	writen := false
	if restart > 0 {
		step = restart
		file, err = os.OpenFile(filename+".ts", os.O_APPEND|os.O_WRONLY, 0666)
		if err != nil {
			fmt.Println("Error: original file not found, creating new file")
		} else {
			defer file.Close()
			writen = true
		}
	}
	for step < int(float64(length)*duration[1]/100) {
		if indexlist[step] != "" && indexlist[step][0] != '#' {
			retry = 0
			start = time.Now()
			for {
				var status int
				data, status, err = request(indexlist[step], 60, header, nil, "GET")
				if err == nil && status == 200 {
					break
				}
				if status == 429 {
					time.Sleep(time.Second)
					continue
				}
				if status == 410 {
					fmt.Println("\nDownload Expired")
					retry = 5
				}
				retry++
				if err == nil {
					err = fmt.Errorf("status Code: %d, %s", status, string(data))
				}
				if retry > 5 {
					fmt.Printf("\nError: %v\n", err)
					fmt.Printf("Failed at %.2f%%\n", float32(step)/float32(length)*100)
					return step
				}
				fmt.Printf("\n\033[2A\033[2KError: %v, Retrying...\n", shortenString(err, 40))
			}
			dur = time.Since(start).Minutes()
			if !writen {
				err = os.WriteFile(filename+".ts", data, 0666)
				if err != nil {
					fmt.Println("DEBUG:215:FAILED TO WRITE DATA, ERROR HANDELING NEEDED")
				}
				file, _ = os.OpenFile(filename+".ts", os.O_APPEND|os.O_WRONLY, 0666)
				defer file.Close()
				writen = true
			} else {
				_, err = file.Write(data)
				if err != nil {
					fmt.Println("DEBUG:223:FAILED TO WRITE DATA, ERROR HANDELING NEEDED")
				}
			}
			avgsize.add(float64(len(data)))
			avgdur.add(dur)
			getavgdur = avgdur.average()
			speed = avgsize.average() / (getavgdur * 60)
			eta = getavgdur * ((float64(length) * duration[1] / 100) - float64(step)) / 2
			percent = float64(step) / float64(length) * 100
			fmt.Printf("\n\033[A\033[2KDownloading: \033[33m%.1f%%\033[0m\tRemaining: %s\t%s", percent, formatMinutes(eta), formatBytesPerSecond(speed))
			if num > 10 {
				step += int(math.Ceil(float64(length) / float64(num)))
			} else {
				step += num
			}
		}
		step++
		if Abort {
			fmt.Printf("\n")
			return step
		}
	}
	fmt.Printf("\n")
	return 0
}

// Takes recurbate video URL and returns playlist raw data and returns file name {indexdata, filename, "done"}
func RecurbateParser(url string, header map[string]string) ([]byte, string, string) {
	downloadLoop := func(url string, timeout int, header map[string]string) (data []byte, err error) {
		retry := 0
		for {
			var status int
			data, status, err = request(url, timeout, header, nil, "GET")
			if err == nil && status == 200 {
				break
			}
			fmt.Printf("Failed Retrying...\033[18D")
			if retry > 5 {
				if err == nil {
					err = fmt.Errorf("status code: %d, %s", status, string(data))
				}
				return
			}
			retry++
			timeout += 30
			time.Sleep(time.Second)
		}
		return
	}
	fmt.Printf("\rDownloading HTML")
	htmldata, err := downloadLoop(url, 10, header)
	if err != nil {
		fmt.Println(err)
		return nil, "", "cloudflare"
	}
	fmt.Printf("\r\033[2KDownloading HTML: Complete\n")
	token, err := searchString(string(htmldata), "data-token=\"", "\"")
	if err != nil {
		fmt.Println(err)
		return nil, "", "panic"
	}
	id := strings.Split(url, "/")[4]
	url = "https://recu.me/api/video/" + id + "?token=" + token
	fmt.Printf("\rGetting Link to Playlist")
	apidata, err := downloadLoop(url, 10, header)
	if err != nil {
		fmt.Println(err)
		return nil, "", "panic"
	}
	fmt.Printf("\r\033[2KGetting Link to Playlist: Complete\n")
	switch string(apidata) {
	case "shall_subscribe":
		return nil, "", "wait"
	case "shall_signin":
		return nil, "", "cookie"
	case "wrong_token":
		return nil, "", "cloudflare"
	}
	url, err = searchString(string(apidata), "<source src=\"", "\"")
	if err != nil {
		fmt.Println(err)
		return nil, "", "panic"
	}
	url = strings.ReplaceAll(url, "amp;", "")
	fmt.Printf("\rDownloading Playlist")
	indexdata, err := downloadLoop(url, 10, header)
	if err != nil {
		fmt.Println(err)
		return nil, "", "panic"
	}
	fmt.Printf("\r\033[2KDownloading Playlist: Complete\n")
	filename, err := searchString(url, "hl/", "/index")
	if err != nil {
		fmt.Println(err)
		return nil, filename, "panic"
	}
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = "CB_" + strings.ReplaceAll(filename, ",", "_")
	for i := 2010; i < 2050; i++ {
		year := fmt.Sprintf("%d", i)
		if strings.Contains(filename, year) {
			filename = strings.Replace(filename, year, year[2:], 1)
			break
		}
	}
	i := 0
	var prefix string
	for {
		j := strings.Index(url[i:], "/")
		if j == -1 {
			prefix = url[:i]
			break
		}
		i = j + i + 1
	}
	playlistString := string(indexdata)
	if !strings.Contains(playlistString, prefix) {
		playlistLines := strings.Split(playlistString, "\n")
		modifiedPlaylist := make([]string, len(playlistLines))
		for i, line := range playlistLines {
			if len(line) > 0 {
				if line[0] == '#' {
					modifiedPlaylist[i] = line
				} else {
					modifiedPlaylist[i] = prefix + line
				}
			}
		}
		modifiedPlaylistString := strings.Join(modifiedPlaylist, "\n")
		indexdata = []byte(modifiedPlaylistString)
	}
	return indexdata, filename, "done"
}

// String Shorten
func shortenString(str any, ln int) string {
	if ln < 0 {
		ln = 0
	}
	switch i := str.(type) {
	case string:
		if len(i) > ln {
			return i[:ln]
		} else {
			return i
		}
	case error:
		if len(i.Error()) > ln {
			return i.Error()[:ln]
		} else {
			return i.Error()
		}
	default:
		return fmt.Sprintf("Type:%v", i)
	}
}

// Returns default templet
func TempletJSON() Templet {
	var jsonTemplet Templet
	jsonTemplet.Header = map[string]string{
		"Accept":             "*/*",
		"Accept-Language":    "en-US,en;q=0.9",
		"Cookie":             "",
		"Origin":             "https://recu.me",
		"Sec-Ch-Ua":          "\"Not_A Brand\";v=\"8\", \"Chromium\";v=\"120\", \"Google Chrome\";v=\"120\"",
		"Sec-Ch-Ua-Mobile":   "?0",
		"Sec-Ch-Ua-Platform": "\"Windows\"",
		"Sec-Fetch-Dest":     "empty",
		"Sec-Fetch-Mode":     "cors",
		"Sec-Fetch-Site":     "cross-site",
		"User-Agent":         "",
	}
	jsonTemplet.Num = 1
	jsonTemplet.Urls = []string{""}
	jsonTemplet.Duration = []float64{0, 100}
	return jsonTemplet
}
