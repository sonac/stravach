package tg

import (
	"context"
	"fmt"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"log/slog"
	"os"
	dbModels "stravach/app/storage/models"
	"strings"
)

func (tg *Telegram) startHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	slog.Debug("received start command", "chatID", update.Message.Chat.ID)
	url := os.Getenv("URL")
	if url == "" {
		slog.Error("URL environment variable not set. Cannot generate auth link.")
		tg.SendMessage(ctx, update.Message.Chat.ID, "Server configuration error. Please contact admin.")
		return
	}
	chatID := update.Message.Chat.ID
	userExists, err := tg.DB.IsUserExistsByChatId(chatID)
	if err != nil {
		slog.Error("failed to check if user exists", "err", err, "chatID", chatID)
		tg.SendMessage(ctx, chatID, defaultBotErrorMessage)
		return
	}
	if !userExists {
		usr := &dbModels.User{TelegramChatId: chatID, StravaId: nil}
		err = tg.DB.CreateUser(usr)
		if err != nil {
			slog.Error("failed to create user", "err", err, "chatID", chatID)
			tg.SendMessage(ctx, chatID, defaultBotErrorMessage)
			return
		}
		slog.Info("New user created", "chatID", chatID)
	}

	link := fmt.Sprintf("%s/api/auth/%d", url, chatID)
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
		tg.SendMessage(ctx, chatID, defaultBotErrorMessage)
		return
	}
	err = tg.refreshActivitiesForUser(usr)
	if err != nil {
		slog.Error("error while refreshing activities for user", "err", err, "userID", usr.ID)
		tg.SendMessage(ctx, chatID, "Failed to refresh activities. Please try again.")
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
		tg.SendMessage(ctx, chatID, defaultBotErrorMessage)
		return
	}
	msgArr := strings.Split(update.Message.Text, " ")
	if len(msgArr) != 2 {
		tg.SendMessage(ctx, chatID, setLanguageUsageMessage)
		return
	}
	language := msgArr[1]
	usr.Language = language
	err = tg.DB.UpdateUser(usr)
	if err != nil {
		slog.Error("failed to update user language", "err", err, "userID", usr.ID, "language", language)
		tg.SendMessage(ctx, chatID, defaultBotErrorMessage)
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

func (tg *Telegram) testPromptHandler(ctx context.Context, _ *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	user, err := tg.DB.GetUserByChatId(chatID)
	if err != nil {
		tg.SendMessage(ctx, chatID, "User not found. Please authenticate first.")
		return
	}

	activityType, prompt, ok := parseTestPromptCommand(update.Message.Text)
	if !ok {
		tg.SendMessage(ctx, chatID, "Usage: /test_prompt <type> <prompt>")
		return
	}

	// Create a temporary UserActivity (fill only required fields)
	activity := dbModels.UserActivity{
		ActivityType: activityType,
		Name:         "default",
		UserID:       user.ID,
	}

	aiResp, err := tg.AI.GenerateBetterNamesWithCustomizedPrompt(activity, user.Language, prompt)
	if err != nil {
		slog.Error("Error while generating names", "err", err)
		tg.SendMessage(ctx, chatID, "Failed to generate names.")
		return
	}

	tg.SendMessage(ctx, chatID, makeNamesListMessage(aiResp))
}
