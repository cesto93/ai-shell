package llm

import (
	"context"
	"os"
)

type OllamaCaller struct {
	inner *OpenAICaller
}

func NewOllamaCaller(model string, executor ToolExecutor) *OllamaCaller {
	baseURL := os.Getenv("OLLAMA_HOST")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaCaller{
		inner: NewOpenAICaller(baseURL+"/v1", "", model, executor),
	}
}

func (o *OllamaCaller) Call(ctx context.Context, systemPrompt string, messages []Message) ([]Message, error) {
	return o.inner.Call(ctx, systemPrompt, messages)
}
