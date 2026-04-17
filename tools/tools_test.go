package tools

import (
	"os"
	"strings"
	"testing"
)

func TestRunCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantErr bool
		checkFn func(string) bool
	}{
		{
			name:    "echo hello",
			command: "echo hello",
			wantErr: false,
			checkFn: func(output string) bool {
				return strings.TrimSpace(output) == "hello"
			},
		},
		{
			name:    "echo with newline",
			command: "printf 'test output'",
			wantErr: false,
			checkFn: func(output string) bool {
				return strings.TrimSpace(output) == "test output"
			},
		},
		{
			name:    "invalid command",
			command: "nonexistentcommand12345",
			wantErr: true,
			checkFn: func(output string) bool {
				return strings.Contains(output, "not found") ||
					strings.Contains(output, "command not found")
			},
		},
		{
			name:    "exit code 1",
			command: "exit 1",
			wantErr: true,
			checkFn: func(output string) bool {
				return true
			},
		},
		{
			name:    "stderr output",
			command: "echo error >&2",
			wantErr: false,
			checkFn: func(output string) bool {
				return strings.Contains(output, "error")
			},
		},
		{
			name:    "combined stdout stderr",
			command: "echo stdout; echo stderr >&2",
			wantErr: false,
			checkFn: func(output string) bool {
				return strings.Contains(output, "stdout") &&
					strings.Contains(output, "stderr")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := RunCommand(tt.command)

			if (err != nil) != tt.wantErr {
				t.Errorf("RunCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.checkFn != nil && !tt.checkFn(output) {
				t.Errorf("RunCommand() output = %q, did not match expected condition", output)
			}
		})
	}
}

func TestFileOperations(t *testing.T) {
	tempFile := "test_file.txt"
	content := "test content"

	t.Run("WriteFile", func(t *testing.T) {
		msg, err := WriteFile(tempFile, content)
		if err != nil {
			t.Errorf("WriteFile() error = %v", err)
		}
		if !strings.Contains(msg, "successfully") {
			t.Errorf("WriteFile() output = %q, expected success message", msg)
		}
	})

	t.Run("ReadFile", func(t *testing.T) {
		got, err := ReadFile(tempFile)
		if err != nil {
			t.Errorf("ReadFile() error = %v", err)
		}
		if got != content {
			t.Errorf("ReadFile() = %q, want %q", got, content)
		}
	})

	t.Run("ReadFile Non-existent", func(t *testing.T) {
		_, err := ReadFile("non_existent_file.txt")
		if err == nil {
			t.Error("ReadFile() expected error for non-existent file, got nil")
		}
	})

	// Cleanup
	os.Remove(tempFile)
}
