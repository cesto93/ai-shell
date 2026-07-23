package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"ai-shell/config"
	"ai-shell/llm"

	"github.com/spf13/cobra"
)

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

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Generate a commit message for staged changes using AI",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCommit()
	},
}

func runCommit() error {
	logOutput, err := exec.Command("git", "log", "--oneline", "-5").Output()
	if err != nil {
		return fmt.Errorf("not a git repository or no commits yet: %w", err)
	}

	diffOutput, err := exec.Command("git", "diff", "--cached").Output()
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

	systemPrompt := "You are a helpful assistant that writes concise git commit messages."
	userPrompt := fmt.Sprintf(`Generate a concise git commit message for the following staged changes.

Recent commits (for context):
%s

Staged changes (git diff --cached):
%s

Write a commit message using conventional commits format (e.g., feat:, fix:, chore:, docs:, refactor:, test:, style:).
The first line should be a concise summary under 72 characters.
If more detail is needed, add a blank line followed by bullet points or a short body.
Use the imperative mood ("add" not "added").
Only output the commit message, nothing else.`,
		strings.TrimSpace(string(logOutput)),
		strings.TrimSpace(string(diffOutput)),
	)

	messages := []llm.Message{
		{Role: "user", Content: userPrompt},
	}

	caller := llm.NewProviderCaller(cfg.LLM.Provider, cfg.LLM.Model, noopExecutor{})
	resultMessages, err := caller.Call(context.Background(), systemPrompt, messages, nil)
	if err != nil {
		return fmt.Errorf("LLM call failed: %w", err)
	}

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

	commitCmd := exec.Command("git", "commit", "-F", tmpFile.Name())
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr

	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	return nil
}
