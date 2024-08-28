package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"stravach/app/server"
	"stravach/app/tg"
	"syscall"

	"github.com/joho/godotenv"
)

var (
	srv      *server.HttpHandler
	telegram *tg.Telegram
	env      string
)

func main() {
	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	ctx := context.Background()
	go srv.Start()
	if env != "DEV" {
		// do not start telegram on dev
		go telegram.Start(ctx)
	}

	slog.Info("press CTRL+C to stop program\n")
	<-sigCh
	slog.Info("Shutting down\n")
	os.Exit(0)
}

func init() {
	err := godotenv.Load()
	env = os.Getenv("ENV")
	if err != nil && env != "PROD" {
		slog.Error("error while initializing godotenv")
		os.Exit(1)
	}
	srv = &server.HttpHandler{}
	slog.SetLogLoggerLevel(slog.LevelDebug.Level())

	tgApiKey := os.Getenv("TELEGRAM_API_KEY")
	telegram, err = tg.NewTelegramClient(tgApiKey)
	if err != nil {
		slog.Error("error while initializing telegram")
		panic(err)
	}

	activitiesChannel := make(chan tg.ActivityForUpdate)
	srv.ActivitiesChannel = activitiesChannel
	telegram.ActivitiesChannel = activitiesChannel

	srv.Init()
}
