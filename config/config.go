package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

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
	Agent      string `mapstructure:"agent"`
	AgentFiles bool   `mapstructure:"agent_files"`
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
		Backend string `mapstructure:"backend"`
	} `mapstructure:"litertlm"`
	Llamacpp struct {
		MMProj string `mapstructure:"mmproj"`
	} `mapstructure:"llamacpp"`
	Tools    map[string]bool   `mapstructure:"tools"`
	Commands map[string]string `mapstructure:"commands"`
}

var configPaths = []string{"."}

var userConfigDirFunc = os.UserConfigDir

var userHomeDirFunc = os.UserHomeDir

var loadEnvFunc = loadEnv

// defaultTools is the default set of enabled tools, used when a config has no
// tools map.
var defaultTools = map[string]bool{
	"RunCommand": true,
	"WriteFile":  true,
	"ReadFile":   true,
	"KVSet":      true,
	"KVGet":      true,
	"KVList":     true,
}

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

func InitLogger(level string) {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slogLevel})
	slog.SetDefault(slog.New(handler))
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
	v.SetDefault("agent", "build")
	v.SetDefault("agent_files", true)
	v.SetDefault("litertlm.backend", "cpu")
	v.SetDefault("tools", defaultTools)

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
				Agent:      "build",
				AgentFiles: true,
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
					Backend string `mapstructure:"backend"`
				}{
					Backend: "cpu",
				},
				Tools: defaultTools,
			}

			if configPath != "" {
				err := os.MkdirAll(configPath, 0755)
				if err == nil {
					defaultConfigFile := filepath.Join(configPath, "config.yaml")
					if _, err := os.Stat(defaultConfigFile); os.IsNotExist(err) {
						content := "log_level: \"info\"\nagent: \"build\"\nagent_files: true\nllm:\n  provider: \"ollama\"\n  model: \"granite4:3b-h\"\n  input_types:\n    - \"text\"\nshell:\n  confirm: true\n  allowed_commands:\n    - \"ls\"\n    - \"pwd\"\n    - \"git\"\nlitertlm:\n  backend: \"cpu\"\ntools:\n  RunCommand: true\n  WriteFile: true\n  ReadFile: true\n  KVSet: true\n  KVGet: true\n  KVList: true\n"
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
		config.Tools = defaultTools
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

// AiShellDir returns ~/.ai-shell, creating it if necessary.
func AiShellDir() (string, error) {
	home, err := userHomeDirFunc()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".ai-shell")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// LibDir returns ~/.ai-shell/lib, the directory holding the in-process
// inference shared libraries (llama.cpp / LiteRT-LM).
func LibDir() (string, error) {
	dir, err := AiShellDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lib"), nil
}

// ModelsDir returns ~/.ai-shell/models/<provider> for the given provider.
func ModelsDir(provider string) (string, error) {
	dir, err := AiShellDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "models", provider), nil
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
	return modelInList(modelName, GetLitertLMModels())
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
		LogLevel   string `yaml:"log_level"`
		Agent      string `yaml:"agent,omitempty"`
		AgentFiles bool   `yaml:"agent_files"`
		LLM        struct {
			Provider   string   `yaml:"provider"`
			Model      string   `yaml:"model"`
			InputTypes []string `yaml:"input_types,omitempty"`
		} `yaml:"llm"`
		Shell struct {
			Confirm         bool     `yaml:"confirm"`
			AllowedCommands []string `yaml:"allowed_commands,omitempty"`
		} `yaml:"shell"`
		LitertLM struct {
			Backend string `yaml:"backend"`
		} `yaml:"litertlm"`
		Llamacpp struct {
			MMProj string `yaml:"mmproj,omitempty"`
		} `yaml:"llamacpp"`
		Tools    map[string]bool   `yaml:"tools,omitempty"`
		Commands map[string]string `yaml:"commands,omitempty"`
	}{
		LogLevel:   cfg.LogLevel,
		Agent:      cfg.Agent,
		AgentFiles: cfg.AgentFiles,
		Tools:      cfg.Tools,
		Commands:   cfg.Commands,
	}
	out.LLM.Provider = cfg.LLM.Provider
	out.LLM.Model = cfg.LLM.Model
	out.LLM.InputTypes = cfg.LLM.InputTypes
	out.Shell.Confirm = cfg.Shell.Confirm
	out.Shell.AllowedCommands = cfg.Shell.AllowedCommands
	out.LitertLM.Backend = cfg.LitertLM.Backend
	out.Llamacpp.MMProj = cfg.Llamacpp.MMProj

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
	allModels = append(allModels, getOpenRouterModelsFunc()...)
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
	} else if modelInList(modelName, getOpenRouterModelsFunc()) {
		cfg.LLM.Provider = "openrouter"
	} else if IsLlamacppModel(modelName) {
		cfg.LLM.Provider = "llamacpp"
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

// SaveMMProj sets the llamacpp vision projector (mmproj) GGUF file in config.
// The name may be a bare filename or an absolute path.
func SaveMMProj(name string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg.Llamacpp.MMProj = name
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
	{Name: "gemini-3.7-flash", Provider: "gemini", InputTypes: []string{"text", "image", "audio"}},
	{Name: "gemini-3.5-flash-lite", Provider: "gemini", InputTypes: []string{"text", "image", "audio"}},
	{Name: "gemma-4-31b-it", Provider: "gemini", InputTypes: []string{"text", "image"}},
	{Name: "gemma-4-26b-a4b-it", Provider: "gemini", InputTypes: []string{"text", "image"}},
}

// GetLitertLMModels lists the native LiteRT-LM models available on disk
// by scanning LITERTLM_MODELS_DIR (default ~/.ai-shell/models/litertlm/)
// for .litertlm files.
func GetLitertLMModels() []ModelInfo {
	dir := os.Getenv("LITERTLM_MODELS_DIR")
	if dir == "" {
		var err error
		dir, err = ModelsDir("litertlm")
		if err != nil {
			return nil
		}
	}
	return scanModels(dir, "litertlm", ".litertlm")
}

// GetLlamacppModels lists the GGUF models on disk (excluding vision projector
// files). Models whose base name matches an available mmproj file get the
// image input type.
func GetLlamacppModels() []ModelInfo {
	dir, err := ModelsDir("llamacpp")
	if err != nil {
		return nil
	}
	all := scanModels(dir, "llamacpp", ".gguf")
	visionKeys := map[string]bool{}
	var models []ModelInfo
	for _, m := range all {
		if strings.Contains(strings.ToLower(m.Name), "mmproj") {
			visionKeys[llamacppVisionKey(m.Name)] = true
		}
	}
	for _, m := range all {
		if strings.Contains(strings.ToLower(m.Name), "mmproj") {
			continue
		}
		if visionKeys[llamacppVisionKey(m.Name)] {
			m.InputTypes = []string{"text", "image"}
		}
		models = append(models, m)
	}
	return models
}

// llamacppQuantRe matches the quantization suffix of a GGUF base name (e.g.
// -Q8_0, -Q4_K_M, -f16) so a model can be matched to its vision projector
// even when their quantizations differ.
var llamacppQuantRe = regexp.MustCompile(`-[Qq][0-9][A-Za-z0-9._]*$|-(?:f16|F16|bf16|BF16)$`)

// llamacppVisionKey normalizes a GGUF base name (mmproj or model) to an
// identity used to pair a model with its vision projector: it strips the
// mmproj marker prefix/suffix and any quantization suffix.
func llamacppVisionKey(name string) string {
	s := strings.TrimPrefix(name, "mmproj-")
	s = strings.TrimSuffix(s, "-mmproj")
	return llamacppQuantRe.ReplaceAllString(s, "")
}

// FindLlamacppMMProj resolves the vision projector (mmproj) GGUF file used for
// image input. It honors, in order: $LLAMACPP_MMPROJ (path or filename in the
// llamacpp models dir), the `llamacpp.mmproj` config field, and finally scans
// the llamacpp models dir for a file whose name contains "mmproj". Returns ""
// when no projector is configured or present.
func FindLlamacppMMProj() (string, error) {
	dir, err := ModelsDir("llamacpp")
	if err != nil {
		return "", err
	}

	cfg, err := LoadConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	name := os.Getenv("LLAMACPP_MMPROJ")
	if name == "" {
		name = cfg.Llamacpp.MMProj
	}
	if name != "" {
		if filepath.IsAbs(name) {
			if _, err := os.Stat(name); err == nil {
				return name, nil
			}
			return "", nil
		}
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		candidate = filepath.Join(dir, name+".gguf")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		return "", nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		lower := strings.ToLower(entry.Name())
		if strings.HasSuffix(lower, ".gguf") && strings.Contains(lower, "mmproj") {
			return filepath.Join(dir, entry.Name()), nil
		}
	}
	return "", nil
}

// scanModels lists the files with the given extension in dir as ModelInfos.
func scanModels(dir, provider, ext string) []ModelInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var models []ModelInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ext) {
			continue
		}
		modelName := strings.TrimSuffix(name, ext)
		info, err := entry.Info()
		size := ""
		if err == nil {
			size = FormatFileSize(info.Size())
		}
		models = append(models, ModelInfo{
			Name:     modelName,
			Provider: provider,
			Size:     size,
		})
	}
	return models
}

