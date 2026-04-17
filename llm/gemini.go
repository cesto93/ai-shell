package llm

import (
	"context"
	"os"
)

type GeminiCaller struct {
	inner *OpenAICaller
}

func NewGeminiCaller(model string, executor ToolExecutor) *GeminiCaller {
	apiKey := os.Getenv("GEMINI_API_KEY")
	return &GeminiCaller{
		inner: NewOpenAICaller("https://generativelanguage.googleapis.com/v1beta/openai", apiKey, model, executor),
	}
}

func (g *GeminiCaller) Call(ctx context.Context, systemPrompt string, messages []Message) ([]Message, error) {
	return g.inner.Call(ctx, systemPrompt, messages)
}
