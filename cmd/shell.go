package cmd

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"ai-shell/config"
	"ai-shell/llm"
	"ai-shell/tools"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ollama/ollama/api"
	"google.golang.org/genai"
)

var (
	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true)

	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00BFFF"))

	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	aiStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00BFFF"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700"))

	cmdStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))
)

var availableCommands = []string{
	"help",
	"get-config",
	"models",
	"reset",
	"exit",
	"quit",
}

type ShellExecutorForLLM struct {
	m *ShellModel
}

func (e *ShellExecutorForLLM) ExecuteTool(call llm.ToolCall) (string, error) {
	cmd, ok := call.Arguments["command"].(string)
	if !ok {
		return "Error: Invalid tool arguments", nil
	}

	confirmMsg := fmt.Sprintf("[Executing: %s]", cmd)
	e.m.messages = append(e.m.messages, Message{role: "tool", content: systemStyle.Render(confirmMsg)})

	if e.m.cfg.Shell.Confirm {
		cmdName := getCommandName(cmd)
		skipConfirm := config.IsAllowedCommand(cmdName, e.m.cfg.Shell.AllowedCommands)
		if !skipConfirm {
			confirm := askConfirmation(cmd)
			if !confirm {
				return "Error: Command execution denied by user", nil
			}
		}
	}

	output, err := tools.RunCommand(cmd)
	if err != nil {
		return fmt.Sprintf("Error: %v\nOutput: %s", err, output), nil
	}
	return output, nil
}

func (e *ShellExecutorForLLM) IsAllowedCommand(cmd string) bool {
	cmdName := getCommandName(cmd)
	return config.IsAllowedCommand(cmdName, e.m.cfg.Shell.AllowedCommands)
}

func (e *ShellExecutorForLLM) AskConfirmation(cmd string) bool {
	return askConfirmation(cmd)
}

func getHistoryFile() string {
	usr, err := user.Current()
	if err != nil {
		return ""
	}
	return filepath.Join(usr.HomeDir, ".ai-shell-history")
}

type Message struct {
	role    string
	content string
}

type responseReadyMsg struct{}

type ShellModel struct {
	teaProgram         *tea.Program
	input              textinput.Model
	messages           []Message
	history            []string
	historyIndex       int
	commandHistoryPath string
	width              int
	height             int
	quitting           bool
	cfg                *config.Config
	ollamaClient       *api.Client
	geminiClient       *genai.Client
	suggestions        []string
	selectedIndex      int
	showSuggestions    bool
	loading            bool
	cancelChan         chan struct{}
	modelMenu          struct {
		active      bool
		models      []config.ModelInfo
		selectedIdx int
	}
}

func NewShellModel() (*ShellModel, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	var ollamaClient *api.Client
	var geminiClient *genai.Client

	if cfg.LLM.Provider == "gemini" {
		ctx := context.Background()
		geminiClient, err = llm.NewGeminiClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini client: %w", err)
		}
	} else {
		ollamaClient, err = api.ClientFromEnvironment()
		if err != nil {
			return nil, fmt.Errorf("failed to create Ollama client: %w", err)
		}
	}

	ti := textinput.New()
	ti.Placeholder = "Ask the AI..."
	ti.Focus()
	ti.Prompt = "ai-shell > "

	historyPath := getHistoryFile()

	m := &ShellModel{
		input:              ti,
		messages:           []Message{},
		history:            loadHistory(historyPath),
		historyIndex:       -1,
		commandHistoryPath: historyPath,
		cfg:                cfg,
		ollamaClient:       ollamaClient,
		geminiClient:       geminiClient,
	}

	return m, nil
}

func (m *ShellModel) SetProgram(p *tea.Program) {
	m.teaProgram = p
}

