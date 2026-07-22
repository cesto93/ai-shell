package cmd

import (
	"context"
	"encoding/base64"
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
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
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

	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#444444"))
)

var availableCommands = []string{
	"help",
	"get-config",
	"config",
	"models",
	"reset",
	"exit",
	"quit",
}

type menuKind int

const (
	menuNone menuKind = iota
	menuModel
	menuConfig
	menuTools
	menuCommands
)

type menuState struct {
	kind        menuKind
	selectedIdx int
	options     []string
	models      []config.ModelInfo
}

func (ms *menuState) open(kind menuKind) {
	ms.kind = kind
	ms.selectedIdx = 0
}

func (ms *menuState) close() {
	ms.kind = menuNone
	ms.options = nil
	ms.models = nil
}

func (ms *menuState) itemCount() int {
	switch ms.kind {
	case menuModel:
		return len(ms.models)
	case menuConfig, menuTools, menuCommands:
		return len(ms.options)
	}
	return 0
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
		e.m.messages = append(e.m.messages, Message{role: "assistant", content: systemStyle.Render(confirmMsg)})

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

		path = strings.TrimPrefix(path, "@")

		if e.m.cfg.Shell.Confirm {
			confirm := e.AskConfirmation(fmt.Sprintf("Write to file %s?", path))
			if !confirm {
				return "Error: File write denied by user", nil
			}
		}

		confirmMsg := fmt.Sprintf("[Writing to file: %s]", path)
		e.m.messages = append(e.m.messages, Message{role: "assistant", content: systemStyle.Render(confirmMsg)})

		return tools.WriteFile(path, content)

	case "ReadFile":
		path, ok := call.Arguments["path"].(string)
		if !ok {
			return "Error: Invalid tool arguments", nil
		}

		path = strings.TrimPrefix(path, "@")

		confirmMsg := fmt.Sprintf("[Reading file: %s]", path)
		e.m.messages = append(e.m.messages, Message{role: "assistant", content: systemStyle.Render(confirmMsg)})

		return tools.ReadFile(path)

	case "KVSet":
		key, ok1 := call.Arguments["key"].(string)
		value, ok2 := call.Arguments["value"].(string)
		if !ok1 || !ok2 {
			return "Error: Invalid tool arguments", nil
		}

		confirmMsg := fmt.Sprintf("[KV Store: Saving %s]", key)
		e.m.messages = append(e.m.messages, Message{role: "assistant", content: systemStyle.Render(confirmMsg)})

		return tools.KVSet(key, value)

	case "KVGet":
		key, ok := call.Arguments["key"].(string)
		if !ok {
			return "Error: Invalid tool arguments", nil
		}

		confirmMsg := fmt.Sprintf("[KV Store: Retrieving %s]", key)
		e.m.messages = append(e.m.messages, Message{role: "assistant", content: systemStyle.Render(confirmMsg)})

		return tools.KVGet(key)

	case "KVList":
		confirmMsg := "[KV Store: Listing keys]"
		e.m.messages = append(e.m.messages, Message{role: "assistant", content: systemStyle.Render(confirmMsg)})

		return tools.KVList()

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
	role       string
	content    string
	images     []string // Base64 encoded images or paths
	toolCallID string
	toolCalls  []llm.OpenAIToolCall
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
	menu               menuState
	commands           []config.CommandInfo
	litertlmService    *LitertLMService
	allowedCmdMode     struct {
		active bool
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
		commands:           config.LoadCommands(cfg),
		confirmationChan:   make(chan bool, 1),
	}

	if cfg.LitertLM.AutoStart {
		port := extractPort(os.Getenv("LITERTLM_BASE_URL"))
		if IsPortAvailable(port) {
			svc := NewLitertLMService(port)
			if err := svc.Start(); err == nil {
				m.litertlmService = svc
			}
		}
	} else if cfg.LLM.Provider == "litertlm" {
		port := extractPort(os.Getenv("LITERTLM_BASE_URL"))
		svc := NewLitertLMService(port)
		if err := svc.Start(); err != nil {
			return nil, fmt.Errorf("failed to start litertlm service: %w", err)
		}
		m.litertlmService = svc
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
			return m.handleConfirmationKeys(msg)
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlD:
			if m.litertlmService != nil {
				m.litertlmService.Stop()
			}
			m.quitting = true
			return m, tea.Quit

		case tea.KeyEnter:
			return m.handleEnterKey()

		case tea.KeyUp:
			if m.menu.kind != menuNone {
				return m.handleMenuNav(-1)
			}
			if m.showSuggestions && len(m.suggestions) > 0 {
				return m.navigateSuggestions(-1)
			}
			return m.navigateHistory(-1)

		case tea.KeyDown:
			if m.menu.kind != menuNone {
				return m.handleMenuNav(1)
			}
			if m.showSuggestions && len(m.suggestions) > 0 {
				return m.navigateSuggestions(1)
			}
			return m.navigateHistory(1)

		case tea.KeyTab:
			return m.handleAutocomplete()

		case tea.KeyEscape:
			return m.handleEscapeKey()
		}

		if m.menu.kind != menuNone {
			return m.handleMenuKeys(msg.String())
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	if !m.allowedCmdMode.active && m.menu.kind == menuNone {
		m.updateSuggestions()
	} else {
		m.showSuggestions = false
	}

	return m, cmd
}

func (m *ShellModel) handleConfirmationKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter, tea.KeyEscape:
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

func (m *ShellModel) handleEnterKey() (tea.Model, tea.Cmd) {
	switch m.menu.kind {
	case menuModel:
		m.selectModel()
		return m, nil
	case menuConfig:
		m.selectConfigOption()
		return m, nil
	case menuTools:
		m.selectToolOption()
		return m, nil
	case menuCommands:
		m.selectCommandOption()
		return m, nil
	}
	if m.showSuggestions && len(m.suggestions) > 0 {
		return m.selectSuggestion()
	}
	return m.handleSubmit()
}

func (m *ShellModel) handleEscapeKey() (tea.Model, tea.Cmd) {
	if m.loading {
		close(m.cancelChan)
		m.loading = false
		m.messages = append(m.messages, Message{role: "system", content: "Request cancelled."})
		return m, nil
	}
	if m.menu.kind != menuNone {
		m.menu.close()
		return m, nil
	}
	if m.allowedCmdMode.active {
		m.allowedCmdMode.active = false
		m.input.Prompt = "ai-shell > "
		m.input.SetValue("")
		return m, nil
	}
	m.input.SetValue("")
	m.showSuggestions = false
	return m, nil
}

func (m *ShellModel) handleMenuNav(dir int) (tea.Model, tea.Cmd) {
	n := m.menu.itemCount()
	if dir < 0 && m.menu.selectedIdx > 0 {
		m.menu.selectedIdx--
	} else if dir > 0 && m.menu.selectedIdx < n-1 {
		m.menu.selectedIdx++
	}
	return m, nil
}

func (m *ShellModel) handleMenuKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		return m.handleMenuNav(1)
	case "k", "up":
		return m.handleMenuNav(-1)
	}
	return m, nil
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

	m.renderMenu(&sb)

	if m.showSuggestions && len(m.suggestions) > 0 {
		sb.WriteString(dimStyle.Render("Suggestions: "))
		for i, suggestion := range m.suggestions {
			display := suggestion
			if lastAt := strings.LastIndex(suggestion, "@"); lastAt != -1 {
				display = suggestion[lastAt:]
			}

			if i == m.selectedIndex {
				sb.WriteString(highlightStyle.Render(" " + display + " "))
			} else {
				sb.WriteString(helpStyle.Render(" " + display + " "))
			}
		}
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

func (m *ShellModel) renderMenu(sb *strings.Builder) {
	switch m.menu.kind {
	case menuModel:
		sb.WriteString(systemStyle.Render("Select Model (↑/↓ to navigate, Enter to select, Esc to cancel):"))
		sb.WriteString("\n")
		for i, model := range m.menu.models {
			marker := " "
			if model.Name == m.cfg.LLM.Model {
				marker = "*"
			}
			label := fmt.Sprintf(" %s %s (%s) ", marker, model.Name, model.Provider)
			if i == m.menu.selectedIdx {
				sb.WriteString(highlightStyle.Render(label))
			} else {
				sb.WriteString(userStyle.Render(label))
			}
			sb.WriteString("\n")
		}
	case menuTools:
		sb.WriteString(systemStyle.Render("Manage Tools (↑/↓ to navigate, Enter to toggle, Esc to back):"))
		sb.WriteString("\n")
		m.renderMenuOptions(sb)
	case menuCommands:
		sb.WriteString(systemStyle.Render("Manage Commands (↑/↓ to navigate, Enter to select, Esc to back):"))
		sb.WriteString("\n")
		m.renderMenuOptions(sb)
	case menuConfig:
		sb.WriteString(systemStyle.Render("Configuration Menu (↑/↓ to navigate, Enter to select, Esc to cancel):"))
		sb.WriteString("\n")
		m.renderMenuOptions(sb)
	}
}

func (m *ShellModel) renderMenuOptions(sb *strings.Builder) {
	for i, opt := range m.menu.options {
		if i == m.menu.selectedIdx {
			sb.WriteString(highlightStyle.Render(fmt.Sprintf(" > %s ", opt)))
		} else {
			sb.WriteString(userStyle.Render(fmt.Sprintf("   %s ", opt)))
		}
		sb.WriteString("\n")
	}
}

func (m *ShellModel) handleSubmit() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
		return m, nil
	}

	var images []string
	if strings.Contains(value, "@") {
		parts := strings.Split(value, " ")
		for i, part := range parts {
			if strings.HasPrefix(part, "@") {
				path := strings.TrimPrefix(part, "@")
				if isImage(path) {
					if encoded, err := encodeImage(path); err == nil {
						images = append(images, encoded)
					}
				}
				parts[i] = path
			}
		}
		value = strings.Join(parts, " ")
	}

	if m.loading {
		return m, nil
	}

	if m.allowedCmdMode.active {
		m.cfg.Shell.AllowedCommands = strings.Split(value, ",")
		if err := config.SaveConfig(m.cfg); err != nil {
			m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Error saving config: %v", err)})
		} else {
			m.messages = append(m.messages, Message{role: "system", content: fmt.Sprintf("Allowed commands updated to: %s", strings.Join(m.cfg.Shell.AllowedCommands, ","))})
		}
		m.allowedCmdMode.active = false
		m.input.Prompt = "ai-shell > "
		m.input.SetValue("")
		return m, nil
	}

	if len(m.history) == 0 || m.history[len(m.history)-1] != value {
		m.history = append(m.history, value)
		saveHistory(m.commandHistoryPath, m.history)
	}
	m.historyIndex = -1

	m.input.SetValue("")

	if strings.HasPrefix(value, "/") {
		return m.handleCommand(strings.TrimPrefix(value, "/"))
	}

	if handled, model, cmd := m.handleBuiltinCommand(value); handled {
		return model, cmd
	}

	m.messages = append(m.messages, Message{role: "user", content: value, images: images})

	m.loading = true
	m.cancelChan = make(chan struct{})

	go m.ElaborateMessage()

	return m, nil
}

