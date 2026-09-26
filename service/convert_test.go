package service

import (
	"testing"

	"edgebot/llm"
)

func TestMessageRoundTripText(t *testing.T) {
	in := llm.Message{
		Role:       "assistant",
		Content:    "hello world",
		ToolCallID: "call_1",
		ToolCalls: []llm.OpenAIToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      "ReadFile",
					Arguments: `{"path":"/tmp/x"}`,
				},
			},
		},
	}

	got := messageFromProto(messageToProto(in))

	if got.Role != in.Role {
		t.Errorf("role = %q, want %q", got.Role, in.Role)
	}
	if got.ToolCallID != in.ToolCallID {
		t.Errorf("tool_call_id = %q, want %q", got.ToolCallID, in.ToolCallID)
	}
	if got.Content != "hello world" {
		t.Errorf("content = %v, want %q", got.Content, "hello world")
	}
	if len(got.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(got.ToolCalls))
	}
	tc := got.ToolCalls[0]
	if tc.ID != "call_1" || tc.Function.Name != "ReadFile" || tc.Function.Arguments != `{"path":"/tmp/x"}` {
		t.Errorf("tool call mismatch: %+v", tc)
	}
}

func TestMessageRoundTripMultimodal(t *testing.T) {
	in := llm.Message{
		Role: "user",
		Content: []llm.ContentPart{
			{Type: "text", Text: "look at this"},
			{Type: "image_url", ImageURL: &llm.ContentImage{URL: "data:image/png;base64,AAAA"}},
			{Type: "input_audio", InputAudio: &llm.InputAudio{Data: "base64audio", Format: "wav"}},
		},
	}

	got := messageFromProto(messageToProto(in))

	parts, ok := got.Content.([]llm.ContentPart)
	if !ok {
		t.Fatalf("content type = %T, want []llm.ContentPart", got.Content)
	}
	if len(parts) != 3 {
		t.Fatalf("parts = %d, want 3", len(parts))
	}
	if parts[0].Text != "look at this" {
		t.Errorf("parts[0].Text = %q, want %q", parts[0].Text, "look at this")
	}
	if parts[1].ImageURL == nil || parts[1].ImageURL.URL != "data:image/png;base64,AAAA" {
		t.Errorf("parts[1] image_url mismatch: %+v", parts[1].ImageURL)
	}
	if parts[2].InputAudio == nil || parts[2].InputAudio.Data != "base64audio" || parts[2].InputAudio.Format != "wav" {
		t.Errorf("parts[2] input_audio mismatch: %+v", parts[2].InputAudio)
	}
}

func TestMessageRoundTripEmptyContent(t *testing.T) {
	in := llm.Message{Role: "assistant"}

	got := messageFromProto(messageToProto(in))

	if got.Role != "assistant" {
		t.Errorf("role = %q, want %q", got.Role, "assistant")
	}
	if got.Content != nil && got.Content != "" {
		t.Errorf("content = %v, want nil or empty", got.Content)
	}
}

func TestMessagesSliceRoundTrip(t *testing.T) {
	in := []llm.Message{
		{Role: "user", Content: "one"},
		{Role: "assistant", Content: "two"},
		{Role: "tool", Content: "three", ToolCallID: "call_9"},
	}

	got := messagesFromProto(messagesToProto(in))

	if len(got) != len(in) {
		t.Fatalf("messages = %d, want %d", len(got), len(in))
	}
	for i := range in {
		if got[i].Role != in[i].Role || got[i].Content != in[i].Content || got[i].ToolCallID != in[i].ToolCallID {
			t.Errorf("message %d mismatch: got %+v, want %+v", i, got[i], in[i])
		}
	}
}

func TestThoughtSignatureRoundTrip(t *testing.T) {
	mkToolCall := func(sig string) llm.OpenAIToolCall {
		var tc llm.OpenAIToolCall
		tc.ID = "call_1"
		tc.Type = "function"
		tc.Function.Name = "ReadFile"
		tc.Function.Arguments = "{}"
		if sig != "" {
			tc.ExtraContent = &llm.ExtraContent{
				Google: &llm.GoogleExtraContent{ThoughtSignature: sig},
			}
		}
		return tc
	}
	in := llm.Message{
		Role:    "assistant",
		Content: "thinking",
		ExtraContent: &llm.ExtraContent{
			Google: &llm.GoogleExtraContent{ThoughtSignature: "MSG_SIG"},
		},
		ToolCalls: []llm.OpenAIToolCall{mkToolCall("SIG_A")},
	}

	got := messageFromProto(messageToProto(in))

	if got.ExtraContent == nil || got.ExtraContent.Google == nil ||
		got.ExtraContent.Google.ThoughtSignature != "MSG_SIG" {
		t.Errorf("message signature = %+v, want MSG_SIG", got.ExtraContent)
	}
	if len(got.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(got.ToolCalls))
	}
	tc := got.ToolCalls[0]
	if tc.ExtraContent == nil || tc.ExtraContent.Google == nil ||
		tc.ExtraContent.Google.ThoughtSignature != "SIG_A" {
		t.Errorf("tool call signature = %+v, want SIG_A", tc.ExtraContent)
	}

	// Content-part signatures round-trip too.
	partIn := llm.Message{
		Role: "user",
		Content: []llm.ContentPart{
			{
				Type: "text",
				Text: "hi",
				ExtraContent: &llm.ExtraContent{
					Google: &llm.GoogleExtraContent{ThoughtSignature: "PART_SIG"},
				},
			},
		},
	}
	partGot := messageFromProto(messageToProto(partIn))
	parts, ok := partGot.Content.([]llm.ContentPart)
	if !ok || len(parts) != 1 {
		t.Fatalf("parts = %T %v, want 1 part", partGot.Content, partGot.Content)
	}
	if parts[0].ExtraContent == nil || parts[0].ExtraContent.Google == nil ||
		parts[0].ExtraContent.Google.ThoughtSignature != "PART_SIG" {
		t.Errorf("part signature = %+v, want PART_SIG", parts[0].ExtraContent)
	}
}
