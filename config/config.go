package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ollama/ollama/api"
	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
)

type CommandInfo struct {
	Name        string
	Description string
	Prompt      string
}

type Config struct {
	ConfigFile string
	LogLevel   string `mapstructure:"log_level"`
	LLM        struct {
		Provider   string   `mapstructure:"provider"`
		Model      string   `mapstructure:"model"`
		InputTypes []string `mapstructure:"input_types"`
	} `mapstructure:"llm"`
	Shell struct {
		Confirm         bool     `mapstructure:"confirm"`
		AllowedCommands []string `mapstructure:"allowed_commands"`
	} `mapstructure:"shell"`
	LitertLM struct {
		AutoStart bool `mapstructure:"auto_start"`
	} `mapstructure:"litertlm"`
	Tools    map[string]bool   `mapstructure:"tools"`
	Commands map[string]string `mapstructure:"commands"`
}

var configPaths = []string{"."}

var userConfigDirFunc = os.UserConfigDir

var loadEnvFunc = loadEnv

func loadEnv() error {
	// Load from user config directory first (global defaults)
	userConfigDir, err := userConfigDirFunc()
	if err == nil {
		globalEnvPath := filepath.Join(userConfigDir, "ai-shell", ".env")
		if _, err := os.Stat(globalEnvPath); err == nil {
			if err := gotenv.Load(globalEnvPath); err != nil {
				return fmt.Errorf("error loading global .env file at %s: %w", globalEnvPath, err)
			}
		}
	}

	// Load from current directory (local overrides)
	envPath := ".env"
	if _, err := os.Stat(envPath); err == nil {
		if err := gotenv.Load(envPath); err != nil {
			return fmt.Errorf("error loading .env file: %w", err)
		}
	}

	return nil
}

