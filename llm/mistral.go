package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type MistralCaller struct {
	client   *http.Client
	model    string
	apiKey   string
	executor ToolExecutor
}

type MistralMessage struct {
	Role      string            `json:"role"`
	Content   string            `json:"content"`
	ToolCalls []MistralToolCall `json:"tool_calls,omitempty"`
}

type MistralToolCall struct {
	ID       string                  `json:"id"`
	Type     string                  `json:"type"`
	Function MistralToolCallFunction `json:"function"`
}

type MistralToolCallFunction struct {
	Name      string `json:"name"`
	Arguments any    `json:"arguments"`
}

type MistralTool struct {
	Type     string              `json:"type"`
	Function MistralToolFunction `json:"function"`
}

type MistralToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  struct {
		Type       string `json:"type"`
		Properties map[string]struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"properties"`
		Required []string `json:"required"`
	} `json:"parameters"`
}

type MistralRequest struct {
	Model       string           `json:"model"`
	Messages    []MistralMessage `json:"messages"`
	Tools       []MistralTool    `json:"tools,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
}

type MistralResponse struct {
	Choices []struct {
		Message MistralMessage `json:"message"`
	} `json:"choices"`
}

type MistralToolUse struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func NewMistralCaller(client *http.Client, model, apiKey string, executor ToolExecutor) *MistralCaller {
	return &MistralCaller{
		client:   client,
		model:    model,
		apiKey:   apiKey,
		executor: executor,
	}
}

func (m *MistralCaller) Call(ctx context.Context, systemPrompt string, messages []Message) ([]Message, error) {
	if m.client == nil {
		return nil, fmt.Errorf("Mistral client is not initialized")
	}

	allMessages := []MistralMessage{
		{Role: "system", Content: systemPrompt},
	}
	for _, msg := range messages {
		role := msg.Role
		if role == "assistant" {
			role = "assistant"
		}
		allMessages = append(allMessages, MistralMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	runCommandTool := MistralTool{
		Type: "function",
		Function: MistralToolFunction{
			Name:        "RunCommand",
			Description: "Execute a shell command and return its output",
		},
	}
	runCommandTool.Function.Parameters.Type = "object"
	runCommandTool.Function.Parameters.Properties = map[string]struct {
		Type        string `json:"type"`
		Description string `json:"description"`
	}{
		"command": {
			Type:        "string",
			Description: "The shell command to execute (e.g., 'ls -la', 'echo hello')",
		},
	}
	runCommandTool.Function.Parameters.Required = []string{"command"}

	originalCount := len(allMessages)

	for {
		reqBody := MistralRequest{
			Model:    m.model,
			Messages: allMessages,
			Tools:    []MistralTool{runCommandTool},
		}

		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", "https://api.mistral.ai/v1/chat/completions", bytes.NewBuffer(jsonBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+m.apiKey)

		resp, err := m.client.Do(req)
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

		var mistralResp MistralResponse
		if err := json.Unmarshal(body, &mistralResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		if len(mistralResp.Choices) == 0 {
			return nil, fmt.Errorf("empty response from Mistral")
		}

		assistantMsg := mistralResp.Choices[0].Message

		allMessages = append(allMessages, assistantMsg)

		toolCalls := m.extractToolCalls(assistantMsg)
		if len(toolCalls) == 0 {
			var result []Message
			for i := originalCount; i < len(allMessages); i++ {
				result = append(result, Message{
					Role:    allMessages[i].Role,
					Content: allMessages[i].Content,
				})
			}
			return result, nil
		}

		for _, tc := range toolCalls {
			if tc.Function.Name == "RunCommand" {
				argsStr, ok := tc.Function.Arguments.(string)
				if !ok {
					result := "Error: Invalid tool arguments"
					allMessages = append(allMessages, MistralMessage{Role: "tool", Content: result})
					continue
				}

				var args map[string]any
				if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
					result := "Error: Invalid tool arguments"
					allMessages = append(allMessages, MistralMessage{Role: "tool", Content: result})
					continue
				}

				cmd, ok := args["command"].(string)
				if !ok {
					result := "Error: Invalid tool arguments"
					allMessages = append(allMessages, MistralMessage{Role: "tool", Content: result})
					continue
				}

				skipConfirm := m.executor.IsAllowedCommand(cmd)

				if !skipConfirm {
					confirm := m.executor.AskConfirmation(cmd)
					if !confirm {
						result := "Error: Command execution denied by user"
						allMessages = append(allMessages, MistralMessage{Role: "tool", Content: result})
						continue
					}
				}

				call := ToolCall{
					Name:      "RunCommand",
					Arguments: map[string]any{"command": cmd},
				}
				output, err := m.executor.ExecuteTool(call)
				if err != nil {
					result := fmt.Sprintf("Error: %v\nOutput: %s", err, output)
					allMessages = append(allMessages, MistralMessage{Role: "tool", Content: result})
				} else {
					allMessages = append(allMessages, MistralMessage{Role: "tool", Content: output})
				}
			}
		}
	}
}

func (m *MistralCaller) extractToolCalls(msg MistralMessage) []MistralToolCall {
	var toolCalls []MistralToolCall
	for _, tc := range toolCalls {
		toolCalls = append(toolCalls, tc)
	}
	return toolCalls
}

func NewMistralClient() (*http.Client, error) {
	apiKey := os.Getenv("MISTRAL_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("MISTRAL_KEY not set in environment")
	}

	return &http.Client{}, nil
}
