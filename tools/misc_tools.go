package tools

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

var Abort bool

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
func PercentPrase(times []any) [2]float32 {
	var start, end float32
	var secs [3]int
	for i, w := range times {
		v, ok := w.(string)
		if !ok {
			fmt.Fprintf(os.Stderr, "timestamps is in wrong format: %v\n", times)
			return [2]float32{0, 100}
		}
		time := strings.Split(v, ":")
		cons := 1
		for j := len(time) - 1; j >= 0; j-- {
			w, err := strconv.Atoi(time[j])
			if err != nil {
				fmt.Fprintf(os.Stderr, "timestamps is in wrong format: %v\n", times)
				return [2]float32{0, 100}
			}
			secs[i] += w * cons
			cons *= 60
		}
	}
	start = float32(secs[0]) / float32(secs[2]) * 100
	end = float32(secs[1]) / float32(secs[2]) * 100
	return [2]float32{start, end}
}

// Converts int in Seconds to a formated string
func FormatMinutes(num float32) string {
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
func FormatBytesPerSecond(num float32) string {
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

type HeaderType int

const (
	PLAYLIST_HEADER HeaderType = iota
	API_PLAYLIST_HEADER
	API_TOKEN_SWAP_HEADER
	HTML_HEADER
)

// Return Formated Headers, url needed only if i is 2
func FormatedHeader(refHeader map[string]string, videoUrl string, headerStyle HeaderType, token *string) (header map[string]string) {
	var xtoken string
	if token != nil {
		xtoken = *token
	}
	header = make(map[string]string)
	for k, v := range refHeader {
		header[k] = v
	}
	header["Accept-Language"] = "en-US,en;q=0.9"
	//header["Accept-Encoding"] = "gzip, deflate, br, zstd"
	header["Connection"] = "keep-alive"
	switch headerStyle {
	case HTML_HEADER:
		header["Accept"] = "*/*"
		header["X-Requested-With"] = "XMLHttpRequest"
		header["Sec-Fetch-Dest"] = "empty"
		header["Sec-Fetch-Mode"] = "cors"
		header["Sec-Fetch-Site"] = "same-origin"
		header["Priority"] = "u=0"
		header["TE"] = "trailers"
	case API_PLAYLIST_HEADER:
		header["Referer"] = videoUrl
		header["Sec-Fetch-Site"] = "same-origin"
		header["X-Requested-With"] = "XMLHttpRequest"
		header["X-Api-Token"] = xtoken
	case API_TOKEN_SWAP_HEADER:
		header["Accept"] = "*/*"
		header["X-Requested-With"] = "XMLHttpRequest"
		header["Content-Type"] = "application/x-www-form-urlencoded; charset=UTF-8"
		header["Origin"] = "https://recu.me"
		header["Sec-Fetch-Dest"] = "empty"
		header["Sec-Fetch-Mode"] = "cors"
		header["Sec-Fetch-Site"] = "same-origin"
		header["Priority"] = "u=4"
		header["X-Api-Token"] = xtoken
	default: // playlist
		delete(header, "Cookie")
		header["Accept"] = "*/*"
		header["Origin"] = "https://recu.me"
		header["Sec-Fetch-Dest"] = "empty"
		header["Sec-Fetch-Mode"] = "cors"
		header["Sec-Fetch-Site"] = "cross-site"
	}
	return
}
