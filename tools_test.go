package main

import (
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
