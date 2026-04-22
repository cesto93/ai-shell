package cmd

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"

	"ai-shell/config"
	"ai-shell/llm"
	"ai-shell/tools"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	"add-cmd",
	"exit",
	"quit",
}

type ShellExecutorForLLM struct {
	m *ShellModel
}

func (e *ShellExecutorForLLM) ExecuteTool(call llm.ToolCall) (string, error) {
	switch call.Name {
	case "RunCommand":
		cmd, ok := call.Arguments["command"].(string)
		if !ok {
			return "Error: Invalid tool arguments", nil
		}

		if e.m.cfg.Shell.Confirm {
			cmdName := getCommandName(cmd)
			skipConfirm := config.IsAllowedCommand(cmdName, e.m.cfg.Shell.AllowedCommands)
			if !skipConfirm {
				confirm := e.AskConfirmation(cmd)
				if !confirm {
					return "Error: Command execution denied by user", nil
				}
			}
		}

		confirmMsg := fmt.Sprintf("[Executing: %s]", cmd)
		e.m.messages = append(e.m.messages, Message{role: "tool", content: systemStyle.Render(confirmMsg)})

		output, err := tools.RunCommand(cmd)
		if err != nil {
			return fmt.Sprintf("Error: %v\nOutput: %s", err, output), nil
		}
		return output, nil

	case "WriteFile":
		path, ok1 := call.Arguments["path"].(string)
		content, ok2 := call.Arguments["content"].(string)
		if !ok1 || !ok2 {
			return "Error: Invalid tool arguments", nil
		}

		// Handle @ prefix from autocomplete
		if strings.HasPrefix(path, "@") {
			path = strings.TrimPrefix(path, "@")
		}

		if e.m.cfg.Shell.Confirm {
			confirm := e.AskConfirmation(fmt.Sprintf("Write to file %s?", path))
			if !confirm {
				return "Error: File write denied by user", nil
			}
		}

		confirmMsg := fmt.Sprintf("[Writing to file: %s]", path)
		e.m.messages = append(e.m.messages, Message{role: "tool", content: systemStyle.Render(confirmMsg)})

		return tools.WriteFile(path, content)

	case "ReadFile":
		path, ok := call.Arguments["path"].(string)
		if !ok {
			return "Error: Invalid tool arguments", nil
		}

		// Handle @ prefix from autocomplete
		if strings.HasPrefix(path, "@") {
			path = strings.TrimPrefix(path, "@")
		}

		confirmMsg := fmt.Sprintf("[Reading file: %s]", path)
		e.m.messages = append(e.m.messages, Message{role: "tool", content: systemStyle.Render(confirmMsg)})

		return tools.ReadFile(path)

	default:
		return fmt.Sprintf("Error: Unknown tool %s", call.Name), nil
	}
}

func (e *ShellExecutorForLLM) IsAllowedCommand(cmd string) bool {
	cmdName := getCommandName(cmd)
	return config.IsAllowedCommand(cmdName, e.m.cfg.Shell.AllowedCommands)
}

func (e *ShellExecutorForLLM) AskConfirmation(cmd string) bool {
	e.m.pendingCommand = cmd
	e.m.waitingConfirm = true
	if e.m.teaProgram != nil {
		e.m.teaProgram.Send(confirmationMsg{cmd: cmd})
	}

	select {
	case result := <-e.m.confirmationChan:
		return result
	case <-e.m.cancelChan:
		e.m.waitingConfirm = false
		return false
	}
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
type confirmationMsg struct {
	cmd string
}

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
	suggestions        []string
	selectedIndex      int
	showSuggestions    bool
	loading            bool
	cancelChan         chan struct{}
	confirmationChan   chan bool
	pendingCommand     string
	waitingConfirm     bool
	modelMenu          struct {
		active      bool
		models      []config.ModelInfo
		selectedIdx int
	}
	addCmdMode struct {
		active bool
		step   int // 0: name, 1: prompt
		name   string
		prompt string
	}
}

