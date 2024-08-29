package server

import (
	"encoding/json"
	"fmt"
	"html/template"
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
	Strava            strava.Strava
	DB                storage.Store
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
	tmplPath := filepath.Join("templates", "index.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		slog.Error("error parsing template", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	err = tmpl.Execute(w, nil)
	if err != nil {
		slog.Error("error executing template", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *HttpHandler) activitiesPageHandler(w http.ResponseWriter, r *http.Request) {
	// Parse user ID from URL or session (assuming user ID is passed via URL)
	userIDStr := strings.TrimPrefix(r.URL.Path, "/user/")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		slog.Error("invalid user id", "error", err)
		http.Error(w, "Invalid User ID", http.StatusBadRequest)
		return
	}

	// Load the HTML template
	tmplPath := filepath.Join("templates", "activities.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		slog.Error("error parsing template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Execute the template with user ID
	data := struct {
		UserID int64
	}{
		UserID: userID,
	}
	err = tmpl.Execute(w, data)
	if err != nil {
		slog.Error("error executing template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
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
	stravaAccessCode := utils.GetCodeFromUrl(r.URL.RawQuery)
	slog.Info(fmt.Sprintf("Updating info for user: %d", chatId))
	if err != nil {
		slog.Error("error while parsing chatId from URL: " + r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, "Error occured during callback")
		return
	}

	usr, err := h.DB.GetUserByChatId(chatId)
	if err != nil {
		slog.Error("error while getting user from chatId", err)
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
	chatIdStr := strings.TrimPrefix(r.URL.Path, "/activities/")
	chatId, err := strconv.ParseInt(chatIdStr, 10, 64)
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

	userActivities, err := h.DB.GetUserActivities(usr.ID)
	if err != nil {
		slog.Error(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Generate the HTML for the activities grid
	var activitiesHTML strings.Builder
	for _, activity := range userActivities {
		activitiesHTML.WriteString(fmt.Sprintf(`
        <div class="card">
            <h3>%s</h3>
            <p><strong>Distance:</strong> %.2f km</p>
            <p><strong>Moving Time:</strong> %d mins</p>
            <p><strong>Elapsed Time:</strong> %d mins</p>
            <p><strong>Type:</strong> %s</p>
            <p><strong>Start Date:</strong> %s</p>
            <p><strong>Avg Heartrate:</strong> %.2f bpm</p>
            <p><strong>Avg Speed:</strong> %.2f km/h</p>
            <button hx-post="/activity/%d" hx-swap="none">Update Activity</button>
        </div>`,
			activity.Name,
			activity.Distance/1000,  // Convert to kilometers
			activity.MovingTime/60,  // Convert to minutes
			activity.ElapsedTime/60, // Convert to minutes
			activity.ActivityType,
			activity.StartDate.Format("2006-01-02 15:04:05"),
			activity.AverageHeartrate,
			activity.AverageSpeed*3.6, // Convert to km/h
			activity.ID))
	}

	// Write the generated HTML to the response
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(activitiesHTML.String()))
	w.WriteHeader(http.StatusOK)
}

func (h *HttpHandler) updateActivity(w http.ResponseWriter, r *http.Request) {
	slog.Debug("got updateActivity request")
	activityIdStr := strings.TrimPrefix(r.URL.Path, "/activity/")
	activityId, err := strconv.ParseInt(activityIdStr, 10, 64)
	if err != nil {
		slog.Error("invalid activity id", "error", err)
		http.Error(w, "Invalid Activity ID", http.StatusBadRequest)
		return
	}

	// Fetch the activity from the database by its ID
	activity, err := h.DB.GetActivityById(activityId)
	if err != nil {
		slog.Error("failed to fetch activity", "error", err)
		http.Error(w, "Failed to fetch activity", http.StatusInternalServerError)
		return
	}

	// Fetch the user associated with the activity
	usr, err := h.DB.GetUserById(activity.UserID)
	if err != nil || usr == nil {
		slog.Error("error while getting user from DB", err)
		http.Error(w, "Failed to fetch user", http.StatusInternalServerError)
		return
	}

	afu := tg.ActivityForUpdate{
		Activity: *activity,
		ChatId:   usr.TelegramChatId,
	}

	h.ActivitiesChannel <- afu
	slog.Info("activity sent to channel", "activityId", activity.ID)

	// Respond with a success message
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Activity sent to the channel successfully"))
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
		activity, _ = h.DB.GetActivityById(activityId)
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
		}
	}

	fmt.Printf("%+v", activity)

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
	http.HandleFunc("/", h.homeHandler)
	http.HandleFunc("/auth/", h.authHandler)
	http.HandleFunc("/auth-callback/", h.authCallbackHandler)
	http.HandleFunc("/activities/", h.getActivities)
	http.HandleFunc("/user/", h.activitiesPageHandler)
	http.HandleFunc("/activity/", h.updateActivity)
	http.HandleFunc("/webhook", h.webhook)

	slog.Info("Starting server on port " + h.Port)
	err := http.ListenAndServe(":"+h.Port, nil)
	if err != nil {
		slog.Error("wasn't able to start the server")
		panic(err)
	}
}
