package llm

import (
	"encoding/json"
	"testing"
)

func TestMessageSerialization(t *testing.T) {
	tests := []struct {
		name     string
		message  Message
		expected string
	}{
		{
			name: "Simple text message",
			message: Message{
				Role:    "user",
				Content: "Hello",
			},
			expected: `{"role":"user","content":"Hello"}`,
		},
		{
			name: "Multimodal message",
			message: Message{
				Role: "user",
				Content: []ContentPart{
					{Type: "text", Text: "What is this?"},
					{
						Type: "image_url",
						ImageURL: &ContentImage{
							URL: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==",
						},
					},
				},
			},
			expected: `{"role":"user","content":[{"type":"text","text":"What is this?"},{"type":"image_url","image_url":{"url":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg=="}}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.message)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}
			if string(data) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(data))
			}
		})
	}
}
