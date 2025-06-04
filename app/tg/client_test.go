package tg

import (
	"context"
	"fmt"
	dbModels "stravach/app/storage/models"
	strava "stravach/app/strava"
	"stravach/mocks"
	"testing"

	bot "github.com/go-telegram/bot"
	botModels "github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/mock"
)

// TestCleanName tests the cleanName method of the Telegram struct.
func TestCleanName(t *testing.T) {
	tgInstance := &Telegram{} // cleanName doesn't depend on other fields of Telegram struct

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple name",
			input: "My Awesome Ride",
			want:  "My Awesome Ride",
		},
		{
			name:  "name with leading/trailing spaces",
			input: "  My Awesome Ride  ",
			want:  "My Awesome Ride",
		},
		{
			name:  "name with special characters",
			input: "My Ride!@#$%^&*()_+-={}|[]\\:\";'<>?,./",
			want:  "My Ride!&()_-\"'?,.",
		},
		{
			name:  "name with multiple spaces between words",
			input: "My    Awesome   Ride",
			want:  "My Awesome Ride",
		},
		{
			name:  "name with numbers",
			input: "Ride 123",
			want:  "Ride 123",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only special characters",
			input: "!@#$%^&*()",
			want:  "!&()",
		},
		{
			name:  "name with hyphens and underscores",
			input: "My_Activity-Ride",
			want:  "My_Activity-Ride",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tgInstance.cleanName(tt.input)
			if got != tt.want {
				t.Errorf("Telegram.cleanName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandleCallbackQuery_NumberSelection(t *testing.T) {
	mbot := &mocks.BotSender{}
	mdb := &mocks.DBStore{}
	mai := &mocks.AI{}
	mstrava := &mocks.StravaService{}

	oldActivity := &dbModels.UserActivity{ID: 99, Name: "Old Name"}

	mdb.On("GetUserByChatId", int64(123)).Return(&dbModels.User{TelegramChatId: 123, Language: "en"}, nil)
	mdb.On("GetActivityById", int64(99)).Return(oldActivity, nil)
	mdb.On("UpdateUser", mock.Anything).Return(nil)
	mdb.On("UpdateUserActivity", mock.Anything).Return(nil)

	mai.On("GenerateBetterNames", *oldActivity, "en").Return([]string{"Morning Ride", "Evening Run"}, nil)

	mbot.On("SendMessage", mock.Anything, mock.Anything).Return(&botModels.Message{}, nil)
	mbot.On("SendMessage", mock.Anything, mock.Anything).Return(&botModels.Message{}, nil)
	mbot.On("SendMessage", mock.Anything, mock.Anything).Return(&botModels.Message{}, nil)

	mstrava.On("RefreshAccessToken", mock.AnythingOfType("string")).Return(&strava.AuthResp{}, nil)
	mstrava.On("UpdateActivity", mock.Anything, mock.MatchedBy(func(a dbModels.UserActivity) bool {
		return a.ID == 99 && a.Name == "Evening Run"
	})).Return(&dbModels.UserActivity{}, nil)

	tgInstance := &Telegram{
		Bot:    mbot,
		DB:     mdb,
		AI:     mai,
		Strava: mstrava,
	}
	update := &botModels.Update{
		CallbackQuery: &botModels.CallbackQuery{
			From: botModels.User{ID: 123},
			Data: "activity:99:2",
		},
	}
	tgInstance.handleCallbackQuery(context.Background(), nil, update)
	mstrava.AssertCalled(t, "UpdateActivity", mock.Anything, mock.MatchedBy(func(a dbModels.UserActivity) bool {
		return a.ID == 99 && a.Name == "Evening Run"
	}))

}

func TestHandleCallbackQuery_Regenerate(t *testing.T) {
	mbot := &mocks.BotSender{}
	mdb := &mocks.DBStore{}
	mai := &mocks.AI{}
	mstrava := &mocks.StravaService{}

	user := &dbModels.User{TelegramChatId: 123, Language: "en"}
	activity := &dbModels.UserActivity{ID: 99, Name: "Old Name"}

	mdb.On("GetUserByChatId", int64(123)).Return(user, nil)
	mdb.On("GetActivityById", int64(99)).Return(activity, nil)

	mbot.On("SendMessage", context.Background(), mock.AnythingOfType("*bot.SendMessageParams")).Return(&botModels.Message{}, nil).Maybe()
	mbot.On("SendMessage", mock.Anything, mock.Anything).Return(&botModels.Message{}, nil)

	tgInstance := &Telegram{
		Bot:               mbot,
		DB:                mdb,
		AI:                mai,
		Strava:            mstrava,
		CustomPromptState: make(map[int64]int64),
		ActivitiesChannel: make(chan ActivityForUpdate, 1), // Add channel if needed by flow
	}
	update := &botModels.Update{
		CallbackQuery: &botModels.CallbackQuery{
			From: botModels.User{ID: 123},
			Data: "activity:99:0",
		},
	}
	tgInstance.handleCallbackQuery(context.Background(), nil, update)

	mdb.AssertExpectations(t)
	mbot.AssertExpectations(t)
	mstrava.AssertExpectations(t)
}

func TestHandleCallbackQuery_CustomPrompt(t *testing.T) {
	mbot := &mocks.BotSender{}
	mdb := &mocks.DBStore{}
	mai := &mocks.AI{}
	mstrava := &mocks.StravaService{}

	user := &dbModels.User{TelegramChatId: 123, Language: "en"}
	activity := &dbModels.UserActivity{ID: 99, Name: "Old Name"}

	mdb.On("GetUserByChatId", int64(123)).Return(user, nil)
	mdb.On("GetActivityById", int64(99)).Return(activity, nil)
	expectedMsgText := fmt.Sprintf(customPromptInstruction, activity.Name)
	mbot.On("SendMessage", context.Background(), mock.MatchedBy(func(params *bot.SendMessageParams) bool {
		return params.ChatID == int64(123) && params.Text == expectedMsgText
	})).Return(&botModels.Message{}, nil)
	mbot.On("SendMessage", mock.Anything, mock.Anything).Return(&botModels.Message{}, nil)
	tgInstance := &Telegram{
		Bot:               mbot,
		DB:                mdb,
		AI:                mai,
		Strava:            mstrava,
		CustomPromptState: make(map[int64]int64),
	}
	update := &botModels.Update{
		CallbackQuery: &botModels.CallbackQuery{
			From: botModels.User{ID: 123},
			Data: "activity:99:C",
		},
	}
	tgInstance.handleCallbackQuery(context.Background(), nil, update)

	mdb.AssertExpectations(t)
	mbot.AssertExpectations(t)
	mai.AssertExpectations(t)
	mstrava.AssertExpectations(t)
}

func TestHandleCallbackQuery_InvalidData(t *testing.T) {
	mbot := &mocks.BotSender{}
	mdb := &mocks.DBStore{}
	mai := &mocks.AI{}
	// No need to set up expectations for this test, as invalid callback data should result in an early return
	mbot.On("SendMessage", mock.Anything, mock.Anything).Return(&botModels.Message{}, nil)
	tgInstance := &Telegram{Bot: mbot, DB: mdb, AI: mai}
	update := &botModels.Update{
		CallbackQuery: &botModels.CallbackQuery{
			From: botModels.User{ID: 123},
			Data: "invalid:data",
		},
	}
	tgInstance.handleCallbackQuery(context.Background(), nil, update)

	mbot.AssertExpectations(t)
	mdb.AssertExpectations(t)
	mai.AssertExpectations(t)
}
