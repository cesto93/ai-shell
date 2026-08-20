//go:build integration

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"ai-shell/config"
)

func TestRunStructuredCommandWithRealLLM(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		model      string
		envVar     string
		defaultURL string
	}{
		{
			name:       "ollama",
			provider:   "ollama",
			model:      "granite4:350m",
			envVar:     "OLLAMA_HOST",
			defaultURL: "http://localhost:11434",
		},
	}

	schema := `{
		"type": "object",
		"properties": {
			"invoice_number": { "type": "string" },
			"vendor":         { "type": "string" },
			"total_amount":   { "type": "number" }
		},
		"required": ["invoice_number", "vendor", "total_amount"]
	}`

	input := `Invoice INV-2024-001
Vendor: Acme Corp
Total: $1,234.56`

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
			os.Chdir(tmpDir)
			defer os.Chdir(origDir)

			os.Setenv(tt.envVar, baseURL)
			defer os.Unsetenv(tt.envVar)

			if err := os.WriteFile(filepath.Join(tmpDir, "invoice.txt"), []byte(input), 0644); err != nil {
				t.Fatal(err)
			}

			schemaPath := filepath.Join(tmpDir, "schema.json")
			if err := os.WriteFile(schemaPath, []byte(schema), 0644); err != nil {
				t.Fatal(err)
			}

			commandsDir := filepath.Join(tmpDir, ".ai-shell", "commands")
			if err := os.MkdirAll(commandsDir, 0755); err != nil {
				t.Fatal(err)
			}
			cmdContent := fmt.Sprintf("---\ndescription: Extract invoice data\nschema: %s\n---\nExtract structured data from the invoice.\n", schemaPath)
			if err := os.WriteFile(filepath.Join(commandsDir, "invoice.md"), []byte(cmdContent), 0644); err != nil {
				t.Fatal(err)
			}

			configYAML := fmt.Sprintf("llm:\n  provider: %s\n  model: %s\n", tt.provider, model)
			if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(configYAML), 0644); err != nil {
				t.Fatal(err)
			}

			cfg, err := config.LoadConfig()
			if err != nil {
				t.Fatalf("failed to load config: %v", err)
			}
			commandOutput = ""
			if err := runCustomCommand(cfg, "invoice", []string{"invoice.txt"}); err != nil {
				t.Fatalf("runCustomCommand() error = %v", err)
			}
		})
	}
}