func LoadConfig() (*Config, error) {
	if err := loadEnvFunc(); err != nil {
		return nil, fmt.Errorf("error loading .env file: %w", err)
	}

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.SetDefault("llm.provider", "ollama")
	v.SetDefault("llm.model", "granite4:3b-h")
	v.SetDefault("shell.confirm", true)
	v.SetDefault("shell.allowed_commands", []string{"ls", "pwd", "git"})
	v.SetDefault("log_level", "info")
	v.SetDefault("litertlm.auto_start", true)
	v.SetDefault("tools", map[string]bool{
		"RunCommand": true,
		"WriteFile":  true,
		"ReadFile":   true,
		"KVSet":      true,
		"KVGet":      true,
		"KVList":     true,
	})

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
				LogLevel:   "info",
				LLM: struct {
					Provider   string   `mapstructure:"provider"`
					Model      string   `mapstructure:"model"`
					InputTypes []string `mapstructure:"input_types"`
				}{
					Provider:   "ollama",
					Model:      "granite4:3b-h",
					InputTypes: []string{"text"},
				},
				Shell: struct {
					Confirm         bool     `mapstructure:"confirm"`
					AllowedCommands []string `mapstructure:"allowed_commands"`
				}{
					Confirm:         true,
					AllowedCommands: []string{"ls", "pwd", "git"},
				},
				LitertLM: struct {
					AutoStart bool `mapstructure:"auto_start"`
				}{
					AutoStart: true,
				},
				Tools: map[string]bool{
					"RunCommand": true,
					"WriteFile":  true,
					"ReadFile":   true,
					"KVSet":      true,
					"KVGet":      true,
					"KVList":     true,
				},
			}

			if configPath != "" {
				err := os.MkdirAll(configPath, 0755)
				if err == nil {
					defaultConfigFile := filepath.Join(configPath, "config.yaml")
					if _, err := os.Stat(defaultConfigFile); os.IsNotExist(err) {
						content := "log_level: \"info\"\nllm:\n  provider: \"ollama\"\n  model: \"granite4:3b-h\"\n  input_types:\n    - \"text\"\nshell:\n  confirm: true\n  allowed_commands:\n    - \"ls\"\n    - \"pwd\"\n    - \"git\"\nlitertlm:\n  auto_start: true\ntools:\n  RunCommand: true\n  WriteFile: true\n  ReadFile: true\n  KVSet: true\n  KVGet: true\n  KVList: true\n"
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

	if config.Tools == nil {
		config.Tools = map[string]bool{
			"RunCommand": true,
			"WriteFile":  true,
			"ReadFile":   true,
			"KVSet":      true,
			"KVGet":      true,
			"KVList":     true,
		}
	}

	if info := lookupModelInfo(config.LLM.Model); info != nil && len(info.InputTypes) > 0 {
		config.LLM.InputTypes = info.InputTypes
	}

	return &config, nil
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

func modelInList(modelName string, models []ModelInfo) bool {
	for _, m := range models {
		if m.Name == modelName {
			return true
		}
	}
	return false
}

func IsLitertLMModel(modelName string) bool {
	models, err := GetLitertLMModels()
	if err != nil {
		return false
	}
	return modelInList(modelName, models)
}

func SaveConfig(cfg *Config) error {
	configFile := cfg.ConfigFile
	if configFile == "" {
		configPath, err := getConfigPathFunc()
		if err != nil {
			return fmt.Errorf("failed to get config path: %w", err)
		}
		configFile = filepath.Join(configPath, "config.yaml")
	}

	out := struct {
		LogLevel string `yaml:"log_level"`
		LLM      struct {
			Provider   string   `yaml:"provider"`
			Model      string   `yaml:"model"`
			InputTypes []string `yaml:"input_types,omitempty"`
		} `yaml:"llm"`
		Shell struct {
			Confirm         bool     `yaml:"confirm"`
			AllowedCommands []string `yaml:"allowed_commands,omitempty"`
		} `yaml:"shell"`
		LitertLM struct {
			AutoStart bool `yaml:"auto_start"`
		} `yaml:"litertlm"`
		Tools    map[string]bool   `yaml:"tools,omitempty"`
		Commands map[string]string `yaml:"commands,omitempty"`
	}{
		LogLevel: cfg.LogLevel,
		Tools:    cfg.Tools,
		Commands: cfg.Commands,
	}
	out.LLM.Provider = cfg.LLM.Provider
	out.LLM.Model = cfg.LLM.Model
	out.LLM.InputTypes = cfg.LLM.InputTypes
	out.Shell.Confirm = cfg.Shell.Confirm
	out.Shell.AllowedCommands = cfg.Shell.AllowedCommands
	out.LitertLM.AutoStart = cfg.LitertLM.AutoStart

	data, err := yaml.Marshal(out)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

func lookupModelInfo(modelName string) *ModelInfo {
	allModels := append([]ModelInfo{}, GeminiModels...)
	allModels = append(allModels, OpenRouterModels...)
	for _, m := range allModels {
		if m.Name == modelName {
			return &m
		}
	}
	return nil
}

func LookupModelInfo(modelName string) *ModelInfo {
	for _, m := range GetAllAvailableModels() {
		if m.Name == modelName {
			return &m
		}
	}
	return nil
}

func SaveModelWithProvider(modelName, provider string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.LLM.Model = modelName
	if provider != "" {
		cfg.LLM.Provider = provider
	} else if modelInList(modelName, GeminiModels) {
		cfg.LLM.Provider = "gemini"
	} else if IsLitertLMModel(modelName) {
		cfg.LLM.Provider = "litertlm"
	} else if modelInList(modelName, OpenRouterModels) {
		cfg.LLM.Provider = "openrouter"
	} else {
		cfg.LLM.Provider = "ollama"
	}

	if info := lookupModelInfo(modelName); info != nil && len(info.InputTypes) > 0 {
		cfg.LLM.InputTypes = info.InputTypes
	} else {
		cfg.LLM.InputTypes = []string{"text"}
	}

	return SaveConfig(cfg)
}

type ModelInfo struct {
	Name       string
	Provider   string
	Size       string
	ModifiedAt string
	InputTypes []string
}

var GeminiModels = []ModelInfo{
	{Name: "gemini-3-flash-preview", Provider: "gemini"},
	{Name: "gemini-3.1-flash-lite-preview", Provider: "gemini"},
	{Name: "gemma-4-31b-it", Provider: "gemini"},
	{Name: "gemma-4-26b-a4b-it", Provider: "gemini"},
}

type openAIModelsResponse struct {
	Object string             `json:"object"`
	Data   []openAIModelEntry `json:"data"`
}

type openAIModelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

func GetLitertLMModels() ([]ModelInfo, error) {
	baseURL := os.Getenv("LITERTLM_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:9379"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	resp, err := http.Get(baseURL + "/v1/models")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch litertlm models: %w", err)
	}
	defer resp.Body.Close()

	var modelsResp openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode litertlm models response: %w", err)
	}

	var result []ModelInfo
	for _, m := range modelsResp.Data {
		result = append(result, ModelInfo{
			Name:       m.ID,
			Provider:   "litertlm",
			InputTypes: []string{"text"},
		})
	}
	return result, nil
}

var OpenRouterModels = []ModelInfo{
	{Name: "nvidia/nemotron-3-super-120b-a12b:free", Provider: "openrouter"},
	{Name: "z-ai/glm-4.5-air:free", Provider: "openrouter"},
	{Name: "minimax/minimax-m2.5:free", Provider: "openrouter"},
}

var getAvailableModelsFunc = GetAvailableModels

func GetAllAvailableModels() []ModelInfo {
	var models []ModelInfo
	if ollamaModels, err := getAvailableModelsFunc(); err == nil {
		models = append(models, ollamaModels...)
	}
	models = append(models, GeminiModels...)
	models = append(models, OpenRouterModels...)
	if litertlmModels, err := GetLitertLMModels(); err == nil {
		models = append(models, litertlmModels...)
	}
	return models
}

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
			Name:     model.Name,
			Provider: "ollama",
		})
	}

	return modelList, nil
}