func NewShellModel() (*ShellModel, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
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
		confirmationChan:   make(chan bool, 1),
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

	case confirmationMsg:
		m.waitingConfirm = true
		m.pendingCommand = msg.cmd
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.waitingConfirm {
			switch msg.Type {
			case tea.KeyEnter:
				// Default to No for safety
				m.waitingConfirm = false
				m.confirmationChan <- false
				return m, nil
			case tea.KeyEscape:
				m.waitingConfirm = false
				m.confirmationChan <- false
				return m, nil
			}

			switch msg.String() {
			case "y", "Y":
				m.waitingConfirm = false
				m.confirmationChan <- true
			case "n", "N":
				m.waitingConfirm = false
				m.confirmationChan <- false
			}
			return m, nil
		}
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
				return m.selectSuggestion()
			}
			return m.handleSubmit()

		case tea.KeyUp:
			if m.showSuggestions && len(m.suggestions) > 0 && !m.modelMenu.active && !m.addCmdMode.active {
				return m.navigateSuggestions(-1)
			}
			if !m.modelMenu.active && !m.addCmdMode.active {
				return m.navigateHistory(-1)
			}

		case tea.KeyDown:
			if m.showSuggestions && len(m.suggestions) > 0 && !m.modelMenu.active && !m.addCmdMode.active {
				return m.navigateSuggestions(1)
			}
			if !m.modelMenu.active && !m.addCmdMode.active {
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
			if m.addCmdMode.active {
				m.addCmdMode.active = false
				m.input.Prompt = "ai-shell > "
				m.input.SetValue("")
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

	if !m.addCmdMode.active {
		m.updateSuggestions()
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
		case "error":
			sb.WriteString(errorStyle.Render("Error: " + msg.content))
			sb.WriteString("\n")
		}
	}

	if m.modelMenu.active {
		sb.WriteString(systemStyle.Render("Select Model (↑/↓ to navigate, Enter to select, Esc to cancel):"))
		sb.WriteString("\n")
		for i, model := range m.modelMenu.models {
			marker := " "
			if model.Name == m.cfg.LLM.Model {
				marker = "*"
			}
			if i == m.modelMenu.selectedIdx {
				sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#444444")).Render(fmt.Sprintf(" %s %s ", marker, model.Name)))
			} else {
				sb.WriteString(userStyle.Render(fmt.Sprintf(" %s %s ", marker, model.Name)))
			}
			sb.WriteString("\n")
		}
	} else if m.showSuggestions && len(m.suggestions) > 0 {
		sb.WriteString(dimStyle.Render("Suggestions: "))
		for i, suggestion := range m.suggestions {
			display := suggestion
			if lastAt := strings.LastIndex(suggestion, "@"); lastAt != -1 {
				display = suggestion[lastAt:]
			}

			if i == m.selectedIndex {
				sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#444444")).Render(" " + display + " "))
			} else {
				sb.WriteString(helpStyle.Render(" " + display + " "))
			}
		}
		sb.WriteString("\n")
	}

	if m.addCmdMode.active {
		sb.WriteString(dimStyle.Render("(Esc to cancel adding command)"))
		sb.WriteString("\n")
	}

	if m.waitingConfirm {
		sb.WriteString(systemStyle.Render(fmt.Sprintf("[LLM wants to execute: %s]", m.pendingCommand)))
		sb.WriteString("\n")
		sb.WriteString(dimStyle.Render("Confirm execution? [y/N]"))
		sb.WriteString("\n")
	} else if m.loading {
		sb.WriteString(systemStyle.Render("Thinking... (Press Esc to cancel)"))
		sb.WriteString("\n")
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

	// Clean up @ prefix from autocomplete
	if strings.Contains(value, "@") {
		parts := strings.Split(value, " ")
		for i, part := range parts {
			if strings.HasPrefix(part, "@") {
				parts[i] = strings.TrimPrefix(part, "@")
			}
		}
		value = strings.Join(parts, " ")
	}

	if m.loading {
		return m, nil
	}

	if m.addCmdMode.active {
		if m.addCmdMode.step == 0 {
			m.addCmdMode.name = strings.TrimPrefix(value, "/")
			m.addCmdMode.step = 1
			m.input.SetValue("")
			m.input.Prompt = fmt.Sprintf("Enter prompt for /%s: ", m.addCmdMode.name)
			return m, nil
		} else {
			m.addCmdMode.prompt = value
			if err := config.SaveCommand(m.addCmdMode.name, m.addCmdMode.prompt); err != nil {
				m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Error saving command: %v", err)})
			} else {
				m.messages = append(m.messages, Message{role: "system", content: fmt.Sprintf("Command /%s added successfully!", m.addCmdMode.name)})
				if newCfg, err := config.LoadConfig(); err == nil {
					m.cfg = newCfg
				}
			}
			m.addCmdMode.active = false
			m.input.Prompt = "ai-shell > "
			m.input.SetValue("")
			return m, nil
		}
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

	go m.callLLM(value)

	return m, nil
}

func (m *ShellModel) handleCommand(input string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}
	cmd := parts[0]
	args := strings.Join(parts[1:], " ")

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

	case "add-cmd":
		m.addCmdMode.active = true
		m.addCmdMode.step = 0
		m.input.SetValue("")
		m.input.Prompt = "Enter command name: /"

	case "reset":
		m.messages = nil

	default:
		if prompt, ok := m.cfg.Commands[cmd]; ok {
			fullPrompt := prompt
			if args != "" {
				fullPrompt = prompt + " " + args
			}
			m.messages = append(m.messages, Message{role: "user", content: fullPrompt})
			m.loading = true
			m.cancelChan = make(chan struct{})
			go m.callLLM(fullPrompt)
			return m, nil
		}
		m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Unknown command: /%s", cmd)})
	}

	return m, nil
}

