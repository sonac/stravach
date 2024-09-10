package tg

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"regexp"
	"stravach/app/openai"
	"stravach/app/storage"
	dbModels "stravach/app/storage/models"
	"stravach/app/strava"
	"stravach/app/utils"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Telegram struct {
	APIKey            string
	Bot               *bot.Bot
	DB                *storage.SQLiteStore
	Strava            *strava.Client
	AI                *openai.OpenAI
	ActivitiesChannel chan ActivityForUpdate
}

type ActivityForUpdate struct {
	Activity dbModels.UserActivity
	ChatId   int64
}

func NewTelegramClient(apiKey string) (*Telegram, error) {
	db := &storage.SQLiteStore{}
	err := db.Connect()
	if err != nil {
		slog.Error("error while connecting to DB")
		panic(err)
	}
	stravaClient := strava.NewStravaClient()
	ai := openai.NewClient()
	return &Telegram{
		DB:     db,
		Strava: stravaClient,
		AI:     ai,
		APIKey: apiKey,
	}, nil
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: getChatId(update),
		Text:   "I am Strava bot! Nice to meet you!",
	})
}

func (tg *Telegram) Start(ctx context.Context) {
	options := []bot.Option{
		bot.WithDefaultHandler(handler),
		bot.WithCallbackQueryDataHandler("activity", bot.MatchTypePrefix, tg.handleCallbackQuery),
	}
	b, err := bot.New(tg.APIKey, options...)
	if err != nil {
		slog.Error("error occured when spinning up the bot", "err", err)
		return
	}
	tg.Bot = b
	tg.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, tg.startHandler)
	tg.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/refresh_activities", bot.MatchTypeExact, tg.refreshActivities)
	go tg.Bot.Start(ctx)
	for activity := range tg.ActivitiesChannel {
		tg.updateActivity(&activity)
	}
}

func (tg *Telegram) startHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	url := os.Getenv("URL")
	chatID := update.Message.Chat.ID
	userExists, err := tg.DB.IsUserExistsByChatId(chatID)
	if err != nil {
		slog.Error(err.Error())
	}
	if !userExists {
		usr := &dbModels.User{TelegramChatId: chatID, StravaId: 0}
		err = tg.DB.CreateUser(usr)
		if err != nil {
			slog.Error(err.Error())
		}
	}

	link := fmt.Sprintf("%s/auth/%d", url, chatID)
	escapedLink := bot.EscapeMarkdownUnescaped(link)
	replyMsg := fmt.Sprintf("Please authorize yourself in Strava %s", escapedLink)
	slog.Info(replyMsg)
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      replyMsg,
		ParseMode: models.ParseModeMarkdown,
	})

	log.Printf("msg is: %+v", msg)
	if err != nil {
		slog.Error(err.Error())
	}
}

func (tg *Telegram) refreshActivities(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	var authData *strava.AuthResp
	usr, err := tg.DB.GetUserByChatId(chatID)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	if usr.AuthRequired() {
		if len(usr.StravaRefreshToken) == 0 {
			authData, err = tg.Strava.Authorize(usr.StravaAccessCode)
			if err != nil {
				slog.Error(err.Error())
				tg.sendMessage(ctx, chatID, "error occured")
				return
			}
			updateAuthData(usr, *authData)
		} else {
			authData, err = tg.Strava.RefreshAccessToken(usr.StravaRefreshToken)
			if err != nil {
				slog.Error(err.Error())
				tg.sendMessage(ctx, chatID, "error occured")
				return
			}
			usr.StravaRefreshToken = authData.RefreshToken
			usr.TokenExpiresAt = &authData.ExpiresAt
		}
	}
	slog.Debug("updating user in refresh activities")
	err = tg.DB.UpdateUser(usr)
	if err != nil {
		slog.Error(err.Error())
		tg.sendMessage(ctx, chatID, "error occured")
		return
	}

	activities, err := strava.GetAllActivities(usr.StravaAccessToken)
	if err != nil {
		slog.Error(err.Error())
		tg.sendMessage(ctx, chatID, "error occured")
		return
	}
	err = tg.DB.CreateUserActivities(usr.ID, activities)
	if err != nil {
		slog.Error(err.Error())
		tg.sendMessage(ctx, chatID, "error occured")
		return
	}
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "activities are refreshed",
	})
	if err != nil {
		slog.Error(err.Error())
		tg.sendMessage(ctx, chatID, "error occured")
		return
	}
}

