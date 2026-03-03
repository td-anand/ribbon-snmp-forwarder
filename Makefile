BINARY   := ribbon-snmp-forwarder
CMD_PATH := ./cmd/server
IMAGE    := ghcr.io/td-anand/ribbon-snmp-forwarder
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

LDFLAGS  := -s -w -X main.version=$(VERSION)
GOFLAGS  := CGO_ENABLED=0

.PHONY: all build test lint vet tidy image run clean help

all: tidy vet test build ## Run tidy, vet, test, and build

build: ## Build the binary for the current platform
	$(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) $(CMD_PATH)

build-linux: ## Cross-compile a Linux/amd64 binary (for container builds)
	$(GOFLAGS) GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY)-linux-amd64 $(CMD_PATH)

test: ## Run all unit tests with race detection
	go test -race -count=1 ./...

test-cover: ## Run tests and generate an HTML coverage report
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (must be installed separately)
	golangci-lint run ./...

tidy: ## Tidy and verify go.mod / go.sum
	go mod tidy
	go mod verify

image: ## Build the container image with Podman
	podman build -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

run: build ## Run the service locally (requires config.yaml)
	./bin/$(BINARY) -config config.yaml

clean: ## Remove build artifacts
	rm -rf bin/ coverage.out coverage.html

help: ## Display this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
