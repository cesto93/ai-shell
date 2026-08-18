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

func TestImagesPresent(t *testing.T) {
	tests := []struct {
		name     string
		messages []Message
		want     bool
	}{
		{
			name:     "text only",
			messages: []Message{{Role: "user", Content: "hello"}},
			want:     false,
		},
		{
			name: "image part with URL",
			messages: []Message{
				{Role: "user", Content: []ContentPart{
					{Type: "text", Text: "what is this?"},
					{Type: "image_url", ImageURL: &ContentImage{URL: "data:image/png;base64,aGk="}},
				}},
			},
			want: true,
		},
		{
			name: "image part without URL",
			messages: []Message{
				{Role: "user", Content: []ContentPart{{Type: "image_url"}}},
			},
			want: false,
		},
		{
			name: "audio only",
			messages: []Message{
				{Role: "user", Content: []ContentPart{{Type: "input_audio", InputAudio: &InputAudio{Data: "aGk=", Format: "wav"}}}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := imagesPresent(tt.messages); got != tt.want {
				t.Errorf("imagesPresent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatVisionContent(t *testing.T) {
	const marker = "<__media__>"

	tests := []struct {
		name     string
		content  any
		wantText string
		wantImgs int
	}{
		{
			name:     "plain string",
			content:  "hello",
			wantText: "hello",
			wantImgs: 0,
		},
		{
			name: "text and image",
			content: []ContentPart{
				{Type: "text", Text: "describe: "},
				{Type: "image_url", ImageURL: &ContentImage{URL: "data:image/png;base64,aGVsbG8="}},
			},
			wantText: "describe: " + marker,
			wantImgs: 1,
		},
		{
			name: "multiple images emit one marker each",
			content: []ContentPart{
				{Type: "image_url", ImageURL: &ContentImage{URL: "data:image/png;base64,aGVsbG8="}},
				{Type: "text", Text: " and "},
				{Type: "image_url", ImageURL: &ContentImage{URL: "data:image/png;base64,d29ybGQ="}},
			},
			wantText: marker + " and " + marker,
			wantImgs: 2,
		},
		{
			name: "invalid data URL is skipped",
			content: []ContentPart{
				{Type: "image_url", ImageURL: &ContentImage{URL: "not-a-data-url"}},
				{Type: "text", Text: "ok"},
			},
			wantText: "ok",
			wantImgs: 0,
		},
		{
			name:     "nil content",
			content:  nil,
			wantText: "",
			wantImgs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, imgs := formatVisionContent(tt.content, marker)
			if text != tt.wantText {
				t.Errorf("formatVisionContent() text = %q, want %q", text, tt.wantText)
			}
			if len(imgs) != tt.wantImgs {
				t.Fatalf("formatVisionContent() imgs = %d, want %d", len(imgs), tt.wantImgs)
			}
			if tt.wantImgs > 0 && string(imgs[0]) != "hello" {
				t.Errorf("formatVisionContent() img[0] = %q, want decoded payload", imgs[0])
			}
		})
	}
}
