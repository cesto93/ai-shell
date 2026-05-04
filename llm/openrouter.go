package llm

import (
	"context"
	"os"
)

type OpenRouterCaller struct {
	inner *OpenAICaller
}

func NewOpenRouterCaller(model string, executor ToolExecutor) *OpenRouterCaller {
	apiKey := os.Getenv("OPEN_ROUTE_KEY")
	return &OpenRouterCaller{
		inner: NewOpenAICaller("https://openrouter.ai/api/v1", apiKey, model, executor),
	}
}

func (o *OpenRouterCaller) Call(ctx context.Context, systemPrompt string, messages []Message) ([]Message, error) {
	return o.inner.Call(ctx, systemPrompt, messages)
}
