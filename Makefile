.PHONY: build run clean test vet fmt help build-arm64

# Default target
.DEFAULT_GOAL := help

# Variables
BINARY_NAME=gollama-ui
CMD_PATH=./cmd/server
BUILD_DIR=.

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build the binary for current platform
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

build-arm64: ## Build the binary for ARM64 (Raspberry Pi)
	@echo "Building $(BINARY_NAME) for ARM64..."
	@GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-arm64 $(CMD_PATH)
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)-arm64"

run: build ## Build and run the server
	@./$(BINARY_NAME)

test: ## Run tests
	@go test -v ./...

vet: ## Run go vet
	@go vet ./...

fmt: ## Format code
	@go fmt ./...

tidy: ## Tidy go modules
	@go mod tidy

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME) $(BINARY_NAME)-arm64
	@echo "Cleaned"

check: vet fmt ## Run code quality checks
	@echo "All checks passed"