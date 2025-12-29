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

// CallOllama sends a prompt to the Ollama model specified in the config
func CallOllama(ctx context.Context, prompt string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	client, err := api.ClientFromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create Ollama client: %w", err)
	}

	req := &api.GenerateRequest{
		Model:  cfg.LLM.Model,
		Prompt: prompt,
	}

	// Callback to handle streaming response
	respFunc := func(resp api.GenerateResponse) error {
		fmt.Print(resp.Response)
		return nil
	}

	if err := client.Generate(ctx, req, respFunc); err != nil {
		return fmt.Errorf("error during generation: %w", err)
	}

	fmt.Println()
	return nil
}
