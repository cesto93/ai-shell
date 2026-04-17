package llm

import (
	"ai-shell/tools"
	"fmt"
)

// Agent represents an AI agent with its prompt, model, provider, and tools.
type Agent struct {
	Prompt   string
	Model    string
	Provider string
	Tools    []any
}

// GetDefaultTools returns the default list of tools for the agent.
func GetDefaultTools() []any {
	return []any{
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "RunCommand",
				"description": "Execute a shell command and return its output",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{
							"type":        "string",
							"description": "The shell command to execute (e.g., 'ls -la', 'echo hello')",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "WriteFile",
				"description": "Write content to a file at the specified path",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "The absolute or relative path to the file",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "The content to write to the file",
						},
					},
					"required": []string{"path", "content"},
				},
			},
		},
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "ReadFile",
				"description": "Read the content of a file at the specified path",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "The absolute or relative path to the file",
						},
					},
					"required": []string{"path"},
				},
			},
		},
	}
}

// GetDefaultSystemPrompt returns the default system prompt based on distro and shell.
func GetDefaultSystemPrompt() string {
	distro := tools.GetDistro()
	shell := tools.GetShell()
	return fmt.Sprintf("You are a helpful shell assistant. The user is running on %s using %s shell.", distro, shell)
}

// NewAgent creates a new Agent with the given parameters.
func NewAgent(model, provider string) *Agent {
	return &Agent{
		Prompt:   GetDefaultSystemPrompt(),
		Model:    model,
		Provider: provider,
		Tools:    GetDefaultTools(),
	}
}
