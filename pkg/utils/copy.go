package utils

import (
	"encoding/json"
)

// CopyStructMatch tries to copy contents from a map[string]interface{} to an interface{}.
func CopyStructMatch(to interface{}, from map[string]interface{}) (err error) {
	str, err := json.Marshal(from)
	if err != nil {
		return err
	}
	json.Unmarshal(str, &to)
	return
}
