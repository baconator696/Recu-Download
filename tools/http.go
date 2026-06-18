package tools

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Returns the raw data from the URL
func Request(url string, timeout int, header map[string]string, body []byte, Type string) (data []byte, statusCode int, err error) {
	req, err := http.NewRequest(Type, url, strings.NewReader(string(body)))
	if err != nil {
		err = fmt.Errorf("http.NewRequest:%v", err)
		return
	}
	for key, value := range header {
		req.Header.Set(key, value)
	}
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("client.Do:%v", err)
		return
	}
	statusCode = resp.StatusCode
	defer resp.Body.Close()
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("io.ReadAll:%v", err)
	}
	return
}
func RequestRetry(url string, retry int, retryState chan error,
	successStatusCode []int, timeoutFailIncreaseInterval int, failSleepTime time.Duration,
	timeout int, header map[string]string, body []byte, Type string) (data []byte, statusCode int, err error,
) {
	if retryState != nil {
		defer close(retryState)
	}
	retry_count := 0
outerLoop:
	for {
		data, statusCode, err = Request(url, timeout, header, body, Type)
		if err == nil {
			for _, successStatusCode := range successStatusCode {
				if successStatusCode == statusCode {
					break outerLoop
				}
			}
		}
		if err == nil {
			err = fmt.Errorf("%s, status code: %d", ANSIColor(ShortenString(string(data), 100), 2), statusCode)
		}
		if retryState != nil {
			retryState <- err
		}
		if retry_count > retry {
			return
		}
		retry_count++
		timeout += timeoutFailIncreaseInterval
		time.Sleep(failSleepTime)
	}
	return
}