func (m *ShellModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *ShellModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case responseReadyMsg:
		m.loading = false
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlD:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyEnter:
			if m.modelMenu.active {
				m.selectModel()
				return m, nil
			}
			if m.showSuggestions && len(m.suggestions) > 0 {
				m.selectSuggestion()
				return m, nil
			}
			return m.handleSubmit()

		case tea.KeyUp:
			if m.showSuggestions && len(m.suggestions) > 0 && !m.modelMenu.active {
				return m.navigateSuggestions(-1)
			}
			if !m.modelMenu.active {
				return m.navigateHistory(-1)
			}

		case tea.KeyDown:
			if m.showSuggestions && len(m.suggestions) > 0 && !m.modelMenu.active {
				return m.navigateSuggestions(1)
			}
			if !m.modelMenu.active {
				return m.navigateHistory(1)
			}

		case tea.KeyTab:
			return m.handleAutocomplete()

		case tea.KeyEscape:
			if m.loading {
				close(m.cancelChan)
				m.loading = false
				m.messages = append(m.messages, Message{role: "system", content: "Request cancelled."})
				return m, nil
			}
			if m.modelMenu.active {
				m.modelMenu.active = false
				return m, nil
			}
			m.input.SetValue("")
			m.showSuggestions = false
			return m, nil
		}

		if m.modelMenu.active {
			switch msg.String() {
			case "j", "down":
				if m.modelMenu.selectedIdx < len(m.modelMenu.models)-1 {
					m.modelMenu.selectedIdx++
				}
			case "k", "up":
				if m.modelMenu.selectedIdx > 0 {
					m.modelMenu.selectedIdx--
				}
			case "enter":
				m.selectModel()
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	if value := m.input.Value(); strings.HasPrefix(value, "/") {
		m.updateSuggestions(strings.TrimPrefix(value, "/"))
	} else {
		m.showSuggestions = false
	}

	return m, cmd
}

func (m *ShellModel) View() string {
	if m.quitting {
		return fmt.Sprintf("%sGoodbye!%s\n", systemStyle.Render(""), "")
	}

	var sb strings.Builder

	for _, msg := range m.messages {
		switch msg.role {
		case "system":
			sb.WriteString(systemStyle.Render(msg.content))
			sb.WriteString("\n")
		case "user":
			sb.WriteString(userStyle.Render("You: " + msg.content))
			sb.WriteString("\n")
		case "assistant":
			sb.WriteString(aiStyle.Render("AI: "))
			sb.WriteString(msg.content)
			sb.WriteString("\n")
		case "tool":
			sb.WriteString(cmdStyle.Render(msg.content))
			sb.WriteString("\n")
		}
	}

	if m.modelMenu.active {
		sb.WriteString(systemStyle.Render("Select Model (↑/↓ to navigate, Enter to select, Esc to cancel):\n"))
		for i, model := range m.modelMenu.models {
			marker := "  "
			if model.Name == m.cfg.LLM.Model {
				marker = "* "
			}
			if i == m.modelMenu.selectedIdx {
				sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#444444")).Render(fmt.Sprintf(" %s%s ", marker, model.Name)))
			} else {
				sb.WriteString(userStyle.Render(fmt.Sprintf(" %s%s ", marker, model.Name)))
			}
			sb.WriteString("\n")
		}
	} else if m.showSuggestions && len(m.suggestions) > 0 {
		sb.WriteString(dimStyle.Render("Suggestions: "))
		for i, suggestion := range m.suggestions {
			if i == m.selectedIndex {
				sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#444444")).Render(" " + suggestion + " "))
			} else {
				sb.WriteString(helpStyle.Render(" " + suggestion + " "))
			}
		}
		sb.WriteString("\n")
	}

	if m.loading {
		sb.WriteString(systemStyle.Render("Thinking... (Press Esc to cancel)\n"))
	} else {
		sb.WriteString(m.input.View())
	}
	sb.WriteString("\n")

	return sb.String()
}

func (m *ShellModel) handleSubmit() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
		return m, nil
	}

	if m.loading {
		return m, nil
	}

	if m.history == nil || len(m.history) == 0 || m.history[len(m.history)-1] != value {
		m.history = append(m.history, value)
		saveHistory(m.commandHistoryPath, m.history)
	}
	m.historyIndex = -1

	m.input.SetValue("")

	if strings.HasPrefix(value, "/") {
		return m.handleCommand(strings.TrimPrefix(value, "/"))
	}

	switch value {
	case "exit", "quit":
		m.quitting = true
		return m, tea.Quit

	case "get-config":
		m.showConfig()
		return m, nil

	case "help":
		m.showHelp()
		return m, nil

	case "models":
		m.openModelMenu()
		return m, nil

	case "reset":
		m.messages = nil
		return m, nil
	}

	m.messages = append(m.messages, Message{role: "user", content: value})

	m.loading = true
	m.cancelChan = make(chan struct{})

	go m.callOllama(value)

	return m, nil
}

func (m *ShellModel) handleCommand(cmd string) (tea.Model, tea.Cmd) {
	cmd = strings.TrimSpace(cmd)

	switch cmd {
	case "exit", "quit":
		m.quitting = true
		return m, tea.Quit

	case "get-config":
		m.showConfig()

	case "help":
		m.showHelp()

	case "models":
		m.openModelMenu()

	case "reset":
		m.messages = nil

	default:
		m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Unknown command: /%s", cmd)})
	}

	return m, nil
}

