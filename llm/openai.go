package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type OpenAICaller struct {
	BaseURL  string
	APIKey   string
	Model    string
	Executor ToolExecutor
	Client   *http.Client
}

type OpenAIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []any     `json:"tools,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

type OpenAIResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

func NewOpenAICaller(baseURL, apiKey, model string, executor ToolExecutor) *OpenAICaller {
	return &OpenAICaller{
		BaseURL:  baseURL,
		APIKey:   apiKey,
		Model:    model,
		Executor: executor,
		Client:   &http.Client{},
	}
}

func (o *OpenAICaller) Call(ctx context.Context, systemPrompt string, messages []Message) ([]Message, error) {
	allMessages := []Message{
		{Role: "system", Content: systemPrompt},
	}
	allMessages = append(allMessages, messages...)

	tools := GetDefaultTools()

	originalCount := len(allMessages)

	for {
		reqBody := OpenAIRequest{
			Model:    o.Model,
			Messages: allMessages,
			Tools:    tools,
		}

		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		if o.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+o.APIKey)
		}

		resp, err := o.Client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
		}

		var openAIResp OpenAIResponse
		if err := json.Unmarshal(body, &openAIResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		if len(openAIResp.Choices) == 0 {
			return nil, fmt.Errorf("empty response from LLM")
		}

		assistantMsg := openAIResp.Choices[0].Message
		allMessages = append(allMessages, assistantMsg)

		if len(assistantMsg.ToolCalls) == 0 {
			return allMessages[originalCount:], nil
		}

		for _, tc := range assistantMsg.ToolCalls {
			var result string
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				result = fmt.Sprintf("Error: Invalid tool arguments: %v", err)
			} else {
				call := ToolCall{
					Name:      tc.Function.Name,
					Arguments: args,
				}
				output, err := o.Executor.ExecuteTool(call)
				if err != nil {
					result = fmt.Sprintf("Error: %v\nOutput: %s", err, output)
				} else {
					result = output
				}
			}

			allMessages = append(allMessages, Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}
}
