package server

import (
	"errors"
	"stravach/app/storage/models"
	"stravach/app/strava"
	"stravach/app/tg"
	"stravach/mocks"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestProcessActivity_ActivityExists(t *testing.T) {
	mockDB := new(mocks.Store)
	mockStrava := &mocks.StravaService{}
	activitiesChannel := make(chan tg.ActivityForUpdate, 1)

	h := &HttpHandler{
		DB:                mockDB,
		Strava:            mockStrava,
		ActivitiesChannel: activitiesChannel,
	}

	existingActivity := &models.UserActivity{ID: 123, IsUpdated: true}
	mockDB.On("IsActivityExists", int64(123)).Return(true, nil)
	mockDB.On("GetActivityById", int64(123)).Return(existingActivity, nil)

	user := &models.User{StravaAccessToken: "access-token"}

	err := h.processActivity(123, user)
	assert.NoError(t, err)
	assert.Empty(t, activitiesChannel)

	mockDB.AssertExpectations(t)
	mockStrava.AssertExpectations(t)
}

func TestProcessActivity_NewActivity(t *testing.T) {
	mockDB := new(mocks.Store)
	mockStrava := &mocks.StravaService{}
	activitiesChannel := make(chan tg.ActivityForUpdate, 1)

	h := &HttpHandler{
		DB:                mockDB,
		Strava:            mockStrava,
		ActivitiesChannel: activitiesChannel,
	}

	mockDB.On("IsActivityExists", int64(123)).Return(false, nil)
	mockStrava.On("GetActivity", "access-token", int64(123)).Return(&models.UserActivity{ID: 123}, nil)
	mockStrava.On("RefreshAccessToken", "refresh-token").Return(&strava.AuthResp{AccessToken: "access-token"}, nil)
	mockDB.On("CreateUserActivity", &models.UserActivity{ID: 123}, int64(1)).Return(nil)
	mockDB.On("UpdateUser", mock.Anything).Return(nil)

	user := &models.User{ID: 1, StravaAccessToken: "access-token", StravaRefreshToken: "refresh-token", TelegramChatId: 456}

	err := h.processActivity(123, user)
	afu := <-activitiesChannel
	assert.NoError(t, err)

	assert.Equal(t, int64(123), afu.Activity.ID)
	assert.Equal(t, int64(456), afu.ChatId)

	mockDB.AssertExpectations(t)
	mockStrava.AssertExpectations(t)
}

func TestProcessActivity_UserAuthRequired(t *testing.T) {
	mockDB := new(mocks.Store)
	mockStrava := &mocks.StravaService{}
	activitiesChannel := make(chan tg.ActivityForUpdate, 1)

	h := &HttpHandler{
		DB:                mockDB,
		Strava:            mockStrava,
		ActivitiesChannel: activitiesChannel,
	}

	mockDB.On("IsActivityExists", int64(123)).Return(false, nil)
	mockStrava.On("RefreshAccessToken", "refresh-token").Return(&strava.AuthResp{AccessToken: "new-access-token"}, nil)
	mockStrava.On("GetActivity", "new-access-token", int64(123)).Return(&models.UserActivity{ID: 123}, nil)
	mockDB.On("CreateUserActivity", &models.UserActivity{ID: 123}, int64(1)).Return(nil)
	mockDB.On("UpdateUser", mock.Anything).Return(nil)

	user := &models.User{
		ID:                 1,
		StravaRefreshToken: "refresh-token",
		TelegramChatId:     456,
	}

	err := h.processActivity(123, user)
	assert.NoError(t, err)
	assert.NotEmpty(t, activitiesChannel)

	afu := <-activitiesChannel
	assert.Equal(t, int64(123), afu.Activity.ID)
	assert.Equal(t, int64(456), afu.ChatId)

	mockDB.AssertExpectations(t)
	mockStrava.AssertExpectations(t)
}

func TestProcessActivity_StravaFetchError(t *testing.T) {
	mockDB := new(mocks.Store)
	mockStrava := &mocks.StravaService{}
	activitiesChannel := make(chan tg.ActivityForUpdate, 1)

	h := &HttpHandler{
		DB:                mockDB,
		Strava:            mockStrava,
		ActivitiesChannel: activitiesChannel,
	}

	mockDB.On("IsActivityExists", int64(123)).Return(false, nil)
	mockStrava.On("RefreshAccessToken", "refresh-token").Return(&strava.AuthResp{AccessToken: "access-token"}, nil)
	mockStrava.On("GetActivity", "access-token", int64(123)).Return(&models.UserActivity{}, errors.New("strava error"))

	user := &models.User{ID: 1, StravaAccessToken: "access-token", StravaRefreshToken: "refresh-token", TelegramChatId: 456}

	err := h.processActivity(123, user)
	assert.Error(t, err)
	assert.Empty(t, activitiesChannel)

	mockDB.AssertExpectations(t)
	mockStrava.AssertExpectations(t)
}
