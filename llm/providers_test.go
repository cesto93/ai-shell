package llm

import (
	"os"
	"testing"
)

func TestGetProviderConfigGemini(t *testing.T) {
	os.Setenv("GEMINI_API_KEY", "gemini-test-key")
	defer os.Unsetenv("GEMINI_API_KEY")

	cfg := getProviderConfig("gemini")

	if cfg.baseURL != "https://generativelanguage.googleapis.com/v1beta/openai" {
		t.Errorf("gemini baseURL = %q, want %q", cfg.baseURL, "https://generativelanguage.googleapis.com/v1beta/openai")
	}
	if cfg.apiKey != "gemini-test-key" {
		t.Errorf("gemini apiKey = %q, want %q", cfg.apiKey, "gemini-test-key")
	}
}

func TestGetProviderConfigOpenRouter(t *testing.T) {
	os.Setenv("OPEN_ROUTE_KEY", "or-test-key")
	defer os.Unsetenv("OPEN_ROUTE_KEY")

	cfg := getProviderConfig("openrouter")

	if cfg.baseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("openrouter baseURL = %q, want %q", cfg.baseURL, "https://openrouter.ai/api/v1")
	}
	if cfg.apiKey != "or-test-key" {
		t.Errorf("openrouter apiKey = %q, want %q", cfg.apiKey, "or-test-key")
	}
}

func TestGetProviderConfigLiteRTLM(t *testing.T) {
	cfg := getProviderConfig("litertlm")

	if cfg.baseURL != "http://localhost:9379/v1" {
		t.Errorf("litertlm baseURL = %q, want %q", cfg.baseURL, "http://localhost:9379/v1")
	}
}

func TestGetProviderConfigLiteRTLMCustomURL(t *testing.T) {
	os.Setenv("LITERTLM_BASE_URL", "http://custom:8080")
	defer os.Unsetenv("LITERTLM_BASE_URL")

	cfg := getProviderConfig("litertlm")

	if cfg.baseURL != "http://custom:8080/v1" {
		t.Errorf("litertlm baseURL = %q, want %q", cfg.baseURL, "http://custom:8080/v1")
	}
}

func TestGetProviderConfigOllamaDefault(t *testing.T) {
	os.Unsetenv("OLLAMA_HOST")

	cfg := getProviderConfig("ollama")

	if cfg.baseURL != "http://localhost:11434/v1" {
		t.Errorf("ollama baseURL = %q, want %q", cfg.baseURL, "http://localhost:11434/v1")
	}
}

func TestGetProviderConfigOllamaCustomHost(t *testing.T) {
	os.Setenv("OLLAMA_HOST", "http://custom-host:5000")
	defer os.Unsetenv("OLLAMA_HOST")

	cfg := getProviderConfig("ollama")

	if cfg.baseURL != "http://custom-host:5000/v1" {
		t.Errorf("ollama baseURL = %q, want %q", cfg.baseURL, "http://custom-host:5000/v1")
	}
}

func TestGetProviderConfigUnknownDefaultsToOllama(t *testing.T) {
	os.Unsetenv("OLLAMA_HOST")

	cfg := getProviderConfig("unknown-provider")

	if cfg.baseURL != "http://localhost:11434/v1" {
		t.Errorf("unknown provider baseURL = %q, want %q", cfg.baseURL, "http://localhost:11434/v1")
	}
}

func TestNewProviderCallerGemini(t *testing.T) {
	os.Setenv("GEMINI_API_KEY", "test-key")
	defer os.Unsetenv("GEMINI_API_KEY")

	caller := NewProviderCaller("gemini", "gemini-3-flash-preview", nil)

	oac, ok := caller.(*OpenAICaller)
	if !ok {
		t.Fatalf("Expected *OpenAICaller, got %T", caller)
	}
	if oac.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want %q", oac.APIKey, "test-key")
	}
	if oac.Model != "gemini-3-flash-preview" {
		t.Errorf("Model = %q, want %q", oac.Model, "gemini-3-flash-preview")
	}
}

func TestNewProviderCallerLiteRTLM(t *testing.T) {
	caller := NewProviderCaller("litertlm", "test-model", nil)

	oac, ok := caller.(*OpenAICaller)
	if !ok {
		t.Fatalf("Expected *OpenAICaller, got %T", caller)
	}
	if oac.BaseURL != "http://localhost:9379/v1" {
		t.Errorf("BaseURL = %q, want %q", oac.BaseURL, "http://localhost:9379/v1")
	}
	if oac.Model != "test-model" {
		t.Errorf("Model = %q, want %q", oac.Model, "test-model")
	}
}

func TestNewProviderCallerOllama(t *testing.T) {
	os.Unsetenv("OLLAMA_HOST")

	caller := NewProviderCaller("ollama", "granite4:3b-h", nil)

	oac, ok := caller.(*OpenAICaller)
	if !ok {
		t.Fatalf("Expected *OpenAICaller, got %T", caller)
	}
	if oac.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("BaseURL = %q, want %q", oac.BaseURL, "http://localhost:11434/v1")
	}
}
