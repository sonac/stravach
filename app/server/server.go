package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"stravach/app/openai"
	"stravach/app/storage"
	"stravach/app/storage/models"
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
	TgApiKey          string
	StaticDir         string
	Strava            strava.StravaService
	DB                storage.Store
	AI                *openai.OpenAI
	ActivitiesChannel chan tg.ActivityForUpdate
	BroadcastChannel  chan tg.BroadcastMessage
	JWT               *utils.JWT
}

type UpdateActivityRequest struct {
	ID         int    `json:"id"`
	UpdateType string `json:"updateType"`
}

type TgUser struct {
	Id        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type TgPayload struct {
	User TgUser `json:"user"`
}

func (h *HttpHandler) Init() {
	h.StravaToken = os.Getenv("STRAVA_CHALLENGE_TOKEN")
	h.Port = os.Getenv("PORT")
	h.Url = os.Getenv("URL")
	h.TgApiKey = os.Getenv("TELEGRAM_API_KEY")
	h.Strava = strava.NewStravaClient()
	h.DB = &storage.SQLiteStore{}
	h.AI = openai.NewClient()
	h.JWT = &utils.JWT{Key: []byte(os.Getenv("JWT_KEY"))}
	h.StaticDir = "./client/dist"
	err := h.DB.Connect()
	if err != nil {
		slog.Error("error while connecting to DB")
		panic(err)
	}
}

// activitiesPageHandler is deprecated. All frontend routing is now handled by React SPA.
// func (h *HttpHandler) activitiesPageHandler(w http.ResponseWriter, r *http.Request) {}

// getTelegramClient returns the Telegram client from the handler (if available)
func (h *HttpHandler) getTelegramClient() (*tg.Telegram, bool) {
	if h.TgApiKey == "" {
		return nil, false
	}
	tgClient, err := tg.NewTelegramClient(h.TgApiKey)
	if err != nil {
		return nil, false
	}
	return tgClient, true
}

// broadcastHandler allows an admin to send a message to all users via Telegram
func (h *HttpHandler) broadcastHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"error": "POST required"}`))
		return
	}
	cookie, err := r.Cookie("auth_token")
	if err != nil {
		slog.Debug(fmt.Sprintf("error while getting cookie: %s, request is: %+v", err, r))
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "missing auth_token"}`))
		return
	}
	userIdPtr, err := h.JWT.GetChatIdFromToken(cookie.Value)
	if err != nil || userIdPtr == nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid token"}`))
		return
	}
	usr, err := h.DB.GetUserById(*userIdPtr)
	if err != nil || usr == nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "user not found"}`))
		return
	}
	if !usr.IsAdmin {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": "admin only"}`))
		return
	}
	var req struct {
		Message string `json:"message"`
	}
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.Message == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "invalid message"}`))
		return
	}

	if h.BroadcastChannel == nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "broadcast channel unavailable"}`))
		return
	}
	h.BroadcastChannel <- tg.BroadcastMessage{Text: req.Message}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"sent": "queued"}`))
}

// userInfoHandler returns the current user's info as JSON, including is_admin
func (h *HttpHandler) userInfoHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("auth_token")
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "missing auth_token"}`))
		return
	}
	userIdPtr, err := h.JWT.GetChatIdFromToken(cookie.Value)
	if err != nil || userIdPtr == nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid token"}`))
		return
	}
	usr, err := h.DB.GetUserById(*userIdPtr)
	if err != nil || usr == nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "user not found"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(usr)
}

