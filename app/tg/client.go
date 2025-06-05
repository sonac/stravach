package tg

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"stravach/app/openai"
	"stravach/app/storage"
	dbModels "stravach/app/storage/models"
	"stravach/app/strava"
	"stravach/app/utils"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	callbackPrefixActivity       = "activity"
	commandStart                 = "/start"
	commandRefreshActivities     = "/refresh_activities"
	commandSetLanguage           = "/set_language"
	defaultBotErrorMessage       = "An error occurred. Please try again later."
	languageSetSuccessMessage    = "Your language was set to %s"
	activitiesRefreshedMessage   = "Activities are refreshed."
	authLinkMessage              = "Please authorize yourself in Strava %s"
	setLanguageUsageMessage      = "Message should be /set_language Language"
	chooseOptionMessage          = "Please choose an option:"
	generatingMessage            = "Generating..."
	customPromptInstruction      = "Please send me your custom prompt for the activity: %s"
	noActivitiesFoundMessage     = "No activities found to update."
	updateSuccessfulMessage      = "Activity '%s' updated successfully!"
	updateFailedMessage          = "Failed to update activity '%s'."
	customPromptSuccessMessage   = "Custom prompt applied. New names generated for '%s'."
	customPromptFailedMessage    = "Failed to apply custom prompt for '%s'."
	generatingBetterNamesMessage = "Generating better names for activity: %s (%d)"
)

type BotSender interface {
	SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
	RegisterHandler(handlerType bot.HandlerType, command string, matchType bot.MatchType, handlerFunc bot.HandlerFunc, middleware ...bot.Middleware) string
	RegisterHandlerMatchFunc(matchFunc bot.MatchFunc, handlerFunc bot.HandlerFunc, middleware ...bot.Middleware) string
	Start(ctx context.Context)
}
type DBStore interface {
	GetUserByChatId(chatID int64) (*dbModels.User, error)
	GetActivityById(activityID int64) (*dbModels.UserActivity, error)
	UpdateUserActivity(activity *dbModels.UserActivity) error
	UpdateUser(user *dbModels.User) error
	IsUserExistsByChatId(chatID int64) (bool, error)
	CreateUser(user *dbModels.User) error
	CreateUserActivities(activities []*dbModels.UserActivity) error
}
type AI interface {
	GenerateBetterNames(activity dbModels.UserActivity, lang string) ([]string, error)
	GenerateBetterNamesWithCustomizedPrompt(activity dbModels.UserActivity, lang, prompt string) ([]string, error)
}

type Telegram struct {
	APIKey            string
	Bot               BotSender
	DB                DBStore
	Strava            strava.StravaService
	AI                AI
	ActivitiesChannel chan ActivityForUpdate
	LastActivity      map[int64]int64              // chatID -> activityID
	NameOptions       map[int64]map[int64][]string // chatID -> activityID -> []options
}

type ActivityForUpdate struct {
	Activity dbModels.UserActivity
	ChatId   int64
}

func NewTelegramClient(apiKey string) (*Telegram, error) {
	db := &storage.SQLiteStore{}
	err := db.Connect()
	if err != nil {
		slog.Error("error while connecting to DB", "err", err)
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	stravaClient := strava.NewStravaClient()
	ai := openai.NewClient()
	return &Telegram{
		DB:                db,
		Strava:            stravaClient,
		AI:                ai,
		APIKey:            apiKey,
		LastActivity:      make(map[int64]int64),
		ActivitiesChannel: make(chan ActivityForUpdate, 10), // buffered channel
		NameOptions:       make(map[int64]map[int64][]string),
	}, nil
}

func (tg *Telegram) Start(ctx context.Context) {
	options := []bot.Option{
		bot.WithCallbackQueryDataHandler(callbackPrefixActivity, bot.MatchTypePrefix, tg.handleCallbackQuery),
	}
	b, err := bot.New(tg.APIKey, options...)
	if err != nil {
		panic(err)
	}
	tg.Bot = b

	defaultHandler := func(upd *models.Update) bool {
		slog.Info(upd.Message.Text)
		return !strings.HasPrefix(upd.Message.Text, "/")
	}
	tg.Bot.RegisterHandler(bot.HandlerTypeMessageText, commandStart, bot.MatchTypeExact, tg.startHandler)
	tg.Bot.RegisterHandler(bot.HandlerTypeMessageText, commandRefreshActivities, bot.MatchTypeExact, tg.refreshActivitiesHandler)
	tg.Bot.RegisterHandler(bot.HandlerTypeMessageText, commandSetLanguage, bot.MatchTypeExact, tg.setLanguageHandler)
	tg.Bot.RegisterHandlerMatchFunc(defaultHandler, tg.messageHandler)
	go tg.Bot.Start(ctx)
	slog.Info("Telegram bot started and listening for updates.")
	for activity := range tg.ActivitiesChannel {
		slog.Info("Received activity to update from channel", "activityID", activity.Activity.ID, "chatID", activity.ChatId)
		tg.updateActivity(&activity)
	}
	slog.Info("ActivitiesChannel closed, stopping activity update processing.")
}

func (tg *Telegram) SendNotification(chatID int64, messages ...string) {
	if len(messages) == 0 {
		slog.Warn("SendNotification called with no messages", "chatID", chatID)
		return
	}
	buttons := make([][]models.InlineKeyboardButton, len(messages))
	for i, msg := range messages {
		buttons[i] = []models.InlineKeyboardButton{
			{Text: msg, CallbackData: callbackPrefixActivity + ":" + msg},
		}
	}

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	_, err := tg.Bot.SendMessage(context.Background(), &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        chooseOptionMessage,
		ReplyMarkup: kb,
	})
	if err != nil {
		slog.Error("error while sending a message with options: ", "err", err, "chatID", chatID)
		tg.sendMessage(context.Background(), chatID, defaultBotErrorMessage)
	}
}

