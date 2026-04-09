package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompleteCommands(t *testing.T) {
	c := &Completer{}

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
	c := &Completer{}

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

func TestCompleteInDir(t *testing.T) {
	c := &Completer{}

	tmpDir := t.TempDir()
	oldCwd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldCwd)

	subDir := filepath.Join(tmpDir, "testdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	testFiles := []string{"file1.txt", "file2.log", "other.doc"}
	for _, name := range testFiles {
		f, err := os.Create(filepath.Join(subDir, name))
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		f.Close()
	}
	os.MkdirAll(filepath.Join(subDir, "subdir"), 0755)

	relDir := filepath.Join(".", "testdir")
	cwd := filepath.Base(tmpDir)

	tests := []struct {
		name         string
		dir          string
		prefix       string
		wantLen      int
		wantContains string
	}{
		{
			name:    "relative dir with file prefix",
			dir:     relDir,
			prefix:  "file",
			wantLen: 2,
		},
		{
			name:    "relative dir with dot prefix",
			dir:     "./testdir",
			prefix:  "file",
			wantLen: 2,
		},
		{
			name:    "absolute dir with file prefix",
			dir:     subDir,
			prefix:  "file",
			wantLen: 2,
		},
		{
			name:         "absolute dir with log extension",
			dir:          subDir,
			prefix:       "file2",
			wantLen:      1,
			wantContains: "file2.log",
		},
		{
			name:    "no matches",
			dir:     subDir,
			prefix:  "nonexistent",
			wantLen: 0,
		},
		{
			name:    "non-existent directory",
			dir:     "/nonexistent/path",
			prefix:  "file",
			wantLen: 0,
		},
		{
			name:    "prefix matches all files",
			dir:     subDir,
			prefix:  "f",
			wantLen: 2,
		},
		{
			name:         "directory path with subdirectory",
			dir:          subDir,
			prefix:       "subdir",
			wantLen:      1,
			wantContains: "subdir/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := c.completeInDir(tt.dir, tt.prefix)
			if len(results) != tt.wantLen {
				t.Errorf("completeInDir() got %d results, want %d", len(results), tt.wantLen)
			}
			if tt.wantContains != "" {
				found := false
				for _, r := range results {
					if strings.Contains(string(r), tt.wantContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("completeInDir() expected to find %q in results", tt.wantContains)
				}
			}
		})
	}

	_ = cwd
}

func TestCompleteFiles(t *testing.T) {
	c := &Completer{}
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
