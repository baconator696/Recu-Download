package maintenance

import (
	"encoding/json"
	"fmt"
	"recurbate/tools"
	"strconv"
	"strings"
)

// Check for update
func CheckUpdate(currentTag string) (err error) {
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	respJson, status, err := tools.Request("https://api.github.com/repos/baconator696/Recu-Download/releases/latest", 2, nil, nil, "GET")
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
			fmt.Printf("%s\n%s\n", resp.(map[string]any)["html_url"].(string), tools.ANSIColor(resp.(map[string]any)["body"].(string), 2))
			return nil
		}
	}
	return nil
}
