# AGENTS.md

## Project

Interactive AI shell TUI (Bubbletea) with 5 LLM providers: Ollama, Gemini, OpenRouter, LitertLM, Llamacpp. OpenAI-compatible HTTP providers (Ollama, Gemini, OpenRouter) share `OpenAICaller` (`llm/openai.go`) via `NewProviderCaller` factory (`llm/providers.go`). Two in-process providers run no HTTP server: `llamacpp` uses `LlamacppCaller` (`llm/llamacpp.go`) with the `yzma` Go binding, and `litertlm` uses `LitertLMCaller` (`llm/litertlm.go`) with the `litertlm-go` binding.

Entry point: `main.go` → `cmd.Execute()`. Default command launches Bubbletea TUI in `cmd/shell.go` (MVU pattern).

## Commands

```
make build              # go build -o ai-shell .
make install            # go install .
make install-yzma       # install yzma CLI + llama.cpp libs to ~/.ai-shell/lib
make install-litertlm   # download LiteRT-LM C-API lib + aux/GPU libs to ~/.ai-shell/lib
make proto              # regenerate service/proto/*.pb.go (needs protoc + plugins)
make coverage           # test + HTML report
go fmt ./... && go vet ./... && go build -o ai-shell . && go test ./...
```

## Packages

| Directory | Contents |
|-----------|----------|
| `cmd/` | Cobra commands: default (TUI shell), `config`, `commit`, `extract`, `pull`, `models`, `commands`, `agents`, `stats`, `context`, `service` |
| `config/` | Viper YAML config, model lists (OpenRouter free models fetched live, 10 min cache), `.env` loading via `gotenv`. Free OpenRouter models filtered by zero pricing and `architecture.output_modalities` (audio-only excluded); `InputTypes` from `input_modalities` |
| `llm/` | `Agent`, `Caller`, `RawCaller` (adds `CallStructured`), `ToolExecutor`, 6 tool definitions, `NewProviderCaller`/`NewProviderCallerRaw` factory, `ProviderConfig`, system prompts, `LlamacppCaller`, `LitertLMCaller`. Tool dispatch via `ToolExecutorPolicy` (`llm/executor.go`, pluggable confirm/execute hooks) shared by shell/CLI/service; `NoopExecutor` runs nothing. Agents: `GetAgentDefs`/`GetAgentDef`; `NewAgentFor` intersects agent allowed tools with user toggles; `NewAgentForSession` adds backend + AGENTS.md. `OpenAICaller` persists `usage` via `stats.RecordUsage` (provider from `BaseURL`) |
| `tools/` | `RunCommand` (bash -c), `ReadFile`, `WriteFile`, KV store (bbolt), `GetDistro`, `GetShell` |
| `stats/` | Persistent token usage store (bbolt at `~/.config/ai-shell/usage.db`): `RecordUsage`, `GetStats`, `Reset` |
| `service/` | gRPC service over a unix socket at `~/.ai-shell/service.sock`. `server.go` (`Server`, swappable `callLLM`) builds the agent via `llm.NewAgentForSession` and runs tools via `ServiceExecutor`; `client.go` (`Client`, `IsActive`, `Chat`, `Stop`, `ErrUnavailable`); `convert.go` maps `llm.Message` ↔ proto. Wire types in `service/proto/` (committed; `make proto` to regenerate) |

## Config layering

1. `~/.config/ai-shell/.env` (global)
2. `./.env` (local overrides)
3. `config.yaml` from `./` or `~/.config/ai-shell/`

`.env.example` documents the recognized env vars. Defaults: provider=ollama, model=granite4:3b-h, log_level=info, confirm=true, allowed_commands=ls,pwd, agent=build, agent_files=true. litertlm backend defaults to `cpu`. All 6 tools enabled by default.

`agent_files` toggles AGENTS.md support: `llm.GetAgentFiles(true)` loads `~/.config/ai-shell/AGENTS.md` and `./AGENTS.md` as extra system-prompt instructions. Toggle via `ai-shell config --agent-files=false`.

