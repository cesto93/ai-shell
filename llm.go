package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ollama/ollama/api"
	"github.com/spf13/viper"
)

// Config structure to match config.yaml
type Config struct {
	LLM struct {
		Model string `mapstructure:"model"`
	} `mapstructure:"llm"`
}

// LoadConfig reads the configuration from config.yaml in various locations
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".") // Current directory

	// Add user-specific config directory
	userConfigDir, err := os.UserConfigDir()
	var configPath string
	if err == nil {
		configPath = filepath.Join(userConfigDir, "ai-shell")
		viper.AddConfigPath(configPath)
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// If config not found, return default configuration
			defaultConfig := &Config{
				LLM: struct {
					Model string `mapstructure:"model"`
				}{
					Model: "granite4:3b-h",
				},
			}

			// Optionally: try to create a default config file in the user config dir
			if configPath != "" {
				err := os.MkdirAll(configPath, 0755)
				if err == nil {
					defaultConfigFile := filepath.Join(configPath, "config.yaml")
					if _, err := os.Stat(defaultConfigFile); os.IsNotExist(err) {
						// Create default config file
						content := "llm:\n  model: \"granite4:3b-h\"\n"
						_ = os.WriteFile(defaultConfigFile, []byte(content), 0644)
					}
				}
			}

			return defaultConfig, nil
		}
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

// PrintConfig prints the current configuration to the console
func PrintConfig() {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Printf("%sError loading config: %v%s\n", ColorYellow, err, ColorReset)
		return
	}

	fmt.Printf("%sCurrent Configuration:%s\n", ColorBold+ColorCyan, ColorReset)
	fmt.Printf("Model: %s%s%s\n", ColorGreen, cfg.LLM.Model, ColorReset)

	// Show config file location
	configUsed := viper.ConfigFileUsed()
	if configUsed != "" {
		fmt.Printf("Config file: %s%s%s\n", ColorBlue, configUsed, ColorReset)
	} else {
		fmt.Printf("Config file: %sNone (using defaults)%s\n", ColorYellow, ColorReset)
	}
}
