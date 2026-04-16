package llm

import (
	"context"
	"fmt"

	"github.com/ollama/ollama/api"
)

type ToolCall struct {
	Name      string
	Arguments map[string]any
}

type ToolExecutor interface {
	ExecuteTool(call ToolCall) (string, error)
	IsAllowedCommand(cmd string) bool
	AskConfirmation(cmd string) bool
}

type OllamaCaller struct {
	client   *api.Client
	model    string
	executor ToolExecutor
}

func NewOllamaCaller(client *api.Client, model string, executor ToolExecutor) *OllamaCaller {
	return &OllamaCaller{
		client:   client,
		model:    model,
		executor: executor,
	}
}

func (o *OllamaCaller) Call(ctx context.Context, systemPrompt string, messages []api.Message) ([]api.Message, error) {
	if o.client == nil {
		return nil, fmt.Errorf("Ollama client is not initialized")
	}

	runCommandTool := api.Tool{
		Type: "function",
		Function: api.ToolFunction{
			Name:        "RunCommand",
			Description: "Execute a shell command and return its output",
			Parameters: api.ToolFunctionParameters{
				Type:     "object",
				Required: []string{"command"},
				Properties: map[string]api.ToolProperty{
					"command": {
						Type:        api.PropertyType{"string"},
						Description: "The shell command to execute (e.g., 'ls -la', 'echo hello')",
					},
				},
			},
		},
	}

	writeFileTool := api.Tool{
		Type: "function",
		Function: api.ToolFunction{
			Name:        "WriteFile",
			Description: "Write content to a file at the specified path",
			Parameters: api.ToolFunctionParameters{
				Type:     "object",
				Required: []string{"path", "content"},
				Properties: map[string]api.ToolProperty{
					"path": {
						Type:        api.PropertyType{"string"},
						Description: "The absolute or relative path to the file",
					},
					"content": {
						Type:        api.PropertyType{"string"},
						Description: "The content to write to the file",
					},
				},
			},
		},
	}

	allMessages := []api.Message{
		{Role: "system", Content: systemPrompt},
	}
	allMessages = append(allMessages, messages...)
	originalCount := len(allMessages)

	for {
		req := &api.ChatRequest{
			Model:    o.model,
			Messages: allMessages,
			Tools:    []api.Tool{runCommandTool, writeFileTool},
			Stream:   new(bool),
		}
		*req.Stream = false

		var response api.ChatResponse
		respFunc := func(resp api.ChatResponse) error {
			response = resp
			return nil
		}

		if err := o.client.Chat(ctx, req, respFunc); err != nil {
			return nil, fmt.Errorf("chat error: %w", err)
		}

		allMessages = append(allMessages, response.Message)

		if len(response.Message.ToolCalls) == 0 {
			return allMessages[originalCount:], nil
		}

		for _, tc := range response.Message.ToolCalls {
			switch tc.Function.Name {
			case "RunCommand":
				cmd, ok := tc.Function.Arguments["command"].(string)
				if !ok {
					result := "Error: Invalid tool arguments"
					allMessages = append(allMessages, api.Message{Role: "tool", Content: result})
					continue
				}

				call := ToolCall{
					Name:      "RunCommand",
					Arguments: map[string]any{"command": cmd},
				}
				output, err := o.executor.ExecuteTool(call)
				if err != nil {
					result := fmt.Sprintf("Error: %v\nOutput: %s", err, output)
					allMessages = append(allMessages, api.Message{Role: "tool", Content: result})
				} else {
					allMessages = append(allMessages, api.Message{Role: "tool", Content: output})
				}

			case "WriteFile":
				path, ok1 := tc.Function.Arguments["path"].(string)
				content, ok2 := tc.Function.Arguments["content"].(string)
				if !ok1 || !ok2 {
					result := "Error: Invalid tool arguments"
					allMessages = append(allMessages, api.Message{Role: "tool", Content: result})
					continue
				}

				call := ToolCall{
					Name:      "WriteFile",
					Arguments: map[string]any{"path": path, "content": content},
				}
				output, err := o.executor.ExecuteTool(call)
				if err != nil {
					result := fmt.Sprintf("Error: %v\nOutput: %s", err, output)
					allMessages = append(allMessages, api.Message{Role: "tool", Content: result})
				} else {
					allMessages = append(allMessages, api.Message{Role: "tool", Content: output})
				}
			}
		}
	}
}
