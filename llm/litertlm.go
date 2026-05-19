package llm

import (
	"context"
	"os"
)

type LitertLMCaller struct {
	inner *OpenAICaller
}

func NewLitertLMCaller(model string, executor ToolExecutor) *LitertLMCaller {
	baseURL := os.Getenv("LITERTLM_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:9379"
	}

	return &LitertLMCaller{
		inner: NewOpenAICaller(baseURL, "", model, executor),
	}
}

func (l *LitertLMCaller) Call(ctx context.Context, systemPrompt string, messages []Message, tools []any) ([]Message, error) {
	return l.inner.Call(ctx, systemPrompt, messages, tools)
}
