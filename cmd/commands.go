package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"ai-shell/config"
	"ai-shell/llm"

	"github.com/spf13/cobra"
)

var runCommandName string
var commandOutput string

var commandsCmd = &cobra.Command{
	Use:   "commands",
	Short: "List all available custom commands",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCommands(args)
	},
}

func init() {
	commandsCmd.Flags().StringVar(&runCommandName, "run", "", "run a custom command by name")
	commandsCmd.Flags().StringVarP(&commandOutput, "output", "o", "", "output file for structured commands (default: stdout)")
	rootCmd.AddCommand(commandsCmd)
}

func runCommands(args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	initLogger(cfg)

	if runCommandName != "" {
		return runCustomCommand(cfg, runCommandName, args)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	dir := filepath.Join(home, ".ai-shell", "commands")
	cmds, err := config.LoadCommandsFromDir(dir)
	if err != nil {
		return fmt.Errorf("failed to load commands: %w", err)
	}

	if len(cmds) == 0 {
		fmt.Println("No commands found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "COMMAND\tDESCRIPTION")
	fmt.Fprintln(w, "-------\t-----------")
	for _, c := range cmds {
		desc := c.Description
		if desc == "" {
			desc = "-"
		}
		fmt.Fprintf(w, "%s\t%s\n", c.Name, desc)
	}
	w.Flush()

	return nil
}

// runCustomCommand runs a custom command through the running service when
// available, falling back to a local LLM call otherwise.
func runCustomCommand(cfg *config.Config, name string, extraArgs []string) error {
	cmds := config.LoadCommands(cfg)
	var found *config.CommandInfo
	for i := range cmds {
		if cmds[i].Name == name {
			found = &cmds[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("command not found: %s", name)
	}

	if found.Schema != "" {
		return runStructuredCommand(cfg, found, extraArgs)
	}

	content, err := buildCommandContent(found.Prompt, extraArgs)
	if err != nil {
		return err
	}

	messages := []llm.Message{
		{Role: "user", Content: content},
	}

	slog.Debug("provider", "name", cfg.LLM.Provider, "model", cfg.LLM.Model)
	slog.Debug("user prompt", "prompt", found.Prompt)

	llmStart := time.Now()
	resultMessages, err := customCommandCallLLM(cfg, messages)
	llmDuration := time.Since(llmStart)
	if err != nil {
		return err
	}

	slog.Debug("timing", "llm", llmDuration)

	if len(resultMessages) == 0 {
		return fmt.Errorf("no response from LLM")
	}

	lastMsg := resultMessages[len(resultMessages)-1]
	responseStr, ok := lastMsg.Content.(string)
	if !ok || strings.TrimSpace(responseStr) == "" {
		return fmt.Errorf("empty response from LLM")
	}

	fmt.Println(strings.TrimSpace(responseStr))
	return nil
}

// runStructuredCommand runs a command that carries a JSON schema in its
// frontmatter. The schema is loaded and the LLM is called with a structured
// output response_format; the JSON result is printed (or written to
// commandOutput). Structured commands bypass the service, like extract did.
func runStructuredCommand(cfg *config.Config, cmd *config.CommandInfo, args []string) error {
	schemaData, err := os.ReadFile(cmd.Schema)
	if err != nil {
		return fmt.Errorf("failed to read schema: %w", err)
	}

	var schemaRaw any
	if err := json.Unmarshal(schemaData, &schemaRaw); err != nil {
		return fmt.Errorf("invalid JSON schema: %w", err)
	}

	content, err := buildCommandContent(cmd.Prompt, args)
	if err != nil {
		return err
	}

	responseFormat := map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   "extracted_data",
			"strict": true,
			"schema": schemaRaw,
		},
	}

	systemPrompt := "You extract structured data from documents and images. Return only valid JSON matching the provided schema."

	messages := []llm.Message{
		{Role: "user", Content: content},
	}

	slog.Debug("provider", "name", cfg.LLM.Provider, "model", cfg.LLM.Model)
	slog.Debug("schema", "path", cmd.Schema)

	caller := llm.NewProviderCallerRaw(cfg.LLM.Provider, cfg.LLM.Model, llm.NoopExecutor{})
	if lc, ok := caller.(*llm.LitertLMCaller); ok {
		lc.Backend = cfg.LitertLM.Backend
	}
	llmStart := time.Now()
	resultMessages, err := caller.CallStructured(context.Background(), systemPrompt, messages, nil, responseFormat)
	llmDuration := time.Since(llmStart)
	if err != nil {
		return fmt.Errorf("LLM call failed: %w", err)
	}

	slog.Debug("timing", "llm", llmDuration)

	if len(resultMessages) == 0 {
		return fmt.Errorf("no response from LLM")
	}

	lastMsg := resultMessages[len(resultMessages)-1]
	contentStr, ok := lastMsg.Content.(string)
	if !ok || strings.TrimSpace(contentStr) == "" {
		return fmt.Errorf("empty response from LLM")
	}

	result := strings.TrimSpace(contentStr)

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(result), "", "  "); err == nil {
		result = prettyJSON.String()
	}

	if commandOutput != "" {
		if err := os.WriteFile(commandOutput, []byte(result+"\n"), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	} else {
		fmt.Println(result)
	}

	return nil
}

// customCommandCallLLM runs the custom command through the running service
// when available, falling back to a local LLM call otherwise.
func customCommandCallLLM(cfg *config.Config, messages []llm.Message) ([]llm.Message, error) {
	req := chatRequestFromConfig(cfg, messages)
	return chatWithServiceFallback(req, func(err error) error {
		return fmt.Errorf("LLM call failed: %w", err)
	}, func() ([]llm.Message, error) {
		agent := llm.NewAgentForSession(cfg.Agent, cfg.LLM.Model, cfg.LLM.Provider, cfg.Tools, cfg.LitertLM.Backend, cfg.AgentFiles)
		slog.Debug("system prompt", "prompt", agent.Prompt)
		return agent.CallLLM(context.Background(), &llm.ToolExecutorPolicy{}, messages)
	})
}
