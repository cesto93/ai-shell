package main

import (
	"os"
	"testing"
)

func TestGetShell(t *testing.T) {
	tests := []struct {
		name   string
		shell  string
		expect string
	}{
		{
			name:   "bash shell",
			shell:  "/bin/bash",
			expect: "/bin/bash",
		},
		{
			name:   "zsh shell",
			shell:  "/bin/zsh",
			expect: "/bin/zsh",
		},
		{
			name:   "empty shell",
			shell:  "",
			expect: "Unknown Shell",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shell == "" {
				os.Unsetenv("SHELL")
			} else {
				os.Setenv("SHELL", tt.shell)
				defer os.Unsetenv("SHELL")
			}

			result := GetShell()
			if result != tt.expect {
				t.Errorf("GetShell() = %q, want %q", result, tt.expect)
			}
		})
	}
}
