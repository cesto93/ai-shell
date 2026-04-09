# AGENTS.md

This document provides guidelines for agents working on the ai-shell codebase.

## Project Overview

AI-Shell is an interactive CLI tool powered by Ollama (local LLM) that helps users with shell commands. Built with Go, it uses Cobra for CLI structure, Viper for config management, and the Ollama API for LLM interactions.

## Build Commands

```bash
# Build the binary
make build
# or: go build -o ai-shell .

# Install to $GOPATH/bin
make install
# or: go install .

# Run the application
./ai-shell

# Format code
go fmt ./...

# Vet code
go vet ./...

# Build and verify
go build -o ai-shell . && ./ai-shell --help
```

## Testing

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run specific test
go test -v -run TestFunctionName ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detector
go test -race ./...
```

When adding tests:
- Create `*_test.go` files in the same package
- Use table-driven tests for multiple test cases
- Use `t.Run()` for subtests

## Code Style

### General Guidelines

- Run `go fmt ./...` before committing
- Run `go vet ./...` to catch common issues
- Use Go 1.25+ features (project requires Go 1.25.5+)
- Maximum line length: 100 characters (soft limit)

### Naming Conventions

| Type | Convention | Example |
|------|------------|---------|
| Packages | lowercase, short | `llm`, `tools` |
| Functions | PascalCase exported, camelCase private | `CallOllama`, `getDistro` |
| Variables | camelCase | `configPath`, `shellPrompt` |
| Constants | PascalCase for exported, camelCase for private | `ColorGreen`, `defaultModel` |
| Interfaces | PascalCase, often with "er" suffix | (none currently) |
| Error variables | `err` prefix or descriptive | `err`, `configNotFound` |

### File Organization

```
main.go       - Entry point, delegates to cmd package
cmd/
    cmd.go           - Cobra command definitions
    interactive.go   - Interactive shell loop, LLM interaction
    completion.go    - Readline autocomplete
config/
    config.go        - Config loading, model management
tools/
    tools.go         - Shell command execution
    system.go        - System info (distro, shell detection)
```

### Imports

Standard library imports first, then third-party (alphabetically):

```go
import (
    "bufio"
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/chzyer/readline"
    "github.com/ollama/ollama/api"
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)
```

### Error Handling

Always wrap errors with context using `fmt.Errorf`:

```go
// Good
return nil, fmt.Errorf("error reading config: %w", err)

// Bad - no context
return nil, err
```

Use early returns for error conditions:

```go
func LoadConfig() (*Config, error) {
    cfg, err := doSomething()
    if err != nil {
        return nil, fmt.Errorf("LoadConfig: %w", err)
    }
    // continue
}
```

Handle errors gracefully in CLI (don't just log and continue silently unless appropriate).

### Context Usage

Pass `context.Context` as first parameter for operations that may be cancelled:

```go
func CallOllama(ctx context.Context, prompt string) error {
    // ...
}
```

### Configuration

Use Viper for config with mapstructure tags:

```go
type Config struct {
    LLM struct {
        Model string `mapstructure:"model"`
    } `mapstructure:"llm"`
}
```

Set sensible defaults with `viper.SetDefault()`.

### CLI Output

Use color constants defined in `cmd/interactive.go`:
- `ColorBlue` - LLM responses
- `ColorCyan` - System info, prompts
- `ColorGreen` - Commands, success
- `ColorYellow` - Warnings, errors
- `ColorBold` - Headings
- `ColorReset` - Reset formatting

### Performance Considerations

- Reuse `bytes.Buffer` for command output when appropriate
- Close resources (files, readline) with `defer`
- Avoid unnecessary allocations in hot paths

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/spf13/viper` | Config management |
| `github.com/ollama/ollama` | LLM API client |
| `github.com/chzyer/readline` | Interactive input |

## Git Workflow

- Commit messages: Imperative mood, 50 chars max subject
  - `Add file completion support`
  - `Fix config loading fallback`
- Branch naming: `feature/...`, `fix/...`, `refactor/...`
- PR title matches commit message style

## Quick Reference

```bash
# Full workflow
go fmt ./... && go vet ./... && go build -o ai-shell . && go test ./...
```