func (tg *Telegram) startHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	slog.Debug("received start command", "chatID", update.Message.Chat.ID)
	url := os.Getenv("URL")
	if url == "" {
		slog.Error("URL environment variable not set. Cannot generate auth link.")
		tg.sendMessage(ctx, update.Message.Chat.ID, "Server configuration error. Please contact admin.")
		return
	}
	chatID := update.Message.Chat.ID
	userExists, err := tg.DB.IsUserExistsByChatId(chatID)
	if err != nil {
		slog.Error("failed to check if user exists", "err", err, "chatID", chatID)
		tg.sendMessage(ctx, chatID, defaultBotErrorMessage)
		return
	}
	if !userExists {
		usr := &dbModels.User{TelegramChatId: chatID, StravaId: nil}
		err = tg.DB.CreateUser(usr)
		if err != nil {
			slog.Error("failed to create user", "err", err, "chatID", chatID)
			tg.sendMessage(ctx, chatID, defaultBotErrorMessage)
			return
		}
		slog.Info("New user created", "chatID", chatID)
	}

	link := fmt.Sprintf("%s/auth/%d", url, chatID)
	escapedLink := bot.EscapeMarkdownUnescaped(link)
	replyMsg := fmt.Sprintf(authLinkMessage, escapedLink)
	slog.Info("Sending auth link", "link", link, "chatID", chatID)
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      replyMsg,
		ParseMode: models.ParseModeMarkdown,
	})

	if err != nil {
		slog.Error("failed to send auth message", "err", err, "chatID", chatID)
	}
}

func (tg *Telegram) refreshActivitiesHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	usr, err := tg.DB.GetUserByChatId(chatID)
	if err != nil {
		slog.Error("failed to get user for refresh activities", "err", err, "chatID", chatID)
		tg.sendMessage(ctx, chatID, defaultBotErrorMessage)
		return
	}
	err = tg.refreshActivitiesForUser(usr)
	if err != nil {
		slog.Error("error while refreshing activities for user", "err", err, "userID", usr.ID)
		tg.sendMessage(ctx, chatID, "Failed to refresh activities. Please try again.")
		return
	}
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: usr.TelegramChatId,
		Text:   activitiesRefreshedMessage,
	})
	if err != nil {
		slog.Error("failed to send activities refreshed message", "err", err, "chatID", chatID)
	}
}

func (tg *Telegram) setLanguageHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	usr, err := tg.DB.GetUserByChatId(chatID)
	if err != nil {
		slog.Error("failed to get user for set language", "err", err, "chatID", chatID)
		tg.sendMessage(ctx, chatID, defaultBotErrorMessage)
		return
	}
	msgArr := strings.Split(update.Message.Text, " ")
	if len(msgArr) != 2 {
		tg.sendMessage(ctx, chatID, setLanguageUsageMessage)
		return
	}
	language := msgArr[1]
	usr.Language = language
	err = tg.DB.UpdateUser(usr)
	if err != nil {
		slog.Error("failed to update user language", "err", err, "userID", usr.ID, "language", language)
		tg.sendMessage(ctx, chatID, defaultBotErrorMessage)
		return
	}
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: usr.TelegramChatId,
		Text:   fmt.Sprintf(languageSetSuccessMessage, language),
	})
	if err != nil {
		slog.Error("failed to send language set confirmation", "err", err, "chatID", chatID)
	}
}

