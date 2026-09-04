package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"ai-shell/stats"
)

type OpenAICaller struct {
	BaseURL  string
	APIKey   string
	Model    string
	Executor ToolExecutor
	Client   *http.Client
}

type OpenAIRequest struct {
	Model          string    `json:"model"`
	Messages       []Message `json:"messages"`
	Tools          []any     `json:"tools,omitempty"`
	Temperature    float64   `json:"temperature,omitempty"`
	ResponseFormat any       `json:"response_format,omitempty"`
}

type OpenAIResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage *OpenAIUsage `json:"usage,omitempty"`
}

type OpenAIUsage struct {
	PromptTokens            int                           `json:"prompt_tokens"`
	CompletionTokens        int                           `json:"completion_tokens"`
	TotalTokens             int                           `json:"total_tokens"`
	Cost                    float64                       `json:"cost"`
	PromptTokensDetails     *OpenAIUsagePromptDetails     `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *OpenAIUsageCompletionDetails `json:"completion_tokens_details,omitempty"`
}

type OpenAIUsagePromptDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type OpenAIUsageCompletionDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

const openAIMaxToolHops = 10

func NewOpenAICaller(baseURL, apiKey, model string, executor ToolExecutor) *OpenAICaller {
	return &OpenAICaller{
		BaseURL:  baseURL,
		APIKey:   apiKey,
		Model:    model,
		Executor: executor,
		Client:   &http.Client{Timeout: 60 * time.Second},
	}
}

func (o *OpenAICaller) isOpenRouter() bool {
	return strings.Contains(o.BaseURL, "openrouter.ai")
}

func (o *OpenAICaller) providerName() string {
	switch {
	case o.isOpenRouter():
		return "openrouter"
	case strings.Contains(o.BaseURL, "generativelanguage.googleapis.com"):
		return "gemini"
	default:
		return "ollama"
	}
}

// recordUsage persists a call's token usage so it can be reported by the
// stats command, aggregating by provider and model.
func (o *OpenAICaller) recordUsage(u *OpenAIUsage) {
	usage := stats.Usage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
		Cost:             u.Cost,
	}
	if u.PromptTokensDetails != nil {
		usage.CachedTokens = u.PromptTokensDetails.CachedTokens
	}
	if u.CompletionTokensDetails != nil {
		usage.ReasoningTokens = u.CompletionTokensDetails.ReasoningTokens
	}
	stats.RecordUsage(o.providerName(), o.Model, usage)
}

func (o *OpenAICaller) Call(ctx context.Context, systemPrompt string, messages []Message, tools []any) ([]Message, error) {
	return o.call(ctx, systemPrompt, messages, tools, nil)
}

func (o *OpenAICaller) CallStructured(ctx context.Context, systemPrompt string, messages []Message, tools []any, responseFormat any) ([]Message, error) {
	return o.call(ctx, systemPrompt, messages, tools, responseFormat)
}

func (o *OpenAICaller) call(ctx context.Context, systemPrompt string, messages []Message, tools []any, responseFormat any) ([]Message, error) {
	allMessages := []Message{
		{Role: "system", Content: systemPrompt},
	}
	allMessages = append(allMessages, messages...)

	originalCount := len(allMessages)

	for hops := 0; hops < openAIMaxToolHops; hops++ {
		rf := responseFormat
		// Structured output mixes poorly with tool calls; omit after first hop
		// or when tools are still expected.
		if hops > 0 {
			rf = nil
		}
		reqBody := OpenAIRequest{
			Model:          o.Model,
			Messages:       allMessages,
			Tools:          tools,
			ResponseFormat: rf,
		}

		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		baseURL := strings.TrimSuffix(o.BaseURL, "/")
		req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
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

		body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
		}

		var openAIResp OpenAIResponse
		if err := json.Unmarshal(body, &openAIResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		if o.isOpenRouter() && openAIResp.Usage != nil {
			u := openAIResp.Usage
			args := []any{
				"prompt_tokens", u.PromptTokens,
				"completion_tokens", u.CompletionTokens,
				"total_tokens", u.TotalTokens,
			}
			if u.Cost > 0 {
				args = append(args, "cost", u.Cost)
			}
			if u.PromptTokensDetails != nil {
				args = append(args, "cached_tokens", u.PromptTokensDetails.CachedTokens)
			}
			if u.CompletionTokensDetails != nil {
				args = append(args, "reasoning_tokens", u.CompletionTokensDetails.ReasoningTokens)
			}
			slog.Debug("openrouter usage", args...)
		}

		if openAIResp.Usage != nil {
			o.recordUsage(openAIResp.Usage)
		}

		if len(openAIResp.Choices) == 0 {
			return nil, fmt.Errorf("empty response from LLM")
		}

		assistantMsg := openAIResp.Choices[0].Message
		allMessages = append(allMessages, assistantMsg)

		if len(assistantMsg.ToolCalls) == 0 {
			return allMessages[originalCount:], nil
		}

		for i, tc := range assistantMsg.ToolCalls {
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
			toolID := tc.ID
			if toolID == "" {
				toolID = fmt.Sprintf("call_%d_%d", hops, i)
			}

			allMessages = append(allMessages, Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: toolID,
			})
		}
	}
	return nil, fmt.Errorf("tool call hop limit exceeded (%d)", openAIMaxToolHops)
}
