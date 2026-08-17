# AGENTS.md

## Project

Interactive AI shell TUI (Bubbletea) with 5 LLM providers: Ollama, Gemini, OpenRouter, LitertLM, Llamacpp. OpenAI-compatible HTTP providers (Ollama, Gemini, OpenRouter) share `OpenAICaller` (`llm/openai.go`) via `NewProviderCaller` factory (`llm/providers.go`). Two in-process providers run no HTTP server: `llamacpp` uses `LlamacppCaller` (`llm/llamacpp.go`) with the `yzma` Go binding (`github.com/hybridgroup/yzma`), and `litertlm` uses `LitertLMCaller` (`llm/litertlm.go`) with the `litertlm-go` binding (`github.com/vladimirvivien/litertlm-go`) loading the LiteRT-LM shared libs + `.litertlm` model from disk.

Entry point: `main.go` → `cmd.Execute()`. Default command launches Bubbletea TUI in `cmd/shell.go` (MVU pattern).

## Commands

```
make build         # go build -o ai-shell .
make install       # go install .
make install-yzma  # install yzma CLI + llama.cpp libraries to ~/.ai-shell/lib
make install-litertlm  # download liblitert-lm.so from the official LiteRT-LM release (litert_lm_c_api zip, LITERTLM_VERSION default v0.16.0), rename to liblitertlm_c_cpu.so into ~/.ai-shell/lib + fetch LiteRT-LM prebuilt aux/GPU libs
make coverage      # test + HTML report
go fmt ./... && go vet ./... && go build -o ai-shell . && go test ./...
```

## Packages

| Directory | Contents |
|-----------|----------|
| `cmd/` | Cobra commands: default (TUI shell), `config`, `commit`, `extract`, `pull`, `models`, `commands`, `agents`, `stats` |
| `config/` | Viper YAML config, model lists (OpenRouter free models fetched live from `https://openrouter.ai/api/v1/models` via `config.GetOpenRouterModels()`, 10 min cache), `.env` loading via `gotenv` |
| `llm/` | `Agent` struct, `Caller` interface, `RawCaller` interface (adds `CallStructured`), `ToolExecutor` interface, 6 OpenAI tool definitions, `NewProviderCaller` factory, `NewProviderCallerRaw` (returns `RawCaller`), `CallStructured` method (with `response_format`), `ProviderConfig` struct, default system prompt, `LlamacppCaller` (in-process llama.cpp via yzma), `LitertLMCaller` (in-process LiteRT-LM via litertlm-go). Built-in agents: `GetAgentDefs()`/`GetAgentDef(name)` return `AgentDef`s (`build` = all tools + default prompt, `plan` = no `WriteFile`/`RunCommand` with its own planning prompt); `NewAgentFor(name, model, provider, cfgTools)` builds an `Agent` whose tools are the intersection of the agent's allowed tools and the user's tool toggles. `OpenAICaller` logs token usage (`openrouter usage` debug line: prompt/completion/total tokens, cost, cached/reasoning tokens when present) but only when `BaseURL` contains `openrouter.ai`; every OpenAI-compatible call with a `usage` in the response is also persisted via `stats.RecordUsage` (provider derived from `BaseURL`). `LlamacppCaller` records prompt/completion tokens (from tokenize + generation loop). LitertLM usage is not tracked (binding exposes no token counts) |
| `tools/` | `RunCommand` (bash -c), `ReadFile`, `WriteFile`, KV store (bbolt), `GetDistro`, `GetShell` |
| `stats/` | Persistent token usage store backed by bbolt at `~/.config/ai-shell/usage.db`. `stats.RecordUsage(provider, model, Usage)` accumulates per (provider, model); `stats.GetStats()` returns sorted `[]Entry` (calls, prompt/completion/total/cached/reasoning tokens, cost); `stats.Reset()` wipes all entries. `dbPathFunc` is swappable for tests |

## Config layering

1. `~/.config/ai-shell/.env` (global)
2. `./.env` (local overrides)
3. `config.yaml` from `./` or `~/.config/ai-shell/`

`.env.example` documents the recognized env vars (`GEMINI_API_KEY`, `OPEN_ROUTE_KEY`, `OLLAMA_HOST`, `LITERTLM_LIB`, `LITERTLM_MODEL`, `LITERTLM_MODELS_DIR`, `LITERTLM_BACKEND`).

Defaults: provider=ollama, model=granite4:3b-h, log_level=info, confirm=true, allowed_commands=ls,pwd, agent=build. litertlm backend defaults to `cpu`. All 6 tools enabled by default. Custom commands stored in config.

Log level values: `debug`, `info`, `warn`, `error`. Uses `log/slog` throughout (no `log` package). Call `config.InitLogger(cfg.LogLevel)` after `LoadConfig()` to configure the global slog level. Debug messages use `slog.Debug`, warnings use `slog.Warn`. Debug logging in `cmd/commit.go` (provider/model/prompt) and `cmd/shell.go` (LLM timing: total/llm/other duration and message count).

