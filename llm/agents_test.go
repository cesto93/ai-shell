package llm

import (
	"strings"
	"testing"
)

func TestGetAllTools(t *testing.T) {
	tools := GetAllTools()
	if len(tools) == 0 {
		t.Fatal("GetAllTools() returned empty slice")
	}

	expectedNames := []string{"RunCommand", "WriteFile", "ReadFile", "KVSet", "KVGet", "KVList"}
	foundNames := make(map[string]bool)

	for _, tool := range tools {
		toolMap, ok := tool.(map[string]any)
		if !ok {
			t.Fatalf("Tool is not map[string]any: %T", tool)
		}
		function, ok := toolMap["function"].(map[string]any)
		if !ok {
			t.Fatal("Tool missing 'function' key")
		}
		name, ok := function["name"].(string)
		if !ok {
			t.Fatal("Tool function missing 'name' key")
		}
		foundNames[name] = true

		if _, ok := function["description"].(string); !ok {
			t.Errorf("Tool %q missing 'description' key", name)
		}
		if _, ok := function["parameters"].(map[string]any); !ok {
			t.Errorf("Tool %q missing 'parameters' key", name)
		}
	}

	for _, expected := range expectedNames {
		if !foundNames[expected] {
			t.Errorf("GetAllTools() missing tool %q", expected)
		}
	}
}

func TestGetEnabledTools(t *testing.T) {
	tests := []struct {
		name        string
		enabledMap  map[string]bool
		expectCount int
	}{
		{
			name:        "nil map returns all tools",
			enabledMap:  nil,
			expectCount: 6,
		},
		{
			name:        "empty map returns all tools",
			enabledMap:  map[string]bool{},
			expectCount: 6,
		},
		{
			name: "disable one tool",
			enabledMap: map[string]bool{
				"RunCommand": false,
			},
			expectCount: 5,
		},
		{
			name: "enable only two tools",
			enabledMap: map[string]bool{
				"RunCommand": true,
				"ReadFile":   true,
				"WriteFile":  false,
				"KVSet":      false,
				"KVGet":      false,
				"KVList":     false,
			},
			expectCount: 2,
		},
		{
			name: "explicitly enabled not in all tools still works",
			enabledMap: map[string]bool{
				"NonExistent": true,
			},
			expectCount: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetEnabledTools(tt.enabledMap)
			if len(got) != tt.expectCount {
				t.Errorf("GetEnabledTools() returned %d tools, want %d", len(got), tt.expectCount)
			}
		})
	}
}

func TestGetEnabledToolsReturnsCorrectTools(t *testing.T) {
	enabledMap := map[string]bool{
		"RunCommand": true,
		"ReadFile":   true,
		"WriteFile":  false,
		"KVSet":      false,
		"KVGet":      false,
		"KVList":     false,
	}

	got := GetEnabledTools(enabledMap)
	names := make(map[string]bool)
	for _, tool := range got {
		toolMap := tool.(map[string]any)
		function := toolMap["function"].(map[string]any)
		names[function["name"].(string)] = true
	}

	if !names["RunCommand"] {
		t.Error("Expected RunCommand to be enabled")
	}
	if !names["ReadFile"] {
		t.Error("Expected ReadFile to be enabled")
	}
	if names["WriteFile"] {
		t.Error("Expected WriteFile to be disabled")
	}
	if names["KVSet"] {
		t.Error("Expected KVSet to be disabled")
	}
}

func TestGetToolDescriptions(t *testing.T) {
	descs := GetToolDescriptions()
	if len(descs) == 0 {
		t.Fatal("GetToolDescriptions() returned empty map")
	}

	expectedTools := []string{"RunCommand", "WriteFile", "ReadFile", "KVSet", "KVGet", "KVList"}
	for _, name := range expectedTools {
		desc, ok := descs[name]
		if !ok {
			t.Errorf("GetToolDescriptions() missing tool %q", name)
			continue
		}
		if desc == "" {
			t.Errorf("Tool %q has empty description", name)
		}
	}
}

func TestGetToolDescriptionsConsistency(t *testing.T) {
	allTools := GetAllTools()
	descs := GetToolDescriptions()

	if len(allTools) != len(descs) {
		t.Errorf("GetAllTools() returned %d tools, GetToolDescriptions() returned %d entries", len(allTools), len(descs))
	}
}

func TestBuildToolDescriptions(t *testing.T) {
	tools := GetAllTools()
	result := buildToolDescriptions(tools)

	if result == "" {
		t.Fatal("buildToolDescriptions() returned empty string")
	}

	if !strings.Contains(result, "- RunCommand:") {
		t.Error("buildToolDescriptions() missing RunCommand")
	}
	if !strings.Contains(result, "- WriteFile:") {
		t.Error("buildToolDescriptions() missing WriteFile")
	}
	if !strings.Contains(result, "- ReadFile:") {
		t.Error("buildToolDescriptions() missing ReadFile")
	}
}

func TestBuildToolDescriptionsEmpty(t *testing.T) {
	result := buildToolDescriptions([]any{})
	if result != "" {
		t.Errorf("buildToolDescriptions() with empty input returned %q, want empty string", result)
	}
}

func TestBuildToolDescriptionsSkipsInvalid(t *testing.T) {
	input := []any{
		"not a map",
		map[string]any{"wrong": "structure"},
		map[string]any{
			"function": map[string]any{
				"name":        "TestTool",
				"description": "A test tool",
			},
		},
	}

	result := buildToolDescriptions(input)
	if !strings.Contains(result, "TestTool") {
		t.Error("buildToolDescriptions() should include valid tool")
	}
	if strings.Contains(result, "not a map") {
		t.Error("buildToolDescriptions() should skip invalid entries")
	}
}

func TestNewAgent(t *testing.T) {
	agent := NewAgent("test-model", "ollama", nil)

	if agent == nil {
		t.Fatal("NewAgent() returned nil")
	}
	if agent.Model != "test-model" {
		t.Errorf("NewAgent().Model = %q, want %q", agent.Model, "test-model")
	}
	if agent.Provider != "ollama" {
		t.Errorf("NewAgent().Provider = %q, want %q", agent.Provider, "ollama")
	}
	if agent.Prompt == "" {
		t.Error("NewAgent().Prompt is empty")
	}
	if len(agent.Tools) == 0 {
		t.Error("NewAgent().Tools is empty")
	}
}

func TestNewAgentWithDisabledTools(t *testing.T) {
	enabledMap := map[string]bool{
		"RunCommand": true,
		"ReadFile":   true,
		"WriteFile":  false,
		"KVSet":      false,
		"KVGet":      false,
		"KVList":     false,
	}

	agent := NewAgent("test-model", "ollama", enabledMap)
	if len(agent.Tools) != 2 {
		t.Errorf("NewAgent() with disabled tools returned %d tools, want 2", len(agent.Tools))
	}
}