func (m *ShellModel) handleAutocomplete() (tea.Model, tea.Cmd) {
	value := m.input.Value()

	if strings.HasPrefix(value, "/") {
		partial := strings.TrimPrefix(value, "/")
		for _, cmd := range availableCommands {
			if strings.HasPrefix(cmd, partial) {
				m.input.SetValue("/" + cmd)
				break
			}
		}
		return m, nil
	}

	lastAt := strings.LastIndex(value, "@")
	if lastAt != -1 {
		partial := value[lastAt+1:]
		dir := "."
		base := partial

		if strings.Contains(partial, "/") {
			dir = filepath.Dir(partial)
			base = filepath.Base(partial)
			if dir == "." {
				dir = "."
			}
		}

		matches := m.completeFiles(dir, base)
		if len(matches) > 0 {
			completed := matches[0]
			newValue := value[:lastAt+1] + completed
			m.input.SetValue(newValue)
		}
	}

	return m, nil
}

func (m *ShellModel) completeFiles(dir, prefix string) []string {
	var results []string

	absDir := dir
	if !filepath.IsAbs(dir) {
		cwd, _ := os.Getwd()
		absDir = filepath.Join(cwd, dir)
	}
	absDir = filepath.Clean(absDir)

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return results
	}

	prefixBase := filepath.Base(prefix)
	prefixDir := filepath.Dir(prefix)
	if prefixDir == "." {
		prefixDir = ""
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefixBase) {
			fullPath := filepath.Join(dir, name)
			if entry.IsDir() {
				results = append(results, fullPath+"/")
			} else {
				results = append(results, fullPath)
			}
		}
	}

	return results
}

func (m *ShellModel) updateSuggestions(filter string) {
	var matches []string
	for _, cmd := range availableCommands {
		if strings.HasPrefix(cmd, filter) {
			matches = append(matches, "/"+cmd)
		}
	}

	if len(matches) > 0 {
		m.suggestions = matches
		m.showSuggestions = true
		m.selectedIndex = 0
	} else {
		m.showSuggestions = false
	}
}

func (m *ShellModel) navigateSuggestions(dir int) (tea.Model, tea.Cmd) {
	newIndex := m.selectedIndex + dir
	if newIndex < 0 {
		newIndex = len(m.suggestions) - 1
	} else if newIndex >= len(m.suggestions) {
		newIndex = 0
	}
	m.selectedIndex = newIndex
	return m, nil
}

func (m *ShellModel) selectSuggestion() (tea.Model, tea.Cmd) {
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.suggestions) {
		m.input.SetValue(m.suggestions[m.selectedIndex])
		m.showSuggestions = false
		return m.handleSubmit()
	}
	return m, nil
}

func (m *ShellModel) navigateHistory(dir int) (tea.Model, tea.Cmd) {
	if len(m.history) == 0 {
		return m, nil
	}

	newIndex := m.historyIndex + dir

	if newIndex < -1 {
		newIndex = -1
	} else if newIndex >= len(m.history) {
		return m, nil
	}

	m.historyIndex = newIndex

	if m.historyIndex == -1 {
		m.input.SetValue("")
	} else {
		m.input.SetValue(m.history[len(m.history)-1-m.historyIndex])
	}

	return m, nil
}

func (m *ShellModel) showHelp() {
	help := `
Commands:
  /help         - Show this help message
  /get-config   - Show current LLM settings
  /models       - Switch to a different model
  /reset        - Clear the screen and messages
  /exit, /quit  - Exit the shell
  /<command>    - Execute a shell command
  @<file>       - Autocomplete file paths
  <text>        - Send text to the AI for a response
`
	m.messages = append(m.messages, Message{role: "system", content: help})
}

func (m *ShellModel) showConfig() {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Provider: %s\n", m.cfg.LLM.Provider))
	sb.WriteString(fmt.Sprintf("Model: %s\n", m.cfg.LLM.Model))
	sb.WriteString(fmt.Sprintf("Confirm Commands: %v\n", m.cfg.Shell.Confirm))
	sb.WriteString(fmt.Sprintf("Allowed Commands: %s\n", m.cfg.Shell.AllowedCommands))
	m.messages = append(m.messages, Message{role: "system", content: sb.String()})
}

