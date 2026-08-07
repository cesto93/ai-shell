SHELL := /bin/bash
AI_SHELL_DIR := $(HOME)/.ai-shell
AI_SHELL_LIB := $(AI_SHELL_DIR)/lib
LITERTLM_TAG ?= main
LITERTLM_PREBUILT := https://github.com/google-ai-edge/LiteRT-LM/raw/$(LITERTLM_TAG)/prebuilt/linux_x86_64
LITERTLM_AUX_LIBS := libGemmaModelConstraintProvider.so libLiteRt.so libLiteRtWebGpuAccelerator.so libLiteRtTopKWebGpuSampler.so

build:
	go build -o ai-shell .
install: install-yzma
	go install .
install-yzma:
	go install github.com/hybridgroup/yzma@latest
	mkdir -p $(AI_SHELL_LIB)
	yzma install --lib $(AI_SHELL_LIB)
	@echo ""
	@echo "✓ yzma + llama.cpp libraries installed to $(AI_SHELL_LIB)"
	@echo "  Set YZMA_LIB=$(AI_SHELL_LIB) in your shell rc file if needed."
install-litertlm:
	@if [ ! -f "$(AI_SHELL_LIB)/liblitert-lm.so" ]; then \
		echo "✗ $(AI_SHELL_LIB)/liblitert-lm.so not found."; \
		echo "  Build it first via the 'Build LiteRT-LM Shared Libraries' workflow (.github/workflows/litertlm.yaml)"; \
		echo "  and place the downloaded artifact (liblitert-lm.so) into $(AI_SHELL_LIB)/."; \
		exit 1; \
	fi
	mkdir -p $(AI_SHELL_LIB)
	@for lib in $(LITERTLM_AUX_LIBS); do \
		if [ ! -f "$(AI_SHELL_LIB)/$$lib" ]; then \
			echo "Downloading $$lib ..."; \
			curl -fsSL "$(LITERTLM_PREBUILT)/$$lib" -o "$(AI_SHELL_LIB)/$$lib" || { echo "✗ failed to download $$lib"; exit 1; }; \
		fi; \
	done
	ln -sfn liblitert-lm.so $(AI_SHELL_LIB)/liblitertlm_c_cpu.so
	@echo ""
	@echo "✓ LiteRT-LM libraries installed to $(AI_SHELL_LIB)"
	@echo "  Model files (.litertlm) go in $(AI_SHELL_DIR)/models/litertlm/"
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
integration:
	go test -tags=integration -v ./cmd/