## Testing

```bash
go test ./...           # all pass (config, llm, tools, stats, cmd)
go test -v -run TestName ./package
go test -cover ./...
```

Test conventions: table-driven tests, `t.Run()` subtests, function variable mocking (e.g. `userConfigDirFunc`, `loadEnvFunc` in `config/`, `dbPathFunc` in `tools/`).

`cmd/commit_test.go` has integration tests for the commit flow (ollama provider) using an httptest server to mock the LLM API.

## TUI conventions (cmd/shell.go)

- `/` commands: help, get-config, config, models, agent, reset, add-cmd, exit, quit
- `@filepath` for image attachments (base64 encoded)
- Tool execution requires user confirmation unless command is in `allowed_commands`
- `/agent` opens an agent selection menu (`menuAgent`); `m.cfg.Agent` names the active agent and is persisted to config. `ElaborateMessage` builds the agent via `llm.NewAgentFor(m.cfg.Agent, ...)`, so the agent's allowed tools (e.g. plan: no WriteFile/RunCommand) are intersected with the user's tool toggles before calling the LLM. `showConfig` marks tools `blocked by agent`
- Lipgloss styles: `promptStyle`, `systemStyle`, `userStyle`, `aiStyle`, `errorStyle`, `cmdStyle`, `helpStyle`, `dimStyle`

## Debug flag (cmd/cmd.go)

- Persistent `--debug` flag on the root command, available to every subcommand (e.g. `ai-shell commit --debug`, `ai-shell models --debug`)
- `cmd.initLogger(cfg)` temporarily forces the slog level to debug when the flag is passed, without touching the saved config — it mutates only the in-memory `cfg.LogLevel` before calling `config.InitLogger`
- All subcommands must call `initLogger(cfg)` (not `config.InitLogger(cfg.LogLevel)`) after `config.LoadConfig()` so the flag override applies

## Config command (cmd/config.go)

- Usable as `ai-shell config` (shows current config via `PrintConfig`) or `ai-shell config --flag value`
- Flags: `--provider`, `--model`, `--agent`, `--log-level`, `--confirm`, `--allowed-commands`, `--backend`, `--enable-tool`, `--disable-tool`, `--add-cmd`, `--rm-cmd`
- Loads config via `config.LoadConfig()`, modifies specified fields, calls `config.SaveConfig()`
- When run without any flags, calls `PrintConfig()` (replaces the removed `get-config` command)
- When `--model` is set without `--provider`, auto-detects provider via `config.LookupModelInfo`
- `--add-cmd` uses `name=prompt` format
- `SaveConfig` now persists `commands` map to YAML

## Commands command (cmd/commands.go)

