package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-shell/llm"
)

func TestBuildCommandContent(t *testing.T) {
	tmpDir := t.TempDir()

	txtPath := filepath.Join(tmpDir, "notes.txt")
	if err := os.WriteFile(txtPath, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	pngPath := filepath.Join(tmpDir, "img.png")
	if err := os.WriteFile(pngPath, []byte{0x89, 'P', 'N', 'G'}, 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		prompt  string
		args    []string
		wantStr string
		wantImg bool
	}{
		{
			name:    "no args",
			prompt:  "Do a thing",
			args:    nil,
			wantStr: "Do a thing",
		},
		{
			name:    "non-file arg appended",
			prompt:  "Do",
			args:    []string{"a", "thing"},
			wantStr: "Do\n\na\n\nthing",
		},
		{
			name:    "text file appended",
			prompt:  "Read",
			args:    []string{txtPath},
			wantStr: "Read\n\nhello world",
		},
		{
			name:    "image becomes content part",
			prompt:  "Describe",
			args:    []string{pngPath},
			wantImg: true,
		},
		{
			name:    "mixed text and image",
			prompt:  "Analyze",
			args:    []string{txtPath, pngPath},
			wantImg: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := buildCommandContent(tt.prompt, tt.args)
			if err != nil {
				t.Fatalf("buildCommandContent() error = %v", err)
			}

			if parts, ok := content.([]llm.ContentPart); ok {
				if !tt.wantImg {
					t.Fatalf("got []ContentPart, want string")
				}
				if len(parts) == 0 || parts[0].Type != "text" {
					t.Errorf("first part = %+v, want text", parts)
				}
				if !strings.HasPrefix(parts[0].Text, tt.prompt) {
					t.Errorf("text part = %q, want prefix %q", parts[0].Text, tt.prompt)
				}
				return
			}

			str, ok := content.(string)
			if !ok {
				t.Fatalf("content type = %T, want string or []ContentPart", content)
			}
			if tt.wantImg {
				t.Fatalf("got string %q, want []ContentPart", str)
			}
			if str != tt.wantStr {
				t.Errorf("content = %q, want %q", str, tt.wantStr)
			}
		})
	}
}