func (tg *Telegram) updateActivity(activity *ActivityForUpdate) {
	usr, err := tg.DB.GetUserByChatId(activity.ChatId)
	if err != nil {
		slog.Error("error while fetching user")
		return
	}

	names, err := tg.AI.GenerateBetterNames(activity.Activity)
	if err != nil {
		slog.Error("error while generating names", "err", err)
		return
	}

	formattedNames := utils.FormatActivityNames(names)
	formattedNames = append(formattedNames, "0. Regenerate")

	var inlineKeyboard [][]models.InlineKeyboardButton
	for _, name := range formattedNames {
		button := models.InlineKeyboardButton{
			Text:         name,
			CallbackData: fmt.Sprintf("activity %d:%s", activity.Activity.ID, cleanName(name)),
		}
		inlineKeyboard = append(inlineKeyboard, []models.InlineKeyboardButton{button})
	}

	msg := &bot.SendMessageParams{
		ChatID: usr.TelegramChatId,
		Text:   "Choose a name for your activity",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: inlineKeyboard,
		},
	}

	_, err = tg.Bot.SendMessage(context.Background(), msg)
	if err != nil {
		slog.Error("error while sending message", "err", err)
		return
	}
}

func (tg *Telegram) sendMessage(ctx context.Context, chatID int64, msg string) {
	_, err := tg.Bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   msg,
	})
	if err != nil {
		slog.Error(err.Error())
	}
}

func (tg *Telegram) SendNotification(chatID int64, messages ...string) {
	buttons := make([][]models.InlineKeyboardButton, len(messages))
	for i, msg := range messages {
		buttons[i] = []models.InlineKeyboardButton{
			{Text: msg, CallbackData: msg},
		}
	}

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	_, err := tg.Bot.SendMessage(context.Background(), &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        "Please choose an option:",
		ReplyMarkup: kb,
	})
	if err != nil {
		slog.Error("error while sending a message: ", "err", err)
	}
}

func (tg *Telegram) handleCallbackQuery(ctx context.Context, b *bot.Bot, update *models.Update) {
	callbackQuery := update.CallbackQuery
	var activityID int64
	_, err := fmt.Sscanf(callbackQuery.Data, "activity %d", &activityID)
	newName := strings.Split(callbackQuery.Data, ":")[1]
	if err != nil {
		slog.Error("error while parsing callback data", "err", err)
		return
	}

	activity, err := tg.DB.GetActivityById(activityID)
	if err != nil {
		slog.Error("error while fetching activity from DB", "err", err)
		return
	}

	usr, err := tg.DB.GetUserByChatId(callbackQuery.From.ID)
	if err != nil {
		slog.Error(err.Error())
		return
	}

	if newName == "Regenerate" {
		afu := ActivityForUpdate{
			Activity: *activity,
			ChatId:   usr.TelegramChatId,
		}
		tg.ActivitiesChannel <- afu
		return
	}

	var authData *strava.AuthResp
	if usr.AuthRequired() {
		if len(usr.StravaRefreshToken) == 0 {
			authData, err = tg.Strava.Authorize(usr.StravaAccessCode)
			if err != nil {
				slog.Error(err.Error())
				return
			}
			updateAuthData(usr, *authData)
		} else {
			authData, err = tg.Strava.RefreshAccessToken(usr.StravaRefreshToken)
			if err != nil {
				slog.Error(err.Error())
				return
			}
			usr.StravaRefreshToken = authData.RefreshToken
			usr.TokenExpiresAt = &authData.ExpiresAt
		}
	}
	slog.Debug("updating user in tg callback query")
	err = tg.DB.UpdateUser(usr)
	if err != nil {
		slog.Error(err.Error())
		return
	}

	activity.Name = newName
	_, err = strava.UpdateActivity(usr.StravaAccessToken, *activity)
	if err != nil {
		slog.Error("error while updating activity", "err", err)
		return
	}

	activity.IsUpdated = true
	err = tg.DB.UpdateUserActivity(activity)
	if err != nil {
		slog.Error("error when updating user activity", "err", err)
	}

	res, err := tg.Bot.AnswerCallbackQuery(context.Background(), &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQuery.ID,
		Text:            "Activity updated",
	})
	if err != nil {
		return
	}
	if !res {
		slog.Error("Error while answering callback")
	}
}

func getChatId(update *models.Update) int64 {
	if update.Message != nil {
		return update.Message.From.ID
	}
	if update.CallbackQuery != nil {
		return update.CallbackQuery.From.ID
	}
	panic("unknown type of update")
}

func cleanName(name string) string {
	re := regexp.MustCompile(`(^\d+\.\s)|(^-\s)`)
	return re.ReplaceAllString(name, "")
}

func updateAuthData(u *dbModels.User, authData strava.AuthResp) {
	u.StravaAccessToken = authData.AccessToken
	u.StravaRefreshToken = authData.RefreshToken
	u.TokenExpiresAt = &authData.ExpiresAt
	u.StravaId = authData.Athlete.Id
}