// refreshLast10ActivitiesHandler refreshes the last 10 activities from Strava for a user and saves them to the DB.
func (h *HttpHandler) refreshLast10ActivitiesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("Method not allowed"))
		return
	}
	usrChatIdStr := strings.TrimPrefix(r.URL.Path, "/api/activities-refresh-last-10/")
	usrChatId, err := strconv.ParseInt(usrChatIdStr, 10, 64)
	if err != nil {
		slog.Error("invalid user id", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Invalid user id"))
		return
	}
	usr, err := h.DB.GetUserByChatId(usrChatId)
	if err != nil || usr == nil {
		slog.Error("failed to get user", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Failed to get user"))
		return
	}
	// Refresh token if needed
	err = RefreshStravaTokenIfNeeded(h.Strava, h.DB, usr)
	if err != nil {
		slog.Error("failed to refresh Strava token", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Failed to refresh Strava token"))
		return
	}
	activities, err := h.Strava.GetLatestActivities(usr.StravaAccessToken, 10)
	if err != nil {
		slog.Error("failed to fetch activities from Strava", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Failed to fetch activities from Strava"))
		return
	}
	if len(activities) == 0 {
		slog.Info("No new activities found from Strava", "userID", usr.ID)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("No new activities found"))
		return
	}
	var activityPtrs []*models.UserActivity
	for i := range activities {
		activities[i].UserID = usr.ID
		activityPtrs = append(activityPtrs, &activities[i])
	}
	err = h.DB.CreateUserActivities(activityPtrs)
	if err != nil {
		slog.Error("failed to save activities to DB", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Failed to save activities to DB"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Last 10 activities refreshed successfully"))
}

func (h *HttpHandler) authHandler(w http.ResponseWriter, r *http.Request) {
	chatId := strings.Split(r.URL.Path, "/")[3] // /api/auth/:chatId
	redirectUrl := fmt.Sprintf("https://www.strava.com/oauth/authorize?client_id=37166&response_type=code&"+
		"redirect_uri=%s/api/auth-callback/%s&approval_prompt=force&scope=read_all,activity:write,activity:read_all", h.Url, chatId)
	http.Redirect(w, r, redirectUrl, http.StatusTemporaryRedirect)
}

func (h *HttpHandler) authCallbackHandler(w http.ResponseWriter, r *http.Request) {
	// Expect path: /api/auth-callback/:chatId
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, "Invalid callback URL")
		return
	}
	chatId, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		slog.Error("error while parsing chatId from URL: " + r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, "Error occured during callback")
		return
	}
	stravaAccessCode := utils.GetCodeFromUrl(r.URL.RawQuery)
	slog.Info(fmt.Sprintf("Updating info for user: %d", chatId))

	usr, err := h.DB.GetUserByChatId(chatId)
	if err != nil {
		slog.Error("error while getting user from chatId", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, "Error occured during callback")
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
	usr.StravaId = &authData.Athlete.Id
	slog.Debug("updating user in auth callback")
	err = h.DB.UpdateUser(usr)
	if err != nil {
		slog.Error(fmt.Sprintf("error while updating user from chatId %s", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := `<!DOCTYPE html>
		<html lang="en">
		<head><meta charset="UTF-8"><title>Auth Success</title></head>
		<body>
		<h2>Authentication successful!</h2>
		<p>You may now close this window.</p>
		</body>
		</html>`
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(resp))
	if err != nil {
		slog.Error("error while writing to response", "err", err)
		return
	}
}

func (h *HttpHandler) tgAuthHandler(w http.ResponseWriter, r *http.Request) {
	slog.Debug("got tg auth request")
	if r.Method != http.MethodPost {
		slog.Error("invalid method", "method", r.Method)
		utils.DebugRequest(r)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Localhost/dev shortcut: if request is from localhost, log in user ID 1
	isLocal := func() bool {
		host := r.Host
		remote := r.RemoteAddr
		origin := r.Header.Get("Origin")
		referer := r.Header.Get("Referer")
		for _, v := range []string{host, remote, origin, referer} {
			if strings.Contains(v, "localhost") || strings.Contains(v, "127.0.0.1") {
				return true
			}
		}
		return false
	}()

	if isLocal {
		slog.Info("Local dev login")
		usr, err := h.DB.GetUserById(1)
		if err != nil || usr == nil {
			slog.Error("Local dev login failed: user 1 not found", "err", err)
			http.Error(w, "Local dev login failed: user 1 not found", http.StatusInternalServerError)
			return
		}
		token, err := h.JWT.GenerateJWTForUser(usr.ID)
		if err != nil {
			slog.Error("error generating JWT token", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:    "auth_token",
			Value:   token.Value,
			Expires: token.ExpiresAt,
			Path:    "/",
		})
		w.WriteHeader(http.StatusOK)
		return
	}

	var payload TgPayload
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		slog.Error("error decoding request body", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	slog.Debug("got payload", "%+v", payload.User.Id)
	usr, err := h.DB.GetUserByChatId(payload.User.Id)
	if err != nil {
		slog.Error("error fetching user from database", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if usr == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	token, err := h.JWT.GenerateJWTForUser(usr.ID)
	if err != nil {
		slog.Error("error generating JWT token", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:    "auth_token",
		Value:   token.Value,
		Expires: token.ExpiresAt,
		Path:    "/",
	})

	w.WriteHeader(http.StatusOK)
}

func (h *HttpHandler) getActivities(w http.ResponseWriter, r *http.Request) {
	slog.Debug("got getActivities request")
	usrChatIdStr := strings.TrimPrefix(r.URL.Path, "/api/activities/")
	usrChatId, err := strconv.ParseInt(usrChatIdStr, 10, 64)
	if err != nil {
		slog.Error(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	usr, err := h.DB.GetUserByChatId(usrChatId)
	if err != nil {
		slog.Error(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	userActivities, err := h.DB.GetUserActivities(usr.ID, 30)
	if err != nil {
		slog.Error(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(userActivities); err != nil {
		slog.Error(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (h *HttpHandler) updateActivity(w http.ResponseWriter, r *http.Request) {
	slog.Debug("got updateActivity request")
	activityIdStr := strings.TrimPrefix(r.URL.Path, "/api/activity/")
	activityId, err := strconv.ParseInt(activityIdStr, 10, 64)
	if err != nil {
		slog.Error("invalid activity id", "error", err)
		http.Error(w, "Invalid Activity ID", http.StatusBadRequest)
		return
	}

	activity, err := h.DB.GetActivityById(activityId)
	if err != nil {
		slog.Error("failed to fetch activity", "error", err)
		http.Error(w, "Failed to fetch activity", http.StatusInternalServerError)
		return
	}

	usr, err := h.DB.GetUserById(activity.UserID)
	if err != nil || usr == nil {
		slog.Error("error while getting user from DB", "err", err)
		http.Error(w, "Failed to fetch user", http.StatusInternalServerError)
		return
	}

	afu := tg.ActivityForUpdate{
		Activity: *activity,
		ChatId:   usr.TelegramChatId,
	}

	h.ActivitiesChannel <- afu
	slog.Info("activity sent to channel", "activityId", activity.ID)

	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte("Activity sent to the channel successfully"))
	if err != nil {
		slog.Error("error while writing to response", "err", err)
		return
	}
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
			_, err = w.Write(jBytes)
			if err != nil {
				slog.Error("error while writing to response", "err", err)
				return
			}
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
	err = h.processActivity(wBody.ObjectId, usr)
	if err != nil {
		slog.Error("error while processing activity", "err", err)
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

func (h *HttpHandler) processActivity(activityId int64, user *models.User) error {
	var activity *models.UserActivity
	exists, err := h.DB.IsActivityExists(activityId)

	if err != nil {
		return err
	}

	if exists {
		slog.Info("activity exists already, probably just got updated")
		activity, err = h.DB.GetActivityById(activityId)
		if err != nil {
			return err
		}
	} else {
		if user.AuthRequired() {
			authData, err := h.Strava.RefreshAccessToken(user.StravaRefreshToken)
			if err != nil {
				return err
			}
			user.StravaAccessToken = authData.AccessToken
		}

		activity, err = h.Strava.GetActivity(user.StravaAccessToken, activityId)
		if err != nil {
			return err
		}
		err = h.DB.CreateUserActivity(activity, user.ID)
		if err != nil {
			return err
		}
		slog.Debug("updating user in webhook")
		err = h.DB.UpdateUser(user)
		if err != nil {
			slog.Error("error while updating user", "err", err)
			return err
		}
	}

	slog.Debug(fmt.Sprintf("%+v", activity))

	if activity != nil && !activity.IsUpdated {
		afu := tg.ActivityForUpdate{
			Activity: *activity,
			ChatId:   user.TelegramChatId,
		}

		h.ActivitiesChannel <- afu
	}

	return nil
}

func (h *HttpHandler) Start() {
	http.HandleFunc("/api/broadcast", h.broadcastHandler)
	http.HandleFunc("/api/user-info", h.userInfoHandler)
	// API routes
	http.HandleFunc("/api/activities/", h.getActivities)
	http.HandleFunc("/api/activities-refresh-last-10/", h.refreshLast10ActivitiesHandler) // NEW
	http.HandleFunc("/api/activity/", h.updateActivity)
	http.HandleFunc("/api/auth/", h.authHandler)
	http.HandleFunc("/api/auth-callback/", h.authCallbackHandler)
	http.HandleFunc("/api/tg-auth", h.tgAuthHandler)
	http.HandleFunc("/api/webhook", h.webhook)
	// tmp solution, can't delete old webhook subscription
	http.HandleFunc("/webhook", h.webhook)

	fs := http.FileServer(http.Dir(h.StaticDir))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Serve static files if they exist
		path := filepath.Join(h.StaticDir, r.URL.Path)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		// Otherwise, serve index.html for React Router
		http.ServeFile(w, r, filepath.Join(h.StaticDir, "index.html"))
	})

	slog.Info("Starting server on port " + h.Port)
	err := http.ListenAndServe(":"+h.Port, nil)
	if err != nil {
		slog.Error("wasn't able to start the server")
		panic(err)
	}
}
