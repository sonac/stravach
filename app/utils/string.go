package utils

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

func GetCodeFromUrl(url string) string {
	slog.Debug(url)
	re := regexp.MustCompile("code=[a-z0-9]*")
	codeString := re.FindString(url)
	slog.Debug(codeString)
	return codeString[5:]
}

func FormatActivityNames(activityNames []string) []string {
	var formattedList []string

	// Regular expression to remove leading numbers, punctuation, and whitespace
	re := regexp.MustCompile(`^\s*[\d\.\-\)\(\s]*`)

	for i, str := range activityNames {
		// Clean the string by removing leading numbers, dashes, or other characters
		cleanedStr := re.ReplaceAllString(str, "")
		// Format the string with the proper index and append it to the formatted list
		formattedStr := fmt.Sprintf("%d. %s", i+1, strings.TrimSpace(cleanedStr))
		formattedList = append(formattedList, formattedStr)
	}

	return formattedList
}