func IsLlamacppModel(modelName string) bool {
	return modelInList(modelName, GetLlamacppModels())
}

// DeleteLocalModel removes a locally downloaded model file (llamacpp GGUF or
// litertlm .litertlm) and returns the paths of the removed files. Deleting a
// llamacpp model also deletes the vision projector (mmproj) files paired with
// it (same vision key, e.g. mmproj-<model>-f16.gguf). Remote models
// (ollama/gemini/openrouter) cannot be deleted.
func DeleteLocalModel(modelName string) ([]string, error) {
	if IsLitertLMModel(modelName) {
		dir := os.Getenv("LITERTLM_MODELS_DIR")
		if dir == "" {
			var err error
			dir, err = ModelsDir("litertlm")
			if err != nil {
				return nil, err
			}
		}
		path, err := removeModelFile(filepath.Join(dir, modelName+".litertlm"))
		if err != nil {
			return nil, err
		}
		return []string{path}, nil
	}
	if IsLlamacppModel(modelName) {
		dir, err := ModelsDir("llamacpp")
		if err != nil {
			return nil, err
		}
		path, err := removeModelFile(filepath.Join(dir, modelName+".gguf"))
		if err != nil {
			return nil, err
		}
		removed := []string{path}
		for _, proj := range llamacppMMProjsForModel(dir, modelName) {
			projPath := filepath.Join(dir, proj+".gguf")
			if _, err := removeModelFile(projPath); err != nil {
				if !os.IsNotExist(err) {
					slog.Warn("failed to delete vision projector", "path", projPath, "error", err)
				}
				continue
			}
			removed = append(removed, projPath)
		}
		return removed, nil
	}
	return nil, fmt.Errorf("model %q is not a locally downloaded model", modelName)
}

