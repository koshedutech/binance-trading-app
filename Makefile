.PHONY: help dev dev-down dev-logs dev-shell dev-rebuild \
        prod prod-down prod-logs prod-restart prod-status \
        db-backup db-restore db-shell db-reset \
        test lint fmt

# Default target
.DEFAULT_GOAL := help

# Variables
APP_NAME := binance-trading-bot
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

help: ## Show this help message
	@echo '╔══════════════════════════════════════════════════════════════════╗'
	@echo '║     Binance Trading Bot - Docker-Only Build Commands             ║'
	@echo '╚══════════════════════════════════════════════════════════════════╝'
	@echo ''
	@echo 'DEVELOPMENT (Port 8094):'
	@echo '  make dev          Start development environment'
	@echo '  make dev-rebuild  Rebuild and restart (use after code changes)'
	@echo '  make dev-down     Stop development containers'
	@echo '  make dev-logs     View development logs'
	@echo '  make dev-shell    Open shell in container'
	@echo ''
	@echo 'PRODUCTION (Port 8095):'
	@echo '  make prod         Start production environment'
	@echo '  make prod-down    Stop production containers'
	@echo '  make prod-logs    View production logs'
	@echo '  make prod-restart Restart production container'
	@echo '  make prod-status  Show production status'
	@echo ''
	@echo 'DATABASE:'
	@echo '  make db-backup    Backup database'
	@echo '  make db-restore   Restore database (FILE=path/to/backup.sql)'
	@echo '  make db-shell     Open PostgreSQL shell'
	@echo '  make db-reset     Reset database (WARNING: deletes all data)'
	@echo ''
	@echo 'UTILITIES:'
	@echo '  make test         Run tests inside container'
	@echo '  make lint         Run linter inside container'
	@echo '  make fmt          Format code inside container'
	@echo ''

# ============================================================================
# DEVELOPMENT COMMANDS (Port 8094)
# ============================================================================

dev: ## Start development environment (port 8094)
	@echo "🐳 Starting development environment..."
	@./scripts/docker-dev.sh -d
	@echo ""
	@echo "✅ Development environment started!"
	@echo "   Web UI: http://localhost:8094"
	@echo "   Logs:   make dev-logs"

dev-rebuild: ## Rebuild and restart development (use after ANY code changes)
	@echo "🔄 Rebuilding development environment..."
	@./scripts/docker-dev.sh
	@echo ""
	@echo "✅ Rebuild complete!"
	@echo "   Web UI: http://localhost:8094"

dev-down: ## Stop development containers
	@echo "🛑 Stopping development containers..."
	@docker-compose down
	@echo "✅ Development containers stopped"

dev-logs: ## View development logs
	@docker-compose logs -f trading-bot

dev-shell: ## Open shell in development container
	@docker-compose exec trading-bot /bin/sh

# ============================================================================
# PRODUCTION COMMANDS (Port 8095)
# ============================================================================

prod: ## Start production environment (port 8095)
	@echo ""
	@echo "⚠️  ════════════════════════════════════════════════════════════"
	@echo "⚠️  WARNING: Starting PRODUCTION mode with REAL MONEY!"
	@echo "⚠️  ════════════════════════════════════════════════════════════"
	@echo ""
	@echo "Checking required environment variables..."
	@test -f .env || (echo "❌ .env file is required" && exit 1)
	@echo "✅ Environment file found"
	@echo ""
	@echo "Starting in 5 seconds... Press Ctrl+C to cancel."
	@sleep 5
	@docker-compose -f docker-compose.prod.yml up -d --build
	@echo ""
	@echo "✅ Production environment started!"
	@echo "   Web UI: http://localhost:8095"
	@echo "   Logs:   make prod-logs"

prod-down: ## Stop production environment
	@echo "🛑 Stopping production environment..."
	@docker-compose -f docker-compose.prod.yml down
	@echo "✅ Production environment stopped"

prod-logs: ## View production logs
	@docker-compose -f docker-compose.prod.yml logs -f trading-bot

prod-restart: ## Restart production trading bot
	@echo "🔄 Restarting production trading bot..."
	@docker-compose -f docker-compose.prod.yml down
	@docker-compose -f docker-compose.prod.yml up -d --build
	@echo "✅ Trading bot restarted"
	@echo "   Web UI: http://localhost:8095"

prod-status: ## Show production status
	@echo "📊 Production Status:"
	@docker-compose -f docker-compose.prod.yml ps

# ============================================================================
# DATABASE COMMANDS
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
# UTILITY COMMANDS (Run inside container)
# ============================================================================

test: ## Run tests inside development container
	@echo "🧪 Running tests..."
	@docker-compose exec trading-bot go test -v ./...

lint: ## Run linter inside development container
	@echo "🔍 Running linter..."
	@docker-compose exec trading-bot golangci-lint run || echo "Note: golangci-lint may not be installed in container"

fmt: ## Format code inside development container
	@echo "✨ Formatting code..."
	@docker-compose exec trading-bot go fmt ./...
	@echo "✅ Code formatted"