func (m *ShellModel) handleBuiltinCommand(cmd string) (handled bool, model tea.Model, cmd2 tea.Cmd) {
	switch cmd {
	case "exit", "quit":
		if m.litertlmService != nil {
			m.litertlmService.Stop()
		}
		m.quitting = true
		return true, m, tea.Quit
	case "get-config":
		m.showConfig()
	case "config":
		m.openConfigMenu()
	case "help":
		m.showHelp()
	case "models":
		m.openModelMenu()
	case "reset":
		m.messages = nil
	default:
		return false, m, nil
	}
	return true, m, nil
}

func (m *ShellModel) handleCommand(input string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}
	cmd := parts[0]
	args := strings.Join(parts[1:], " ")

	if handled, model, cmd2 := m.handleBuiltinCommand(cmd); handled {
		return model, cmd2
	}

	for _, c := range m.commands {
		if c.Name == cmd {
			fullPrompt := c.Prompt
			if args != "" {
				fullPrompt = c.Prompt + " " + args
			}
			m.messages = append(m.messages, Message{role: "user", content: fullPrompt})
			m.loading = true
			m.cancelChan = make(chan struct{})
			go m.ElaborateMessage()
			return m, nil
		}
	}
	m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Unknown command: /%s", cmd)})
	return m, nil
}

