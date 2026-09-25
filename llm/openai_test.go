package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestOpenAIThinkEffortSent(t *testing.T) {
	var got OpenAIRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req OpenAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		got = req
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi"}}]}`))
	}))
	defer srv.Close()

	caller := NewOpenAICallerWithThink(srv.URL, "", "m", &mockExecutor{}, ThinkEffortLow)
	if _, err := caller.Call(context.Background(), "sys", []Message{{Role: "user", Content: "hi"}}, nil); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if got.ReasoningEffort == nil || *got.ReasoningEffort != "low" {
		t.Errorf("reasoning_effort = %+v, want low", got.ReasoningEffort)
	}
	if got.Reasoning != nil {
		t.Errorf("reasoning = %+v, want omitted (non-OpenRouter sends reasoning_effort only)", got.Reasoning)
	}
}

func TestOpenAIThinkEffortSentOpenRouter(t *testing.T) {
	caller := NewOpenAICallerWithThink(
		"https://openrouter.ai/api/v1",
		"", "m", &mockExecutor{}, ThinkEffortLow,
	)
	reasoningEffort, reasoning := caller.reasoningFields()
	if reasoningEffort != nil {
		t.Errorf("reasoning_effort = %+v, want omitted on OpenRouter", reasoningEffort)
	}
	if reasoning == nil || reasoning.Effort != "low" {
		t.Errorf("reasoning = %+v, want {low} on OpenRouter", reasoning)
	}
}

func TestOpenAIThinkEffortSentGemini(t *testing.T) {
	caller := NewOpenAICallerWithThink(
		"https://generativelanguage.googleapis.com/v1beta/openai",
		"", "m", &mockExecutor{}, ThinkEffortLow,
	)
	reasoningEffort, reasoning := caller.reasoningFields()
	if reasoningEffort == nil || *reasoningEffort != "low" {
		t.Errorf("reasoning_effort = %+v, want low on Gemini", reasoningEffort)
	}
	if reasoning != nil {
		t.Errorf("reasoning = %+v, want omitted on Gemini (rejected with 400)", reasoning)
	}
}

func TestOpenAIThinkEffortOmittedByDefault(t *testing.T) {
	var got OpenAIRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req OpenAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		got = req
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi"}}]}`))
	}))
	defer srv.Close()

	caller := NewOpenAICaller(srv.URL, "", "m", &mockExecutor{})
	if _, err := caller.Call(context.Background(), "sys", []Message{{Role: "user", Content: "hi"}}, nil); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if got.ReasoningEffort != nil {
		t.Errorf("reasoning_effort = %q, want omitted", *got.ReasoningEffort)
	}
	if got.Reasoning != nil {
		t.Errorf("reasoning = %+v, want omitted", got.Reasoning)
	}
}

func TestProviderCallerWithThinkPropagates(t *testing.T) {
	c := NewProviderCallerWithThink("ollama", "m", nil, ThinkEffortHigh)
	oc, ok := c.(*OpenAICaller)
	if !ok {
		t.Fatalf("caller type = %T, want *OpenAICaller", c)
	}
	if oc.ThinkEffort != ThinkEffortHigh {
		t.Errorf("ThinkEffort = %q, want high", oc.ThinkEffort)
	}
	lc := NewProviderCallerWithThink("llamacpp", "m", nil, ThinkEffortNone)
	llc, ok := lc.(*LlamacppCaller)
	if !ok {
		t.Fatalf("caller type = %T, want *LlamacppCaller", lc)
	}
	if !llc.NoThink {
		t.Error("llamacpp think=none should set NoThink")
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