func (m *ShellModel) openModelMenu() {
	models, err := config.GetAvailableModels()
	if err != nil {
		m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Error loading models: %v", err)})
		return
	}

	if len(models) == 0 {
		m.messages = append(m.messages, Message{role: "system", content: "No models found. Please install models using 'ollama pull <model>'"})
		return
	}

	m.modelMenu.models = models
	m.modelMenu.active = true
	m.modelMenu.selectedIdx = 0
	m.input.SetValue("")
}

func (m *ShellModel) selectModel() {
	if m.modelMenu.selectedIdx < 0 || m.modelMenu.selectedIdx >= len(m.modelMenu.models) {
		return
	}

	selectedModel := m.modelMenu.models[m.modelMenu.selectedIdx].Name
	if err := config.SaveModel(selectedModel); err != nil {
		m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Error saving model: %v", err)})
	} else {
		m.messages = append(m.messages, Message{role: "system", content: fmt.Sprintf("Switched to model: %s", selectedModel)})
		if newCfg, err := config.LoadConfig(); err == nil {
			m.cfg = newCfg
		}
	}

	m.modelMenu.active = false
}

func (m *ShellModel) callOllama(prompt string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-m.cancelChan:
			cancel()
		}
	}()

	distro := tools.GetDistro()
	shell := tools.GetShell()
	systemPrompt := fmt.Sprintf("You are a helpful shell assistant. The user is running on %s using %s shell.", distro, shell)

	m.messages = append(m.messages, Message{role: "system", content: "Thinking..."})
	if m.teaProgram != nil {
		m.teaProgram.Send(responseReadyMsg{})
	}

	executor := &ShellExecutorForLLM{m: m}

	var resultMessages []llm.Message
	var err error

	if m.cfg.LLM.Provider == "gemini" {
		geminiCaller := llm.NewGeminiCaller(m.geminiClient, m.cfg.LLM.Model, executor)
		var geminiMessages []llm.Message
		for _, msg := range m.messages {
			if msg.role == "user" || msg.role == "assistant" || msg.role == "tool" {
				geminiMessages = append(geminiMessages, llm.Message{Role: msg.role, Content: msg.content})
			}
		}
		geminiMessages = append(geminiMessages, llm.Message{Role: "user", Content: prompt})
		resultMessages, err = geminiCaller.Call(ctx, systemPrompt, geminiMessages)
	} else {
		ollamaCaller := llm.NewOllamaCaller(m.ollamaClient, m.cfg.LLM.Model, executor)
		var apiMessages []api.Message
		for _, msg := range m.messages {
			if msg.role == "user" || msg.role == "assistant" || msg.role == "tool" {
				apiMessages = append(apiMessages, api.Message{Role: msg.role, Content: msg.content})
			}
		}
		apiMessages = append(apiMessages, api.Message{Role: "user", Content: prompt})
		var ollamaMsgs []api.Message
		ollamaMsgs, err = ollamaCaller.Call(ctx, systemPrompt, apiMessages)
		if err == nil {
			for _, msg := range ollamaMsgs {
				resultMessages = append(resultMessages, llm.Message{Role: msg.Role, Content: msg.Content})
			}
		}
	}

	if err != nil {
		m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Error: %v", err)})
		m.loading = false
		return
	}

	for _, msg := range resultMessages {
		switch msg.Role {
		case "user":
			m.messages = append(m.messages, Message{role: "user", content: msg.Content})
		case "assistant":
			m.messages = append(m.messages, Message{role: "assistant", content: msg.Content})
		case "tool":
			m.messages = append(m.messages, Message{role: "tool", content: cmdStyle.Render(msg.Content)})
		}
	}

	m.loading = false
}

func askConfirmation(cmd string) bool {
	fmt.Printf("\n%s[LLM wants to execute: %s]%s\n", systemStyle.Render(""), cmd, "")
	fmt.Printf("%sConfirm execution? [y/N]: %s", dimStyle.Render(""), "")

	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))

	return response == "y" || response == "yes"
}

func getCommandName(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) > 0 {
		return parts[0]
	}
	return cmd
}

func loadHistory(path string) []string {
	if path == "" {
		return []string{}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return []string{}
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return result
}

func saveHistory(path string, history []string) {
	if path == "" || len(history) == 0 {
		return
	}

	content := strings.Join(history, "\n")
	os.WriteFile(path, []byte(content), 0644)
}

func RunShell() error {
	m, err := NewShellModel()
	if err != nil {
		return fmt.Errorf("failed to create shell model: %w", err)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	m.SetProgram(p)

	_, err = p.Run()
	return err
}