// llamacppMMProjsForModel lists the vision projector base names in dir whose
// vision key matches the given model's, so deleting the model also removes its
// paired mmproj files.
func llamacppMMProjsForModel(dir, modelName string) []string {
	key := llamacppVisionKey(modelName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var projs []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".gguf") || !strings.Contains(lower, "mmproj") {
			continue
		}
		if llamacppVisionKey(strings.TrimSuffix(name, ".gguf")) == key {
			projs = append(projs, strings.TrimSuffix(name, ".gguf"))
		}
	}
	return projs
}

func removeModelFile(path string) (string, error) {
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("model file not found at %s", path)
		}
		return "", fmt.Errorf("failed to delete model file: %w", err)
	}
	return path, nil
}

// openRouterModelsURL is the OpenRouter API endpoint listing all models.
var openRouterModelsURL = "https://openrouter.ai/api/v1/models"

// openRouterModelsCache caches the fetched list of free OpenRouter models for
// a short TTL to avoid hitting the API on every models listing.
var (
	openRouterModelsCache    []ModelInfo
	openRouterModelsCached   time.Time
	openRouterModelsCacheTTL = 10 * time.Minute
)

// getOpenRouterModelsFunc is swappable for tests.
var getOpenRouterModelsFunc = GetOpenRouterModels

// GetOpenRouterModels returns the list of free OpenRouter models, fetched
// from the OpenRouter API and cached briefly.
func GetOpenRouterModels() []ModelInfo {
	if openRouterModelsCache != nil && time.Since(openRouterModelsCached) < openRouterModelsCacheTTL {
		return openRouterModelsCache
	}
	openRouterModelsCache = fetchOpenRouterFreeModels()
	openRouterModelsCached = time.Now()
	return openRouterModelsCache
}

type openRouterModelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		Pricing struct {
			Prompt     string `json:"prompt"`
			Completion string `json:"completion"`
		} `json:"pricing"`
		Architecture struct {
			InputModalities  []string `json:"input_modalities"`
			OutputModalities []string `json:"output_modalities"`
		} `json:"architecture"`
	} `json:"data"`
}

// fetchOpenRouterFreeModels GETs openRouterModelsURL and keeps only the models
// whose prompt and completion pricing is zero and whose output modalities do
// not include audio (excludes music/sound generation models like Lyria).
func fetchOpenRouterFreeModels() []ModelInfo {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(openRouterModelsURL)
	if err != nil {
		slog.Debug("failed to fetch OpenRouter models", "error", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Debug("openrouter models request failed", "status", resp.StatusCode)
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug("failed to read OpenRouter models response", "error", err)
		return nil
	}
	var payload openRouterModelsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Debug("failed to parse OpenRouter models response", "error", err)
		return nil
	}
	var models []ModelInfo
	for _, m := range payload.Data {
		if !isZeroPrice(m.Pricing.Prompt) || !isZeroPrice(m.Pricing.Completion) {
			continue
		}
		if slices.Contains(m.Architecture.OutputModalities, "audio") {
			continue
		}
		inputTypes := m.Architecture.InputModalities
		if len(inputTypes) == 0 {
			inputTypes = []string{"text"}
		}
		models = append(models, ModelInfo{Name: m.ID, Provider: "openrouter", InputTypes: inputTypes})
	}
	return models
}

func isZeroPrice(s string) bool {
	f, err := strconv.ParseFloat(s, 64)
	return err == nil && f == 0
}

var getAvailableModelsFunc = GetAvailableModels

func GetAllAvailableModels() []ModelInfo {
	var models []ModelInfo
	if ollamaModels, err := getAvailableModelsFunc(); err == nil {
		models = append(models, ollamaModels...)
	}
	models = append(models, GeminiModels...)
	models = append(models, getOpenRouterModelsFunc()...)
	models = append(models, GetLitertLMModels()...)
	models = append(models, GetLlamacppModels()...)
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

// GetCommandName returns the first word of a shell command string.
func GetCommandName(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) > 0 {
		return parts[0]
	}
	return cmd
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

// FormatFileSize renders a byte count as a human-readable size (e.g. "1.5 MB").
func FormatFileSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
