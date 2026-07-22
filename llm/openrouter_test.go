package llm

import (
	"os"
	"testing"
)

func TestNewProviderCallerOpenRouter(t *testing.T) {
	key := "test-key"
	os.Setenv("OPEN_ROUTE_KEY", key)
	defer os.Unsetenv("OPEN_ROUTE_KEY")

	model := "test-model"
	caller := NewProviderCaller("openrouter", model, nil)

	oac, ok := caller.(*OpenAICaller)
	if !ok {
		t.Fatalf("Expected *OpenAICaller, got %T", caller)
	}
	if oac.APIKey != key {
		t.Errorf("Expected APIKey %q, got %q", key, oac.APIKey)
	}
	if oac.BaseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("Expected BaseURL %q, got %q", "https://openrouter.ai/api/v1", oac.BaseURL)
	}
	if oac.Model != model {
		t.Errorf("Expected Model %q, got %q", model, oac.Model)
	}
}
