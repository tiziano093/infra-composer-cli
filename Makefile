.PHONY: help build test lint fmt clean release docker docs tidy

BINARY_NAME := infra-composer
PKG         := github.com/tiziano093/infra-composer-cli
VERSION     ?= v0.1.0-dev
BUILD_TIME  := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")

LDFLAGS := -ldflags "-s -w \
	-X main.Version=$(VERSION) \
	-X main.BuildTime=$(BUILD_TIME) \
	-X main.GitCommit=$(GIT_COMMIT)"

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Build CLI binary
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/infra-composer

run: build ## Run CLI (use ARGS="--version" or ARGS="--help" or ARGS="catalog build")
	./bin/$(BINARY_NAME) $(ARGS)

test: ## Run all tests with coverage
	go test ./... -v -cover -coverprofile=coverage.out

lint: ## Run linter
	golangci-lint run ./...

fmt: ## Format code
	go fmt ./...
	@command -v goimports >/dev/null && goimports -w . || echo "goimports not installed, skipping"

tidy: ## Tidy go modules
	go mod tidy

clean: ## Remove build artifacts
	rm -rf bin/ build/ coverage.out

release: ## Build cross-platform release binaries
	./scripts/release.sh

docker: ## Build Docker image
	docker build -t ghcr.io/tiziano093/$(BINARY_NAME):$(VERSION) .

docs: ## Generate CLI reference docs
	go run ./cmd/infra-composer/_docs

.DEFAULT_GOAL := help
