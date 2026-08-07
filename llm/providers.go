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
	case "litertlm", "llamacpp":
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
	switch provider {
	case "llamacpp":
		return NewLlamacppCaller(model, executor)
	case "litertlm":
		return NewLitertLMCaller(model, executor)
	}
	cfg := getProviderConfig(provider)
	return NewOpenAICaller(cfg.BaseURL, cfg.APIKey, model, executor)
}

func NewProviderCallerRaw(provider, model string, executor ToolExecutor) RawCaller {
	switch provider {
	case "llamacpp":
		return NewLlamacppCaller(model, executor)
	case "litertlm":
		return NewLitertLMCaller(model, executor)
	}
	cfg := getProviderConfig(provider)
	return NewOpenAICaller(cfg.BaseURL, cfg.APIKey, model, executor)
}

func (a *Agent) CallLLM(ctx context.Context, executor ToolExecutor, messages []Message) ([]Message, error) {
	caller := NewProviderCaller(a.Provider, a.Model, executor)
	if lc, ok := caller.(*LitertLMCaller); ok {
		lc.Backend = a.Backend
	}
	return caller.Call(ctx, a.Prompt, messages, a.Tools)
}
