.PHONY: help build run test clean install dev config docker-build docker-run \
        prod-build prod-up prod-down prod-logs prod-restart frontend-build \
        db-backup db-restore lint fmt test-coverage

# Default target
.DEFAULT_GOAL := help

# Variables
APP_NAME := binance-trading-bot
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

help: ## Show this help message
	@echo '╔══════════════════════════════════════════════════════════════════╗'
	@echo '║           Binance Trading Bot - Build Commands                    ║'
	@echo '╚══════════════════════════════════════════════════════════════════╝'
	@echo ''
	@echo 'Development:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | grep -E "build|run|dev|test|install|clean|config|lint|fmt"
	@echo ''
	@echo 'Docker (Development):'
	@awk 'BEGIN {FS = ":.*?## "} /^docker-[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ''
	@echo 'Production:'
	@awk 'BEGIN {FS = ":.*?## "} /^prod-[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ''
	@echo 'Database:'
	@awk 'BEGIN {FS = ":.*?## "} /^db-[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ============================================================================
# Development Commands
# ============================================================================

build: ## Build the trading bot binary
	@echo "🔨 Building $(APP_NAME) $(VERSION)..."
	@mkdir -p bin
	@go build $(LDFLAGS) -o bin/$(APP_NAME) main.go
	@echo "✅ Build complete: bin/$(APP_NAME)"

run: ## Run the trading bot locally
	@echo "🚀 Starting trading bot..."
	@go run main.go

dev: ## Run with auto-reload (requires: go install github.com/cosmtrek/air@latest)
	@air

test: ## Run all tests
	@echo "🧪 Running tests..."
	@go test -v ./...

test-coverage: ## Run tests with coverage report
	@echo "🧪 Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "📊 Coverage report: coverage.html"

install: ## Install Go dependencies
	@echo "📦 Installing dependencies..."
	@go mod download
	@go mod tidy
	@echo "✅ Dependencies installed"

clean: ## Clean build artifacts
	@echo "🧹 Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "✅ Clean complete"

config: ## Generate sample configuration files
	@echo "📝 Generating configuration files..."
	@cp -n config.json.example config.json 2>/dev/null || true
	@cp -n .env.example .env 2>/dev/null || true
	@echo "✅ Configuration files created. Edit with your API keys."

lint: ## Run linter (requires: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@echo "🔍 Running linter..."
	@golangci-lint run

fmt: ## Format Go code
	@echo "✨ Formatting code..."
	@go fmt ./...
	@echo "✅ Code formatted"

frontend-build: ## Build React frontend
	@echo "🎨 Building frontend..."
	@cd web && npm install && npm run build
	@echo "✅ Frontend built: web/dist/"

# ============================================================================
# Docker Development Commands
# ============================================================================

docker-build: ## Build Docker image for development
	@echo "🐳 Building Docker image..."
	@docker build -t $(APP_NAME):dev .
	@echo "✅ Image built: $(APP_NAME):dev"

docker-run: ## Run development containers
	@echo "🐳 Starting development containers..."
	@docker-compose up -d
	@echo "✅ Containers started. Web UI: http://localhost:8088"

docker-stop: ## Stop development containers
	@echo "🛑 Stopping containers..."
	@docker-compose down
	@echo "✅ Containers stopped"

docker-logs: ## View container logs
	@docker-compose logs -f trading-bot

docker-shell: ## Open shell in trading-bot container
	@docker-compose exec trading-bot /bin/sh

# ============================================================================
# Production Commands
# ============================================================================

prod-build: ## Build production Docker image
	@echo "🏭 Building production image..."
	@docker build -t $(APP_NAME):$(VERSION) -t $(APP_NAME):latest .
	@echo "✅ Production image built: $(APP_NAME):$(VERSION)"

prod-up: ## Start production environment
	@echo ""
	@echo "⚠️  ════════════════════════════════════════════════════════════"
	@echo "⚠️  WARNING: Starting PRODUCTION mode with REAL MONEY!"
	@echo "⚠️  ════════════════════════════════════════════════════════════"
	@echo ""
	@echo "Checking required environment variables..."
	@test -n "$(BINANCE_API_KEY)" || (echo "❌ BINANCE_API_KEY is required" && exit 1)
	@test -n "$(BINANCE_SECRET_KEY)" || (echo "❌ BINANCE_SECRET_KEY is required" && exit 1)
	@test -n "$(DB_PASSWORD)" || (echo "❌ DB_PASSWORD is required" && exit 1)
	@echo "✅ Environment variables OK"
	@echo ""
	@echo "Starting in 5 seconds... Press Ctrl+C to cancel."
	@sleep 5
	@docker-compose -f docker-compose.prod.yml up -d
	@echo ""
	@echo "✅ Production environment started!"
	@echo "   Web UI: http://localhost:8088"
	@echo "   Logs:   make prod-logs"

prod-down: ## Stop production environment
	@echo "🛑 Stopping production environment..."
	@docker-compose -f docker-compose.prod.yml down
	@echo "✅ Production environment stopped"

prod-logs: ## View production logs
	@docker-compose -f docker-compose.prod.yml logs -f trading-bot

prod-restart: ## Restart production trading bot
	@echo "🔄 Restarting production trading bot..."
	@docker-compose -f docker-compose.prod.yml restart trading-bot
	@echo "✅ Trading bot restarted"

prod-status: ## Show production status
	@echo "📊 Production Status:"
	@docker-compose -f docker-compose.prod.yml ps

prod-pull: ## Pull latest images
	@docker-compose -f docker-compose.prod.yml pull

prod-backup: ## Run database backup now
	@echo "💾 Running database backup..."
	@docker-compose -f docker-compose.prod.yml --profile with-backup run --rm backup

# ============================================================================
# Database Commands
# ============================================================================

db-backup: ## Backup database to backups/ directory
	@echo "💾 Backing up database..."
	@mkdir -p backups
	@docker-compose exec postgres pg_dump -U trading_bot trading_bot > backups/backup_$(shell date +%Y%m%d_%H%M%S).sql
	@echo "✅ Backup saved to backups/"

db-restore: ## Restore database from backup (usage: make db-restore FILE=backups/backup.sql)
	@test -n "$(FILE)" || (echo "❌ FILE is required. Usage: make db-restore FILE=backups/backup.sql" && exit 1)
	@echo "📥 Restoring database from $(FILE)..."
	@docker-compose exec -T postgres psql -U trading_bot trading_bot < $(FILE)
	@echo "✅ Database restored"

db-shell: ## Open PostgreSQL shell
	@docker-compose exec postgres psql -U trading_bot trading_bot

db-reset: ## Reset database (WARNING: deletes all data)
	@echo "⚠️  This will DELETE ALL DATA. Press Ctrl+C to cancel."
	@sleep 5
	@docker-compose down -v
	@docker-compose up -d postgres
	@echo "✅ Database reset complete"

# ============================================================================
# Quick Start
# ============================================================================

quick-start: config docker-build docker-run ## Quick start for first-time setup
	@echo ""
	@echo "🎉 Quick start complete!"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Edit .env with your Binance API keys"
	@echo "  2. Restart: make docker-run"
	@echo "  3. Open: http://localhost:8088"
