//go:build integration

package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func requireProvider(t *testing.T, baseURL, model string) {
	t.Helper()
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(strings.TrimRight(baseURL, "/") + "/v1/models")
	if err != nil {
		t.Skipf("%s unreachable: %v", baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("%s returned status %d", baseURL, resp.StatusCode)
	}
	var mr modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		t.Skipf("%s returned invalid JSON: %v", baseURL, err)
	}
	if len(mr.Data) == 0 {
		t.Skipf("provider at %s returned no models", baseURL)
	}
	for _, m := range mr.Data {
		if m.ID == model {
			return
		}
	}
	var names []string
	for _, m := range mr.Data {
		names = append(names, m.ID)
	}
	t.Fatalf("model %q not found at %s; available: %s (set TEST_MODEL to override)", model, baseURL, strings.Join(names, ", "))
}

func mockGitExecCommand(diffPath string) func(string, ...string) *exec.Cmd {
	return func(command string, args ...string) *exec.Cmd {
		if command != "git" {
			return exec.Command(command, args...)
		}
		switch {
		case len(args) >= 2 && args[0] == "log" && args[1] == "--oneline":
			return exec.Command("sh", "-c",
				`echo "abc1234 fix: previous change"`)
		case len(args) >= 2 && args[0] == "diff" && args[1] == "--cached":
			return exec.Command("sh", "-c",
				"cat '"+diffPath+"'")
		case args[0] == "add":
			return exec.Command("true")
		case args[0] == "reset":
			return exec.Command("true")
		case args[0] == "commit":
			return exec.Command("true")
		default:
			return exec.Command("true")
		}
	}
}

func TestRunCommitWithRealLLM(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		model      string
		envVar     string
		defaultURL string
	}{
		{
			name:       "ollama-granite4-350m",
			provider:   "ollama",
			model:      "granite4:350m",
			envVar:     "OLLAMA_HOST",
			defaultURL: "http://localhost:11434",
		},
		{
			name:       "ollama-granite4-350m-h",
			provider:   "ollama",
			model:      "granite4:350m-h",
			envVar:     "OLLAMA_HOST",
			defaultURL: "http://localhost:11434",
		},
		{
			name:       "ollama-gemma3-270m",
			provider:   "ollama",
			model:      "gemma3:270m",
			envVar:     "OLLAMA_HOST",
			defaultURL: "http://localhost:11434",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseURL := os.Getenv(tt.envVar)
			if baseURL == "" {
				baseURL = tt.defaultURL
			}

			model := tt.model
			if m := os.Getenv("TEST_MODEL"); m != "" {
				model = m
			}

			requireProvider(t, baseURL, model)

			tmpDir := t.TempDir()

			origDir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}

			diffPath, err := filepath.Abs(filepath.Join("testdata", "commit_diff.txt"))
			if err != nil {
				t.Fatal(err)
			}

			os.Chdir(tmpDir)
			defer os.Chdir(origDir)

			os.Setenv(tt.envVar, baseURL)
			defer os.Unsetenv(tt.envVar)

			origExecCommand := execCommand
			execCommand = mockGitExecCommand(diffPath)
			defer func() { execCommand = origExecCommand }()

			configYAML := fmt.Sprintf("llm:\n  provider: %s\n  model: %s\n", tt.provider, model)
			if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(configYAML), 0644); err != nil {
				t.Fatal(err)
			}

			dryRun = false
			commitAll = false
			defer func() {
				dryRun = false
				commitAll = false
			}()

			if err := runCommit(); err != nil {
				t.Fatalf("runCommit() error = %v", err)
			}
		})
	}
}
