# AI-Shell

An interactive shell powered by AI (**Ollama, Gemini, OpenRouter, LitertLM, Llamacpp**) that helps you with commands, explanations, and automation directly from your terminal.

## Features

- **Interactive AI Chat**: Ask questions about shell commands, scripting, or general knowledge.
- **Rich TUI**: Built with Bubbletea, featuring a modern interface with command history, autocomplete, and real-time feedback.
- **System Awareness**: Automatically detects your Linux distribution and shell to provide tailored advice.
- **Autonomous Tool Use**: The AI can execute shell commands (`RunCommand`), read files (`ReadFile`), and write files (`WriteFile`) autonomously (with your confirmation).
- **Multi-Provider Support**: Ollama, Google Gemini, OpenRouter, LitertLM, and in-process Llamacpp (via yzma).
- **Configurable**: Easily switch models and providers via a YAML configuration file.

## Prerequisites

- **Go**: Version 1.25.5 or later.
- **Providers**:
  - **Ollama**: Must be running locally (default, port 11434).
  - **Gemini**: Requires `GEMINI_API_KEY` environment variable.
  - **OpenRouter**: Requires `OPEN_ROUTE_KEY` environment variable.
  - **LitertLM**: In-process LiteRT-LM via litertlm-go. Requires a `.litertlm` model and the LiteRT-LM shared libraries (see LitertLM Provider).
  - **Llamacpp**: In-process llama.cpp via yzma. Requires GGUF model and yzma libs in `~/.ai-shell/models/llamacpp/` (see Installation).
- **LLM Model**: By default, it expects the `granite4:3b-h` model, but this can be changed in the config.

## Installation

You can build and install the binary using the provided Makefile:

```bash
# Clone the repository
git clone https://github.com/yourusername/ai-shell.git
cd ai-shell

# Build the binary
make build

# Or install it to your $GOPATH/bin (includes yzma libs)
make install

# Install yzma CLI and llama.cpp libraries to ~/.ai-shell/models/llamacpp/
make install-yzma
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

The application looks for a `.env` file in:
1. The current directory.
2. `~/.config/ai-shell/.env`

Required environment variables depending on your provider:
- **Ollama**: No API key needed (local)
- **Gemini**: `GEMINI_API_KEY`
- **OpenRouter**: `OPEN_ROUTE_KEY`

The application also looks for a `config.yaml` file in:
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

- **`llm.provider`**: The AI provider to use (`ollama`, `gemini`, `openrouter`, `litertlm` or `llamacpp`).
- **`llm.model`**: The specific model name. For `litertlm`, this is a `.litertlm` filename without extension. For `llamacpp`, this is a GGUF filename (e.g., `granite4-3b-h.Q4_K_M.gguf`).
- **`litertlm.backend`**: The LiteRT-LM inference backend (`cpu` or `gpu`, default `cpu`).
- **`shell.confirm`**: If `true`, the application will always ask for confirmation before executing an AI-suggested command.
- **`shell.allowed_commands`**: A comma-separated list of safe commands that the AI can execute without requiring user confirmation (e.g., "ls,pwd,date").

## LitertLM Provider

The `litertlm` provider runs LiteRT-LM inference **in-process** using the [litertlm-go](https://github.com/vladimirvivien/litertlm-go) Go binding. It loads the LiteRT-LM shared libraries and a `.litertlm` model directly into the ai-shell process — no server binary needed.

### Setup

1. Build `liblitert-lm.so` via the "Build LiteRT-LM Shared Libraries" GitHub workflow (`.github/workflows/litertlm.yaml`) and place it in `~/.ai-shell/lib/`. Then fetch the prebuilt aux/GPU libraries and set up the required symlink:
   ```bash
   make install-litertlm
   ```
   This downloads the prebuilt LiteRT-LM libs into `~/.ai-shell/lib/` and symlinks `liblitert-lm.so` → `liblitertlm_c_cpu.so` (the filename the binding actually dlopens).

2. Place a `.litertlm` model file in `~/.ai-shell/models/litertlm/` (or set `LITERTLM_MODELS_DIR` / `LITERTLM_MODEL`). You can download one with `ai-shell pull`:
   ```bash
   ai-shell pull <repo> <filename.litertlm>
   ```
   Files ending in `.litertlm` are saved to `~/.ai-shell/models/litertlm/` and auto-registered as LiteRT-LM models; any other file is treated as a GGUF model for the `llamacpp` provider.

3. Configure `~/.config/ai-shell/config.yaml`:
   ```yaml
   llm:
     provider: "litertlm"
     model: "gemma-4-E2B-it"
   litertlm:
     backend: "cpu"
   ```

   The `.litertlm` extension is appended automatically if omitted. The backend (`cpu`/`gpu`, default `cpu`) can also be set via `LITERTLM_BACKEND`.

### Limitations

- **Structured output** (`ai-shell extract`) uses schema prompting, not the binding's native structured-output path.
- **No API key or base URL** needed — purely local.

## Llamacpp Provider

The `llamacpp` provider runs llama.cpp inference **in-process** using the [yzma](https://github.com/hybridgroup/yzma) Go binding. Unlike other providers that send HTTP requests to a server, it loads the model directly into the ai-shell process — no external service needed.

### Setup

1. Install yzma and the llama.cpp shared libraries:
   ```bash
   make install-yzma
   ```
   This downloads the llama.cpp `.so` files to `~/.ai-shell/models/llamacpp/`.

2. Place a GGUF model file in `~/.ai-shell/models/llamacpp/`. For example:
   ```bash
   wget -O ~/.ai-shell/models/llamacpp/granite4-3b-h.Q4_K_M.gguf <url>
   ```

3. Configure `~/.config/ai-shell/config.yaml`:
   ```yaml
   llm:
     provider: "llamacpp"
     model: "granite4-3b-h.Q4_K_M.gguf"
   ```

   The `.gguf` extension is appended automatically if omitted.

### Limitations

- **Structured output** (`ai-shell extract`) is not supported.
- **Image/audio input** is not supported.
- **No API key or base URL** needed — purely local.

## How it works

AI-Shell uses a unified OpenAI-compatible API to communicate with your LLM models. It uses a system prompt to inform the LLM about your environment (e.g., "running on Ubuntu 22.04 using /bin/bash"). 

When the LLM decides it needs to perform an action, it can use the following tools:
- **`RunCommand`**: Executes a shell command and returns the output.
- **`ReadFile`**: Reads the content of a specified file.
- **`WriteFile`**: Writes content to a specified file.
- **`KVSet`/`KVGet`/`KVList`**: Persistent key-value store (bbolt).

Commands are executed via `bash -c`. For safety, execution of commands not in `allowed_commands` always requires explicit user confirmation (`y/N`).

## License

MIT
