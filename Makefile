SHELL := /bin/bash
AI_SHELL_DIR := $(HOME)/.ai-shell
AI_SHELL_LIB := $(AI_SHELL_DIR)/lib
LITERTLM_TAG ?= main
LITERTLM_VERSION ?= v0.16.0
LITERTLM_PREBUILT := https://github.com/google-ai-edge/LiteRT-LM/raw/$(LITERTLM_TAG)/prebuilt/linux_x86_64
LITERTLM_AUX_LIBS := libGemmaModelConstraintProvider.so libLiteRt.so libLiteRtWebGpuAccelerator.so libLiteRtTopKWebGpuSampler.so
LITERTLM_RELEASE_URL ?= https://github.com/google-ai-edge/LiteRT-LM/releases/download/$(LITERTLM_VERSION)/litert_lm_c_api-0.1.0.zip

build:
	go build -o ai-shell .
install:
	go install .
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
	  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	  service/proto/service.proto
	@echo "Regenerated service/proto/*.pb.go (commit the generated files; protoc is only needed for regeneration)"
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
		echo "Downloading liblitert-lm.so from $(LITERTLM_RELEASE_URL) ..."; \
		tmp=$$(mktemp -d); \
		curl -fsSL "$(LITERTLM_RELEASE_URL)" -o "$$tmp/litert_lm_c_api.zip" || { echo "✗ failed to download LiteRT-LM release $(LITERTLM_VERSION)"; exit 1; }; \
		unzip -jo "$$tmp/litert_lm_c_api.zip" "lib/linux_x86_64/liblitert-lm.so" -d "$(AI_SHELL_LIB)" || { echo "✗ failed to extract liblitert-lm.so"; exit 1; }; \
		mv "$(AI_SHELL_LIB)/liblitert-lm.so" "$(AI_SHELL_LIB)/liblitertlm_c_cpu.so" || { echo "✗ failed to rename liblitert-lm.so"; exit 1; }; \
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
