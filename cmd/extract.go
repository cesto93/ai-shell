package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ai-shell/config"
	"ai-shell/llm"

	"github.com/spf13/cobra"
)

type noopExtractExecutor struct{}

func (n noopExtractExecutor) ExecuteTool(call llm.ToolCall) (string, error) {
	return "", nil
}
func (n noopExtractExecutor) IsAllowedCommand(cmd string) bool {
	return true
}
func (n noopExtractExecutor) AskConfirmation(cmd string) bool {
	return true
}

var extractOutput string

var extractCmd = &cobra.Command{
	Use:   "extract <input> <schema>",
	Short: "Extract structured data from a document using a JSON schema",
	Long: `Extract structured data from a document (.txt, .md, .pdf) using a JSON schema.
The document text is sent to the LLM which returns data conforming to the schema.

Example:
  ai-shell extract invoice.pdf schema.json
  ai-shell extract notes.txt schema.json --output result.json`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runExtract(args[0], args[1])
	},
}

func init() {
	rootCmd.AddCommand(extractCmd)
	extractCmd.Flags().StringVarP(&extractOutput, "output", "o", "", "output file (default: stdout)")
}

func readInputFile(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", path, err)
		}
		return string(data), nil
	case ".pdf":
		return readPDF(path)
	default:
		return "", fmt.Errorf("unsupported file extension %q (supported: .txt, .md, .pdf)", ext)
	}
}

func readPDF(path string) (string, error) {
	out, err := exec.Command("pdftotext", path, "-").Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext failed (is poppler-utils installed?): %w", err)
	}
	return string(out), nil
}

func runExtract(inputPath, schemaPath string) error {
	text, err := readInputFile(inputPath)
	if err != nil {
		return err
	}

	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema: %w", err)
	}

	var schemaRaw any
	if err := json.Unmarshal(schemaData, &schemaRaw); err != nil {
		return fmt.Errorf("invalid JSON schema: %w", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	responseFormat := map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   "extracted_data",
			"strict": true,
			"schema": schemaRaw,
		},
	}

	systemPrompt := "You extract structured data from documents. Return only valid JSON matching the provided schema."
	userPrompt := fmt.Sprintf("Extract structured data from the following document text according to the schema:\n\n%s", text)

	messages := []llm.Message{
		{Role: "user", Content: userPrompt},
	}

	if cfg.LogLevel == "debug" {
		log.Printf("[debug] provider=%s model=%s", cfg.LLM.Provider, cfg.LLM.Model)
	}

	caller := llm.NewProviderCallerRaw(cfg.LLM.Provider, cfg.LLM.Model, noopExtractExecutor{})
	llmStart := time.Now()
	resultMessages, err := caller.CallStructured(context.Background(), systemPrompt, messages, nil, responseFormat)
	llmDuration := time.Since(llmStart)
	if err != nil {
		return fmt.Errorf("LLM call failed: %w", err)
	}

	if cfg.LogLevel == "debug" {
		log.Printf("[debug] timing: llm=%v", llmDuration)
	}

	if len(resultMessages) == 0 {
		return fmt.Errorf("no response from LLM")
	}

	lastMsg := resultMessages[len(resultMessages)-1]
	content, ok := lastMsg.Content.(string)
	if !ok || strings.TrimSpace(content) == "" {
		return fmt.Errorf("empty response from LLM")
	}

	result := strings.TrimSpace(content)

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(result), "", "  "); err == nil {
		result = prettyJSON.String()
	}

	if extractOutput != "" {
		if err := os.WriteFile(extractOutput, []byte(result+"\n"), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	} else {
		fmt.Println(result)
	}

	return nil
}
