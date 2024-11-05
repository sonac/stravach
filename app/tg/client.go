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
	CustomPromptState map[int64]int64 // chatID -> activityID
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
		DB:                db,
		Strava:            stravaClient,
		AI:                ai,
		APIKey:            apiKey,
		CustomPromptState: make(map[int64]int64),
	}, nil
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: getChatId(update),
		Text:   "I am Strava bot! Nice to meet you!",
	})
	if err != nil {
		slog.Error("error in handler", "err", err)
		return
	}
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
	tg.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/refresh_activities", bot.MatchTypeExact, tg.refreshActivitiesHandler)
	tg.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/set_language", bot.MatchTypePrefix, tg.setLanguageHandler)
	tg.Bot.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypePrefix, tg.messageHandler)
	go tg.Bot.Start(ctx)
	for activity := range tg.ActivitiesChannel {
		tg.updateActivity(&activity)
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
		slog.Error("error while sending a message: ", "err", err, "data: ", fmt.Sprintf("%+v", kb))
		tg.sendMessage(context.Background(), chatID, "wasn't able to generate message")
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

func (tg *Telegram) refreshActivitiesHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	usr, err := tg.DB.GetUserByChatId(chatID)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	err = tg.refreshActivitiesForUser(usr)
	if err != nil {
		slog.Error("error while refreshing activities for user", "err", err.Error())
	}
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: usr.TelegramChatId,
		Text:   "activities are refreshed",
	})
	if err != nil {
		slog.Error(err.Error())
		tg.sendMessage(ctx, chatID, "error occured")
		return
	}
}

func (tg *Telegram) setLanguageHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	usr, err := tg.DB.GetUserByChatId(chatID)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	msgArr := strings.Split(update.Message.Text, " ")
	if len(msgArr) != 2 {
		tg.sendMessage(ctx, chatID, "Message should be /set_language Language")
		return
	}
	language := msgArr[1]
	usr.Language = language
	err = tg.DB.UpdateUser(usr)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: usr.TelegramChatId,
		Text:   "Your language was set to " + language,
	})
	if err != nil {
		slog.Error(err.Error())
		tg.sendMessage(ctx, chatID, "error occured")
		return
	}
}

func (tg *Telegram) messageHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatId := update.Message.Chat.ID
	activityID, awaitingPrompt := tg.CustomPromptState[chatId]
	if !awaitingPrompt {
		return
	}

	customPrompt := update.Message.Text
	delete(tg.CustomPromptState, chatId)

	activity, err := tg.DB.GetActivityById(activityID)
	if err != nil {
		slog.Error("error while fetching activity", "err", err)
		return
	}

	names, err := tg.AI.GenerateBetterNamesWithCustomizedPrompt(*activity, customPrompt)
	if err != nil {
		slog.Error("error while generating names", "err", err)
		return
	}

	formattedNames := utils.FormatActivityNames(names)
	formattedNames = append(formattedNames, "0. Regenerate", "Enter custom prompt")

	var inlineKeyboard [][]models.InlineKeyboardButton
	for _, name := range formattedNames {
		button := models.InlineKeyboardButton{
			Text:         name,
			CallbackData: fmt.Sprintf("activity %d:%s", activityID, cleanName(name)),
		}
		inlineKeyboard = append(inlineKeyboard, []models.InlineKeyboardButton{button})
	}

	msg := &bot.SendMessageParams{
		ChatID: chatId,
		Text:   "Choose a name for your activity",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: inlineKeyboard,
		},
	}
	_, err = b.SendMessage(ctx, msg)
	if err != nil {
		slog.Error("error while sending message", "err", err)
	}
}

func (tg *Telegram) updateActivity(activity *ActivityForUpdate) {
	usr, err := tg.DB.GetUserByChatId(activity.ChatId)
	if err != nil {
		slog.Error("error while fetching user")
		return
	}

	names, err := tg.AI.GenerateBetterNames(activity.Activity, usr.Language)
	if err != nil {
		slog.Error("error while generating names", "err", err)
		return
	}

	formattedNames := utils.FormatActivityNames(names)
	formattedNames = append(formattedNames, "0. Regenerate", "Enter custom prompt")

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
		slog.Debug("keyboard is: " + fmt.Sprintf("%+v", inlineKeyboard))
		slog.Debug("names are: " + fmt.Sprintf("%+v", formattedNames))
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

func (tg *Telegram) handleCallbackQuery(ctx context.Context, b *bot.Bot, update *models.Update) {
	callbackQuery := update.CallbackQuery
	var activityID int64
	_, err := fmt.Sscanf(callbackQuery.Data, "activity %d", &activityID)
	action := strings.Split(callbackQuery.Data, ":")[1]
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

	if action == "Regenerate" {
		afu := ActivityForUpdate{
			Activity: *activity,
			ChatId:   usr.TelegramChatId,
		}
		tg.ActivitiesChannel <- afu
		return
	}

	if action == "Enter custom prompt" {
		tg.CustomPromptState[callbackQuery.From.ID] = activityID
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callbackQuery.From.ID,
			Text:   "Please enter your custom prompt for generating activity names:",
		})
		if err != nil {
			slog.Error("error while sending message", "err", err)
		}
		return
	}

	if usr.AuthRequired() {
		err = tg.refreshAuthForUser(usr)
		if err != nil {
			slog.Error(err.Error())
			return
		}
	}
	slog.Debug("updating user in tg callback query")
	err = tg.DB.UpdateUser(usr)
	if err != nil {
		slog.Error(err.Error())
		return
	}

	activity.Name = action
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

func (tg *Telegram) refreshActivitiesForUser(usr *dbModels.User) error {
	if usr.AuthRequired() {
		err := tg.refreshAuthForUser(usr)
		if err != nil {
			return err
		}
	}
	slog.Debug("updating user in refresh activities")
	err := tg.DB.UpdateUser(usr)
	if err != nil {
		return err
	}

	activities, err := strava.GetAllActivities(usr.StravaAccessToken)
	if err != nil && err.Error() == utils.UNAUTHORIZED {
		err = tg.refreshAuthForUser(usr)
		if err != nil {
			return err
		}
		return tg.refreshActivitiesForUser(usr)
	}
	if err != nil {
		slog.Error("error while fetching activities", "err", err.Error())
		return err
	}

	err = tg.DB.CreateUserActivities(usr.ID, activities)
	if err != nil {
		return err
	}
	return nil
}

func (tg *Telegram) refreshAuthForUser(usr *dbModels.User) error {
	authData, err := tg.Strava.RefreshAccessToken(usr.StravaRefreshToken)
	if err != nil {
		return err
	}
	usr.StravaRefreshToken = authData.RefreshToken
	usr.StravaAccessToken = authData.AccessToken
	usr.TokenExpiresAt = &authData.ExpiresAt

	slog.Debug("updating user in telegram")
	err = tg.DB.UpdateUser(usr)
	if err != nil {
		return err
	}
	return nil
}

func getChatId(update *models.Update) int64 {
	if update.Message != nil {
		return update.Message.From.ID
	}
	if update.CallbackQuery != nil {
		return update.CallbackQuery.From.ID
	}
	slog.Error("unknown type of update")
	return 0
}

func cleanName(name string) string {
	re := regexp.MustCompile(`(^\d+\.\s)|(^-\s)`)
	newStr := re.ReplaceAllString(name, "")
	if len(newStr) > 44 {
		return newStr[0:40] + "..."
	}
	return newStr
}
