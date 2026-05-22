package llm

import (
	"context"
	"os"
)

type LlamacppCaller struct {
	inner *OpenAICaller
}

func NewLlamacppCaller(model string, executor ToolExecutor) *LlamacppCaller {
	baseURL := os.Getenv("LLAMACPP_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return &LlamacppCaller{
		inner: NewOpenAICaller(baseURL+"/v1", "", model, executor),
	}
}

func (l *LlamacppCaller) Call(ctx context.Context, systemPrompt string, messages []Message, tools []any) ([]Message, error) {
	return l.inner.Call(ctx, systemPrompt, messages, tools)
}
