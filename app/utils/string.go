package utils

import (
	"log/slog"
	"regexp"
)

func GetCodeFromUrl(url string) string {
	slog.Debug(url)
	re := regexp.MustCompile("code=[a-z0-9]")
	codeString := re.FindString(url)
	return codeString[5:]
}
