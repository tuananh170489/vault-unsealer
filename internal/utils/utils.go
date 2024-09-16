package utils

import (
	"fmt"
)

// ConvertListToMap converts a list of strings to a map of strings
func ConvertListToMap(keys []string) map[string]string {
	resultMap := make(map[string]string)
	for index, key := range keys {
		resultMap[fmt.Sprintf("key%d", index)] = key
	}
	return resultMap
}
