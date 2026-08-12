package llm

import (
	"bytes"
	"os"
	"path/filepath"
	"text/template"

	"ai-shell/tools"
)

const defaultPrompt = `You are an expert shell assistant operating inside a shell on the user machine.
You help users by reading files, executing commands, editing code, and writing new files.

The user machine OS is {{.Distro}} and uses the {{.Shell}} shell.
Current working directory: {{.Cwd}}.

Available tools:
{{.Tools}}`

const planPrompt = `You are a planning agent operating inside a shell on the user machine.
Your job is to analyze problems, read files, and produce a clear, actionable plan.
You cannot execute shell commands or write files — focus on analysis and planning, not on doing the work yourself.

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

// GetAgentSystemPrompt returns the system prompt for the named agent, rendered
// with the given tool list. Falls back to the default prompt for unknown names.
func GetAgentSystemPrompt(agentName string, toolList []any) string {
	raw := []byte(defaultPrompt)
	if agentName == "plan" {
		raw = []byte(planPrompt)
	} else if agentName == "" || agentName == "build" {
		raw = readPromptFile()
	}
	return renderPromptTemplate(raw, toolList)
}

// renderPromptTemplate renders a prompt template with the current distro,
// shell, cwd, and tool list.
func renderPromptTemplate(raw []byte, toolList []any) string {
	cwd, _ := os.Getwd()
	data := PromptData{
		Distro: tools.GetDistro(),
		Shell:  tools.GetShell(),
		Cwd:    cwd,
		Tools:  buildToolDescriptions(toolList),
	}

	tmpl, err := template.New("prompt").Parse(string(raw))
	if err != nil {
		return "You are a helpful shell assistant."
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "You are a helpful shell assistant."
	}

	return buf.String()
}
