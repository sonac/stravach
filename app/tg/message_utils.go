package tg

import (
	"fmt"
	"github.com/go-telegram/bot/models"
	"strings"
)

func makeNamesListMessage(aiResp string) string {
	names := strings.Split(aiResp, "\n")
	maxOptions := 9
	var listText string
	for i, name := range names {
		listText += fmt.Sprintf("%d. %s\n", i+1, cleanName(name))
		if i == maxOptions-1 {
			break
		}
	}
	listText += "0. 🔄 Regenerate\nC. ✏️ Enter custom prompt"
	return "*Select a number with new name:*\n\n" + listText
}

func makeInlineKeyboardForNames(activityID int64, aiResp string) [][]models.InlineKeyboardButton {
	names := strings.Split(aiResp, "\n")
	maxOptions := 9
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
	return inlineKeyboard
}

func parseTestPromptCommand(text string) (activityType, prompt string, ok bool) {
	parts := strings.Fields(text)
	if len(parts) < 3 {
		return "", "", false
	}

	activityType = strings.ToLower(parts[1])
	prompt = strings.Join(parts[2:], " ")

	// If the entire prompt is enclosed in double quotes, unquote it.
	if len(prompt) > 1 && strings.HasPrefix(prompt, `"`) && strings.HasSuffix(prompt, `"`) {
		prompt = prompt[1 : len(prompt)-1]
	}

	return activityType, prompt, true
}
