package llm

import "context"

type Message struct {
	Role       string            `json:"role"`
	Content    string            `json:"content"`
	ToolCalls  []OpenAIToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
}

type OpenAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type ToolCall struct {
	Name      string
	Arguments map[string]any
}

type ToolExecutor interface {
	ExecuteTool(call ToolCall) (string, error)
	IsAllowedCommand(cmd string) bool
	AskConfirmation(cmd string) bool
}

type Caller interface {
	Call(ctx context.Context, systemPrompt string, messages []Message) ([]Message, error)
}
