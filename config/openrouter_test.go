package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsOpenRouterModel(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"nvidia/nemotron-3-super-120b-a12b:free", true},
		{"z-ai/glm-4.5-air:free", true},
		{"minimax/minimax-m2.5:free", true},
		{"gemini-3-flash-preview", false},
		{"mistral-small", false},
		{"other-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := IsOpenRouterModel(tt.model); got != tt.want {
				t.Errorf("IsOpenRouterModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestSaveModelWithOpenRouter(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ai-shell")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		t.Fatalf("Failed to create temp config dir: %v", err)
	}

	origGetConfigPath := getConfigPathFunc
	getConfigPathFunc = func() (string, error) {
		return configPath, nil
	}
	defer func() { getConfigPathFunc = origGetConfigPath }()

	origUserConfigDirFunc := userConfigDirFunc
	userConfigDirFunc = func() (string, error) {
		return tmpDir, nil
	}
	defer func() { userConfigDirFunc = origUserConfigDirFunc }()

	origConfigPaths := configPaths
	defer func() { configPaths = origConfigPaths }()
	configPaths = []string{configPath}

	model := "nvidia/nemotron-3-super-120b-a12b:free"
	if err := SaveModelWithProvider(model, ""); err != nil {
		t.Fatalf("SaveModelWithProvider() error = %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.LLM.Provider != "openrouter" {
		t.Errorf("Expected provider 'openrouter', got %q", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != model {
		t.Errorf("Expected model %q, got %q", model, cfg.LLM.Model)
	}
}
