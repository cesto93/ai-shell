package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ollama/ollama/api"
	"github.com/spf13/viper"
)

type Config struct {
	ConfigFile string
	LLM        struct {
		Model string `mapstructure:"model"`
	} `mapstructure:"llm"`
	Shell struct {
		Confirm         bool   `mapstructure:"confirm"`
		AllowedCommands string `mapstructure:"allowed_commands"`
	} `mapstructure:"shell"`
}

var configPaths = []string{"."}

var userConfigDirFunc = os.UserConfigDir

func LoadConfig() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.SetDefault("llm.model", "granite4:3b-h")
	v.SetDefault("shell.confirm", true)
	v.SetDefault("shell.allowed_commands", "ls,pwd")

	for _, path := range configPaths {
		v.AddConfigPath(path)
	}

	userConfigDir, err := userConfigDirFunc()
	var configPath string
	if err == nil {
		configPath = filepath.Join(userConfigDir, "ai-shell")
		v.AddConfigPath(configPath)
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			defaultConfig := &Config{
				ConfigFile: "",
				LLM: struct {
					Model string `mapstructure:"model"`
				}{
					Model: "granite4:3b-h",
				},
				Shell: struct {
					Confirm         bool   `mapstructure:"confirm"`
					AllowedCommands string `mapstructure:"allowed_commands"`
				}{
					Confirm:         true,
					AllowedCommands: "ls,pwd",
				},
			}

			if configPath != "" {
				err := os.MkdirAll(configPath, 0755)
				if err == nil {
					defaultConfigFile := filepath.Join(configPath, "config.yaml")
					if _, err := os.Stat(defaultConfigFile); os.IsNotExist(err) {
						content := "llm:\n  model: \"granite4:3b-h\"\nshell:\n  confirm: true\n  allowed_commands: \"ls,pwd\"\n"
						_ = os.WriteFile(defaultConfigFile, []byte(content), 0644)
						defaultConfig.ConfigFile = defaultConfigFile
					}
				}
			}

			return defaultConfig, nil
		}
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}
	config.ConfigFile = v.ConfigFileUsed()

	return &config, nil
}

func PrintConfig() {
	cfg, err := LoadConfig()
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

var getConfigPathFunc = getConfigPath

func getConfigPath() (string, error) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	configPath := filepath.Join(userConfigDir, "ai-shell")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return "", err
	}
	return configPath, nil
}

func SaveModel(modelName string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	configFile := cfg.ConfigFile
	if configFile == "" {
		configPath, err := getConfigPathFunc()
		if err != nil {
			return fmt.Errorf("failed to get config path: %w", err)
		}
		configFile = filepath.Join(configPath, "config.yaml")
	}

	cfg.LLM.Model = modelName

	content := fmt.Sprintf("llm:\n  model: %q\nshell:\n  confirm: %v\n  allowed_commands: %q\n", cfg.LLM.Model, cfg.Shell.Confirm, cfg.Shell.AllowedCommands)
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("%sModel changed to: %s%s%s\n", ColorGreen, ColorBold, modelName, ColorReset)
	return nil
}

type ModelInfo struct {
	Name       string
	Size       string
	ModifiedAt string
}

var getAvailableModelsFunc = GetAvailableModels

func GetAvailableModels() ([]ModelInfo, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to create Ollama client: %w", err)
	}

	models, err := client.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	var modelList []ModelInfo
	for _, model := range models.Models {
		modelList = append(modelList, ModelInfo{
			Name: model.Name,
		})
	}

	return modelList, nil
}

func SelectModel() error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	models, err := getAvailableModelsFunc()
	if err != nil {
		return err
	}

	if len(models) == 0 {
		fmt.Printf("%sNo models found. Please install models using 'ollama pull <model>'%s\n", ColorYellow, ColorReset)
		return nil
	}

	fmt.Printf("%sAvailable Models:%s\n\n", ColorBold+ColorCyan, ColorReset)

	for i, model := range models {
		marker := "  "
		if model.Name == cfg.LLM.Model {
			marker = "* "
		}
		fmt.Printf("%s[%d]%s %s%s%s\n", ColorCyan, i+1, ColorReset, marker, ColorGreen, model.Name)
	}

	fmt.Printf("\n%sEnter number to select model (or press Enter to cancel): %s", ColorBold, ColorReset)

	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(input)

	if input == "" {
		fmt.Printf("%sSelection cancelled.%s\n", ColorYellow, ColorReset)
		return nil
	}

	choice, err := strconv.Atoi(input)
	if err != nil || choice < 1 || choice > len(models) {
		fmt.Printf("%sInvalid selection.%s\n", ColorYellow, ColorReset)
		return nil
	}

	selectedModel := models[choice-1].Name
	return SaveModel(selectedModel)
}

func isAllowedCommand(cmd string, allowedList string) bool {
	if allowedList == "" {
		return false
	}
	allowed := strings.Split(allowedList, ",")
	for _, a := range allowed {
		a = strings.TrimSpace(a)
		if a == cmd {
			return true
		}
	}
	return false
}
