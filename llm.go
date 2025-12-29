package main

import (
	"context"
	"fmt"

	"github.com/ollama/ollama/api"
	"github.com/spf13/viper"
)

// Config structure to match config.yaml
type Config struct {
	LLM struct {
		Model string `mapstructure:"model"`
	} `mapstructure:"llm"`
}

// LoadConfig reads the configuration from config.yaml
func LoadConfig() (*Config, error) {
	viper.SetConfigFile("config.yaml")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

// CallOllama sends a prompt to the Ollama model specified in the config and handles tools
func CallOllama(ctx context.Context, prompt string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	client, err := api.ClientFromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create Ollama client: %w", err)
	}

	// Define the RunCommand tool
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

	distro := GetDistro()
	shell := GetShell()
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

		// Add model's message to conversation history
		messages = append(messages, response.Message)

		if len(response.Message.ToolCalls) == 0 {
			break
		}

		// Handle tool calls
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

				fmt.Printf("\n%s[Executing: %s]%s\n", ColorCyan, cmd, ColorReset)
				output, err := RunCommand(cmd)
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
