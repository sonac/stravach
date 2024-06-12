package main

import (
	"log/slog"
	"os"
	"os/signal"
	"stravach/app/server"
	"syscall"

	"github.com/joho/godotenv"
)

var srv *server.HttpHandler

func main() {
	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go srv.Start()

	slog.Info("press CTRL+C to stop programm\n")
	<-sigCh
	slog.Info("Shutting down\n")
	os.Exit(0)
}

func init() {
	err := godotenv.Load()
	if err != nil {
		slog.Error("error while initializing godotenv")
		panic(err)
	}
	srv = &server.HttpHandler{}
	srv.Init()
}
