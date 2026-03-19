package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompleteCommands(t *testing.T) {
	c := &completer{}

	tests := []struct {
		name     string
		prefix   string
		wantLen  int
		wantCmds []string
	}{
		{
			name:     "help command",
			prefix:   "/he",
			wantLen:  1,
			wantCmds: []string{"lp"},
		},
		{
			name:     "get-config command",
			prefix:   "/get",
			wantLen:  1,
			wantCmds: []string{"-config"},
		},
		{
			name:     "exit command",
			prefix:   "/ex",
			wantLen:  1,
			wantCmds: []string{"it"},
		},
		{
			name:     "quit command",
			prefix:   "/qu",
			wantLen:  1,
			wantCmds: []string{"it"},
		},
		{
			name:     "no match",
			prefix:   "/xyz",
			wantLen:  0,
			wantCmds: nil,
		},
		{
			name:     "empty prefix",
			prefix:   "/",
			wantLen:  len(availableCommands),
			wantCmds: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, _ := c.completeCommands(tt.prefix)

			if len(results) != tt.wantLen {
				t.Errorf("completeCommands() got %d results, want %d", len(results), tt.wantLen)
			}

			if tt.wantLen > 0 && len(results) > 0 && len(tt.wantCmds) > 0 {
				firstResult := string(results[0])
				if firstResult != tt.wantCmds[0] {
					t.Errorf("completeCommands() first result = %q, want %q", firstResult, tt.wantCmds[0])
				}
			}
		})
	}
}

func TestDo(t *testing.T) {
	c := &completer{}

	tests := []struct {
		name      string
		line      string
		pos       int
		wantNil   bool
		wantCmds  bool
		wantFiles bool
	}{
		{
			name:     "command with slash",
			line:     "/he",
			pos:      2,
			wantNil:  false,
			wantCmds: true,
		},
		{
			name:     "no at symbol",
			line:     "hello",
			pos:      5,
			wantNil:  true,
			wantCmds: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, _ := c.Do([]rune(tt.line), tt.pos)

			if tt.wantNil && results != nil {
				t.Errorf("Do() got non-nil results, want nil")
			}

			if tt.wantCmds && len(results) == 0 {
				t.Errorf("Do() got no results, expected command completions")
			}
		})
	}
}

func TestIsInCompletion(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		pos       int
		wantFound bool
		wantAt    int
	}{
		{
			name:      "has at symbol",
			line:      "test@fil",
			pos:       8,
			wantFound: true,
			wantAt:    4,
		},
		{
			name:      "no at symbol",
			line:      "testfile",
			pos:       8,
			wantFound: false,
			wantAt:    -1,
		},
		{
			name:      "empty line",
			line:      "",
			pos:       0,
			wantFound: false,
			wantAt:    -1,
		},
		{
			name:      "at at symbol",
			line:      "@@file",
			pos:       5,
			wantFound: true,
			wantAt:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, at := isInCompletion([]rune(tt.line), tt.pos)
			if found != tt.wantFound {
				t.Errorf("isInCompletion() found = %v, want %v", found, tt.wantFound)
			}
			if at != tt.wantAt {
				t.Errorf("isInCompletion() at = %v, want %v", at, tt.wantAt)
			}
		})
	}
}

func TestCompleteFiles(t *testing.T) {
	c := &completer{}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	tmpDir := t.TempDir()
	oldCwd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldCwd)

	testFiles := []string{"test1.txt", "test2.txt", "other.log"}
	for _, name := range testFiles {
		f, err := os.Create(name)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		f.Close()
	}
	os.MkdirAll("subdir", 0755)
	f, _ := os.Create(filepath.Join("subdir", "nested.txt"))
	f.Close()

	tests := []struct {
		name    string
		prefix  string
		wantLen int
	}{
		{
			name:    "match multiple files",
			prefix:  "test",
			wantLen: 2,
		},
		{
			name:    "match single file",
			prefix:  "other",
			wantLen: 1,
		},
		{
			name:    "no matches",
			prefix:  "nonexistent",
			wantLen: 0,
		},
		{
			name:    "match directory",
			prefix:  "sub",
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := c.completeFiles(tt.prefix)
			if len(results) != tt.wantLen {
				t.Errorf("completeFiles() got %d results, want %d", len(results), tt.wantLen)
			}
		})
	}

	_ = cwd
}
