package cmd

import (
	"context"
	"fmt"
	"strings"

	"ai-shell/config"
	"ai-shell/tools"

	"github.com/ollama/ollama/api"
	"github.com/spf13/cobra"
)

const (
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorReset  = "\033[0m"
	ColorBold   = "\033[1m"
	Prompt      = ColorBold + ColorGreen + "ai-shell > " + ColorReset
)

var getConfigCmd = &cobra.Command{
	Use:   "get-config",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		PrintConfig()
	},
}

func PrintConfig() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("%sError loading config: %v%s\n", ColorYellow, err, ColorReset)
		return
	}

	fmt.Printf("%sCurrent Configuration:%s\n", ColorBold+ColorCyan, ColorReset)
	fmt.Printf("Model: %s%s%s\n", ColorGreen, cfg.LLM.Model, ColorReset)
	fmt.Printf("Confirm Commands: %s%v%s\n", ColorGreen, cfg.Shell.Confirm, ColorReset)
	fmt.Printf("Allowed Commands: %s%s%s\n", ColorGreen, cfg.Shell.AllowedCommands, ColorReset)

	if cfg.ConfigFile != "" {
		fmt.Printf("Config file: %s%s%s\n", ColorBlue, cfg.ConfigFile, ColorReset)
	} else {
		fmt.Printf("Config file: %sNone (using defaults)%s\n", ColorYellow, ColorReset)
	}
}

func StartInteractiveShell() {
	PrintInteractiveHelp()

	rl, err := NewReadline()
	if err != nil {
		fmt.Printf("%sError initializing readline: %v%s\n", ColorYellow, err, ColorReset)
		return
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
				if err := config.SelectModel(); err != nil {
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

func CallOllama(ctx context.Context, prompt string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	client, err := api.ClientFromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create Ollama client: %w", err)
	}

	runCommandTool := api.Tool{
		Type: "function",
		Function: api.ToolFunction{
			Name:        "RunCommand",
			Description: "Execute a shell command and return its output",
			Parameters: api.ToolFunctionParameters{
				Type:     "object",
				Required: []string{"command"},
				Properties: map[string]api.ToolProperty{
					"command": {
						Type:        api.PropertyType{"string"},
						Description: "The shell command to execute (e.g., 'ls -la', 'echo hello')",
					},
				},
			},
		},
	}

	distro := tools.GetDistro()
	shell := tools.GetShell()
	systemPrompt := fmt.Sprintf("You are a helpful shell assistant. The user is running on %s using %s shell. Use this information to tailor your commands and advice.", distro, shell)

	messages := []api.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}

	for {
		req := &api.ChatRequest{
			Model:    cfg.LLM.Model,
			Messages: messages,
			Tools:    []api.Tool{runCommandTool},
			Stream:   new(bool),
		}
		*req.Stream = false

		var response api.ChatResponse
		respFunc := func(resp api.ChatResponse) error {
			response = resp
			if resp.Message.Content != "" {
				fmt.Print(resp.Message.Content)
			}
			return nil
		}

		if err := client.Chat(ctx, req, respFunc); err != nil {
			return fmt.Errorf("error during chat: %w", err)
		}

		messages = append(messages, response.Message)

		if len(response.Message.ToolCalls) == 0 {
			break
		}

		for _, tc := range response.Message.ToolCalls {
			if tc.Function.Name == "RunCommand" {
				cmd, ok := tc.Function.Arguments["command"].(string)
				if !ok {
					result := "Error: Invalid tool arguments"
					fmt.Printf("\n%s%s%s\n", ColorYellow, result, ColorReset)
					messages = append(messages, api.Message{
						Role:    "tool",
						Content: result,
					})
					continue
				}

				cmdName := getCommandName(cmd)
				skipConfirm := config.IsAllowedCommand(cmdName, cfg.Shell.AllowedCommands)

				if cfg.Shell.Confirm && !skipConfirm {
					fmt.Printf("\n%s[LLM wants to execute: %s]%s\n", ColorCyan, cmd, ColorReset)
					fmt.Printf("%sConfirm execution? [y/N]: %s", ColorBold, ColorReset)
					var response string
					fmt.Scanln(&response)
					response = strings.ToLower(strings.TrimSpace(response))
					if response != "y" && response != "yes" {
						result := "Error: Command execution denied by user"
						fmt.Printf("%s%s%s\n", ColorYellow, result, ColorReset)
						messages = append(messages, api.Message{
							Role:    "tool",
							Content: result,
						})
						continue
					}
				} else {
					fmt.Printf("\n%s[Executing: %s]%s\n", ColorCyan, cmd, ColorReset)
				}
				output, err := tools.RunCommand(cmd)
				if err != nil {
					result := fmt.Sprintf("Error: %v\nOutput: %s", err, output)
					fmt.Printf("%s%s%s\n", ColorYellow, result, ColorReset)
					messages = append(messages, api.Message{
						Role:    "tool",
						Content: result,
					})
				} else {
					fmt.Printf("%s%s%s", ColorReset, output, ColorReset)
					messages = append(messages, api.Message{
						Role:    "tool",
						Content: output,
					})
				}
			}
		}
	}

	fmt.Println()
	return nil
}

func getCommandName(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) > 0 {
		return parts[0]
	}
	return cmd
}
