package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ai-shell",
	Short: "AI Shell is an interactive shell powered by AI",
	Long:  `An interactive shell powered by AI (Ollama) that can help you with commands and explanations.`,
	Example: `  ai-shell
  ai-shell get-config
  echo "how do I list files?" | ai-shell`,
	Run: func(cmd *cobra.Command, args []string) {
		startInteractiveShell()
	},
}

var configCmd = &cobra.Command{
	Use:   "get-config",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		PrintConfig()
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func startInteractiveShell() {
	fmt.Printf("%s%sStarting AI Shell...%s\n", ColorBold, ColorCyan, ColorReset)
	PrintInteractiveHelp()

	scanner := bufio.NewScanner(os.Stdin)
	ctx := context.Background()

	for {
		fmt.Printf("%s%sai-shell > %s", ColorBold, ColorGreen, ColorReset)
		if !scanner.Scan() {
			break
		}

		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			continue
		}

		if trimmed == "exit" || trimmed == "quit" {
			break
		}

		if trimmed == "get-config" {
			PrintConfig()
			continue
		}

		if trimmed == "help" {
			PrintInteractiveHelp()
			continue
		}

		// Handle shell commands
		if strings.HasPrefix(trimmed, "!") {
			executeShellCommand(trimmed[1:])
			continue
		}

		fmt.Printf("%sLLM Response:%s\n", ColorBlue, ColorReset)
		err := CallOllama(ctx, trimmed)
		if err != nil {
			fmt.Printf("%sError: %v%s\n", ColorYellow, err, ColorReset)
		}
	}

	fmt.Printf("%sGoodbye!%s\n", ColorCyan, ColorReset)
}

func PrintInteractiveHelp() {
	fmt.Printf("%s%sAI Shell Interactive Mode%s\n", ColorBold, ColorCyan, ColorReset)
	fmt.Printf("Type your requests to the AI or use special commands below.\n\n")
	fmt.Printf("%sCommands:%s\n", ColorBold, ColorReset)
	fmt.Printf("  %shelp%s           - Show this help message\n", ColorGreen, ColorReset)
	fmt.Printf("  %sget-config%s     - Show current LLM settings\n", ColorGreen, ColorReset)
	fmt.Printf("  %sexit%s, %squit%s      - Exit the shell\n", ColorGreen, ColorReset, ColorGreen, ColorReset)
	fmt.Printf("  %s! <command>%s    - Execute a system shell command directly\n", ColorGreen, ColorReset)
	fmt.Printf("  %s<text>%s         - Send text to the AI for a response\n\n", ColorGreen, ColorReset)
}

func executeShellCommand(commandLine string) {
	commandLine = strings.TrimSpace(commandLine)
	if commandLine == "" {
		return
	}

	cfg, err := LoadConfig()
	if err == nil && cfg.Shell.Confirm {
		fmt.Printf("%sConfirm execution of: %s%s [y/N]: ", ColorBold, commandLine, ColorReset)
		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Printf("%sCommand cancelled.%s\n", ColorYellow, ColorReset)
			return
		}
	}

	cmd := exec.Command("bash", "-c", commandLine)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err = cmd.Run()
	if err != nil {
		fmt.Printf("%sCommand failed: %v%s\n", ColorYellow, err, ColorReset)
	}
}
