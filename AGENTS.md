# AGENTS.md

## Project

Interactive AI shell TUI (Bubbletea) with 4 LLM providers: Ollama, Gemini, OpenRouter, LitertLM. All providers wrap `OpenAICaller` (`llm/openai.go`) with different base URLs and API keys — the common entry point for adding a new provider.

Entry point: `main.go` → `cmd.Execute()`. Default command launches Bubbletea TUI in `cmd/shell.go` (1279 lines, MVU pattern). When the `litertlm` provider is configured, `cmd/litertlm_service.go` auto-starts `litert-lm serve --port <port> --api openai` as a background process and health-checks it before the TUI begins.

## Commands

```
make build         # go build -o ai-shell .
make install       # go install .
make coverage      # test + HTML report
go fmt ./... && go vet ./... && go build -o ai-shell . && go test ./...
```

## Packages

| Directory | Contents |
|-----------|----------|
| `cmd/` | Cobra commands: default (TUI shell), `get-config` |
| `config/` | Viper YAML config, model lists, `.env` loading via `gotenv` |
| `llm/` | `Agent` struct, `Caller` interface, `ToolExecutor` interface, 6 OpenAI tool definitions, default system prompt |
| `tools/` | `RunCommand` (bash -c), `ReadFile`, `WriteFile`, KV store (bbolt), `GetDistro`, `GetShell` |

## Config layering

1. `~/.config/ai-shell/.env` (global)
2. `./.env` (local overrides)
3. `config.yaml` from `./` or `~/.config/ai-shell/`

Defaults: provider=ollama, model=granite4:3b-h, confirm=true, allowed_commands=ls,pwd. All 6 tools enabled by default. Custom commands stored in config.

## Testing

```bash
go test ./...           # all pass (config, llm, tools)
go test -v -run TestName ./package
go test -cover ./...
```

Test conventions: table-driven tests, `t.Run()` subtests, function variable mocking (e.g. `userConfigDirFunc`, `loadEnvFunc` in `config/`, `dbPathFunc` in `tools/`).

TUI code in `cmd/` has no tests yet.

## TUI conventions (cmd/shell.go)

- `/` commands: help, get-config, config, models, reset, add-cmd, exit, quit
- `@filepath` for image attachments (base64 encoded)
- Tool execution requires user confirmation unless command is in `allowed_commands`
- Lipgloss styles: `promptStyle`, `systemStyle`, `userStyle`, `aiStyle`, `errorStyle`, `cmdStyle`, `helpStyle`, `dimStyle`

## Key gotchas

- `config.LoadConfig()` may return partial defaults on error — check both return values
- `.env` files are loaded at startup via `gotenv.Load()` — place API keys there, not in config.yaml
- KV store uses bbolt at `~/.config/ai-shell/kv_store.db`
- System prompt is a Go constant in `llm/prompt.go`, copied to `~/.ai-shell/PROMPT.md` on first run — always read from there, never from local file
- 100 char soft line limit, Go 1.25.5+
