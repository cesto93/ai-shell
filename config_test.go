package main

import (
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
