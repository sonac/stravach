package utils

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

func DebugResponse(resp *http.Response) {
	c := resp
	b, err := io.ReadAll(c.Body)
	if err != nil {
		slog.Error(err.Error())
	}
	slog.Debug(fmt.Sprintf("got response %s", string(b)))
}

func DebugRequest(resp *http.Request) {
	c := resp
	b, err := io.ReadAll(c.Body)
	if err != nil {
		slog.Error(err.Error())
	}
	slog.Debug(fmt.Sprintf("got response %s", string(b)))
}
