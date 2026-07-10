# Image URL to use for building/pushing image targets
IMG ?= jupyter-k8s-ws-proxy:latest
TAG ?= latest

# Container tool (finch preferred for OSS)
CONTAINER_TOOL ?= finch
BUILD_OPTS :=

# Linter version — keep in sync with CI
GOLANGCI_LINT_VERSION ?= v2.4.0

ifeq ($(CONTAINER_TOOL),finch)
  export KIND_EXPERIMENTAL_PROVIDER=finch
  export GOPROXY=direct
  BUILD_OPTS := $(shell if [ -f /etc/os-release ]; then echo "--network host"; else echo ""; fi)
endif

# Binary name
BINARY := ws-proxy

# Setting SHELL to bash allows bash commands to be executed by recipes.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

.PHONY: release
release: build lint test ## Run all checks required before PR submission.

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: deps
deps: ## Download dependencies.
	go mod download
	go mod tidy

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run unit tests.
	go test -race -coverprofile=coverage.out ./internal/...

.PHONY: test-verbose
test-verbose: fmt vet ## Run unit tests with verbose output.
	go test -race -v -coverprofile=coverage.out ./internal/...

.PHONY: lint
lint: ## Run golangci-lint linter.
	golangci-lint run

.PHONY: lint-fix
lint-fix: ## Run golangci-lint linter and perform fixes.
	golangci-lint run --fix

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

##@ E2E Testing

KIND_CLUSTER ?= ws-proxy-e2e
E2E_IMAGE ?= jupyter-k8s-ws-proxy:test

.PHONY: setup-test-e2e
setup-test-e2e: docker-build ## Create Kind cluster and load proxy image for E2E tests.
	@command -v kind >/dev/null 2>&1 || { echo "kind is not installed"; exit 1; }
	@case "$$(kind get clusters)" in \
		*"$(KIND_CLUSTER)"*) \
			echo "Deleting existing Kind cluster '$(KIND_CLUSTER)'..."; \
			kind delete cluster --name $(KIND_CLUSTER) ;; \
	esac
	@echo "Creating Kind cluster '$(KIND_CLUSTER)'..."
	@kind create cluster --name $(KIND_CLUSTER)
	@echo "Loading proxy image into Kind..."
	@$(CONTAINER_TOOL) tag $(IMG) $(E2E_IMAGE)
	@$(CONTAINER_TOOL) save $(E2E_IMAGE) -o /tmp/ws-proxy-e2e.tar
	@kind load image-archive /tmp/ws-proxy-e2e.tar --name $(KIND_CLUSTER)
	@rm -f /tmp/ws-proxy-e2e.tar
	@echo "E2E cluster ready."

.PHONY: test-e2e
test-e2e: ## Run E2E tests (requires setup-test-e2e to have been run).
	KIND_CLUSTER=$(KIND_CLUSTER) go test -tags=e2e ./test/e2e/ -v -timeout 10m -ginkgo.v

.PHONY: test-e2e-focus
test-e2e-focus: ## Run specific E2E test. Usage: make test-e2e-focus FOCUS="should proxy"
	@if [ -z "$(FOCUS)" ]; then echo "Error: FOCUS is required. Usage: make test-e2e-focus FOCUS=\"should proxy\""; exit 1; fi
	KIND_CLUSTER=$(KIND_CLUSTER) go test -tags=e2e ./test/e2e/ -v -timeout 10m -ginkgo.v -ginkgo.focus="$(FOCUS)"

.PHONY: test-e2e-full
test-e2e-full: setup-test-e2e test-e2e cleanup-test-e2e ## Full E2E: setup, run, cleanup.

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## Delete the Kind cluster used for E2E tests.
	kind delete cluster --name $(KIND_CLUSTER) || true

##@ Clean

.PHONY: clean
clean: ## Remove build artifacts.
	rm -rf bin/ coverage.out
