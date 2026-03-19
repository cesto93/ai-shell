package main

import (
	"context"
	"fmt"
	"os"
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
	PrintInteractiveHelp()

	rl, err := NewReadline()
	if err != nil {
		fmt.Printf("%sError initializing readline: %v%s\n", ColorYellow, err, ColorReset)
		os.Exit(1)
	}
	defer rl.Close()

	ctx := context.Background()

	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}

		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "/") {
			trimmed = trimmed[1:]
			trimmed = strings.TrimSpace(trimmed)

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

			if trimmed == "models" {
				if err := SelectModel(); err != nil {
					fmt.Printf("%sError: %v%s\n", ColorYellow, err, ColorReset)
				}
				continue
			}

			fmt.Printf("%sUnknown command: /%s%s\n", ColorYellow, trimmed, ColorReset)
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

		fmt.Printf("%sLLM Response:%s\n", ColorBlue, ColorReset)
		err = CallOllama(ctx, trimmed)
		if err != nil {
			fmt.Printf("%sError: %v%s\n", ColorYellow, err, ColorReset)
		}
	}

	fmt.Printf("%sGoodbye!%s\n", ColorCyan, ColorReset)
}

func PrintInteractiveHelp() {
	fmt.Printf("Type your requests to the AI or use special commands below.\n\n")
	fmt.Printf("%sCommands (slash syntax):%s\n", ColorBold, ColorReset)
	fmt.Printf("  %s/help%s         - Show this help message\n", ColorGreen, ColorReset)
	fmt.Printf("  %s/get-config%s   - Show current LLM settings\n", ColorGreen, ColorReset)
	fmt.Printf("  %s/models%s       - Switch to a different model\n", ColorGreen, ColorReset)
	fmt.Printf("  %s/exit%s, %s/quit%s   - Exit the shell\n", ColorGreen, ColorReset, ColorGreen, ColorReset)
	fmt.Printf("  %s/<command>%s     - Use commands (Tab to autocomplete)\n", ColorGreen, ColorReset)
	fmt.Printf("  %s@<file>%s       - Autocomplete file paths (Tab after @)\n", ColorGreen, ColorReset)
	fmt.Printf("  %s<text>%s        - Send text to the AI for a response\n\n", ColorGreen, ColorReset)
}
