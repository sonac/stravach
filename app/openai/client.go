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
	"stravach/app/utils"
	"strconv"
	"strings"
)

type AIRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ResponseFormat struct {
	Type       string     `json:"type,omitempty"`
	JSONSchema JSONSchema `json:"json_schema,omitempty"`
}

type JSONSchema struct {
	Schema Schema `json:"schema"`
	Name   string `json:"name"`
}

type Schema struct {
	Type string `json:"type"`
}

type Response struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type CompletionMessage struct {
	Content    Content `json:"content"`
	Role       string  `json:"role"`
	StopReason string  `json:"stop_reason"`
}

type MetaResponse struct {
	CompletionMessage CompletionMessage `json:"completion_message"`
	Metrics           interface{}       `json:"metrics"`
}

type OpenAI struct {
	ApiKey string
}

// IsActivityNameSuggestion returns true if the message contains suggestions for activity names, using OpenAI
func (ai *OpenAI) IsActivityNameSuggestion(message string) (bool, error) {
	prompt := "Does the following message contain suggestions for names for activities? Answer only 'yes' or 'no'. Message: " + message
	resp, err := ai.sendRequest(prompt)
	if err != nil || len(resp) == 0 {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(resp[0]))
	return strings.HasPrefix(answer, "yes"), nil
}

func NewClient() *OpenAI {
	apiKey := os.Getenv("LLAMA_API_KEY")
	return &OpenAI{
		ApiKey: apiKey,
	}
}

func (ai *OpenAI) GenerateBetterNames(activity models.UserActivity, language string) ([]string, error) {
	prompt := fmt.Sprintf("Generate a several, new-line separated funny names for the following activity: %s, of type %s, duration: %d seconds in %s language. This is for my Strava.",
		activity.Name, activity.ActivityType, activity.ElapsedTime, language)
	return ai.sendRequest(prompt)
}

func (ai *OpenAI) GenerateBetterNamesWithCustomizedPrompt(activity models.UserActivity, lang, prompt string) ([]string, error) {
	fullPrompt := fmt.Sprintf("Generate up to three, new-line separated names for the following activity: %s, of type %s. "+
		"Language: %s. I want this to be used in names: '%s'. If you think that what I suggested can be a name - just return it. "+
		"If it's a long message that contains something that looks like a name - return it in formatted way (e.g. 'evening run' should be 'Evening Run')."+
		"In any other way return just new names, nothing else should be included in the response",
		activity.Name, activity.ActivityType, lang, prompt)
	return ai.sendRequest(fullPrompt)
}

func (ai *OpenAI) FormatActivityName(name string) (string, error) {
	prompt := fmt.Sprintf("Format name for a Strava acitvity, return only new name: %s", name)
	res, err := ai.sendRequest(prompt)
	if err != nil || len(res) == 0 {
		return "", err
	}
	return res[0], nil
}

func (ai *OpenAI) CheckIfItsAName(msg string) (bool, error) {
	fullPrompt := fmt.Sprintf("Does this look like a name for a Strava activity? %s", msg)
	resp, err := ai.sendStructuredRequest(fullPrompt)
	if err != nil {
		slog.Error("error while sending request to AI")
		return false, err
	}
	return strconv.ParseBool(resp)
}

func (ai *OpenAI) sendStructuredRequest(prompt string) (string, error) {
	slog.Debug(prompt)
	messages := []Message{
		{
			Role:    "system",
			Content: "You are a helpful assistant",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}
	responseFormat := ResponseFormat{
		Type: "json_schema",
		JSONSchema: JSONSchema{
			Schema: Schema{
				Type: "boolean",
			},
			Name: "Snitch",
		},
	}
	requestBody, err := json.Marshal(AIRequest{
		Model:          "Llama-3.3-70B-Instruct",
		Messages:       messages,
		ResponseFormat: &responseFormat,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.llama.com/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ai.ApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("OpenAI API returned non 200: %s", resp.Status)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil
	}

	slog.Debug(string(body))

	var metaAIResponse MetaResponse
	err = json.Unmarshal(body, &metaAIResponse)
	if err != nil {
		return "", err
	}

	if metaAIResponse.CompletionMessage.Content.Text == "" {
		return "", fmt.Errorf("no respnse from OpenAI")
	}

	return metaAIResponse.CompletionMessage.Content.Text, nil
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
		Model:    "Llama-4-Maverick-17B-128E-Instruct-FP8",
		Messages: messages,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://api.llama.com/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ai.ApiKey)

	slog.Debug(fmt.Sprintf("%+v", req))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		utils.DebugResponse(resp)
		return nil, fmt.Errorf("AI API returned non 200: %s", resp.Status)
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

	slog.Debug(string(body))

	var metaAIResponse MetaResponse
	err = json.Unmarshal(body, &metaAIResponse)
	if err != nil {
		return nil, err
	}

	if metaAIResponse.CompletionMessage.Content.Text == "" {
		return nil, fmt.Errorf("no respnse from OpenAI")
	}

	names := strings.Split(metaAIResponse.CompletionMessage.Content.Text, "\n")
	return names, nil
}