func (tg *Telegram) messageHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	activityID, ok := tg.LastActivity[chatID]
	if ok {
		if tg.NameOptions[chatID] == nil {
			tg.NameOptions[chatID] = make(map[int64][]string)
		}
	}

	if strings.HasPrefix(update.Message.Text, "/") {
		slog.Debug("this is a command, skipping message handler", "text", update.Message.Text)
		return
	}

	customPrompt := update.Message.Text

	activity, err := tg.DB.GetActivityById(activityID)
	if err != nil {
		slog.Error("error while fetching activity for custom prompt", "err", err, "activityID", activityID)
		tg.sendMessage(ctx, chatID, defaultBotErrorMessage)
		return
	}

	slog.Info("Generating names with custom prompt", "activityID", activity.ID, "prompt", customPrompt)
	tg.sendMessage(ctx, chatID, fmt.Sprintf(generatingMessage))
	usr, err := tg.DB.GetUserByChatId(chatID)
	if err != nil || usr == nil {
		slog.Error("error fetching user for custom prompt language", "err", err, "chatID", chatID)
		tg.sendMessage(ctx, chatID, defaultBotErrorMessage)
		return
	}
	names, err := tg.AI.GenerateBetterNamesWithCustomizedPrompt(*activity, usr.Language, customPrompt)
	if err != nil {
		slog.Error("error while generating names with custom prompt", "err", err, "activityID", activity.ID)
		tg.sendMessage(ctx, chatID, fmt.Sprintf(customPromptFailedMessage, activity.Name))
		return
	}

	slog.Info("Generated names with custom prompt", "activityID", activity.ID, "names", names)
	tg.sendMessage(ctx, chatID, fmt.Sprintf(customPromptSuccessMessage, activity.Name))

	tg.NameOptions[chatID][activityID] = names
	tg.LastActivity[chatID] = activityID

	var listText string
	maxOptions := 9
	for i, name := range names {
		listText += fmt.Sprintf("%d. %s\n", i+1, name)
		if i == maxOptions-1 {
			break
		}
	}
	listText += "0. 🔄 Regenerate\nC. ✏️ Enter custom prompt"

	msgText := "*Select a number with new name:*\n\n" + listText

	tg.sendMessage(context.Background(), chatID, msgText)
	if len(names) < maxOptions {
		maxOptions = len(names)
	}
	var inlineKeyboard [][]models.InlineKeyboardButton
	var row []models.InlineKeyboardButton
	for i := 0; i < maxOptions; i++ {
		button := models.InlineKeyboardButton{
			Text:         fmt.Sprintf("%d", i+1),
			CallbackData: fmt.Sprintf("%s:%d:%d", callbackPrefixActivity, activityID, i+1),
		}
		row = append(row, button)
		if len(row) == 3 {
			inlineKeyboard = append(inlineKeyboard, row)
			row = []models.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		inlineKeyboard = append(inlineKeyboard, row)
	}
	// add regenerate and custom prompt buttons as a final row
	finalRow := []models.InlineKeyboardButton{
		{
			Text:         "🔄 Regenerate",
			CallbackData: fmt.Sprintf("%s:%d:0", callbackPrefixActivity, activityID),
		},
		{
			Text:         "✏️ Custom",
			CallbackData: fmt.Sprintf("%s:%d:C", callbackPrefixActivity, activityID),
		},
	}
	inlineKeyboard = append(inlineKeyboard, finalRow)

	msg := &bot.SendMessageParams{
		ChatID: chatID,
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
	// store generated names in memory for this chatID/activityID
	if tg.NameOptions[activity.ChatId] == nil {
		tg.NameOptions[activity.ChatId] = make(map[int64][]string)
	}
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
	tg.NameOptions[activity.ChatId][activity.Activity.ID] = names
	tg.LastActivity[activity.ChatId] = activity.Activity.ID

	slog.Info("Generated names for activity", "activityID", activity.Activity.ID, "names", names)
	tg.sendMessage(context.Background(), activity.ChatId, fmt.Sprintf(generatingBetterNamesMessage, activity.Activity.Name, activity.Activity.ID))

	var listText string
	maxOptions := 9
	for i, name := range names {
		listText += fmt.Sprintf("%d. %s\n", i+1, name)
		if i == maxOptions-1 {
			break
		}
	}
	listText += "0. 🔄 Regenerate\nC. ✏️ Enter custom prompt"

	msgText := "*Select a number with new name:*\n\n" + listText

	tg.sendMessage(context.Background(), activity.ChatId, msgText)

	if len(names) < maxOptions {
		maxOptions = len(names)
	}
	var inlineKeyboard [][]models.InlineKeyboardButton
	var row []models.InlineKeyboardButton
	for i := 0; i < maxOptions; i++ {
		button := models.InlineKeyboardButton{
			Text:         fmt.Sprintf("%d", i+1),
			CallbackData: fmt.Sprintf("%s:%d:%d", callbackPrefixActivity, activity.Activity.ID, i+1),
		}
		row = append(row, button)
		if len(row) == 3 {
			inlineKeyboard = append(inlineKeyboard, row)
			row = []models.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		inlineKeyboard = append(inlineKeyboard, row)
	}
	finalRow := []models.InlineKeyboardButton{
		{
			Text:         "🔄 Regenerate",
			CallbackData: fmt.Sprintf("%s:%d:0", callbackPrefixActivity, activity.Activity.ID),
		},
		{
			Text:         "✏️ Custom",
			CallbackData: fmt.Sprintf("%s:%d:C", callbackPrefixActivity, activity.Activity.ID),
		},
	}
	inlineKeyboard = append(inlineKeyboard, finalRow)

	msg := &bot.SendMessageParams{
		ChatID: activity.ChatId,
		Text:   "Please choose by pressing a button below:",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: inlineKeyboard,
		},
	}

	_, err = tg.Bot.SendMessage(context.Background(), msg)
	if err != nil {
		slog.Error("error while sending activity names with options: ", "err", err, "chatID", activity.ChatId)
		tg.sendMessage(context.Background(), activity.ChatId, defaultBotErrorMessage)
	}
}

func (tg *Telegram) sendMessage(ctx context.Context, chatID int64, msg string) {
	_, err := tg.Bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   msg,
	})
	if err != nil {
		slog.Error("error while sending message: ", "err", err, "chatID", chatID, "msg", msg)
	}
}

