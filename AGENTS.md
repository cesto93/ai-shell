# AGENTS.md

## Project

Interactive AI shell TUI (Bubbletea) with 5 LLM providers: Ollama, Gemini, OpenRouter, LitertLM, Llamacpp. OpenAI-compatible providers (Ollama, Gemini, OpenRouter, LiteRT-LM) share `OpenAICaller` (`llm/openai.go`) via `NewProviderCaller` factory (`llm/providers.go`). The `llamacpp` provider uses `LlamacppCaller` (`llm/llamacpp.go`) with the `yzma` Go binding (`github.com/hybridgroup/yzma`) for in-process llama.cpp inference — no HTTP server required.

Entry point: `main.go` → `cmd.Execute()`. Default command launches Bubbletea TUI in `cmd/shell.go` (MVU pattern). When the `litertlm` provider is configured, `cmd/litertlm_service.go` auto-starts `litert-lm serve --port <port> --api openai` as a background process and health-checks it before the TUI begins.

## Commands

```
make build         # go build -o ai-shell .
make install       # go install .
make install-yzma  # install yzma CLI + llama.cpp libraries to ~/.ai-shell/lib
make coverage      # test + HTML report
go fmt ./... && go vet ./... && go build -o ai-shell . && go test ./...
```

## Packages

| Directory | Contents |
|-----------|----------|
| `cmd/` | Cobra commands: default (TUI shell), `get-config`, `config`, `commit`, `extract`, `pull`, `models` |
| `config/` | Viper YAML config, model lists, `.env` loading via `gotenv` |
| `llm/` | `Agent` struct, `Caller` interface, `RawCaller` interface (adds `CallStructured`), `ToolExecutor` interface, 6 OpenAI tool definitions, `NewProviderCaller` factory, `NewProviderCallerRaw` (returns `RawCaller`), `CallStructured` method (with `response_format`), `ProviderConfig` struct, default system prompt, `LlamacppCaller` (in-process llama.cpp via yzma) |
| `tools/` | `RunCommand` (bash -c), `ReadFile`, `WriteFile`, KV store (bbolt), `GetDistro`, `GetShell` |

## Config layering

1. `~/.config/ai-shell/.env` (global)
2. `./.env` (local overrides)
3. `config.yaml` from `./` or `~/.config/ai-shell/`

Defaults: provider=ollama, model=granite4:3b-h, log_level=info, confirm=true, allowed_commands=ls,pwd. All 6 tools enabled by default. Custom commands stored in config.

Log level values: `debug`, `info`, `warn`, `error`. Uses `log/slog` throughout (no `log` package). Call `config.InitLogger(cfg.LogLevel)` after `LoadConfig()` to configure the global slog level. Debug messages use `slog.Debug`, warnings use `slog.Warn`. Debug logging in `cmd/commit.go` (provider/model/prompt) and `cmd/shell.go` (LLM timing: total/llm/other duration and message count).

## Testing

```bash
go test ./...           # all pass (config, llm, tools, cmd)
go test -v -run TestName ./package
go test -cover ./...
```

Test conventions: table-driven tests, `t.Run()` subtests, function variable mocking (e.g. `userConfigDirFunc`, `loadEnvFunc` in `config/`, `dbPathFunc` in `tools/`).

`cmd/commit_test.go` has integration tests for the commit flow (both litertlm and ollama providers) using an httptest server to mock the LLM API.

## TUI conventions (cmd/shell.go)

- `/` commands: help, get-config, config, models, reset, add-cmd, exit, quit
- `@filepath` for image attachments (base64 encoded)
- Tool execution requires user confirmation unless command is in `allowed_commands`
- Lipgloss styles: `promptStyle`, `systemStyle`, `userStyle`, `aiStyle`, `errorStyle`, `cmdStyle`, `helpStyle`, `dimStyle`

## Config command (cmd/config.go)

- Usable as `ai-shell config --flag value`
- Flags: `--provider`, `--model`, `--log-level`, `--confirm`, `--allowed-commands`, `--auto-start`, `--enable-tool`, `--disable-tool`, `--add-cmd`, `--rm-cmd`
- Loads config via `config.LoadConfig()`, modifies specified fields, calls `config.SaveConfig()`
- When `--model` is set without `--provider`, auto-detects provider via `config.LookupModelInfo`
- `--add-cmd` uses `name=prompt` format
- `SaveConfig` now persists `commands` map to YAML

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
- Downloads a GGUF model from HuggingFace to `~/.ai-shell/models/llamacpp/`
- Shows progress bar with percentage and bytes downloaded
- Auto-updates config via `config.SaveModelWithProvider(modelName, "llamacpp")`
- Cleans up partial file on download failure
- Uses `net/http` directly (no external download libraries)

## Models command (cmd/models.go)

- Usable as `ai-shell models`
- Lists all available models from all providers in a table: Model, Provider, Size
- Sorted by provider then model name
- For llamacpp models, shows the GGUF file size (from `config.GetLlamacppModels()`, now populates `Size` via `entry.Info()`)
- Uses `text/tabwriter` for aligned output
- Current model is prefixed with `* `

## CI workflows (.github/workflows)

- `ci.yml`: runs format/vet/build/test
- `litertlm.yaml` (manual, `workflow_dispatch`): builds LiteRT-LM `//c:litert-lm` with Bazel and uploads `liblitert-lm.so`. Already builds Linux x86-64 only (job runs on `ubuntu-latest`; target/linkopts are Linux-specific). Speed levers, highest impact first: (1) `--config=public_cache` pulls prebuilt artifacts from LiteRT-LM's public read-only remote cache (`storage.googleapis.com/litert-bazel-artifacts`); (2) `setup-bazel` `bazelisk-cache: true`, `disk-cache: ${{ github.workflow }}`, `repository-cache: true`; (3) `--distdir` avoids the unreliable zlib.net download, with the tarball restored/saved via `actions/cache`. When changing the zlib version, bump the cache `key` and the tarball's sha256 checksum. The checkout uses `lfs: true` because prebuilt `.so` files are Git LFS pointers — without it the linker fails with `unknown directive: version`.

## Key gotchas

- `config.LoadConfig()` may return partial defaults on error — check both return values
- `.env` files are loaded at startup via `gotenv.Load()` — place API keys there, not in config.yaml
- KV store uses bbolt at `~/.config/ai-shell/kv_store.db`
- `config.InitLogger(cfg.LogLevel)` must be called after each `config.LoadConfig()` to configure the global slog level — it's already called in shell/commit/extract commands
- System prompt is a Go constant in `llm/prompt.go`, copied to `~/.ai-shell/PROMPT.md` on first run — always read from there, never from local file
- 100 char soft line limit, Go 1.26.0+
- Llamacpp provider (`llm/llamacpp.go`) loads the llama.cpp shared library from `~/.ai-shell/lib/` and GGUF models from `~/.ai-shell/models/llamacpp/`. The `model` config field should be the GGUF filename without extension (e.g. `granite4-3b-h.Q4_K_M`); the provider appends `.gguf` as fallback. Run `make install-yzma` to download the llama.cpp shared libraries to `~/.ai-shell/lib/`. Then place GGUF model files in `~/.ai-shell/models/llamacpp/`. Structured output (`extract` command) is not supported for this provider.
- `/models` menu lists llamacpp models by scanning `~/.ai-shell/models/llamacpp/` for `.gguf` files via `config.GetLlamacppModels()`. The `config.IsLlamacppModel()` function is used by `SaveModelWithProvider` for auto-detection when no provider is explicitly set.
