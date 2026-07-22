package llm

import (
	"context"
	"os"
)

type providerConfig struct {
	baseURL string
	apiKey  string
}

func getProviderConfig(provider string) providerConfig {
	switch provider {
	case "gemini":
		return providerConfig{
			baseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
			apiKey:  os.Getenv("GEMINI_API_KEY"),
		}
	case "openrouter":
		return providerConfig{
			baseURL: "https://openrouter.ai/api/v1",
			apiKey:  os.Getenv("OPEN_ROUTE_KEY"),
		}
	case "litertlm":
		baseURL := os.Getenv("LITERTLM_BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:9379"
		}
		return providerConfig{baseURL: baseURL + "/v1"}
	default: // ollama
		baseURL := os.Getenv("OLLAMA_HOST")
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return providerConfig{baseURL: baseURL + "/v1"}
	}
}

func NewProviderCaller(provider, model string, executor ToolExecutor) Caller {
	cfg := getProviderConfig(provider)
	return NewOpenAICaller(cfg.baseURL, cfg.apiKey, model, executor)
}

func (a *Agent) CallLLM(ctx context.Context, executor ToolExecutor, messages []Message) ([]Message, error) {
	caller := NewProviderCaller(a.Provider, a.Model, executor)
	return caller.Call(ctx, a.Prompt, messages, a.Tools)
}
