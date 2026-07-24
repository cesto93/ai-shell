SHELL := /bin/bash
AI_SHELL_DIR := $(HOME)/.ai-shell

build:
	go build -o ai-shell .
install: install-yzma
	go install .
install-yzma:
	go install github.com/hybridgroup/yzma@latest
	mkdir -p $(AI_SHELL_DIR)
	yzma install --lib $(AI_SHELL_DIR)
	@echo ""
	@echo "✓ yzma + llama.cpp libraries installed to $(AI_SHELL_DIR)"
	@echo "  Set YZMA_LIB=$(AI_SHELL_DIR) in your shell rc file if needed."
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
integration:
	go test -tags=integration -v ./cmd/
