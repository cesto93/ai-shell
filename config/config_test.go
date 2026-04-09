package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ai-shell")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		t.Fatalf("Failed to create temp config dir: %v", err)
	}

	origConfigPaths := configPaths
	defer func() { configPaths = origConfigPaths }()
	configPaths = []string{tmpDir}

	origUserConfigDirFunc := userConfigDirFunc
	userConfigDirFunc = func() (string, error) {
		return tmpDir, nil
	}
	defer func() { userConfigDirFunc = origUserConfigDirFunc }()

	tests := []struct {
		name          string
		configContent string
		configExists  bool
		wantModel     string
		wantConfirm   bool
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
		},
		{
			name:          "default config when file not found",
			configContent: "",
			configExists:  false,
			wantModel:     "granite4:3b-h",
			wantConfirm:   true,
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile := filepath.Join(configPath, "config.yaml")
			if tt.configExists {
				if err := os.WriteFile(configFile, []byte(tt.configContent), 0644); err != nil {
					t.Fatalf("Failed to write config file: %v", err)
				}
			} else {
				os.Remove(configFile)
			}

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

	configFile := filepath.Join(configPath, "config.yaml")
	initialConfig := `llm:
  model: "initial-model"
shell:
  confirm: true
  allowed_commands: "ls,pwd"
`

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

func TestSaveModelCreatesNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ai-shell")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		t.Fatalf("Failed to create temp config dir: %v", err)
	}
	configFile := filepath.Join(configPath, "config.yaml")

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

	if _, err := os.Stat(configFile); err == nil {
		os.Remove(configFile)
	}

	newModel := "brand-new-model:latest"
	if err := SaveModel(newModel); err != nil {
		t.Fatalf("SaveModel() error = %v", err)
	}

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Fatal("SaveModel did not create config file")
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	if !bytes.Contains(data, []byte(`model: "brand-new-model:latest"`)) {
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

func TestSelectModelSavesNewModel(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ai-shell")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		t.Fatalf("Failed to create temp config dir: %v", err)
	}
	configFile := filepath.Join(configPath, "config.yaml")

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

	initialConfig := `llm:
  model: "initial-model"
shell:
  confirm: true
`
	if err := os.WriteFile(configFile, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	origModels := getAvailableModelsFunc
	getAvailableModelsFunc = func() ([]ModelInfo, error) {
		return []ModelInfo{
			{Name: "model1"},
			{Name: "model2"},
			{Name: "model3"},
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
		w.WriteString("2\n")
		w.Close()
	}()

	err = SelectModel()
	if err != nil {
		t.Fatalf("SelectModel() error = %v", err)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	if !bytes.Contains(data, []byte(`model: "model2"`)) {
		t.Errorf("config file does not contain selected model 'model2', got: %s", string(data))
	}
}

func TestIsAllowedCommand(t *testing.T) {
	tests := []struct {
		name        string
		cmd         string
		allowedList string
		want        bool
	}{
		{
			name:        "command is allowed",
			cmd:         "ls",
			allowedList: "ls,pwd,cat",
			want:        true,
		},
		{
			name:        "command is not allowed",
			cmd:         "rm",
			allowedList: "ls,pwd,cat",
			want:        false,
		},
		{
			name:        "empty allowed list",
			cmd:         "ls",
			allowedList: "",
			want:        false,
		},
		{
			name:        "command with spaces in allowed list",
			cmd:         "ls",
			allowedList: "ls , pwd",
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAllowedCommand(tt.cmd, tt.allowedList)
			if got != tt.want {
				t.Errorf("IsAllowedCommand(%q, %q) = %v, want %v", tt.cmd, tt.allowedList, got, tt.want)
			}
		})
	}
}
