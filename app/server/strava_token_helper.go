package server

import (
	"stravach/app/storage/models"
	"stravach/app/strava"
	"time"
)

// RefreshStravaTokenIfNeeded checks if the user's Strava token is expired and refreshes it if needed.
// If refreshed, updates the user object and returns true. Returns error if refresh fails.
func RefreshStravaTokenIfNeeded(stravaSvc interface{
	RefreshAccessToken(refreshToken string) (*strava.AuthResp, error)
}, db interface{
	UpdateUser(user *models.User) error
}, user *models.User) error {
	now := time.Now().Unix()
	if user.TokenExpiresAt == nil || (*user.TokenExpiresAt) < now {
		resp, err := stravaSvc.RefreshAccessToken(user.StravaRefreshToken)
		if err != nil {
			return err
		}
		user.StravaAccessToken = resp.AccessToken
		user.StravaRefreshToken = resp.RefreshToken
		user.TokenExpiresAt = &resp.ExpiresAt
		return db.UpdateUser(user)
	}
	return nil
}