- Usable as `ai-shell commands` (lists custom commands) or `ai-shell commands --run <name> [args...]` to run one non-interactively
- `--run` looks the command up via `config.LoadCommands` (merges `.ai-shell/commands/*.md` files and config `commands` map, matching the shell's `/name` handling); extra args are appended to the command prompt
- Running uses `llm.NewAgentFor(cfg.Agent, ...)` with the agent's system prompt and a `cliExecutor` (in `cmd/commands.go`) that executes all tools without confirmation, prints the final assistant text to stdout

## Commit command (cmd/commit.go)

- Usable as `ai-shell commit` or via `go run . commit`
- Flag `-A` / `--all` stages all changes (`git add -A`) before committing
- Flag `-d` / `--dry-run` prints the commit message without committing; if used with `-A`, files are unstaged (`git reset`) after the dry run
- Runs `git log --oneline -5` and `git diff --cached`, sends both as context to the LLM
- Uses `llm.NewProviderCaller` directly (not through `Agent`) with a noop executor and no tools
- Strips markdown code fences from the LLM response
- Writes the message to a temp file and runs `git commit -F <file>`

## Extract command (cmd/extract.go)

- Usable as `ai-shell extract <input> <schema>`
- Two positional args: input file (.txt/.md/.pdf) and JSON schema file
- Flag `-o` / `--output` writes result to a file (default: stdout)
- Reads text from input (uses `pdftotext` for PDF)
- Calls LLM with `response_format: json_schema` for structured output
- Uses `llm.NewProviderCallerRaw` to get `RawCaller` and calls `CallStructured`

## Pull command (cmd/pull.go)

- Usable as `ai-shell pull <repo> <filename>`
- Downloads a model from HuggingFace
- Destination chosen by filename extension: `.litertlm` → `~/.ai-shell/models/litertlm/` + litertlm provider; otherwise `.gguf` → `~/.ai-shell/models/llamacpp/` + llamacpp provider
- Shows progress bar with percentage and bytes downloaded
- Auto-updates config via `config.SaveModelWithProvider(modelName, provider)`
- Cleans up partial file on download failure
- Uses `net/http` directly (no external download libraries)

## Agents command (cmd/agents.go)

- Usable as `ai-shell agents`
- Lists all agents from `llm.GetAgentDefs()` in a table: AGENT, DESCRIPTION, TOOLS
- Sorted by agent name; uses `text/tabwriter` for aligned output
- Current agent (from config `agent` field) is prefixed with `* `

## Models command (cmd/models.go)

- Usable as `ai-shell models`
- Lists all available models from all providers in a table: Model, Provider, Size
- Sorted by provider then model name
- For llamacpp models, shows the GGUF file size (from `config.GetLlamacppModels()`, now populates `Size` via `entry.Info()`)
- Uses `text/tabwriter` for aligned output
- Current model is prefixed with `* `

## Stats command (cmd/stats.go)

- Usable as `ai-shell stats`
- Prints a table of aggregated token usage per provider and model: CALLS, INPUT (prompt), OUTPUT (completion), CACHED, REASONING, TOTAL, COST, plus a TOTAL row
- Data comes from `stats.GetStats()` (bbolt at `~/.config/ai-shell/usage.db`)
- Flag `--reset` clears all recorded usage (idempotent — no-op when empty)
- Usage is recorded automatically: `OpenAICaller` records every OpenAI-compatible response that carries a `usage` (ollama/gemini/openrouter, provider derived from `BaseURL`); `LlamacppCaller` records prompt tokens (from tokenize) and completion tokens (from the generation loop). LitertLM usage is not tracked.

## CI workflows (.github/workflows)

- `ci.yml`: runs format/vet/build/test

## Key gotchas

- `config.LoadConfig()` may return partial defaults on error — check both return values
- `.env` files are loaded at startup via `gotenv.Load()` — place API keys there, not in config.yaml
- KV store uses bbolt at `~/.config/ai-shell/kv_store.db`
- Usage stats use bbolt at `~/.config/ai-shell/usage.db` (recorded by every LLM call that reports usage)
- `config.InitLogger(cfg.LogLevel)` must be called after each `config.LoadConfig()` to configure the global slog level — it's already called in shell/commit/extract commands
- System prompts are Go constants in `llm/prompt.go` (`BuildPrompt`, `PlanPrompt`), copied to `~/.ai-shell/BUILDPROMPT.md` and `~/.ai-shell/PLANPROMPT.md` on first run — always read from there, never from local file
- 100 char soft line limit, Go 1.26.0+
- Llamacpp provider (`llm/llamacpp.go`) loads the llama.cpp shared library from `~/.ai-shell/lib/` and GGUF models from `~/.ai-shell/models/llamacpp/`. The `model` config field should be the GGUF filename without extension (e.g. `granite4-3b-h.Q4_K_M`); the provider appends `.gguf` as fallback. Run `make install-yzma` to download the llama.cpp shared libraries to `~/.ai-shell/lib/`. Then place GGUF model files in `~/.ai-shell/models/llamacpp/`. Structured output (`extract` command) is not supported for this provider.
- LitertLM provider (`llm/litertlm.go`) loads the LiteRT-LM shared libraries from `$LITERTLM_LIB` (default `~/.ai-shell/lib`) and a `.litertlm` model via `$LITERTLM_MODEL` or by scanning `$LITERTLM_MODELS_DIR` (default `~/.ai-shell/models/litertlm/`). The binding dlopens fixed filenames: `libGemmaModelConstraintProvider.so` (required) plus the main C-API lib `liblitertlm_c_cpu.so` (cpu) / `liblitertlm_c.so` (gpu) — it does NOT look for `liblitert-lm.so`. Run `make install-litertlm` to fetch the main C-API lib from the official LiteRT-LM release (renamed from `liblitert-lm.so` to `liblitertlm_c_cpu.so`) plus the prebuilt aux/GPU libs from the LiteRT-LM repo; no symlink is needed. The `model` config field should match the `.litertlm` filename without extension; the provider appends `.litertlm` as fallback. Backend comes from config `litertlm.backend` (`cpu`/`gpu`, default `cpu`), overridable via `$LITERTLM_BACKEND`. A single `litertlm.Client` is cached per (lib dir, model path, backend) — the engine loads at most once per process. `litertlm.SetMinLogLevel(litertlm.LogQuiet)` is set before engine init to silence the C-side NPU/GPU registry, absl, and loader warnings that otherwise flood stderr (LogWarning is too chatty). Tools are manual-dispatch `RawTool`s (max 5 hops); multimodal images/audio use `SendMulti`. `CallStructured` (extract) prompt-engines the JSON schema instead of using `GenerateData[T]` (that path needs a compile-time struct).
- `/models` menu lists llamacpp models by scanning `~/.ai-shell/models/llamacpp/` for `.gguf` files via `config.GetLlamacppModels()`, and litertlm models by scanning the model dir for `.litertlm` files via `config.GetLitertLMModels()`. The `config.IsLlamacppModel()` / `config.IsLitertLMModel()` functions are used by `SaveModelWithProvider` for auto-detection when no provider is explicitly set.
