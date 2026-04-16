package llm

import (
	"context"
	"fmt"
	"os"
	"strings"

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
	if g.client == nil {
		return nil, fmt.Errorf("Gemini client is not initialized")
	}

	tools := &genai.Tool{
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
			{
				Name:        "WriteFile",
				Description: "Write content to a file at the specified path",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"path": {
							Type:        genai.TypeString,
							Description: "The absolute or relative path to the file",
						},
						"content": {
							Type:        genai.TypeString,
							Description: "The content to write to the file",
						},
					},
					Required: []string{"path", "content"},
				},
			},
		},
	}

	config := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{tools},
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: systemPrompt}},
		},
	}

	history := make([]*genai.Content, 0)
	for i := 0; i < len(messages)-1; i++ {
		msg := messages[i]
		role := "user"
		if msg.Role == "assistant" {
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

	lastUserMsg := messages[len(messages)-1].Content
	newMessages := []Message{}

	resp, err := chat.SendMessage(ctx, genai.Part{Text: lastUserMsg})
	if err != nil {
		return nil, fmt.Errorf("send message error: %w", err)
	}

	for {
		hasToolCall, toolCallPart, toolResult := g.processTools(resp)
		
		if !hasToolCall {
			responseText := extractTextFromResponse(resp)
			if responseText == "" {
				if len(resp.Candidates) > 0 {
					newMessages = append(newMessages, Message{Role: "assistant", Content: "(no text response)"})
					return newMessages, nil
				}
				return nil, fmt.Errorf("empty response from Gemini")
			}
			newMessages = append(newMessages, Message{Role: "assistant", Content: responseText})
			return newMessages, nil
		}

		newMessages = append(newMessages, Message{Role: "tool", Content: toolResult})

		resp, err = chat.SendMessage(ctx, genai.Part{
			FunctionResponse: &genai.FunctionResponse{
				Name: toolCallPart.FunctionCall.Name,
				Response: map[string]any{
					"output": toolResult,
				},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("error sending tool response: %w", err)
		}
	}
}

func extractTextFromResponse(resp *genai.GenerateContentResponse) string {
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return ""
	}

	var sb strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			sb.WriteString(part.Text)
		}
	}
	return sb.String()
}

func (g *GeminiCaller) processTools(resp *genai.GenerateContentResponse) (bool, *genai.Part, string) {
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return false, nil, ""
	}

	content := resp.Candidates[0].Content
	var toolPart *genai.Part
	for _, part := range content.Parts {
		if part.FunctionCall != nil {
			toolPart = part
			break
		}
	}

	if toolPart == nil {
		return false, nil, ""
	}

	fc := toolPart.FunctionCall
	switch fc.Name {
	case "RunCommand":
		args := fc.Args
		if args == nil {
			return true, toolPart, "Error: Invalid tool arguments"
		}

		cmd, ok := args["command"].(string)
		if !ok {
			return true, toolPart, "Error: Invalid tool arguments"
		}

		call := ToolCall{
			Name:      "RunCommand",
			Arguments: map[string]any{"command": cmd},
		}
		output, err := g.executor.ExecuteTool(call)
		if err != nil {
			return true, toolPart, fmt.Sprintf("Error: %v\nOutput: %s", err, output)
		}
		return true, toolPart, output

	case "WriteFile":
		args := fc.Args
		if args == nil {
			return true, toolPart, "Error: Invalid tool arguments"
		}

		path, ok1 := args["path"].(string)
		content, ok2 := args["content"].(string)
		if !ok1 || !ok2 {
			return true, toolPart, "Error: Invalid tool arguments"
		}

		call := ToolCall{
			Name:      "WriteFile",
			Arguments: map[string]any{"path": path, "content": content},
		}
		output, err := g.executor.ExecuteTool(call)
		if err != nil {
			return true, toolPart, fmt.Sprintf("Error: %v\nOutput: %s", err, output)
		}
		return true, toolPart, output

	default:
		return false, toolPart, "Error: Unknown tool"
	}
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
