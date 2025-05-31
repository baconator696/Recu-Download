package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var Abort bool

// Check for update
func CheckUpdate(currentTag string) (err error) {
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	respJson, status, err := Request("https://api.github.com/repos/baconator696/Recu-Download/releases/latest", 2, nil, nil, "GET")
	if err != nil {
		return
	} else if status != 200 {
		return fmt.Errorf("status: %d, %s", status, string(respJson))
	}
	var resp any
	err = json.Unmarshal(respJson, &resp)
	if err != nil {
		return
	}
	if resp.(map[string]any)["prerelease"].(bool) {
		return
	}
	newTag := resp.(map[string]any)["tag_name"].(string)
	newTag = strings.ReplaceAll(newTag, "v", "")
	newNums := strings.Split(newTag, ".")
	currentTag = strings.ReplaceAll(currentTag, "v", "")
	currentNums := strings.Split(currentTag, ".")
	for i, v := range newNums {
		current, err := strconv.Atoi(currentNums[i])
		if err != nil {
			continue
		}
		new, err := strconv.Atoi(v)
		if err != nil {
			continue
		}
		if new > current {
			fmt.Printf("New Update Available: v%s\n", newTag)
			fmt.Printf("%s\n%s\n", resp.(map[string]any)["html_url"].(string), ANSIColor(resp.(map[string]any)["body"].(string), 2))
			return nil
		}
	}
	return nil
}

// Returns the raw data from the URL
func Request(url string, timeout int, header map[string]string, body []byte, Type string) ([]byte, int, error) {
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

// Parses executatables arguments to prevent runtime errors
func Argparser(n int) string {
	if len(os.Args) > n {
		return os.Args[n]
	}
	return ""
}

// ANSI Color
func ANSIColor(str any, mod int, color ...int) (final string) {
	var x, r, g, b int
	var rgb bool
	if len(color) == 1 {
		x = color[0]
	} else if len(color) == 3 {
		rgb = true
		r = color[0]
		g = color[1]
		b = color[2]
	}
	var res int
	switch {
	case mod == 1:
		res = 22
	case mod == 21:
		res = 24
	case mod >= 2 && mod <= 9:
		res = mod + 20
	case (mod >= 30 && mod <= 38) || (mod >= 90 && mod <= 97):
		res = 39
	case (mod >= 40 && mod <= 48) || (mod >= 100 && mod <= 107):
		res = 49
	}
	if mod == 38 || mod == 48 {
		if rgb {
			final = fmt.Sprintf("\033[%d;2;%d;%d;%dm%v\033[%dm", mod, r, g, b, str, res)
		} else {
			final = fmt.Sprintf("\033[%d;5;%dm%v\033[%dm", mod, x, str, res)
		}
	} else {
		final = fmt.Sprintf("\033[%dm%v\033[%dm", mod, str, res)
	}
	return
}

// Looks for the first occurence of start and end and returns the string in between
func SearchString(str string, start string, end string) (string, error) {
	if len(str) <= len(start)+len(end) {
		return "", fmt.Errorf("search term longer than the given string")
	}
	index1 := strings.Index(str, start)
	index2 := strings.Index(str[index1+len(start):], end)
	if index1 == -1 || index2 == -1 {
		return "", fmt.Errorf("could not find {%v} and/or {%v} in {%v}", start, end, ANSIColor(str, 2))
	}
	return str[index1+len(start) : index1+len(start)+index2], nil
}

// String Shorten
func ShortenString(str any, ln int) string {
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

// convert timestamps into percent
func PercentPrase(times []any) []float64 {
	var start, end float64
	var secs [3]int
	for i, w := range times {
		v, ok := w.(string)
		if !ok {
			fmt.Fprintf(os.Stderr, "timestamps is in wrong format: %v\n", times)
			return nil
		}
		time := strings.Split(v, ":")
		cons := 1
		for j := len(time) - 1; j >= 0; j-- {
			w, err := strconv.Atoi(time[j])
			if err != nil {
				fmt.Fprintf(os.Stderr, "timestamps is in wrong format: %v\n", times)
				return nil
			}
			secs[i] += w * cons
			cons *= 60
		}
	}
	start = float64(secs[0]) / float64(secs[2]) * 100
	end = float64(secs[1]) / float64(secs[2]) * 100
	return []float64{start, end}
}

// Defines the Average Buffer
type AvgBuffer struct {
	data []float64
	pos  int
	size int
}

// Returns the Average of all the floats in the buffer
func (buff AvgBuffer) Average() (avg float64) {
	for _, value := range buff.data {
		avg += value
	}
	avg /= float64(len(buff.data))
	return
}

// Adds a number to the average buffer
func (buff *AvgBuffer) Add(add float64) {
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

// Converts int in Seconds to a formated string
func FormatMinutes(num float64) string {
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
func FormatBytesPerSecond(num float64) string {
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

// Return Formated Headers, url needed only if i is 2
func FormatedHeader(refHeader map[string]string, videoUrl string, i int) (header map[string]string) {
	header = make(map[string]string)
	for k, v := range refHeader {
		header[k] = v
	}
	header["Accept"] = "*/*"
	header["Accept-Language"] = "en-US,en;q=0.9"
	header["Origin"] = "https://recu.me"
	header["Priority"] = "u=1, i"
	header["Sec-Ch-Ua"] = `"Chromium";v="128", "Not;A=Brand";v="24"`
	header["Sec-Ch-Ua-Full-Version-List"] = `"Chromium";v="128.0.6613.120", "Not;A=Brand";v="24.0.0.0"`
	header["Sec-Ch-Ua-Mobile"] = "?0"
	header["Sec-Ch-Ua-Platform"] = `"Windows"`
	header["Sec-Fetch-Dest"] = "empty"
	header["Sec-Fetch-Mode"] = "cors"
	header["Sec-Ch-Ua-Arch"] = `"x86"`
	header["Sec-Ch-Ua-Bitness"] = `"64"`
	header["Sec-Ch-Ua-Full-Version"] = `"128.0.2739.67"`
	header["Sec-Ch-Ua-Model"] = `""`
	header["Sec-Ch-Ua-Platform-Version"] = `"15.0.0"`
	switch i {
	case 1: // html
		header["Accept"] = "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"
		header["Referer"] = "https://recu.me/"
		header["Sec-Fetch-Dest"] = "document"
		header["Sec-Fetch-Mode"] = "navigate"
		header["Sec-Fetch-Site"] = "none"
		header["Sec-Fetch-User"] = "?1"
		header["Upgrade-Insecure-Requests"] = "1"
	case 2: // playlist link
		header["Referer"] = videoUrl
		header["Sec-Fetch-Site"] = "same-origin"
		header["X-Requested-With"] = "XMLHttpRequest"
	default: // playlist
		header["Sec-Fetch-Site"] = "cross-site"
		delete(header, "Cookie")
		delete(header, "Sec-Ch-Ua-Full-Version-List")
		delete(header, "Sec-Ch-Ua-Arch")
		delete(header, "Sec-Ch-Ua-Bitness")
		delete(header, "Sec-Ch-Ua-Full-Version")
		delete(header, "Sec-Ch-Ua-Model")
		delete(header, "Sec-Ch-Ua-Platform-Version")
	}
	return
}
