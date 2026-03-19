package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ai-shell")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		t.Fatalf("Failed to create temp config dir: %v", err)
	}

	tests := []struct {
		name          string
		configContent string
		configExists  bool
		wantModel     string
		wantConfirm   bool
		resetViper    bool
	}{
		{
			name: "valid config file",
			configContent: `
llm:
  model: "custom-model:latest"
shell:
  confirm: false
`,
			configExists: true,
			wantModel:    "custom-model:latest",
			wantConfirm:  false,
			resetViper:   true,
		},
		{
			name:          "default config when file not found",
			configContent: "",
			configExists:  false,
			wantModel:     "granite4:3b-h",
			wantConfirm:   true,
			resetViper:    true,
		},
		{
			name: "partial config uses defaults",
			configContent: `
llm:
  model: "partial-model"
`,
			configExists: true,
			wantModel:    "partial-model",
			wantConfirm:  true,
			resetViper:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()

			configFile := filepath.Join(configPath, "config.yaml")
			if tt.configExists {
				if err := os.WriteFile(configFile, []byte(tt.configContent), 0644); err != nil {
					t.Fatalf("Failed to write config file: %v", err)
				}
			} else if _, err := os.Stat(configFile); err == nil {
				os.Remove(configFile)
			}

			viper.SetConfigName("config")
			viper.SetConfigType("yaml")
			viper.SetDefault("llm.model", "granite4:3b-h")
			viper.SetDefault("shell.confirm", true)
			viper.AddConfigPath(tmpDir)
			viper.AddConfigPath(configPath)

			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}

			if cfg.LLM.Model != tt.wantModel {
				t.Errorf("LoadConfig().LLM.Model = %q, want %q", cfg.LLM.Model, tt.wantModel)
			}

			if cfg.Shell.Confirm != tt.wantConfirm {
				t.Errorf("LoadConfig().Shell.Confirm = %v, want %v", cfg.Shell.Confirm, tt.wantConfirm)
			}
		})
	}
}

func TestSaveModel(t *testing.T) {
	configPath, err := getConfigPath()
	if err != nil {
		t.Fatalf("getConfigPath() error = %v", err)
	}

	configFile := filepath.Join(configPath, "config.yaml")
	initialConfig := `llm:
  model: "initial-model"
shell:
  confirm: true
  allowed_commands: "ls,pwd"
`

	origData := []byte(`llm:
  model: "granite4:3b-h"
shell:
  confirm: true
  allowed_commands: "ls,pwd"
`)
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		origData = nil
	} else if err == nil {
		origData, _ = os.ReadFile(configFile)
	}

	defer func() {
		if origData != nil {
			os.WriteFile(configFile, origData, 0644)
		} else {
			os.Remove(configFile)
		}
	}()

	if err := os.WriteFile(configFile, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	newModel := "new-model:latest"
	if err := SaveModel(newModel); err != nil {
		t.Fatalf("SaveModel() error = %v", err)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	if !bytes.Contains(data, []byte(`model: "new-model:latest"`)) {
		t.Errorf("config file does not contain new model, got: %s", string(data))
	}
}

func TestSelectModelNoModels(t *testing.T) {
	origModels := getAvailableModelsFunc
	getAvailableModelsFunc = func() ([]ModelInfo, error) {
		return []ModelInfo{}, nil
	}
	defer func() { getAvailableModelsFunc = origModels }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()
	os.Stdin = r
	defer w.Close()

	go func() {
		w.WriteString("\n")
		w.Close()
	}()

	err = SelectModel()
	if err != nil {
		t.Fatalf("SelectModel() error = %v", err)
	}
}

func TestSelectModelEmptySelection(t *testing.T) {
	origModels := getAvailableModelsFunc
	getAvailableModelsFunc = func() ([]ModelInfo, error) {
		return []ModelInfo{
			{Name: "model1"},
			{Name: "model2"},
		}, nil
	}
	defer func() { getAvailableModelsFunc = origModels }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()
	os.Stdin = r
	defer w.Close()

	go func() {
		w.WriteString("\n")
		w.Close()
	}()

	err = SelectModel()
	if err != nil {
		t.Fatalf("SelectModel() error = %v", err)
	}
}

func TestSelectModelInvalidInput(t *testing.T) {
	origModels := getAvailableModelsFunc
	getAvailableModelsFunc = func() ([]ModelInfo, error) {
		return []ModelInfo{
			{Name: "model1"},
			{Name: "model2"},
		}, nil
	}
	defer func() { getAvailableModelsFunc = origModels }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()
	os.Stdin = r
	defer w.Close()

	go func() {
		w.WriteString("999\n")
		w.Close()
	}()

	err = SelectModel()
	if err != nil {
		t.Fatalf("SelectModel() error = %v", err)
	}
}
