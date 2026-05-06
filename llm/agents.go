package llm

import (
	"ai-shell/tools"
	"fmt"
	"os"
)

// Agent represents an AI agent with its prompt, model, provider, and tools.
type Agent struct {
	Prompt   string
	Model    string
	Provider string
	Tools    []any
}

// GetAllTools returns the full list of tools for the agent.
func GetAllTools() []any {
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
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "KVSet",
				"description": "Save a value to the KV store with a given key",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key": map[string]any{
							"type":        "string",
							"description": "The key to save",
						},
						"value": map[string]any{
							"type":        "string",
							"description": "The value to save",
						},
					},
					"required": []string{"key", "value"},
				},
			},
		},
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "KVGet",
				"description": "Retrieve a value from the KV store by key",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key": map[string]any{
							"type":        "string",
							"description": "The key to retrieve",
						},
					},
					"required": []string{"key"},
				},
			},
		},
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "KVList",
				"description": "List all keys currently in the KV store",
				"parameters": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
	}
}

// GetEnabledTools filters the full list of tools based on the enabledMap.
func GetEnabledTools(enabledMap map[string]bool) []any {
	allTools := GetAllTools()
	if enabledMap == nil {
		return allTools
	}

	var enabledTools []any
	for _, t := range allTools {
		if toolMap, ok := t.(map[string]any); ok {
			if function, ok := toolMap["function"].(map[string]any); ok {
				if name, ok := function["name"].(string); ok {
					if enabled, exists := enabledMap[name]; !exists || enabled {
						enabledTools = append(enabledTools, t)
					}
				}
			}
		}
	}
	return enabledTools
}

// GetDefaultSystemPrompt returns the default system prompt based on distro, shell, and current directory.
func GetDefaultSystemPrompt() string {
	distro := tools.GetDistro()
	shell := tools.GetShell()
	cwd, _ := os.Getwd()
	return fmt.Sprintf("You are a helpful shell assistant. The user is running on %s using %s shell. Current working directory: %s", distro, shell, cwd)
}

// NewAgent creates a new Agent with the given parameters.
func NewAgent(model, provider string, toolsEnabled map[string]bool) *Agent {
	return &Agent{
		Prompt:   GetDefaultSystemPrompt(),
		Model:    model,
		Provider: provider,
		Tools:    GetEnabledTools(toolsEnabled),
	}
}