Logging uses `log/slog`; call `config.InitLogger(cfg.LogLevel)` after `LoadConfig()`.

## Testing

```bash
go test ./...
go test -v -run TestName ./package
go test -cover ./...
```

Conventions: table-driven tests, `t.Run()` subtests, function variable mocking (`userConfigDirFunc`, `loadEnvFunc`, `dbPathFunc`).

## TUI conventions (cmd/shell.go)

- `/` commands: help, get-config, config, models, agent, reset, add-cmd, exit, quit
- `@filepath` for image attachments (base64 encoded)
- Tool execution requires confirmation unless command is in `allowed_commands`
- `/agent` opens a selection menu; `m.cfg.Agent` is persisted to config. `ElaborateMessage` builds the agent via `llm.NewAgentForSession(...)`
- Lipgloss styles: `promptStyle`, `systemStyle`, `userStyle`, `aiStyle`, `errorStyle`, `cmdStyle`, `helpStyle`, `dimStyle`

## Debug flag (cmd/cmd.go)

Persistent `--debug` flag on the root command. `cmd.initLogger(cfg)` temporarily forces debug level in memory. All subcommands must call `initLogger(cfg)` after `config.LoadConfig()`.

## Config command (cmd/config.go)

- Usable as `ai-shell config` (shows current config via `PrintConfig`) or `ai-shell config --flag value`
- Flags: `--provider`, `--model`, `--agent`, `--agent-files`, `--log-level`, `--confirm`, `--allowed-commands`, `--backend`, `--enable-tool`, `--disable-tool`, `--add-cmd`, `--rm-cmd`
- `--model` without `--provider` auto-detects provider via `config.LookupModelInfo`; `--add-cmd` uses `name=prompt` format

## Commands command (cmd/commands.go)

- Usable as `ai-shell commands` (lists custom commands) or `ai-shell commands --run <name> [args...]`
- `--run` looks up via `config.LoadCommands` (merges `.ai-shell/commands/*.md` files and config `commands` map); extra args are appended to the prompt
- Uses `llm.NewAgentFor` with `llm.ToolExecutorPolicy{}` (no confirmation); prints final assistant text to stdout

## Commit command (cmd/commit.go)

- Usable as `ai-shell commit` or via `go run . commit`
- `-A` / `--all` stages all changes; `-d` / `--dry-run` prints without committing (and unstages if used with `-A`)
- Sends `git log --oneline -5` + `git diff --cached` as context; uses `llm.NewProviderCaller` with `llm.NoopExecutor` (no tools)
- Strips markdown code fences, writes to a temp file, runs `git commit -F <file>`

## Extract command (cmd/extract.go)

- Usable as `ai-shell extract <input> <schema>`
- Input file (.txt/.md/.pdf/.png/.jpg/.jpeg/.gif/.webp) and JSON schema file; `-o` / `--output` writes to a file (default stdout)
- Uses `pdftotext` for PDFs; images sent as multimodal `[]ContentPart` (via shell helpers)
- Calls `llm.NewProviderCallerRaw(...).CallStructured` with `response_format: json_schema`

## Pull command (cmd/pull.go)

- Usable as `ai-shell pull <repo> <model> [mmproj]`
- Downloads from HuggingFace; `.litertlm` → `~/.ai-shell/models/litertlm/`, else `.gguf` → `~/.ai-shell/models/llamacpp/`
- Progress bar; auto-updates config via `config.SaveModelWithProvider` (second file auto-detected as vision projector); cleans up partial files on failure

## Agents command (cmd/agents.go)

- Usable as `ai-shell agents`
- Lists `llm.GetAgentDefs()` in a table (AGENT, DESCRIPTION, TOOLS); current agent prefixed with `* `; sorted; `text/tabwriter`

## Models command (cmd/models.go)

- Usable as `ai-shell models`
- Lists models in a table (Model, Provider, Size, Input Types); current model prefixed with `* `
- `-s` / `--set <model>` sets the current model; `-d` / `--delete <model>` deletes a local GGUF/`.litertlm` file (also removes paired mmproj for llamacpp)
- For llamacpp, Size from GGUF file info. Input types: gemini hardcoded; openrouter from API `input_modalities`; llamacpp `text, image` when an `mmproj-*` file matches; ollama/litertlm `-`

