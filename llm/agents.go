package llm

import (
	"ai-shell/tools"
	"bytes"
	"context"
	"log"
	"os"
	"text/template"
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

// GetToolDescriptions returns a map of tool name to description extracted from GetAllTools.
func GetToolDescriptions() map[string]string {
	descs := make(map[string]string)
	for _, t := range GetAllTools() {
		if toolMap, ok := t.(map[string]any); ok {
			if function, ok := toolMap["function"].(map[string]any); ok {
				name, _ := function["name"].(string)
				desc, _ := function["description"].(string)
				if name != "" {
					descs[name] = desc
				}
			}
		}
	}
	return descs
}

type PromptData struct {
	Distro string
	Shell  string
	Cwd    string
	Tools  string
}

func buildToolDescriptions(tools []any) string {
	var sb bytes.Buffer
	for _, t := range tools {
		toolMap, ok := t.(map[string]any)
		if !ok {
			continue
		}
		function, ok := toolMap["function"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := function["name"].(string)
		desc, _ := function["description"].(string)
		sb.WriteString("- ")
		sb.WriteString(name)
		sb.WriteString(": ")
		sb.WriteString(desc)
		sb.WriteString("\n")
	}
	return sb.String()
}

// GetDefaultSystemPrompt returns the default system prompt based on distro, shell, and current directory.
func GetDefaultSystemPrompt(enabledTools map[string]bool) string {
	cwd, _ := os.Getwd()
	data := PromptData{
		Distro: tools.GetDistro(),
		Shell:  tools.GetShell(),
		Cwd:    cwd,
		Tools:  buildToolDescriptions(GetEnabledTools(enabledTools)),
	}

	raw, err := os.ReadFile("PROMPT.md")
	if err != nil {
		log.Fatalf("Failed to read PROMPT.md: %v", err)
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

// CallLLM calls the LLM using the agent's provider, model, prompt, and tools.
func (a *Agent) CallLLM(ctx context.Context, executor ToolExecutor, messages []Message) ([]Message, error) {
	var caller Caller
	switch a.Provider {
	case "gemini":
		caller = NewGeminiCaller(a.Model, executor)
	case "litertlm":
		caller = NewLitertLMCaller(a.Model, executor)
	case "openrouter":
		caller = NewOpenRouterCaller(a.Model, executor)
	default:
		caller = NewOllamaCaller(a.Model, executor)
	}
	return caller.Call(ctx, a.Prompt, messages, a.Tools)
}

// NewAgent creates a new Agent with the given parameters.
func NewAgent(model, provider string, toolsEnabled map[string]bool) *Agent {
	return &Agent{
		Prompt:   GetDefaultSystemPrompt(toolsEnabled),
		Model:    model,
		Provider: provider,
		Tools:    GetEnabledTools(toolsEnabled),
	}
}
