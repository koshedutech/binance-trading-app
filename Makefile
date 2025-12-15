.PHONY: help build run test clean install dev config

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the trading bot
	@echo "Building trading bot..."
	@go build -o bin/trading-bot main.go
	@echo "Build complete: bin/trading-bot"

run: ## Run the trading bot
	@echo "Starting trading bot..."
	@go run main.go

dev: ## Run with auto-reload (requires air: go install github.com/cosmtrek/air@latest)
	@air

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

install: ## Install dependencies
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

config: ## Generate sample configuration files
	@echo "Generating configuration files..."
	@cp -n config.json.example config.json 2>/dev/null || true
	@cp -n .env.example .env 2>/dev/null || true
	@echo "Configuration files created. Please edit with your API keys."

lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t binance-trading-bot:latest .

docker-run: ## Run in Docker
	@echo "Running in Docker..."
	@docker run --env-file .env binance-trading-bot:latest

dry-run: ## Run in dry-run mode (testnet)
	@echo "Running in DRY RUN mode..."
	@BINANCE_TESTNET=true go run main.go

production: ## Run in production mode (WARNING: uses real money)
	@echo "WARNING: Running in PRODUCTION mode with REAL money!"
	@echo "Press Ctrl+C to cancel, or wait 5 seconds to continue..."
	@sleep 5
	@BINANCE_TESTNET=false go run main.go
