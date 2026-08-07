SHELL := /bin/bash
AI_SHELL_DIR := $(HOME)/.ai-shell
AI_SHELL_LIB := $(AI_SHELL_DIR)/lib
LITERTLM_TAG ?= main
LITERTLM_PREBUILT := https://github.com/google-ai-edge/LiteRT-LM/raw/$(LITERTLM_TAG)/prebuilt/linux_x86_64
LITERTLM_AUX_LIBS := libGemmaModelConstraintProvider.so libLiteRt.so libLiteRtWebGpuAccelerator.so libLiteRtTopKWebGpuSampler.so
LITERTLM_ARTIFACT_OWNER ?= cesto93
LITERTLM_ARTIFACT_REPO ?= ai-shell
LITERTLM_ARTIFACT_RUN ?= 31204830010
LITERTLM_ARTIFACT_NAME ?= litert-lm-linux-amd64
LITERTLM_ARTIFACT_URL ?= https://nightly.link/$(LITERTLM_ARTIFACT_OWNER)/$(LITERTLM_ARTIFACT_REPO)/actions/runs/$(LITERTLM_ARTIFACT_RUN)/$(LITERTLM_ARTIFACT_NAME).zip

build:
	go build -o ai-shell .
install:
	go install .
install-yzma:
	go install github.com/hybridgroup/yzma@latest
	mkdir -p $(AI_SHELL_LIB)
	yzma install --lib $(AI_SHELL_LIB)
	@echo ""
	@echo "✓ yzma + llama.cpp libraries installed to $(AI_SHELL_LIB)"
	@echo "  Set YZMA_LIB=$(AI_SHELL_LIB) in your shell rc file if needed."
install-litertlm:
	mkdir -p $(AI_SHELL_LIB)
	@if [ ! -f "$(AI_SHELL_LIB)/liblitertlm_c_cpu.so" ]; then \
		echo "Downloading liblitertlm_c_cpu.so from $(LITERTLM_ARTIFACT_URL) ..."; \
		tmp=$$(mktemp -d); \
		curl -fsSL "$(LITERTLM_ARTIFACT_URL)" -o "$$tmp/$(LITERTLM_ARTIFACT_NAME).zip" || { echo "✗ failed to download $(LITERTLM_ARTIFACT_NAME)"; exit 1; }; \
		unzip -jo "$$tmp/$(LITERTLM_ARTIFACT_NAME).zip" "liblitertlm_c_cpu.so" -d "$(AI_SHELL_LIB)" || { echo "✗ failed to extract liblitertlm_c_cpu.so"; exit 1; }; \
		rm -rf "$$tmp"; \
	fi
	@for lib in $(LITERTLM_AUX_LIBS); do \
		if [ ! -f "$(AI_SHELL_LIB)/$$lib" ]; then \
			echo "Downloading $$lib ..."; \
			curl -fsSL "$(LITERTLM_PREBUILT)/$$lib" -o "$(AI_SHELL_LIB)/$$lib" || { echo "✗ failed to download $$lib"; exit 1; }; \
		fi; \
	done
	@echo ""
	@echo "✓ LiteRT-LM libraries installed to $(AI_SHELL_LIB)"
	@echo "  Model files (.litertlm) go in $(AI_SHELL_DIR)/models/litertlm/"
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
integration:
	go test -tags=integration -v ./cmd/
