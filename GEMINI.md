# Gemini CLI Context: AI-Shell

This document provides essential context and instructions for AI agents interacting with the `ai-shell` codebase.

## Project Overview

`ai-shell` is an interactive terminal application that brings AI-powered assistance directly to the command line. It features a rich Text User Interface (TUI) and supports multiple LLM providers, including Ollama (local), Google Gemini, and Mistral.

### Core Technologies
- **Language:** Go (v1.25.5+)
- **CLI Framework:** [Cobra](https://github.com/spf13/cobra)
- **TUI Framework:** [Bubbletea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [Lipgloss](https://github.com/charmbracelet/lipgloss)
- **Configuration:** [Viper](https://github.com/spf13/viper) (YAML/Environment variables)
- **AI Integrations:** Unified OpenAI-compatible API (Ollama, Gemini, Mistral)

### Architecture
- **`cmd/`**: Contains the CLI entry points and the TUI implementation (`shell.go`).
- **`llm/`**: Provider-specific implementations (`gemini.go`, `mistral.go`, `ollama.go`) and the `ToolExecutor` interface for AI-driven system interactions.
- **`config/`**: Handles configuration loading, model lists, and YAML persistence.
- **`tools/`**: System-level utilities for command execution and environment detection (OS/Shell).

## Building and Running

### Commands
- **Build:** `make build` (produces `ai-shell` binary)
- **Install:** `make install` (installs to `$GOPATH/bin`)
- **Test:** `go test ./...`
- **Coverage:** `make coverage` (generates `coverage.html`)

### Environment Setup
- **Ollama:** Must be running locally for the default provider.
- **API Keys:** 
  - `GEMINI_API_KEY`: Required for Gemini models.
  - `MISTRAL_KEY`: Required for Mistral models.
- **Config File:** Located at `~/.config/ai-shell/config.yaml` or in the current directory.

## Development Conventions

- **TUI Pattern:** The interactive shell follows the Model-View-Update (MVU) pattern from Bubbletea. Logic for handling input and rendering the UI resides in `cmd/shell.go`.
- **LLM Abstraction:** LLM providers are implemented as "callers" that satisfy provider-specific requirements while utilizing a common `ToolExecutor` for shell command execution.
- **Safety & Security:**
  - AI-driven command execution requires user confirmation unless the command is explicitly listed in `shell.allowed_commands` within the configuration.
  - Never hardcode API keys; use environment variables or `.env` files.
- **Error Handling:** Use idiomatic Go error handling. TUI errors should be reported via the `Message` slice with the `error` role for visibility in the terminal.
- **Testing:** New features should include tests in their respective packages (e.g., `config_test.go`, `system_test.go`).

## Key Files
- `cmd/shell.go`: Main TUI loop and LLM orchestration.
- `llm/gemini.go`: Implementation of the Google Gemini caller with tool-calling support.
- `config/config.go`: Configuration schema and persistence logic.
- `tools/system.go`: Safe execution of shell commands and environment detection.
