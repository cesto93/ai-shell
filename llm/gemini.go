package llm

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/genai"
)

type GeminiCaller struct {
	client   *genai.Client
	model    string
	executor ToolExecutor
}

func NewGeminiCaller(client *genai.Client, model string, executor ToolExecutor) *GeminiCaller {
	return &GeminiCaller{
		client:   client,
		model:    model,
		executor: executor,
	}
}

func (g *GeminiCaller) Call(ctx context.Context, systemPrompt string, messages []Message) ([]Message, error) {
	runCommandTool := []*genai.Tool{
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        "RunCommand",
					Description: "Execute a shell command and return its output",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"command": {
								Type:        genai.TypeString,
								Description: "The shell command to execute (e.g., 'ls -la', 'echo hello')",
							},
						},
						Required: []string{"command"},
					},
				},
			},
		},
	}

	config := &genai.GenerateContentConfig{
		Tools: runCommandTool,
		ToolConfig: &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingConfigModeAuto,
			},
		},
	}

	history := make([]*genai.Content, 0, len(messages)+1)

	history = append(history, &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: systemPrompt}},
	})

	for _, msg := range messages {
		role := msg.Role
		if role == "system" {
			role = "model"
		}
		history = append(history, &genai.Content{
			Role:  role,
			Parts: []*genai.Part{{Text: msg.Content}},
		})
	}

	chat, err := g.client.Chats.Create(ctx, g.model, config, history)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat: %w", err)
	}

	for {
		lastMsg := history[len(history)-1]
		userContent := lastMsg.Parts[0].Text

		var resp *genai.GenerateContentResponse
		for resp == nil {
			resp, err = chat.SendMessage(ctx, genai.Part{Text: userContent})
			if err != nil {
				return nil, fmt.Errorf("send message error: %w", err)
			}
		}

		responseText := resp.Text()
		if responseText == "" {
			return nil, fmt.Errorf("empty response")
		}

		assistantMsg := Message{Role: "assistant", Content: responseText}
		history = append(history, &genai.Content{
			Role:  "model",
			Parts: []*genai.Part{{Text: responseText}},
		})

		hasToolCall, toolResult := g.processTools(resp, history)
		if !hasToolCall {
			return []Message{{Role: "user", Content: messages[len(messages)-1].Content}, assistantMsg}, nil
		}

		history = append(history, &genai.Content{
			Role:  "user",
			Parts: []*genai.Part{{Text: toolResult}},
		})
	}
}

func (g *GeminiCaller) processTools(resp *genai.GenerateContentResponse, history []*genai.Content) (bool, string) {
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return false, ""
	}

	content := resp.Candidates[0].Content
	if len(content.Parts) == 0 {
		return false, ""
	}

	part := content.Parts[0]
	if part.FunctionCall == nil {
		return false, ""
	}

	fc := part.FunctionCall
	if fc.Name != "RunCommand" {
		return false, ""
	}

	args := fc.Args
	if args == nil {
		return true, "Error: Invalid tool arguments"
	}

	cmd, ok := args["command"].(string)
	if !ok {
		return true, "Error: Invalid tool arguments"
	}

	skipConfirm := g.executor.IsAllowedCommand(cmd)
	if !skipConfirm {
		confirm := g.executor.AskConfirmation(cmd)
		if !confirm {
			return true, "Error: Command execution denied by user"
		}
	}

	call := ToolCall{
		Name:      "RunCommand",
		Arguments: map[string]any{"command": cmd},
	}
	output, err := g.executor.ExecuteTool(call)
	if err != nil {
		return true, fmt.Sprintf("Error: %v\nOutput: %s", err, output)
	}
	return true, output
}

type Message struct {
	Role    string
	Content string
}

func NewGeminiClient(ctx context.Context) (*genai.Client, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY not set in environment")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return client, nil
}