## Stats command (cmd/stats.go)

- Usable as `ai-shell stats`
- Aggregated token usage table (CALLS, INPUT, OUTPUT, CACHED, REASONING, TOTAL, COST + TOTAL row) from `stats.GetStats()`
- `--reset` clears all usage

## Context command (cmd/context.go)

- Usable as `ai-shell context`
- Shows AGENTS.md files read into context (path, word count, token estimate) plus the active agent's system prompt size
- `--prompt` prints the system prompt; `--agents` prints the AGENTS.md texts
- Uses `llm.GetAgentFileInfo(cfg.AgentFiles)`; warns when `agent_files` is disabled

## Service command (cmd/service.go)

- Usable as `ai-shell service` (foreground), `service --stop`, or `service --status`
- grpc-go server on unix socket `~/.ai-shell/service.sock`; stale socket removed when no live service answers `Ping`
- Sessions route through the service when `service.IsActive()` (shell, custom commands, commit); `cmd/service_helpers.go` provides `chatRequestFromConfig` and `chatWithServiceFallback` (falls back to local on `service.ErrUnavailable`)

## CI workflows (.github/workflows)

- `ci.yml`: runs format/vet/build/test

## Key gotchas

- `config.LoadConfig()` may return partial defaults on error — check both return values
- `.env` files loaded at startup via `gotenv.Load()` — place API keys there, not in config.yaml
- KV store at `~/.config/ai-shell/kv_store.db`; usage stats at `~/.config/ai-shell/usage.db`
- System prompts are Go constants in `llm/prompt.go`, copied to `~/.ai-shell/BUILDPROMPT.md`/`PLANPROMPT.md` on first run — always read from there, never local file
- 100 char soft line limit, Go 1.26.0+
- Llamacpp: `model` config is the GGUF filename without extension (`.gguf` appended as fallback). Run `make install-yzma` for libs; place GGUFs in `~/.ai-shell/models/llamacpp/`. Structured output uses a GBNF grammar (`jsonSchemaToGBNF`, `llm/gbnf.go`) with `llama.SamplerInitGrammar`. Image input uses yzma's `pkg/mtmd` (`setupVision`, `buildVisionPrompt`/`generateVision`) when a vision projector (`config.FindLlamacppMMProj()`) is present; init failures degrade to text-only. mmproj files are excluded from `config.GetLlamacppModels()`. Audio input is not supported.
- LitertLM: loads libs from `$LITERTLM_LIB` (default `~/.ai-shell/lib`) and `.litertlm` model via `$LITERTLM_MODEL` or scan of `$LITERTLM_MODELS_DIR`. Binding dlopens fixed filenames: `libGemmaModelConstraintProvider.so` + `liblitertlm_c_cpu.so` (cpu) / `liblitertlm_c.so` (gpu) — NOT `liblitert-lm.so`. Run `make install-litertlm` to fetch them. Backend from config `litertlm.backend` (default `cpu`), overridable via `$LITERTLM_BACKEND`. A single client is cached per (lib dir, model path, backend). Tools are manual-dispatch `RawTool`s (max 5 hops); multimodal uses `SendMulti`; `CallStructured` prompt-engines the schema.
- `/models` menu scans `config.GetLlamacppModels()` / `config.GetLitertLMModels()`; `config.IsLlamacppModel()` / `IsLitertLMModel()` used by `SaveModelWithProvider` for provider auto-detection.
- Service: `service.IsActive()` pings the socket; `ServiceExecutor` denies non-allowed `RunCommand`s and all `WriteFile` when `confirm` is true; `extract` never routes through the service. Wire types regenerate via `make proto` (not needed to build).
- Shared `~/.ai-shell` dirs resolved via `config.AiShellDir()`, `config.LibDir()`, `config.ModelsDir(provider)`. Helpers: `config.GetCommandName(cmd)`, `config.FormatFileSize(b)`.
