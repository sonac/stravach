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

// Mocking the dependencies
type MockDB struct {
	mock.Mock
}

func (mdb *MockDB) Connect() error {
	return nil
}

func (mdb *MockDB) GetActivityById(activityId int64) (*models.UserActivity, error) {
	args := mdb.Called(activityId)
	return args.Get(0).(*models.UserActivity), args.Error(1)
}

func (mdb *MockDB) CreateUserActivity(activity *models.UserActivity, userId int64) error {
	args := mdb.Called(activity, userId)
	return args.Error(0)
}

func (mdb *MockDB) UpdateUser(user *models.User) error {
	args := mdb.Called(user)
	return args.Error(0)
}

func (mdb *MockDB) GetUserByStravaId(stravaId int64) (*models.User, error) {
	args := mdb.Called(stravaId)
	return args.Get(0).(*models.User), args.Error(1)
}

func (mdb *MockDB) GetUserActivities(userId int64) ([]models.UserActivity, error) {
	args := mdb.Called(userId)
	return args.Get(0).([]models.UserActivity), args.Error(1)
}

func (mdb *MockDB) GetUserByChatId(chatId int64) (*models.User, error) {
	args := mdb.Called(chatId)
	return args.Get(0).(*models.User), args.Error(1)
}

func (mdb *MockDB) GetUserById(id int64) (*models.User, error) {
	args := mdb.Called(id)
	return args.Get(0).(*models.User), args.Error(1)
}

func (mdb *MockDB) IsActivityExists(activityId int64) (bool, error) {
	args := mdb.Called(activityId)
	return args.Get(0).(bool), args.Error(1)
}

// Use mockery-generated StravaService mock from mocks package

func TestProcessActivity_ActivityExists(t *testing.T) {
	mockDB := new(MockDB)
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
	mockDB := new(MockDB)
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
	mockDB := new(MockDB)
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
	mockDB := new(MockDB)
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
