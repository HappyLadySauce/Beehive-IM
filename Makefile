# Beehive-IM development Makefile / Beehive-IM 开发 Makefile
#
# Usage / 用法:
#   make
#   make check
#   make build
#   make build SERVICE=edge
#   make run SERVICE=gateway
#   make generate SERVICE=auth
#   make migrate MODE=adaptive

GO           := go
GOFLAGS      ?=
GOLANGCI_LINT ?= golangci-lint
BIN_DIR      := bin
SERVICES     := auth user presence conversation message notification gateway edge
COMPOSE_FILE := docker/Infrastructure/docker-compose.yaml
COMPOSE      := docker compose -f $(COMPOSE_FILE)

.DEFAULT_GOAL := help

ifeq ($(OS),Windows_NT)
    GENERATE_CMD = powershell -NoProfile -ExecutionPolicy Bypass -File scripts/generate.ps1
    ifdef SERVICE
        GENERATE_ARGS = -Service $(SERVICE)
    endif

    MIGRATE_CMD = powershell -NoProfile -ExecutionPolicy Bypass -File sql/migrate.ps1
    ifndef MODE
        MODE = versioned
    endif
    MIGRATE_ARGS = -Mode $(MODE)
    ifdef DB_DSN
        MIGRATE_ARGS += -Dsn $(DB_DSN)
    endif
    ifeq ($(MIGRATION_FORCE),1)
        MIGRATE_ARGS += -Force
    endif
    ifeq ($(MIGRATION_REAPPLY),1)
        MIGRATE_ARGS += -Reapply
    endif
    ifeq ($(VERBOSE),1)
        MIGRATE_ARGS += -Verbose
    endif
else
    GENERATE_CMD = ./scripts/generate.sh
    GENERATE_ARGS = $(SERVICE)

    MIGRATE_CMD = ./sql/migrate.sh
    export MODE ?= versioned
    ifdef DB_DSN
        export DB_DSN
    endif
    ifdef MIGRATION_FORCE
        export MIGRATION_FORCE
    endif
    ifdef MIGRATION_REAPPLY
        export MIGRATION_REAPPLY
    endif
    ifdef VERBOSE
        export VERBOSE
    endif
endif

.PHONY: help check generate tidy fmt vet lint test test-v test-race \
        build build-all run migrate migrate-adaptive \
        infra-up infra-down infra-ps

## help: Show available targets
ifeq ($(OS),Windows_NT)
help:
	@powershell -NoProfile -Command "$$pat='^## ([a-zA-Z0-9_-]+): (.+)$$'; Get-Content 'Makefile' | ForEach-Object { if ($$_ -match $$pat) { [PSCustomObject]@{Target=$$Matches[1]; Desc=$$Matches[2]} } } | Sort-Object Target | ForEach-Object { Write-Host ('  {0,-18} {1}' -f $$_.Target, $$_.Desc) }"
else
help:
	@awk '/^## [a-zA-Z0-9_-]+:/ { line=$$0; sub(/^## /, "", line); split(line, parts, ": "); printf "  \033[36m%-18s\033[0m %s\n", parts[1], parts[2] }' Makefile | sort
endif

## check: Run fmt, vet, lint, and tests
check: fmt vet lint test

## generate: Generate RPC code from proto (optional SERVICE=auth)
generate:
	$(GENERATE_CMD) $(GENERATE_ARGS)

## tidy: Tidy go.mod and go.sum
tidy:
	$(GO) mod tidy

## fmt: Format all Go source files
fmt:
	$(GO) fmt ./...

## vet: Run go vet on all packages
vet:
	$(GO) vet ./...

## lint: Run golangci-lint on all packages
lint:
	$(GOLANGCI_LINT) run ./...

## test: Run all tests
test:
	$(GO) test $(GOFLAGS) ./...

## test-v: Run all tests with verbose output
test-v:
	$(GO) test -v $(GOFLAGS) ./...

## test-race: Run all tests with the race detector
test-race:
	$(GO) test -race $(GOFLAGS) ./...

## build: Build service binaries into bin/ (optional SERVICE=edge)
build:
ifdef SERVICE
	@$(MAKE) $(BIN_DIR)/beehiveim-$(SERVICE)
else
	@$(MAKE) build-all
endif

build-all: $(addprefix $(BIN_DIR)/beehiveim-,$(SERVICES))

$(BIN_DIR)/beehiveim-%:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./services/$*

## run: Run one service (SERVICE=edge)
run:
ifndef SERVICE
	$(error SERVICE is required, e.g. make run SERVICE=edge)
endif
	$(GO) run $(GOFLAGS) ./services/$(SERVICE) -f ./services/$(SERVICE)/etc/beehiveim.$(SERVICE).yaml

## migrate: Run database migrations (MODE=versioned|adaptive, optional DB_DSN)
migrate:
	$(MIGRATE_CMD) $(MIGRATE_ARGS)

## migrate-adaptive: Run migrations in adaptive mode
migrate-adaptive:
	$(MAKE) migrate MODE=adaptive

## infra-up: Start local infrastructure (postgres, redis, etcd, rabbitmq)
infra-up:
	$(COMPOSE) up -d

## infra-down: Stop local infrastructure
infra-down:
	$(COMPOSE) down

## infra-ps: Show local infrastructure status
infra-ps:
	$(COMPOSE) ps
