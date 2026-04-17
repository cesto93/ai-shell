package llm

import (
	"context"
	"os"
)

type MistralCaller struct {
	inner *OpenAICaller
}

func NewMistralCaller(model string, executor ToolExecutor) *MistralCaller {
	apiKey := os.Getenv("MISTRAL_KEY")
	return &MistralCaller{
		inner: NewOpenAICaller("https://api.mistral.ai/v1", apiKey, model, executor),
	}
}

func (m *MistralCaller) Call(ctx context.Context, systemPrompt string, messages []Message) ([]Message, error) {
	return m.inner.Call(ctx, systemPrompt, messages)
}
