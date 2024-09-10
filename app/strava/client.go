package strava

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"stravach/app/storage/models"
	"stravach/app/utils"
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

type UpdatableActivity struct {
	Name string `json:"name"`
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

const (
	authUrl              = "https://www.strava.com/oauth/token"
	athleteActivitiesUrl = "https://www.strava.com/api/v3/athlete/activities"
	activityUrl          = "https://www.strava.com/api/v3/activities"
)

var Handler HTTPClient

func init() {
	Handler = &http.Client{}
}

type Strava interface {
	Authorize(accessCode string) (*AuthResp, error)
	RefreshAccessToken(refreshToken string) (*AuthResp, error)
	GetActivity(accessToken string, activityId int64) (*models.UserActivity, error)
}

var _ Strava = (*Client)(nil)

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

func (c *Client) GetActivity(accessToken string, activityId int64) (*models.UserActivity, error) {
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
	if resp.StatusCode >= 300 {
		slog.Error("got bad resp from strava", "status", resp.Status)
		utils.DebugResponse(resp)
		return nil, errors.New("bad response from strava")
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
	var totalActivities []models.UserActivity
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
		break
	}
	return &totalActivities, nil
}

func UpdateActivity(accessToken string, activity models.UserActivity) (*models.UserActivity, error) {
	url := fmt.Sprintf("%s/%d", activityUrl, activity.ID)
	updActivity := UpdatableActivity{Name: activity.Name}
	body, err := json.Marshal(updActivity)
	if err != nil {
		slog.Error("error while marshalling activity")
		return nil, err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := Handler.Do(req)
	if err != nil {
		slog.Error("error occurred during request creation")
		return nil, err
	}

	if resp.StatusCode >= 300 {
		utils.DebugResponse(resp)
		return nil, errors.New("received unsuccessful response")
	}

	var updatedActivity models.UserActivity
	err = json.NewDecoder(resp.Body).Decode(&updatedActivity)
	if err != nil {
		slog.Error("error occurred during response decode handling")
		return nil, err
	}
	return &activity, nil
}

func getActivities(accessToken string, page int) ([]models.UserActivity, error) {
	url := fmt.Sprintf("%s?page=%d", athleteActivitiesUrl, page)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Error("error occurred during request creation")
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	var activities []models.UserActivity
	resp, err := Handler.Do(req)
	if err != nil {
		slog.Error("error occurred during request handling")
		return nil, err
	}
	if resp != nil && resp.StatusCode == 401 {
		slog.Error("received unsuccessful response")
		return nil, errors.New(utils.UNAUTHORIZED)
	}
	err = json.NewDecoder(resp.Body).Decode(&activities)
	if err != nil {
		slog.Error("error occurred during response decode handling")
		return nil, err
	}
	return activities, nil
}

func (c *Client) auth(authPayload AuthReqBody) (*AuthResp, error) {
	url := fmt.Sprintf("%s?client_id=%s&client_secret=%s&code=%s&refresh_token=%s&grant_type=%s", authUrl, authPayload.ClientId, authPayload.ClientSecret, authPayload.Code, authPayload.RefreshToken, authPayload.GrantType)
	slog.Debug(url)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}
	var authResp AuthResp
	resp, err := Handler.Do(req)
	if err != nil {
		slog.Error("error while fetching auth request from strava")
		return nil, err
	}
	if resp.StatusCode >= 300 {
		utils.DebugResponse(resp)
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
