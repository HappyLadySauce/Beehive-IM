GOHOSTOS := $(shell go env GOHOSTOS)
VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
SERVICE_NAME := user

ifeq ($(GOHOSTOS), windows)
	Git_Bash := $(subst \,/,$(subst cmd\,bin\bash.exe,$(dir $(shell where git))))
	API_PROTO_FILES := $(shell $(Git_Bash) -c "find ../../api/$(SERVICE_NAME)/v1 -name '*.proto'")
	COMMON_PROTO_FILES := $(shell $(Git_Bash) -c "find ../../api/common -name '*.proto' 2>/dev/null || true")
	INTERNAL_PROTO_FILES := $(shell $(Git_Bash) -c "find internal -name '*.proto'")
else
	API_PROTO_FILES := $(shell find ../../api/$(SERVICE_NAME)/v1 -name '*.proto')
	COMMON_PROTO_FILES := $(shell find ../../api/common -name '*.proto' 2>/dev/null)
	INTERNAL_PROTO_FILES := $(shell find internal -name '*.proto')
endif

.PHONY: init
# install protoc plugins
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v2@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	go install github.com/google/wire/cmd/wire@latest

.PHONY: api
# generate API proto for user service
api:
	protoc --proto_path=../../api \
	       --proto_path=../../third_party \
	       --go_out=paths=source_relative:../../api \
	       --go-http_out=paths=source_relative:../../api \
	       --go-grpc_out=paths=source_relative:../../api \
	       --go-errors_out=paths=source_relative:../../api \
	       --openapi_out=fq_schema_naming=true,default_response=false:../../ \
	       $(API_PROTO_FILES) $(COMMON_PROTO_FILES)

.PHONY: config
# generate internal conf proto
config:
	protoc --proto_path=./internal \
	       --proto_path=../../third_party \
	       --go_out=paths=source_relative:./internal \
	       $(INTERNAL_PROTO_FILES)

.PHONY: generate
generate:
	go generate ./...
	go mod tidy

.PHONY: wire
wire:
	cd cmd/$(SERVICE_NAME) && wire

.PHONY: build
build:
ifeq ($(GOHOSTOS), windows)
	if not exist bin mkdir bin
else
	mkdir -p bin/
endif
	go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/$(SERVICE_NAME) ./cmd/$(SERVICE_NAME)

.PHONY: run
run:
	go run ./cmd/$(SERVICE_NAME) -conf ./configs

.PHONY: all
all:
	$(MAKE) api
	$(MAKE) config
	$(MAKE) generate
	$(MAKE) wire

.PHONY: help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