func (m *ShellModel) handleAutocomplete() (tea.Model, tea.Cmd) {
	value := m.input.Value()

	if strings.HasPrefix(value, "/") && !strings.Contains(value, " ") {
		partial := strings.TrimPrefix(value, "/")
		// Check built-in commands
		for _, cmd := range availableCommands {
			if strings.HasPrefix(cmd, partial) {
				m.input.SetValue("/" + cmd)
				return m, nil
			}
		}
		// Check custom commands (sorted for consistency)
		var customCmds []string
		for cmd := range m.cfg.Commands {
			customCmds = append(customCmds, cmd)
		}
		sort.Strings(customCmds)

		for _, cmd := range customCmds {
			if strings.HasPrefix(cmd, partial) {
				m.input.SetValue("/" + cmd)
				return m, nil
			}
		}
		return m, nil
	}

	lastAt := strings.LastIndex(value, "@")
	if lastAt != -1 {
		partial := value[lastAt+1:]
		dir, base := filepath.Split(partial)

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

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefix) {
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

func (m *ShellModel) updateSuggestions() {
	value := m.input.Value()
	var matches []string

	// Command suggestions: only if starts with / and no space yet
	if strings.HasPrefix(value, "/") && !strings.Contains(value, " ") {
		filter := strings.TrimPrefix(value, "/")
		for _, cmd := range availableCommands {
			if strings.HasPrefix(cmd, filter) {
				matches = append(matches, "/"+cmd)
			}
		}
		var customCmds []string
		for cmd := range m.cfg.Commands {
			customCmds = append(customCmds, cmd)
		}
		sort.Strings(customCmds)
		for _, cmd := range customCmds {
			if strings.HasPrefix(cmd, filter) {
				exists := false
				for _, m := range matches {
					if m == "/"+cmd {
						exists = true
						break
					}
				}
				if !exists {
					matches = append(matches, "/"+cmd)
				}
			}
		}
	}

	// File suggestions: if @ is present
	if lastAt := strings.LastIndex(value, "@"); lastAt != -1 {
		partial := value[lastAt+1:]
		dir, base := filepath.Split(partial)

		fileMatches := m.completeFiles(dir, base)
		for _, fm := range fileMatches {
			matches = append(matches, value[:lastAt+1]+fm)
		}
	}

	if len(matches) > 0 {
		m.suggestions = matches
		m.showSuggestions = true
		if m.selectedIndex >= len(m.suggestions) {
			m.selectedIndex = 0
		}
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
		suggestion := m.suggestions[m.selectedIndex]
		m.input.SetValue(suggestion)
		m.showSuggestions = false

		if strings.HasPrefix(suggestion, "/") && !strings.Contains(suggestion, "@") {
			return m.handleSubmit()
		}
		return m, nil
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
	var sb strings.Builder
	sb.WriteString("\nCommands:\n")
	sb.WriteString("  /help         - Show this help message\n")
	sb.WriteString("  /get-config   - Show current LLM settings\n")
	sb.WriteString("  /models       - Switch to a different model\n")
	sb.WriteString("  /add-cmd      - Configure a new command\n")
	sb.WriteString("  /reset        - Clear the screen and messages\n")
	sb.WriteString("  /exit, /quit  - Exit the shell\n")
	sb.WriteString("  /<command>    - Execute a shell command\n")
	sb.WriteString("  @<file>       - Autocomplete file paths\n")
	sb.WriteString("  <text>        - Send text to the AI for a response\n")

	if len(m.cfg.Commands) > 0 {
		sb.WriteString("\nUser Commands:\n")
		for cmd := range m.cfg.Commands {
			sb.WriteString(fmt.Sprintf("  /%-12s - %s\n", cmd, m.cfg.Commands[cmd]))
		}
	}

	m.messages = append(m.messages, Message{role: "system", content: sb.String()})
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

	models = append(models, config.GeminiModels...)
	models = append(models, config.MistralModels...)

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
	provider := "ollama"
	if config.IsGeminiModel(selectedModel) {
		provider = "gemini"
	} else if config.IsMistralModel(selectedModel) {
		provider = "mistral"
	}
	if err := config.SaveModelWithProvider(selectedModel, provider); err != nil {
		m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Error saving model: %v", err)})
	} else {
		m.messages = append(m.messages, Message{role: "system", content: fmt.Sprintf("Switched to model: %s", selectedModel)})
		if newCfg, err := config.LoadConfig(); err == nil {
			m.cfg = newCfg
		}
	}

	m.modelMenu.active = false
}

func (m *ShellModel) callLLM(prompt string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-m.cancelChan:
			cancel()
		}
	}()

	agent := llm.NewAgent(m.cfg.LLM.Model, m.cfg.LLM.Provider)

	executor := &ShellExecutorForLLM{m: m}

	var caller llm.Caller
	switch agent.Provider {
	case "gemini":
		caller = llm.NewGeminiCaller(agent.Model, executor)
	case "mistral":
		caller = llm.NewMistralCaller(agent.Model, executor)
	default:
		caller = llm.NewOllamaCaller(agent.Model, executor)
	}

	var commonMessages []llm.Message
	for _, msg := range m.messages {
		if msg.role == "user" || msg.role == "assistant" || msg.role == "tool" {
			commonMessages = append(commonMessages, llm.Message{Role: msg.role, Content: msg.content})
		}
	}

	resultMessages, err := caller.Call(ctx, agent.Prompt, commonMessages)

	if err != nil {
		m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Error: %v", err)})
		m.loading = false
		if m.teaProgram != nil {
			m.teaProgram.Send(responseReadyMsg{})
		}
		return
	}

	for _, msg := range resultMessages {
		switch msg.Role {
		case "user":
			m.messages = append(m.messages, Message{role: "user", content: msg.Content})
		case "assistant":
			m.messages = append(m.messages, Message{role: "assistant", content: msg.Content})
		case "tool":
			m.messages = append(m.messages, Message{role: "tool", content: msg.Content})
		}
	}

	m.loading = false
	if m.teaProgram != nil {
		m.teaProgram.Send(responseReadyMsg{})
	}
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