func (tg *Telegram) handleCallbackQuery(ctx context.Context, _ *bot.Bot, update *models.Update) {
	// Now callback data is activity:<activityID>:<option>

	chatID := update.CallbackQuery.From.ID
	callbackData := update.CallbackQuery.Data

	slog.Debug("received callback query", "chatID", chatID, "callbackData", callbackData)

	parts := strings.Split(callbackData, ":")
	if len(parts) < 3 || parts[0] != callbackPrefixActivity {
		tg.sendMessage(ctx, chatID, "Invalid callback data.")
		return
	}
	activityIDStr := parts[1]
	option := parts[2]

	activityID, err := strconv.ParseInt(activityIDStr, 10, 64)
	if err != nil {
		tg.sendMessage(ctx, chatID, "Invalid activity ID.")
		return
	}

	if option == "0" {
		tg.handleRegenerateNames(ctx, chatID, activityID)
		return
	}
	if option == "C" {
		tg.handleCustomPromptSetup(ctx, chatID, activityID)
		return
	}

	idx, err := strconv.Atoi(option)
	if err != nil || idx < 1 {
		tg.sendMessage(ctx, chatID, "Invalid selection.")
		return
	}

	nameOptions, ok := tg.NameOptions[chatID]
	if !ok {
		tg.sendMessage(ctx, chatID, "No name options found. Please regenerate.")
		return
	}
	options, ok := nameOptions[activityID]
	if !ok || idx > len(options) {
		tg.sendMessage(ctx, chatID, "Invalid selection.")
		return
	}
	selectedName := options[idx-1]
	// Clean up after selection
	delete(nameOptions, activityID)
	if len(nameOptions) == 0 {
		delete(tg.NameOptions, chatID)
	}
	tg.handleActivitySelection(ctx, chatID, activityID, selectedName)
}

func (tg *Telegram) handleCustomPromptSetup(ctx context.Context, chatID int64, activityID int64) {
	activity, err := tg.DB.GetActivityById(activityID)
	if err != nil {
		slog.Error("Failed to get activity for custom prompt setup", "activityID", activityID, "err", err)
		tg.sendMessage(ctx, chatID, defaultBotErrorMessage)
		return
	}
	tg.LastActivity[chatID] = activityID
	msg := fmt.Sprintf(customPromptInstruction, activity.Name)
	tg.sendMessage(ctx, chatID, msg)
	slog.Info("Set custom prompt state for user", "chatID", chatID, "activityID", activityID)
}

