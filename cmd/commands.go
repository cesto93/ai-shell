package cmd

import (
	"context"
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

var commandsCmd = &cobra.Command{
	Use:   "commands",
	Short: "List all available custom commands",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCommands(args)
	},
}

func init() {
	commandsCmd.Flags().StringVar(&runCommandName, "run", "", "run a custom command by name")
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

	fullPrompt := found.Prompt
	if args := strings.TrimSpace(strings.Join(extraArgs, " ")); args != "" {
		fullPrompt = found.Prompt + " " + args
	}

	messages := []llm.Message{
		{Role: "user", Content: fullPrompt},
	}

	slog.Debug("provider", "name", cfg.LLM.Provider, "model", cfg.LLM.Model)
	slog.Debug("user prompt", "prompt", fullPrompt)

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
	content, ok := lastMsg.Content.(string)
	if !ok || strings.TrimSpace(content) == "" {
		return fmt.Errorf("empty response from LLM")
	}

	fmt.Println(strings.TrimSpace(content))
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
