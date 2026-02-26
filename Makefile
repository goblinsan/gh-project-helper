BINARY_NAME := gh-project-helper
MODULE := github.com/goblinsan/gh-project-helper
BUILD_DIR := ./cmd/gh-project-helper

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS := -s -w \
	-X '$(MODULE)/cmd/gh-project-helper/commands.Version=$(VERSION)' \
	-X '$(MODULE)/cmd/gh-project-helper/commands.Commit=$(COMMIT)' \
	-X '$(MODULE)/cmd/gh-project-helper/commands.Date=$(DATE)'

.PHONY: all build clean test lint vet fmt install

all: test build

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) $(BUILD_DIR)

install:
	go install -ldflags "$(LDFLAGS)" $(BUILD_DIR)

test:
	go test ./... -v

vet:
	go vet ./...

fmt:
	gofmt -s -w .

lint: vet
	@which golangci-lint > /dev/null 2>&1 || echo "Install golangci-lint: https://golangci-lint.run/usage/install/"
	golangci-lint run ./...

clean:
	rm -f $(BINARY_NAME)
	rm -rf dist/ build/

validate:
	go run $(BUILD_DIR) validate -f plan.yaml

dry-run:
	go run $(BUILD_DIR) apply -f plan.yaml --dry-run

.DEFAULT_GOAL := all
