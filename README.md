# AI-Shell 🤖🐚

An interactive shell powered by AI (**Ollama, Gemini, Mistral**) that helps you with commands, explanations, and automation directly from your terminal.

## Features

- **Interactive AI Chat**: Ask questions about shell commands, scripting, or general knowledge.
- **Rich TUI**: Built with Bubbletea, featuring a modern interface with command history, autocomplete, and real-time feedback.
- **System Awareness**: Automatically detects your Linux distribution and shell to provide tailored advice.
- **Autonomous Tool Use**: The AI can execute shell commands (`RunCommand`), read files (`ReadFile`), and write files (`WriteFile`) autonomously (with your confirmation).
- **Multi-Provider Support**: Supports Ollama (local), Google Gemini, and Mistral through a unified API.
- **Configurable**: Easily switch models and providers via a YAML configuration file.

## Prerequisites

- **Go**: Version 1.25.5 or later.
- **Providers**:
  - **Ollama**: Must be installed and running for local models (default).
  - **Gemini**: Requires `GEMINI_API_KEY` environment variable.
  - **Mistral**: Requires `MISTRAL_KEY` environment variable.
- **LLM Model**: By default, it expects the `granite4:3b-h` model, but this can be changed in the config.

## Installation

You can build and install the binary using the provided Makefile:

```bash
# Clone the repository
git clone https://github.com/yourusername/ai-shell.git
cd ai-shell

# Build the binary
make build

# Or install it to your $GOPATH/bin
make install
```

## Usage

Start the interactive shell by running:

```bash
ai-shell
```

### Interactive Commands

Within the `ai-shell >` prompt, you can use the following commands (with or without the `/` prefix):

- **Type anything**: Send a request to the AI (e.g., "how do I find large files?").
- **`help`**: Show the help menu.
- **`get-config`**: See current model and configuration file location.
- **`models`**: Open a menu to select the model.
- **`reset`**: Clear the chat history.
- **`exit` or `quit`**: Close the shell.

### TUI Shortcuts

- **Arrows (↑/↓)**: Navigate through command history.
- **Tab**: Trigger autocomplete for commands.
- **Ctrl+C**: Stop the current operation or exit.
- **Esc**: Cancel the current request or clear the input.

### Pipe Support

You can also pipe questions directly into `ai-shell`:

```bash
echo "how do I list files by size?" | ai-shell
```

## Configuration

The application looks for a `config.yaml` file in:
1. The current directory.
2. `~/.config/ai-shell/config.yaml`

Default configuration:

```yaml
llm:
  provider: "ollama"
  model: "granite4:3b-h"
shell:
  confirm: true
  allowed_commands: "ls,pwd"
```

### Configuration Options

- **`llm.provider`**: The AI provider to use (`ollama`, `gemini`, or `mistral`).
- **`llm.model`**: The specific model name.
- **`shell.confirm`**: If `true`, the application will always ask for confirmation before executing an AI-suggested command.
- **`shell.allowed_commands`**: A comma-separated list of safe commands that the AI can execute without requiring user confirmation (e.g., "ls,pwd,date").

## How it works

AI-Shell uses a unified OpenAI-compatible API to communicate with your LLM models. It uses a system prompt to inform the LLM about your environment (e.g., "running on Ubuntu 22.04 using /bin/bash"). 

When the LLM decides it needs to perform an action, it can use the following tools:
- **`RunCommand`**: Executes a shell command and returns the output.
- **`ReadFile`**: Reads the content of a specified file.
- **`WriteFile`**: Writes content to a specified file.

Commands are executed via `bash -c`. For safety, execution of commands not in `allowed_commands` always requires explicit user confirmation (`y/N`).

## License

MIT
