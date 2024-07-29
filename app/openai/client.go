package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"stravach/app/storage/models"
	"strings"
)

type OpenAIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIResponse struct {
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

func (ai *OpenAI) GenerateBetterNames(activity models.UserActivity) ([]string, error) {
	prompt := fmt.Sprintf("Generate a several, new-line separated witty name for the following activity: %s, %s, duration: %d seconds",
		activity.Name, activity.ActivityType, activity.ElapsedTime)
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
	requestBody, err := json.Marshal(OpenAIRequest{
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
		return nil, fmt.Errorf("OpenAI API returned non 200: ", resp.Status)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil
	}

	var openAIResponse OpenAIResponse
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