func (tg *Telegram) handleRegenerateNames(ctx context.Context, chatID int64, activityID int64) {
	activity, err := tg.DB.GetActivityById(activityID)
	if err != nil {
		slog.Error("Failed to get activity for regeneration", "activityID", activityID, "err", err)
		tg.sendMessage(ctx, chatID, defaultBotErrorMessage)
		return
	}

	tg.ActivitiesChannel <- ActivityForUpdate{
		Activity: *activity,
		ChatId:   chatID,
	}
	slog.Info("Sent activity for name regeneration to channel", "chatID", chatID, "activityID", activityID)
	tg.sendMessage(ctx, chatID, fmt.Sprintf(generatingMessage))
}

func (tg *Telegram) handleActivitySelection(ctx context.Context, chatID int64, activityID int64, newName string) {
	usr, err := tg.DB.GetUserByChatId(chatID)
	if err != nil {
		slog.Error("Failed to get user for activity update", "chatID", chatID, "err", err)
		tg.sendMessage(ctx, chatID, defaultBotErrorMessage)
		return
	}

	activity, err := tg.DB.GetActivityById(activityID)
	if err != nil {
		slog.Error("Failed to get activity for update", "activityID", activityID, "err", err)
		tg.sendMessage(ctx, chatID, defaultBotErrorMessage)
		return
	}

	originalName := activity.Name
	activity.Name = tg.cleanName(newName)
	activity.IsUpdated = true

	err = tg.refreshAuthForUser(usr)
	if err != nil {
		slog.Error("Failed to refresh auth for user before updating activity", "userID", usr.ID, "err", err)
		tg.sendMessage(ctx, chatID, "Authentication error. Please try /start again.")
		return
	}

	_, err = tg.Strava.UpdateActivity(usr.StravaAccessToken, *activity)
	if err != nil {
		slog.Error("Failed to update activity name on Strava", "activityID", activity.ID, "newName", activity.Name, "err", err)
		tg.sendMessage(ctx, chatID, fmt.Sprintf(updateFailedMessage, originalName))
		return
	}

	err = tg.DB.UpdateUserActivity(activity)
	if err != nil {
		slog.Error("Failed to update activity in DB after Strava update", "activityID", activity.ID, "err", err)
		tg.sendMessage(ctx, chatID, fmt.Sprintf("Activity '%s' updated on Strava, but local sync failed. Please try /refresh_activities.", activity.Name))
		return
	}

	slog.Info("Activity name updated successfully", "activityID", activity.ID, "newName", activity.Name)
	tg.sendMessage(ctx, chatID, fmt.Sprintf(updateSuccessfulMessage, activity.Name))
}

func (tg *Telegram) refreshActivitiesForUser(usr *dbModels.User) error {
	err := tg.refreshAuthForUser(usr)
	if err != nil {
		return err
	}

	activities, err := tg.Strava.GetAllActivities(usr.StravaAccessToken)
	if err != nil && err.Error() == utils.UNAUTHORIZED {
		err = tg.refreshAuthForUser(usr)
		if err != nil {
			return err
		}
		activities, err = tg.Strava.GetAllActivities(usr.StravaAccessToken)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if activities == nil || len(*activities) == 0 {
		slog.Info("No activities found for user on Strava", "userID", usr.ID)
		tg.sendMessage(context.Background(), usr.TelegramChatId, noActivitiesFoundMessage)
		return nil
	}

	slog.Info("Fetched activities from Strava", "count", len(*activities), "userID", usr.ID)
	var activityPtrs []*dbModels.UserActivity
	for _, a := range *activities {
		a.UserID = usr.ID
		activityPtrs = append(activityPtrs, &a)
	}
	err = tg.DB.CreateUserActivities(activityPtrs)
	if err != nil {
		return err
	}

	tg.sendMessage(context.Background(), usr.TelegramChatId, "activities are updated")
	return nil
}

func (tg *Telegram) refreshAuthForUser(usr *dbModels.User) error {
	if !usr.AuthRequired() {
		return nil
	}
	slog.Info("Refreshing Strava token for user", "userID", usr.ID)
	authResp, err := tg.Strava.RefreshAccessToken(usr.StravaRefreshToken)
	if err != nil {
		return err
	}
	usr.StravaAccessToken = authResp.AccessToken
	usr.StravaRefreshToken = authResp.RefreshToken
	usr.TokenExpiresAt = &authResp.ExpiresAt
	return tg.DB.UpdateUser(usr)
}

// cleanName removes leading/trailing spaces and special characters from the activity name.
func (tg *Telegram) cleanName(name string) string {
	name = strings.TrimSpace(name)

	re := regexp.MustCompile(`[^a-zA-Z0-9\s\-_.,!?'"()&]+`)
	name = re.ReplaceAllString(name, "")
	re = regexp.MustCompile(`\s+`)
	name = re.ReplaceAllString(name, " ")
	return name
}
