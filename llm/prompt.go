package llm

import (
	"os"
	"path/filepath"
)

const defaultPrompt = `You are an expert shell assistant operating inside a shell on the user machine.
You help users by reading files, executing commands, editing code, and writing new files.

The user machine OS is {{.Distro}} and uses the {{.Shell}} shell.
Current working directory: {{.Cwd}}.

Available tools:
{{.Tools}}`

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".ai-shell")
	os.MkdirAll(dir, 0o755)

	dest := filepath.Join(dir, "PROMPT.md")
	if _, err := os.Stat(dest); err == nil {
		return
	}
	os.WriteFile(dest, []byte(defaultPrompt), 0o644)
}

func GetDefaultPromptBytes() []byte {
	return []byte(defaultPrompt)
}
