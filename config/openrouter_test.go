package config

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestModelInListOpenRouter(t *testing.T) {
	origGetOpenRouterModelsFunc := getOpenRouterModelsFunc
	getOpenRouterModelsFunc = func() []ModelInfo {
		return []ModelInfo{
			{Name: "nvidia/nemotron-3-super-120b-a12b:free", Provider: "openrouter"},
			{Name: "google/gemma-4-31b-it:free", Provider: "openrouter"},
			{Name: "deepseek/deepseek-v4-flash-0731", Provider: "openrouter"},
		}
	}
	defer func() { getOpenRouterModelsFunc = origGetOpenRouterModelsFunc }()

	tests := []struct {
		model string
		want  bool
	}{
		{"nvidia/nemotron-3-super-120b-a12b:free", true},
		{"google/gemma-4-31b-it:free", true},
		{"deepseek/deepseek-v4-flash-0731", true},
		{"gemini-3-flash-preview", false},
		{"other-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := modelInList(tt.model, getOpenRouterModelsFunc()); got != tt.want {
				t.Errorf("modelInList(%q, GetOpenRouterModels()) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestGetOpenRouterModels(t *testing.T) {
	origURL := openRouterModelsURL
	defer func() { openRouterModelsURL = origURL }()
	origCache := openRouterModelsCache
	defer func() { openRouterModelsCache = origCache }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"data":[
			{"id":"org/free-1","pricing":{"prompt":"0","completion":"0"}},
			{"id":"org/free-2","pricing":{"prompt":"0.00","completion":"0"}},
			{"id":"org/paid-1","pricing":{"prompt":"0.0000001","completion":"0.00000025"}}
		]}`)
	}))
	defer server.Close()

	openRouterModelsURL = server.URL
	openRouterModelsCache = nil

	models := GetOpenRouterModels()

	if len(models) != 2 {
		t.Fatalf("GetOpenRouterModels() = %d models, want 2", len(models))
	}
	names := map[string]bool{}
	for _, m := range models {
		if m.Provider != "openrouter" {
			t.Errorf("model %q provider = %q, want openrouter", m.Name, m.Provider)
		}
		names[m.Name] = true
	}
	if !names["org/free-1"] || !names["org/free-2"] {
		t.Errorf("GetOpenRouterModels() = %v, want free models org/free-1 and org/free-2", names)
	}
	if names["org/paid-1"] {
		t.Errorf("GetOpenRouterModels() includes paid model org/paid-1")
	}
}

func TestGetOpenRouterModelsFetchError(t *testing.T) {
	origURL := openRouterModelsURL
	defer func() { openRouterModelsURL = origURL }()
	origCache := openRouterModelsCache
	defer func() { openRouterModelsCache = origCache }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	openRouterModelsURL = server.URL
	openRouterModelsCache = nil

	if models := GetOpenRouterModels(); models != nil {
		t.Errorf("GetOpenRouterModels() = %v, want nil on error", models)
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

	origGetOpenRouterModelsFunc := getOpenRouterModelsFunc
	getOpenRouterModelsFunc = func() []ModelInfo {
		return []ModelInfo{{Name: "nvidia/nemotron-3-super-120b-a12b:free", Provider: "openrouter"}}
	}
	defer func() { getOpenRouterModelsFunc = origGetOpenRouterModelsFunc }()

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
