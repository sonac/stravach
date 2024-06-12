package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"stravach/app/storage"
	"stravach/app/strava"
	"stravach/app/utils"
	"strconv"
	"strings"
)

type HttpHandler struct {
	Url         string
	Port        string
	StravaToken string
	Strava      *strava.Client
	DB          *storage.SQLiteStore
}

func (h *HttpHandler) Init() {
	h.StravaToken = os.Getenv("STRAVA_CHALLENGE_TOKEN")
	h.Port = os.Getenv("PORT")
	h.Url = os.Getenv("URL")
	h.Strava = strava.NewStravaClient()
	h.DB = &storage.SQLiteStore{}
	err := h.DB.Connect()
	if err != nil {
		slog.Error("error while connecting to DB")
		panic(err)
	}
}

func (h *HttpHandler) homeHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := fmt.Fprintf(w, "Welcome!")
	if err != nil {
		slog.Error("error during home response")
	}
}

func (h *HttpHandler) authHandler(w http.ResponseWriter, r *http.Request) {
	chatId := strings.Split(r.URL.Path, "/")[2]
	redirectUrl := fmt.Sprintf("https://www.strava.com/oauth/authorize?client_id=37166&response_type=code&"+
		"redirect_uri=%s/auth-callback/%s&approval_prompt=force&scope=read_all,activity:read", h.Url, chatId)
	http.Redirect(w, r, redirectUrl, http.StatusTemporaryRedirect)
}

func (h *HttpHandler) authCallbackHandler(w http.ResponseWriter, r *http.Request) {
	chatId, err := strconv.ParseInt(strings.Split(r.URL.Path, "/")[2], 10, 64)
	if err != nil {
		slog.Error("error while parsing chatId from URL: " + r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error occured during callback")
		return
	}
	stravaAccessCode := utils.GetCodeFromUrl(r.URL.RawQuery)
	usr, err := h.DB.GetUserByChatId(chatId)
	if err != nil {
		slog.Error("error while getting user from chatId", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error occured during callback")
		return
	}
	usr.StravaAccessCode = stravaAccessCode
	_ = h.DB.UpdateUser(usr)
}

func (h *HttpHandler) Start() {
	http.HandleFunc("/", h.homeHandler)

	slog.Info("Starting server on port " + h.Port)
	err := http.ListenAndServe(":"+h.Port, nil)
	if err != nil {
		slog.Error("wasn't able to start the server")
		panic(err)
	}
}
