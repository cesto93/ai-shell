package llm

import (
	"context"
	"os"
)

type ProviderConfig struct {
	BaseURL string
	APIKey  string
}

func getProviderConfig(provider string) ProviderConfig {
	switch provider {
	case "gemini":
		return ProviderConfig{
			BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
			APIKey:  os.Getenv("GEMINI_API_KEY"),
		}
	case "openrouter":
		return ProviderConfig{
			BaseURL: "https://openrouter.ai/api/v1",
			APIKey:  os.Getenv("OPEN_ROUTE_KEY"),
		}
	case "litertlm":
		baseURL := os.Getenv("LITERTLM_BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:9379"
		}
		return ProviderConfig{BaseURL: baseURL + "/v1"}
	case "llamacpp":
		return ProviderConfig{}
	default: // ollama
		baseURL := os.Getenv("OLLAMA_HOST")
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return ProviderConfig{BaseURL: baseURL + "/v1"}
	}
}

func NewProviderCaller(provider, model string, executor ToolExecutor) Caller {
	if provider == "llamacpp" {
		return NewLlamacppCaller(model, executor)
	}
	cfg := getProviderConfig(provider)
	return NewOpenAICaller(cfg.BaseURL, cfg.APIKey, model, executor)
}

func NewProviderCallerRaw(provider, model string, executor ToolExecutor) RawCaller {
	if provider == "llamacpp" {
		return NewLlamacppCaller(model, executor)
	}
	cfg := getProviderConfig(provider)
	return NewOpenAICaller(cfg.BaseURL, cfg.APIKey, model, executor)
}

func (a *Agent) CallLLM(ctx context.Context, executor ToolExecutor, messages []Message) ([]Message, error) {
	caller := NewProviderCaller(a.Provider, a.Model, executor)
	return caller.Call(ctx, a.Prompt, messages, a.Tools)
}
