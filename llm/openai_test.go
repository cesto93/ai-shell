package llm

import (
	"testing"
)

func TestNewOpenAICaller(t *testing.T) {
	caller := NewOpenAICaller("http://example.com/v1", "test-api-key", "test-model", nil)

	if caller == nil {
		t.Fatal("NewOpenAICaller() returned nil")
	}
	if caller.BaseURL != "http://example.com/v1" {
		t.Errorf("BaseURL = %q, want %q", caller.BaseURL, "http://example.com/v1")
	}
	if caller.APIKey != "test-api-key" {
		t.Errorf("APIKey = %q, want %q", caller.APIKey, "test-api-key")
	}
	if caller.Model != "test-model" {
		t.Errorf("Model = %q, want %q", caller.Model, "test-model")
	}
	if caller.Client == nil {
		t.Error("Client is nil")
	}
	if caller.Executor != nil {
		t.Error("Executor should be nil when nil is passed")
	}
}

func TestNewOpenAICallerWithExecutor(t *testing.T) {
	mock := &mockExecutor{}
	caller := NewOpenAICaller("http://example.com/v1", "key", "model", mock)

	if caller.Executor != mock {
		t.Error("Executor not set correctly")
	}
}

func TestOpenAIRequestSerialization(t *testing.T) {
	req := OpenAIRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
		},
		Tools: []any{
			map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": "RunCommand",
				},
			},
		},
		Temperature: 0.7,
	}

	if req.Model != "test-model" {
		t.Errorf("Model = %q, want %q", req.Model, "test-model")
	}
	if len(req.Messages) != 2 {
		t.Errorf("Messages length = %d, want 2", len(req.Messages))
	}
	if len(req.Tools) != 1 {
		t.Errorf("Tools length = %d, want 1", len(req.Tools))
	}
	if req.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want 0.7", req.Temperature)
	}
}

type mockExecutor struct{}

func (m *mockExecutor) ExecuteTool(call ToolCall) (string, error) {
	return "mock output", nil
}

func (m *mockExecutor) IsAllowedCommand(cmd string) bool {
	return true
}

func (m *mockExecutor) AskConfirmation(cmd string) bool {
	return true
}
