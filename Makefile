.PHONY: generate

# Cross-platform RPC code generation / 跨平台 RPC 代码生成
#   make generate
#   make generate SERVICE=auth

ifeq ($(OS),Windows_NT)
    GENERATE_CMD = powershell -NoProfile -ExecutionPolicy Bypass -File scripts/generate.ps1
    ifdef SERVICE
        GENERATE_ARGS = -Service $(SERVICE)
    endif
else
    GENERATE_CMD = ./scripts/generate.sh
    GENERATE_ARGS = $(SERVICE)
endif

generate:
	$(GENERATE_CMD) $(GENERATE_ARGS)
