package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"stravach/app/storage/models"
	"strings"
)

type AIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Response struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

type OpenAI struct {
	ApiKey string
}

func NewClient() *OpenAI {
	apiKey := os.Getenv("OPENAI_API_KEY")
	return &OpenAI{
		ApiKey: apiKey,
	}
}

func (ai *OpenAI) GenerateBetterNames(activity models.UserActivity, language string) ([]string, error) {
	prompt := fmt.Sprintf("Generate a several, new-line separated funny names for the following activity: %s, of type %s, duration: %d seconds in %s language. This is for my Strava.",
		activity.Name, activity.ActivityType, activity.ElapsedTime, language)
	return ai.sendRequest(prompt)
}

func (ai *OpenAI) GenerateBetterNamesWithCustomizedPrompt(activity models.UserActivity, customPrompt string) ([]string, error) {
	prompt := fmt.Sprintf("Generate a several, new-line separated names for the following activity: %s, of type %s, duration: %d seconds. Use this also as an input for prompt: %s. This is for my Strava.",
		activity.Name, activity.ActivityType, activity.ElapsedTime, customPrompt)
	return ai.sendRequest(prompt)
}

func (ai *OpenAI) sendRequest(prompt string) ([]string, error) {
	slog.Debug(prompt)
	messages := []Message{
		{
			Role:    "system",
			Content: "You are a helpful assistant that generates witty names for activities.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}
	requestBody, err := json.Marshal(AIRequest{
		Model:    "gpt-4o",
		Messages: messages,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ai.ApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OpenAI API returned non 200: %s", resp.Status)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil
	}

	var openAIResponse Response
	err = json.Unmarshal(body, &openAIResponse)
	if err != nil {
		return nil, err
	}

	if len(openAIResponse.Choices) == 0 {
		return nil, fmt.Errorf("no respnse from OpenAI")
	}

	names := strings.Split(openAIResponse.Choices[0].Message.Content, "\n")
	return names, nil
}
