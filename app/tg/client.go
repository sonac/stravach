package tg

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Telegram struct {
	Bot *bot.Bot
}

func NewTelegramClient(apiKey string) (*Telegram, error) {
	options := []bot.Option{
		bot.WithDefaultHandler(handler),
	}
	b, err := bot.New(apiKey, options...)
	if err != nil {
		slog.Error("error occured when spinning up the bot")
		return nil, err
	}
	return &Telegram{
		Bot: b,
	}, nil
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "I am Strava bot! Nice to meet you!",
	})
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
		slog.Error("error while sending a message: ", err)
	}
}
