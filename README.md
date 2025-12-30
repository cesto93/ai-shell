# AI-Shell 🤖🐚

An interactive shell powered by AI (**Ollama**) that helps you with commands, explanations, and automation directly from your terminal.

## Features

- **Interactive AI Chat**: Ask questions about shell commands, scripting, or general knowledge.
- **System Awareness**: Automatically detects your Linux distribution and shell to provide tailored advice.
- **Direct Command Execution**: Run system commands without leaving the AI shell using the `!` prefix.
- **Autonomous Tool Use**: The AI can execute shell commands itself to gather information or perform tasks (enabled via Ollama tool-calling).
- **Configurable**: Easily switch models via a YAML configuration file.

## Prerequisites

- **Go**: Version 1.21 or later.
- **Ollama**: Must be installed and running. You can download it from [ollama.com](https://ollama.com).
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

Within the `ai-shell >` prompt:

- **Type anything**: Send a request to the AI (e.g., "how do I find large files?").
- **`! <command>`**: Execute a shell command directly (e.g., `! ls -la`).
- **`help`**: Show the help menu.
- **`get-config`**: See current model and configuration file location.
- **`exit` or `quit`**: Close the shell.

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
  model: "granite4:3b-h"
shell:
  confirm: true
```

To change the model, simply update the `model` field in your config file.

## How it works

AI-Shell uses the [Ollama Go API](https://github.com/ollama/ollama) to communicate with your local models. It uses a system prompt to inform the LLM about your environment (e.g., "running on Ubuntu 22.04 using /bin/bash"). When the LLM decides it needs to run a command, it uses a defined `RunCommand` tool, which executes the command and feeds the output back to the LLM.

## License

MIT
