package strava

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"stravach/app/storage/models"
)

type Client struct {
	ClientId     string
	ClientSecret string
}

type AuthReqBody struct {
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	GrantType    string `json:"grant_type"`
}

type AuthResp struct {
	RefreshToken string      `json:"refresh_token"`
	AccessToken  string      `json:"access_token"`
	Athlete      AthleteInfo `json:"athlete"`
	ExpiresAt    int64       `json:"expires_at"`
}

type AthleteInfo struct {
	Username string `json:"username"`
	Id       int64  `json:"id"`
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

const (
	authUrl       = "https://www.strava.com/oauth/token"
	activitiesUrl = "https://www.strava.com/api/v3/athlete/activities"
)

var Handler HTTPClient

func init() {
	Handler = &http.Client{}
}

func NewStravaClient() *Client {
	clientId := os.Getenv("STRAVA_CLIENT_ID")
	clientSecret := os.Getenv("STRAVA_CLIENT_SECRET")
	return &Client{
		ClientId:     clientId,
		ClientSecret: clientSecret,
	}
}

func (c *Client) Authorize(accessCode string) (*AuthResp, error) {
	ap := c.getAuthPayload(accessCode, "")
	return c.auth(ap)
}

func (c *Client) RefreshAccessToken(refreshToken string) (*AuthResp, error) {
	ap := c.getAuthPayload("", refreshToken)
	return c.auth(ap)
}

func GetActivity(accessToken string, activityId int64) (*models.UserActivity, error) {
	url := fmt.Sprintf("https://www.strava.com/api/v3/activities/%d?include_all_efforts=", activityId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Error("error occured during request creation")
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	var activity models.UserActivity
	resp, err := Handler.Do(req)
	if err != nil {
		slog.Error("error occured during request handling")
		return nil, err
	}
	err = json.NewDecoder(resp.Body).Decode(&activity)
	if err != nil {
		slog.Error("error occured during response decode handling")
		return nil, err
	}
	return &activity, nil
}

func GetAllActivities(accessToken string) (*[]models.UserActivity, error) {
	curPage := 1
	totalActivities := []models.UserActivity{}
	for {
		curActivities, err := getActivities(accessToken, curPage)
		if err != nil {
			slog.Error("error while fetching activities")
			return nil, err
		}
		if len(curActivities) == 0 {
			slog.Info("finished fetching activities")
			break
		}
		totalActivities = append(totalActivities, curActivities...)
		curPage++
	}
	return &totalActivities, nil
}

func getActivities(accessToken string, page int) ([]models.UserActivity, error) {
	url := fmt.Sprintf("%s?page=%d", activitiesUrl, page)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Error("error occured during request creation")
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	var activities []models.UserActivity
	resp, err := Handler.Do(req)
	if err != nil {
		slog.Error("error occured during request handling")
		return nil, err
	}
	err = json.NewDecoder(resp.Body).Decode(&activities)
	if err != nil {
		slog.Error("error occured during response decode handling")
		return nil, err
	}
	return activities, nil
}

func (c *Client) auth(authPayload AuthReqBody) (*AuthResp, error) {
	url := fmt.Sprintf("%s?client_id=%s&client_secret=%s&code=%s&refresh_token=%s&grant_type=%s", authUrl, authPayload.ClientId, authPayload.ClientSecret, authPayload.Code, authPayload.RefreshToken, authPayload.GrantType)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}
	var authResp AuthResp
	resp, err := Handler.Do(req)
	if err != nil {
		return nil, err
	}
	err = json.NewDecoder(resp.Body).Decode(&authResp)
	if err != nil {
		return nil, err
	}
	return &authResp, nil
}

func (c *Client) getAuthPayload(code string, refreshToken string) AuthReqBody {
	grantType := "authorization_code"
	if code == "" {
		grantType = "refresh_token"
	}
	return AuthReqBody{
		ClientId:     c.ClientId,
		ClientSecret: c.ClientSecret,
		RefreshToken: refreshToken,
		Code:         code,
		GrantType:    grantType,
	}
}