func (m *ShellModel) handleAutocomplete() (tea.Model, tea.Cmd) {
	value := m.input.Value()

	if strings.HasPrefix(value, "/") && !strings.Contains(value, " ") {
		partial := strings.TrimPrefix(value, "/")
		for _, cmd := range availableCommands {
			if strings.HasPrefix(cmd, partial) {
				m.input.SetValue("/" + cmd)
				break
			}
		}
		if m.input.Value() == value {
			for _, c := range m.commands {
				if strings.HasPrefix(c.Name, partial) {
					m.input.SetValue("/" + c.Name)
					break
				}
			}
		}
	} else {
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
	}

	m.updateSuggestions()
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

	if strings.HasPrefix(value, "/") && !strings.Contains(value, " ") {
		filter := strings.TrimPrefix(value, "/")
		for _, cmd := range availableCommands {
			if strings.HasPrefix(cmd, filter) {
				matches = append(matches, "/"+cmd)
			}
		}
		for _, c := range m.commands {
			if strings.HasPrefix(c.Name, filter) {
				exists := false
				for _, m := range matches {
					if m == "/"+c.Name {
						exists = true
						break
					}
				}
				if !exists {
					matches = append(matches, "/"+c.Name)
				}
			}
		}
	}

	if lastAt := strings.LastIndex(value, "@"); lastAt != -1 {
		partial := value[lastAt+1:]
		dir, base := filepath.Split(partial)

		fileMatches := m.completeFiles(dir, base)
		for _, fm := range fileMatches {
			matches = append(matches, value[:lastAt+1]+fm)
		}
	}

	if len(matches) > 0 {
		if len(matches) == 1 && matches[0] == value {
			m.showSuggestions = false
			return
		}
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

	m.updateSuggestions()
	return m, nil
}

func (m *ShellModel) sortedToolNames() []string {
	var names []string
	for name := range m.cfg.Tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (m *ShellModel) showHelp() {
	var sb strings.Builder
	sb.WriteString("\nCommands:\n")
	sb.WriteString("  /help         - Show this help message\n")
	sb.WriteString("  /config       - Show configuration menu\n")
	sb.WriteString("  /get-config   - Show current LLM settings\n")
	sb.WriteString("  /models       - Switch to a different model\n")
	sb.WriteString("  /reset        - Clear the screen and messages\n")
	sb.WriteString("  /exit, /quit  - Exit the shell\n")
	sb.WriteString("  /<command>    - Execute a shell command\n")
	sb.WriteString("  @<file>       - Autocomplete file paths\n")
	sb.WriteString("  <text>        - Send text to the AI for a response\n")

	if len(m.commands) > 0 {
		sb.WriteString("\nUser Commands:\n")
		for _, c := range m.commands {
			if c.Description != "" {
				sb.WriteString(fmt.Sprintf("  /%-12s - %s\n", c.Name, c.Description))
			} else {
				sb.WriteString(fmt.Sprintf("  /%-12s - %s\n", c.Name, c.Prompt))
			}
		}
	}

	sb.WriteString("\nEnv Files:\n")
	envPaths := config.GetEnvPaths()
	for _, path := range envPaths {
		sb.WriteString(fmt.Sprintf("  %s\n", path))
	}

	m.messages = append(m.messages, Message{role: "system", content: sb.String()})
}

func (m *ShellModel) showConfig() {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Provider: %s\n", m.cfg.LLM.Provider))
	sb.WriteString(fmt.Sprintf("Model: %s\n", m.cfg.LLM.Model))
	sb.WriteString(fmt.Sprintf("Confirm Commands: %v\n", m.cfg.Shell.Confirm))
	sb.WriteString(fmt.Sprintf("Allowed Commands: %s\n", strings.Join(m.cfg.Shell.AllowedCommands, ",")))

	toolDescs := llm.GetToolDescriptions()
	sb.WriteString("\nTools:\n")
	for _, name := range m.sortedToolNames() {
		status := "disabled"
		if m.cfg.Tools[name] {
			status = "enabled"
		}
		desc := toolDescs[name]
		sb.WriteString(fmt.Sprintf("  %s (%s)", name, status))
		if desc != "" {
			sb.WriteString(fmt.Sprintf(" - %s", desc))
		}
		sb.WriteString("\n")
	}

	if len(m.commands) > 0 {
		sb.WriteString("\nCommands:\n")
		for _, c := range m.commands {
			if c.Description != "" {
				sb.WriteString(fmt.Sprintf("  /%s - %s\n", c.Name, c.Description))
			} else {
				sb.WriteString(fmt.Sprintf("  /%s - %s\n", c.Name, c.Prompt))
			}
		}
	}

	m.messages = append(m.messages, Message{role: "system", content: sb.String()})
}

func (m *ShellModel) openModelMenu() {
	models := config.GetAllAvailableModels()

	if len(models) == 0 {
		m.messages = append(m.messages, Message{role: "system", content: "No models found. Please install models using 'ollama pull <model>'"})
		return
	}

	m.menu.models = models
	m.menu.open(menuModel)
	m.input.SetValue("")
}

func (m *ShellModel) openConfigMenu() {
	m.menu.options = []string{
		fmt.Sprintf("Confirm Commands: %v", m.cfg.Shell.Confirm),
		fmt.Sprintf("Allowed Commands: %s", strings.Join(m.cfg.Shell.AllowedCommands, ",")),
		"Manage Tools",
		"Manage Commands",
		"Change Model",
		"Back",
	}
	m.menu.open(menuConfig)
	m.input.SetValue("")
}

func (m *ShellModel) selectConfigOption() {
	switch m.menu.selectedIdx {
	case 0:
		m.cfg.Shell.Confirm = !m.cfg.Shell.Confirm
		if err := config.SaveConfig(m.cfg); err != nil {
			m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Error saving config: %v", err)})
		} else {
			m.messages = append(m.messages, Message{role: "system", content: fmt.Sprintf("Confirm Commands set to: %v", m.cfg.Shell.Confirm)})
		}
		m.menu.close()
	case 1:
		m.allowedCmdMode.active = true
		m.input.SetValue(strings.Join(m.cfg.Shell.AllowedCommands, ","))
		m.input.Prompt = "Allowed commands (comma separated): "
		m.menu.close()
	case 2:
		m.menu.close()
		m.openToolsMenu()
	case 3:
		m.menu.close()
		m.openCommandsMenu()
	case 4:
		m.menu.close()
		m.openModelMenu()
	case 5:
		m.menu.close()
	}
}

func (m *ShellModel) openCommandsMenu() {
	m.menu.options = []string{}
	for _, c := range m.commands {
		if c.Description != "" {
			m.menu.options = append(m.menu.options, fmt.Sprintf("/%s - %s", c.Name, c.Description))
		} else {
			m.menu.options = append(m.menu.options, fmt.Sprintf("/%s - %s", c.Name, c.Prompt))
		}
	}
	m.menu.options = append(m.menu.options, "Back")

	m.menu.open(menuCommands)
	m.input.SetValue("")
}

func (m *ShellModel) selectCommandOption() {
	m.menu.close()
	m.openConfigMenu()
}

func (m *ShellModel) openToolsMenu() {
	toolDescs := llm.GetToolDescriptions()

	m.menu.options = []string{}
	for _, name := range m.sortedToolNames() {
		status := "Disabled"
		if m.cfg.Tools[name] {
			status = "Enabled"
		}
		desc := toolDescs[name]
		option := fmt.Sprintf("%s: %s", name, status)
		if desc != "" {
			option = fmt.Sprintf("%s: %s - %s", name, status, desc)
		}
		m.menu.options = append(m.menu.options, option)
	}
	m.menu.options = append(m.menu.options, "Back")

	m.menu.open(menuTools)
	m.input.SetValue("")
}

func (m *ShellModel) selectToolOption() {
	if m.menu.selectedIdx == len(m.menu.options)-1 {
		m.menu.close()
		m.openConfigMenu()
		return
	}

	selectedTool := m.sortedToolNames()[m.menu.selectedIdx]
	m.cfg.Tools[selectedTool] = !m.cfg.Tools[selectedTool]

	if err := config.SaveConfig(m.cfg); err != nil {
		m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Error saving config: %v", err)})
	} else {
		status := "disabled"
		if m.cfg.Tools[selectedTool] {
			status = "enabled"
		}
		m.messages = append(m.messages, Message{role: "system", content: fmt.Sprintf("Tool %s %s", selectedTool, status)})
	}

	m.openToolsMenu()
}

func (m *ShellModel) selectModel() {
	if m.menu.selectedIdx < 0 || m.menu.selectedIdx >= len(m.menu.models) {
		return
	}

	modelInfo := m.menu.models[m.menu.selectedIdx]
	selectedModel := modelInfo.Name
	provider := modelInfo.Provider

	if err := config.SaveModelWithProvider(selectedModel, provider); err != nil {
		m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Error saving model: %v", err)})
	} else {
		m.messages = append(m.messages, Message{role: "system", content: fmt.Sprintf("Switched to model: %s", selectedModel)})
		if newCfg, err := config.LoadConfig(); err == nil {
			m.cfg = newCfg
		}
	}

	if provider == "litertlm" && (m.litertlmService == nil || !m.litertlmService.IsRunning()) {
		port := extractPort(os.Getenv("LITERTLM_BASE_URL"))
		svc := NewLitertLMService(port)
		if err := svc.Start(); err != nil {
			m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Failed to start litertlm service: %v", err)})
		} else {
			m.litertlmService = svc
		}
	}

	m.menu.close()
}

func (m *ShellModel) ElaborateMessage() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-m.cancelChan
		cancel()
	}()

	agent := llm.NewAgent(m.cfg.LLM.Model, m.cfg.LLM.Provider, m.cfg.Tools)

	executor := &ShellExecutorForLLM{m: m}

	var commonMessages []llm.Message
	for _, msg := range m.messages {
		if msg.role == "user" || msg.role == "assistant" || msg.role == "tool" {
			var content any = msg.content
			if len(msg.images) > 0 {
				parts := []llm.ContentPart{
					{Type: "text", Text: msg.content},
				}
				for _, img := range msg.images {
					parts = append(parts, llm.ContentPart{
						Type:     "image_url",
						ImageURL: &llm.ContentImage{URL: img},
					})
				}
				content = parts
			}
			commonMessages = append(commonMessages, llm.Message{Role: msg.role, Content: content, ToolCallID: msg.toolCallID, ToolCalls: msg.toolCalls})
		}
	}

	resultMessages, err := agent.CallLLM(ctx, executor, commonMessages)

	if err != nil {
		m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Error: %v", err)})
		m.loading = false
		if m.teaProgram != nil {
			m.teaProgram.Send(responseReadyMsg{})
		}
		return
	}

	for _, msg := range resultMessages {
		contentStr := ""
		if s, ok := msg.Content.(string); ok {
			contentStr = s
		} else {
			contentStr = fmt.Sprintf("%v", msg.Content)
		}

		switch msg.Role {
		case "user":
			m.messages = append(m.messages, Message{role: "user", content: contentStr})
		case "assistant":
			m.messages = append(m.messages, Message{role: "assistant", content: contentStr, toolCalls: msg.ToolCalls})
		case "tool":
			m.messages = append(m.messages, Message{role: "tool", content: contentStr, toolCallID: msg.ToolCallID})
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

func isImage(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp"
}

func encodeImage(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	mimeType := "image/jpeg"
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	}

	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data)), nil
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