func SelectModel() error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	models := GetAllAvailableModels()

	if len(models) == 0 {
		fmt.Printf("No models found. Please install models using 'ollama pull <model>'\n")
		return nil
	}

	fmt.Printf("Available Models:\n\n")

	for i, model := range models {
		marker := "  "
		if model.Name == cfg.LLM.Model {
			marker = "* "
		}
		fmt.Printf("[%d] %s%s (%s)\n", i+1, marker, model.Name, model.Provider)
	}

	fmt.Printf("\nEnter number to select model (or press Enter to cancel): ")

	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(input)

	if input == "" {
		fmt.Printf("Selection cancelled.\n")
		return nil
	}

	choice, err := strconv.Atoi(input)
	if err != nil || choice < 1 || choice > len(models) {
		fmt.Printf("Invalid selection.\n")
		return nil
	}

	selectedModel := models[choice-1].Name
	selectedProvider := models[choice-1].Provider
	return SaveModelWithProvider(selectedModel, selectedProvider)
}

func IsAllowedCommand(cmd string, allowedList []string) bool {
	if len(allowedList) == 0 {
		return false
	}
	for _, a := range allowedList {
		a = strings.TrimSpace(a)
		if a == cmd {
			return true
		}
	}
	return false
}

func GetEnvPaths() []string {
	var paths []string

	userConfigDir, err := userConfigDirFunc()
	if err == nil {
		globalEnvPath := filepath.Join(userConfigDir, "ai-shell", ".env")
		paths = append(paths, globalEnvPath)
	}

	paths = append(paths, ".env")

	return paths
}

var getUserConfigDirFunc = os.UserConfigDir

func LoadCommands(cfg *Config) []CommandInfo {
	var fileCmds []CommandInfo
	dirs := loadCommandDirs()
	for _, dir := range dirs {
		cmds, _ := LoadCommandsFromDir(dir)
		fileCmds = append(fileCmds, cmds...)
	}

	configCmds := make(map[string]CommandInfo)
	for name, prompt := range cfg.Commands {
		configCmds[name] = CommandInfo{
			Name:        name,
			Description: prompt,
			Prompt:      prompt,
		}
	}

	for _, cmd := range fileCmds {
		configCmds[cmd.Name] = cmd
	}

	var result []CommandInfo
	for _, cmd := range configCmds {
		result = append(result, cmd)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func LoadCommandsFromDir(dir string) ([]CommandInfo, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create commands directory: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read commands directory: %w", err)
	}

	var commands []CommandInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		cmd, err := parseCommandFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		cmd.Name = strings.TrimSuffix(entry.Name(), ".md")
		commands = append(commands, cmd)
	}

	return commands, nil
}

func loadCommandDirs() []string {
	var dirs []string

	cwd, err := os.Getwd()
	if err == nil {
		dirs = append(dirs, filepath.Join(cwd, ".ai-shell", "commands"))
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		dirs = append(dirs, filepath.Join(homeDir, ".ai-shell", "commands"))
	}

	return dirs
}

func parseCommandFile(path string) (CommandInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CommandInfo{}, err
	}

	content := string(data)
	var cmd CommandInfo

	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			frontmatter := strings.TrimSpace(parts[1])
			var meta struct {
				Description string `yaml:"description"`
			}
			if err := yaml.Unmarshal([]byte(frontmatter), &meta); err == nil {
				cmd.Description = meta.Description
			}
			cmd.Prompt = strings.TrimSpace(parts[2])
			return cmd, nil
		}
	}

	cmd.Prompt = strings.TrimSpace(content)
	return cmd, nil
}

func EnsureCommandsDir() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	localDir := filepath.Join(cwd, ".ai-shell", "commands")
	return os.MkdirAll(localDir, 0755)
}
