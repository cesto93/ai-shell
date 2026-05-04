package llm

import (
	"os"
	"testing"
)

func TestNewOpenRouterCaller(t *testing.T) {
	key := "test-key"
	os.Setenv("OPEN_ROUTE_KEY", key)
	defer os.Unsetenv("OPEN_ROUTE_KEY")

	model := "test-model"
	caller := NewOpenRouterCaller(model, nil)

	if caller.inner.APIKey != key {
		t.Errorf("Expected APIKey %q, got %q", key, caller.inner.APIKey)
	}
	if caller.inner.BaseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("Expected BaseURL %q, got %q", "https://openrouter.ai/api/v1", caller.inner.BaseURL)
	}
	if caller.inner.Model != model {
		t.Errorf("Expected Model %q, got %q", model, caller.inner.Model)
	}
}
