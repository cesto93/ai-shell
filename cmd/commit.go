package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"ai-shell/config"
	"ai-shell/llm"
	"ai-shell/service"

	"github.com/spf13/cobra"
)

var execCommand = exec.Command

type noopExecutor struct{}

func (n noopExecutor) ExecuteTool(call llm.ToolCall) (string, error) {
	return "", nil
}

func (n noopExecutor) IsAllowedCommand(cmd string) bool {
	return true
}

func (n noopExecutor) AskConfirmation(cmd string) bool {
	return true
}

// commitCallLLM runs the commit prompt through the running service when
// available, falling back to a local LLM call otherwise.
func commitCallLLM(cfg *config.Config, systemPrompt string, messages []llm.Message) ([]llm.Message, error) {
	if service.IsActive() {
		req := service.ChatRequest{
			Messages:        messages,
			SystemPrompt:    systemPrompt,
			Model:           cfg.LLM.Model,
			Provider:        cfg.LLM.Provider,
			Confirm:         cfg.Shell.Confirm,
			AllowedCommands: cfg.Shell.AllowedCommands,
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		result, err := service.Chat(ctx, req)
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, service.ErrUnavailable) {
			return nil, err
		}
		slog.Debug("service unavailable, falling back to local execution", "err", err)
	}

	caller := llm.NewProviderCaller(cfg.LLM.Provider, cfg.LLM.Model, noopExecutor{})
	if lc, ok := caller.(*llm.LitertLMCaller); ok {
		lc.Backend = cfg.LitertLM.Backend
	}
	return caller.Call(context.Background(), systemPrompt, messages, nil)
}

var commitAll bool
var dryRun bool

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Generate a commit message for staged changes using AI",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCommit()
	},
}

func init() {
	commitCmd.Flags().BoolVarP(&commitAll, "all", "A", false, "stage all changes before committing")
	commitCmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "print the commit message without creating a commit")
}

func runCommit() error {
	var staged bool
	if commitAll {
		if out, err := execCommand("git", "add", "-A").CombinedOutput(); err != nil {
			return fmt.Errorf("git add -A failed: %w\n%s", err, out)
		}
		staged = true
	}

	diffOutput, err := execCommand("git", "diff", "--cached").Output()
	if err != nil {
		return fmt.Errorf("failed to get staged diff: %w", err)
	}

	if strings.TrimSpace(string(diffOutput)) == "" {
		return fmt.Errorf("no staged changes to commit (use git add to stage files)")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	initLogger(cfg)

	systemPrompt := "You are a helpful assistant that writes concise git commit messages."
	userPrompt := fmt.Sprintf(`Generate a concise git commit message for the following staged changes.

Staged changes (git diff --cached):
%s

Write a commit message using conventional commits format (e.g., feat:, fix:, chore:, docs:, refactor:, test:, style:).
The first line should be a concise summary under 72 characters.
If more detail is needed, add a blank line followed by bullet points or a short body.
Use the imperative mood ("add" not "added").
Only output the commit message, nothing else.`,
		strings.TrimSpace(string(diffOutput)),
	)

	messages := []llm.Message{
		{Role: "user", Content: userPrompt},
	}

	slog.Debug("provider", "name", cfg.LLM.Provider, "model", cfg.LLM.Model)
	slog.Debug("system prompt", "prompt", systemPrompt)
	slog.Debug("user prompt", "prompt", userPrompt)

	llmStart := time.Now()
	resultMessages, err := commitCallLLM(cfg, systemPrompt, messages)
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

	msg := strings.TrimSpace(content)
	msg = strings.TrimPrefix(msg, "```")
	msg = strings.TrimSuffix(msg, "```")
	msg = strings.TrimPrefix(msg, "text\n")
	msg = strings.TrimPrefix(msg, "markdown\n")
	msg = strings.TrimSpace(msg)

	if msg == "" {
		return fmt.Errorf("empty commit message after cleanup")
	}

	fmt.Printf("\n%s\n\n", msg)

	if dryRun {
		if staged {
			execCommand("git", "reset").Run()
		}
		return nil
	}

	tmpFile, err := os.CreateTemp("", "commit-msg-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(msg); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write commit message: %w", err)
	}
	tmpFile.Close()

	commitCmd := execCommand("git", "commit", "-F", tmpFile.Name())
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr

	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	return nil
}
