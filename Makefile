# Image URL to use for building/pushing image targets
IMG ?= jupyter-k8s-ws-proxy:latest
TAG ?= latest

# Container tool (finch preferred for OSS)
CONTAINER_TOOL ?= finch
BUILD_OPTS :=

ifeq ($(CONTAINER_TOOL),finch)
  export KIND_EXPERIMENTAL_PROVIDER=finch
  export GOPROXY=direct
  BUILD_OPTS := $(shell if [ -f /etc/os-release ]; then echo "--network host"; else echo ""; fi)
endif

# Binary name
BINARY := ws-proxy

# Go parameters
GOBIN=$(shell go env GOPATH)/bin

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: deps
deps: ## Download dependencies
	go mod download
	go mod tidy

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test -race -coverprofile=coverage.out ./...

.PHONY: test-verbose
test-verbose: fmt vet ## Run tests with verbose output.
	go test -race -v -coverprofile=coverage.out ./...

.PHONY: lint
lint: ## Run golangci-lint linter.
	golangci-lint run

.PHONY: lint-fix
lint-fix: ## Run golangci-lint linter and perform fixes.
	golangci-lint run --fix

.PHONY: verify
verify: fmt vet lint test ## Run all checks (fmt, vet, lint, test).

##@ Build

.PHONY: build
build: fmt vet ## Build the binary.
	CGO_ENABLED=0 go build -a -o bin/$(BINARY) ./cmd/ws-proxy

.PHONY: run
run: build ## Run locally (for development).
	./bin/$(BINARY)

##@ Container

.PHONY: docker-build
docker-build: ## Build container image.
	$(CONTAINER_TOOL) build $(BUILD_OPTS) -t $(IMG) .

.PHONY: docker-push
docker-push: ## Push container image.
	$(CONTAINER_TOOL) push $(IMG)

.PHONY: docker-build-push
docker-build-push: docker-build docker-push ## Build and push container image.

##@ Clean

.PHONY: clean
clean: ## Remove build artifacts.
	rm -rf bin/ coverage.out
