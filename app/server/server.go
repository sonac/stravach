package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"stravach/app/openai"
	"stravach/app/storage"
	"stravach/app/strava"
	"stravach/app/tg"
	"stravach/app/utils"
	"strconv"
	"strings"
)

type HttpHandler struct {
	Url               string
	Port              string
	StravaToken       string
	Strava            *strava.Client
	DB                *storage.SQLiteStore
	AI                *openai.OpenAI
	ActivitiesChannel chan tg.ActivityForUpdate
}

type UpdateActivityRequest struct {
	ID         int    `json:"id"`
	UpdateType string `json:"updateType"`
}

func (h *HttpHandler) Init() {
	h.StravaToken = os.Getenv("STRAVA_CHALLENGE_TOKEN")
	h.Port = os.Getenv("PORT")
	h.Url = os.Getenv("URL")
	h.Strava = strava.NewStravaClient()
	h.DB = &storage.SQLiteStore{}
	h.AI = openai.NewClient()
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
		"redirect_uri=%s/auth-callback/%s&approval_prompt=force&scope=read_all,activity:write,activity:read_all", h.Url, chatId)
	http.Redirect(w, r, redirectUrl, http.StatusTemporaryRedirect)
}

func (h *HttpHandler) authCallbackHandler(w http.ResponseWriter, r *http.Request) {
	chatId, err := strconv.ParseInt(strings.Split(r.URL.Path, "/")[2], 10, 64)
	slog.Info(fmt.Sprintf("Updating info for user: %d", chatId))
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
	authData, err := h.Strava.Authorize(usr.StravaAccessCode)
	if err != nil {
		slog.Error("error while authorizing new user", "err", err.Error())
		return
	}
	usr.StravaAccessToken = authData.AccessToken
	usr.StravaRefreshToken = authData.RefreshToken
	usr.StravaId = authData.Athlete.Id
	slog.Debug("updating user in auth callback")
	err = h.DB.UpdateUser(usr)
	if err != nil {
		slog.Error(fmt.Sprintf("error while updating user from chatId %s", err))
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error occured during callback")
		return
	}
}

func (h *HttpHandler) getActivities(w http.ResponseWriter, r *http.Request) {
	chatId, err := strconv.ParseInt(strings.Split(r.URL.Path, "/")[2], 10, 64)
	if err != nil {
		slog.Error(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	usr, err := h.DB.GetUserByChatId(chatId)
	if err != nil {
		slog.Error(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	userActivities, err := h.DB.GetuserActivities(usr.ID)
	if err != nil {
		slog.Error(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	jBytes, err := json.Marshal(userActivities)
	if err != nil {
		slog.Error(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jBytes)
	w.WriteHeader(http.StatusOK)
}

func (h *HttpHandler) updateActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateActivityRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		slog.Error("failed to decode request body", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	activity, err := h.DB.GetActivityById(int64(req.ID))
	if err != nil {
		slog.Error("failed to fetch activity", "error", err, "activityId", req.ID)
		http.Error(w, "Failed to fetch activity", http.StatusInternalServerError)
		return
	}

	usr, err := h.DB.GetUserById(activity.UserID)
	if err != nil || usr == nil {
		slog.Error("error while getting user from DB", err)
		http.Error(w, "Failed to fetch user", http.StatusInternalServerError)
		return
	}
	switch req.UpdateType {
	case "NAME":
		afu := tg.ActivityForUpdate{
			Activity: *activity,
			ChatId:   usr.TelegramChatId,
		}
		h.ActivitiesChannel <- afu
	default:
		http.Error(w, "invalid update type", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Activity updated successfully"))
}

func (h *HttpHandler) webhookVerify(w http.ResponseWriter, r *http.Request) {
	vals := r.URL.Query()
	mode := vals.Get("hub.mode")
	token := vals.Get("hub.verify_token")
	challenge := vals.Get("hub.challenge")

	if mode != "" && token != "" {
		if mode == "subscribe" && token == h.StravaToken {
			slog.Info("webhook verified")
			jBytes, err := json.Marshal(map[string]string{"hub.challenge": challenge})
			if err != nil {
				slog.Error("error while marshalling challenge response", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			slog.Info("challenge completed")
			w.Write(jBytes)
			return
		}
	}
	w.WriteHeader(http.StatusForbidden)
}

func (h *HttpHandler) webhookActivity(w http.ResponseWriter, r *http.Request) {
	type webhookBody struct {
		ObjectId int64 `json:"object_id"`
		OwnerId  int64 `json:"owner_id"`
	}
	var wBody webhookBody
	err := json.NewDecoder(r.Body).Decode(&wBody)
	if err != nil {
		slog.Error("error while reading request body", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	slog.Info("got webhookBody" + fmt.Sprintf("%+v", wBody))
	usr, err := h.DB.GetUserByStravaId(wBody.OwnerId)
	if err != nil {
		slog.Error(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if usr.AuthRequired() {
		authData, err := h.Strava.RefreshAccessToken(usr.StravaRefreshToken)
		if err != nil {
			slog.Error(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		usr.StravaAccessToken = authData.AccessToken
	}

	activity, err := strava.GetActivity(usr.StravaAccessToken, wBody.ObjectId)
	if err != nil {
		slog.Error(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	slog.Info("got webhookBody" + fmt.Sprintf("%+v", activity))

	existingActivity, _ := h.DB.GetActivityById(activity.ID)

	if existingActivity != nil {
		slog.Info("activity exists already, probably just got updated")
	} else {
		err = h.DB.CreateUserActivity(activity, usr.ID)
		if err != nil {
			slog.Error("error while adding activity", "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	slog.Debug("updating user in webhook")
	err = h.DB.UpdateUser(usr)
	if err != nil {
		slog.Error("error while updating user", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	slog.Info("activity added for user" + usr.Username + " with id: " + strconv.FormatInt(activity.ID, 10))

	dbActivity, err := h.DB.GetActivityById(activity.ID)
	if err != nil {
		slog.Error("error when fetching activity from DB", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !dbActivity.IsUpdated {
		afu := tg.ActivityForUpdate{
			Activity: *dbActivity,
			ChatId:   usr.TelegramChatId,
		}

		h.ActivitiesChannel <- afu
	}
}

func (h *HttpHandler) webhook(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.webhookVerify(w, r)
	case http.MethodPost:
		h.webhookActivity(w, r)
	default:
		slog.Error("method is not supported")
	}
}

func (h *HttpHandler) Start() {
	http.HandleFunc("/", h.homeHandler)
	http.HandleFunc("/auth/", h.authHandler)
	http.HandleFunc("/auth-callback/", h.authCallbackHandler)
	http.HandleFunc("/activities/", h.getActivities)
	http.HandleFunc("/activity", h.updateActivity)
	http.HandleFunc("/webhook", h.webhook)

	slog.Info("Starting server on port " + h.Port)
	err := http.ListenAndServe(":"+h.Port, nil)
	if err != nil {
		slog.Error("wasn't able to start the server")
		panic(err)
	}
}
