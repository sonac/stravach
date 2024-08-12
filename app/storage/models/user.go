package models

import (
	"time"
)

type User struct {
	ID                 int64  `json:"id,omitempty"`
	StravaId           int64  `json:"strava_id"`
	TelegramChatId     int64  `json:"telegram_chat_id"`
	Username           string `json:"username"`
	Email              string `json:"email"`
	StravaRefreshToken string `json:"strava_refresh_token"`
	StravaAccessToken  string `json:"strava_access_token"`
	StravaAccessCode   string `json:"strava_access_code"`
	TokenExpiresAt     *int64 `json:"token_expires_at"`
}

func (u User) AuthRequired() bool {
	now := time.Now().Unix()
	if u.TokenExpiresAt != nil && (*u.TokenExpiresAt) > now {
		return false
	}
	return true
}

type UserActivity struct {
	ID               int64     `json:"id,omitempty"`
	UserID           int64     `json:"user_id"`
	Name             string    `json:"name"`
	Distance         float64   `json:"distance"`
	MovingTime       int64     `json:"moving_time"`
	ElapsedTime      int64     `json:"elapsed_time"`
	ActivityType     string    `json:"type"`
	StartDate        time.Time `json:"start_date"`
	AverageHeartrate float64   `json:"average_heartrate"`
	AverageSpeed     float64   `json:"average_speed"`
	IsUpdated        bool      `json:"is_updated"`
}
