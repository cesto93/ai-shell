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
	"ai-shell/tools"

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

type cliExecutor struct{}

func (cliExecutor) ExecuteTool(call llm.ToolCall) (string, error) {
	switch call.Name {
	case "RunCommand":
		cmd, ok := call.Arguments["command"].(string)
		if !ok {
			return "Error: Invalid tool arguments", nil
		}
		output, err := tools.RunCommand(cmd)
		if err != nil {
			return fmt.Sprintf("Error: %v\nOutput: %s", err, output), nil
		}
		return output, nil

	case "WriteFile":
		path, ok1 := call.Arguments["path"].(string)
		content, ok2 := call.Arguments["content"].(string)
		if !ok1 || !ok2 {
			return "Error: Invalid tool arguments", nil
		}
		return tools.WriteFile(strings.TrimPrefix(path, "@"), content)

	case "ReadFile":
		path, ok := call.Arguments["path"].(string)
		if !ok {
			return "Error: Invalid tool arguments", nil
		}
		return tools.ReadFile(strings.TrimPrefix(path, "@"))

	case "KVSet":
		key, ok1 := call.Arguments["key"].(string)
		value, ok2 := call.Arguments["value"].(string)
		if !ok1 || !ok2 {
			return "Error: Invalid tool arguments", nil
		}
		return tools.KVSet(key, value)

	case "KVGet":
		key, ok := call.Arguments["key"].(string)
		if !ok {
			return "Error: Invalid tool arguments", nil
		}
		return tools.KVGet(key)

	case "KVList":
		return tools.KVList()

	default:
		return fmt.Sprintf("Error: Unknown tool %s", call.Name), nil
	}
}

func (cliExecutor) IsAllowedCommand(cmd string) bool {
	return true
}

func (cliExecutor) AskConfirmation(cmd string) bool {
	return true
}

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

	agent := llm.NewAgentFor(cfg.Agent, cfg.LLM.Model, cfg.LLM.Provider, cfg.Tools)
	agent.Backend = cfg.LitertLM.Backend
	agent.AgentFiles = llm.GetAgentFiles(cfg.AgentFiles)

	messages := []llm.Message{
		{Role: "user", Content: fullPrompt},
	}

	slog.Debug("provider", "name", cfg.LLM.Provider, "model", cfg.LLM.Model)
	slog.Debug("system prompt", "prompt", agent.Prompt)
	slog.Debug("user prompt", "prompt", fullPrompt)

	llmStart := time.Now()
	resultMessages, err := agent.CallLLM(context.Background(), cliExecutor{}, messages)
	llmDuration := time.Since(llmStart)
	if err != nil {
		return fmt.Errorf("LLM call failed: %w", err)
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
