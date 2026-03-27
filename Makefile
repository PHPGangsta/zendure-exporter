# Makefile for zendure-exporter
#
# Targets
# -------
#   build        Compile the binary into ./bin/zendure-exporter
#   test         Run the full test suite with the race detector
#   lint         Run golangci-lint (requires golangci-lint in PATH)
#   run          Build and run the exporter with config.yml (Ctrl-C to stop)
#   clean        Remove build artefacts
#   check        Run test and lint together (CI-style gate)
#   help         Print this target list
#
# Variables
# ---------
#   VERSION      Version string embedded in the binary (default: git describe or "dev")
#   CONFIG       Config file used by the run target (default: config.yml)
#   GOFLAGS      Extra flags forwarded to go build / go test

BINARY     := zendure-exporter
CMD        := ./cmd/$(BINARY)
BIN_DIR    := bin
OUTPUT     := $(BIN_DIR)/$(BINARY)

# Derive a version from git tags; fall back to "dev" when git is unavailable.
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

CONFIG     ?= config.yml

LDFLAGS    := -s -w -X main.version=$(VERSION)
BUILD_CMD  := CGO_ENABLED=0 go build $(GOFLAGS) -ldflags="$(LDFLAGS)"

.PHONY: build test lint run clean check help

## build: compile the binary into bin/
build: $(OUTPUT)

$(OUTPUT): $(shell find . -name '*.go') go.sum
	@mkdir -p $(BIN_DIR)
	$(BUILD_CMD) -o $(OUTPUT) $(CMD)
	@echo "built $(OUTPUT) (version=$(VERSION))"

## test: run tests with the race detector
test:
	go test -race $(GOFLAGS) ./...

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## run: build and start the exporter (uses CONFIG, default config.yml)
run: build
	$(OUTPUT) -config $(CONFIG)

## clean: remove build artefacts
clean:
	rm -rf $(BIN_DIR)

## check: run test + lint (used as a CI gate)
check: test lint

## help: list available targets
help:
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/## /  /'
