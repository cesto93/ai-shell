package cmd

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"ai-shell/config"
	"ai-shell/tools"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ollama/ollama/api"
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

type ShellModel struct {
	input              textinput.Model
	messages           []Message
	history            []string
	historyIndex       int
	commandHistoryPath string
	width              int
	height             int
	quitting           bool
	cfg                *config.Config
	client             *api.Client
}

func NewShellModel() (*ShellModel, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	client, err := api.ClientFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to create Ollama client: %w", err)
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
		client:             client,
	}

	return m, nil
}

func (m *ShellModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *ShellModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
			return m.handleSubmit()

		case tea.KeyUp:
			return m.navigateHistory(-1)

		case tea.KeyDown:
			return m.navigateHistory(1)

		case tea.KeyTab:
			return m.handleAutocomplete()

		case tea.KeyEscape:
			m.input.SetValue("")
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
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

	sb.WriteString(m.input.View())
	sb.WriteString("\n")

	return sb.String()
}

func (m *ShellModel) handleSubmit() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
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
		if err := config.SelectModel(); err != nil {
			m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Error: %v", err)})
		}
		if newCfg, err := config.LoadConfig(); err == nil {
			m.cfg = newCfg
		}
		return m, nil

	case "reset":
		m.messages = nil
		return m, nil
	}

	m.messages = append(m.messages, Message{role: "user", content: value})

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
		if err := config.SelectModel(); err != nil {
			m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Error: %v", err)})
		}
		if newCfg, err := config.LoadConfig(); err == nil {
			m.cfg = newCfg
		}

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
	sb.WriteString(fmt.Sprintf("Model: %s\n", m.cfg.LLM.Model))
	sb.WriteString(fmt.Sprintf("Confirm Commands: %v\n", m.cfg.Shell.Confirm))
	sb.WriteString(fmt.Sprintf("Allowed Commands: %s\n", m.cfg.Shell.AllowedCommands))
	m.messages = append(m.messages, Message{role: "system", content: sb.String()})
}

func (m *ShellModel) callOllama(prompt string) {
	ctx := context.Background()

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
	systemPrompt := fmt.Sprintf("You are a helpful shell assistant. The user is running on %s using %s shell.", distro, shell)

	messages := []api.Message{
		{Role: "system", Content: systemPrompt},
	}
	for _, msg := range m.messages {
		if msg.role == "user" || msg.role == "assistant" || msg.role == "tool" {
			messages = append(messages, api.Message{Role: msg.role, Content: msg.content})
		}
	}
	messages = append(messages, api.Message{Role: "user", Content: prompt})

	tea.Println(systemStyle.Render("Thinking..."))

	for {
		req := &api.ChatRequest{
			Model:    m.cfg.LLM.Model,
			Messages: messages,
			Tools:    []api.Tool{runCommandTool},
			Stream:   new(bool),
		}
		*req.Stream = false

		var response api.ChatResponse
		respFunc := func(resp api.ChatResponse) error {
			response = resp
			return nil
		}

		if err := m.client.Chat(ctx, req, respFunc); err != nil {
			m.messages = append(m.messages, Message{role: "error", content: fmt.Sprintf("Error: %v", err)})
			return
		}

		messages = append(messages, response.Message)

		if len(response.Message.ToolCalls) == 0 {
			if response.Message.Content != "" {
				m.messages = append(m.messages, Message{role: "assistant", content: response.Message.Content})
			}
			break
		}

		for _, tc := range response.Message.ToolCalls {
			if tc.Function.Name == "RunCommand" {
				cmd, ok := tc.Function.Arguments["command"].(string)
				if !ok {
					result := "Error: Invalid tool arguments"
					m.messages = append(m.messages, Message{role: "tool", content: errorStyle.Render(result)})
					messages = append(messages, api.Message{Role: "tool", Content: result})
					continue
				}

				cmdName := getCommandName(cmd)
				skipConfirm := config.IsAllowedCommand(cmdName, m.cfg.Shell.AllowedCommands)

				confirmMsg := fmt.Sprintf("[Executing: %s]", cmd)
				m.messages = append(m.messages, Message{role: "tool", content: systemStyle.Render(confirmMsg)})

				if m.cfg.Shell.Confirm && !skipConfirm {
					confirm := askConfirmation(cmd)
					if !confirm {
						result := "Error: Command execution denied by user"
						m.messages = append(m.messages, Message{role: "tool", content: errorStyle.Render(result)})
						messages = append(messages, api.Message{Role: "tool", Content: result})
						continue
					}
				}

				output, err := tools.RunCommand(cmd)
				if err != nil {
					result := fmt.Sprintf("Error: %v\nOutput: %s", err, output)
					m.messages = append(m.messages, Message{role: "tool", content: errorStyle.Render(result)})
				} else {
					m.messages = append(m.messages, Message{role: "tool", content: cmdStyle.Render(output)})
				}
				messages = append(messages, api.Message{Role: "tool", Content: output})
			}
		}
	}
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

	_, err = p.Run()
	return err
}
