#!/bin/bash
#
# Production Deployment Script for Trading Bot
# One-command deployment with safety checks
#
# Usage: /opt/trading-bot/scripts/deploy.sh
# Or with custom branch: /opt/trading-bot/scripts/deploy.sh main

set -e

# Configuration
DEPLOY_DIR="/opt/trading-bot"
BRANCH="${1:-main}"
LOG_FILE="/opt/trading-bot/logs/deploy.log"
BACKUP_DIR="/opt/trading-bot/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Ensure directories exist
mkdir -p "$(dirname "$LOG_FILE")"
mkdir -p "$BACKUP_DIR"

# Logging function
log() {
    echo -e "${GREEN}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "$LOG_FILE"
}

error() {
    echo -e "${RED}[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $1${NC}" | tee -a "$LOG_FILE"
    exit 1
}

warning() {
    echo -e "${YELLOW}[$(date '+%Y-%m-%d %H:%M:%S')] WARNING: $1${NC}" | tee -a "$LOG_FILE"
}

log "=========================================="
log "Starting deployment process"
log "Deploy Directory: $DEPLOY_DIR"
log "Branch: $BRANCH"
log "=========================================="

# Pre-deployment checks
log "Running pre-deployment checks..."

if [ ! -d "$DEPLOY_DIR" ]; then
    error "Deploy directory does not exist: $DEPLOY_DIR"
fi

cd "$DEPLOY_DIR"

# Check if this is a git repository
if [ ! -d ".git" ]; then
    error "Not a git repository"
fi

# Check if we have uncommitted changes
if [ -n "$(git status --porcelain)" ]; then
    warning "Uncommitted changes detected:"
    git status --short | tee -a "$LOG_FILE"
    read -p "Continue with deployment? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        error "Deployment cancelled by user"
    fi
fi

# Check Docker is running
log "Checking Docker availability..."
if ! docker info > /dev/null 2>&1; then
    error "Docker is not running"
fi

# Check docker-compose
if ! docker compose version > /dev/null 2>&1; then
    error "docker compose is not available"
fi

# Create pre-deployment backup
log "Creating pre-deployment database backup..."
if [ -f "scripts/backup.sh" ]; then
    if bash "scripts/backup.sh" >> "$LOG_FILE" 2>&1; then
        log "Pre-deployment backup completed"
    else
        error "Failed to create pre-deployment backup"
    fi
else
    warning "Backup script not found, skipping backup"
fi

# Pull latest code
log "Pulling latest code from Git..."
if git fetch origin > /dev/null 2>&1; then
    log "✓ Git fetch completed"
else
    error "Failed to fetch from Git"
fi

if git checkout "$BRANCH" > /dev/null 2>&1; then
    log "✓ Switched to branch: $BRANCH"
else
    error "Failed to checkout branch: $BRANCH"
fi

if git pull origin "$BRANCH" > /dev/null 2>&1; then
    log "✓ Git pull completed"
    git log -1 --oneline | tee -a "$LOG_FILE"
else
    error "Failed to pull from branch: $BRANCH"
fi

# Stop autopilot
log "Stopping autopilot (if enabled)..."
if curl -s -X POST "http://localhost:8095/api/futures/autopilot/stop" \
    -H "Content-Type: application/json" \
    > /dev/null 2>&1; then
    log "✓ Autopilot stopped"
    sleep 2
else
    warning "Could not stop autopilot (may not be enabled)"
fi

# Build new images
log "Building Docker images..."
if docker compose -f docker-compose.prod.yml build >> "$LOG_FILE" 2>&1; then
    log "✓ Docker build completed"
else
    error "Docker build failed"
fi

# Stop running services
log "Stopping running services..."
if docker compose -f docker-compose.prod.yml down >> "$LOG_FILE" 2>&1; then
    log "✓ Services stopped"
else
    warning "Could not stop services cleanly"
fi

# Start new services
log "Starting new services..."
if docker compose -f docker-compose.prod.yml up -d >> "$LOG_FILE" 2>&1; then
    log "✓ Services started"
else
    error "Failed to start services"
fi

# Wait for services to be ready
log "Waiting for services to become ready..."
sleep 5

# Health checks
log "Performing health checks..."

# Check PostgreSQL
log "Checking PostgreSQL connectivity..."
for i in {1..30}; do
    if docker compose exec -T postgres \
        psql -U trading_bot -d trading_bot -c "SELECT NOW();" > /dev/null 2>&1; then
        log "✓ PostgreSQL is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        error "PostgreSQL failed to become ready within 30 seconds"
    fi
    sleep 1
done

# Check application health
log "Checking application health..."
for i in {1..60}; do
    http_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "http://localhost:8095/api/health" 2>/dev/null || echo "000")
    if [ "$http_code" = "200" ]; then
        log "✓ Application is healthy (HTTP 200)"
        break
    fi
    if [ $i -eq 60 ]; then
        error "Application health check failed after 60 seconds (HTTP $http_code)"
    fi
    sleep 1
done

# Check logs for errors
log "Checking application logs for startup errors..."
if docker compose logs --tail=50 trading-bot | grep -i "fatal\|panic\|error" > /dev/null; then
    warning "Potential errors found in application logs:"
    docker compose logs --tail=10 trading-bot | grep -i "error" | tee -a "$LOG_FILE" || true
else
    log "✓ No obvious errors in application logs"
fi

# Verify container restarts
log "Checking if containers restarted unexpectedly..."
if docker compose ps | grep -q "0 seconds"; then
    warning "Container has restarted recently:"
    docker compose ps | tee -a "$LOG_FILE"
else
    log "✓ Containers are running normally"
fi

# Display logs
log "=========================================="
log "Last 20 lines of application log:"
log "=========================================="
docker compose logs --tail=20 trading-bot | tee -a "$LOG_FILE"

log "=========================================="
log "Deployment completed successfully!"
log "=========================================="
log ""
log "Next steps:"
log "1. Verify trading bot is accessible at: https://your-domain.com"
log "2. Check the dashboard: https://your-domain.com"
log "3. Monitor logs: docker compose logs -f trading-bot"
log "4. Enable autopilot when ready: Dashboard > Settings > Autopilot"
log ""
log "View deployment log:"
log "  tail -f $LOG_FILE"
log ""
log "Rollback to previous version (if needed):"
log "  git checkout HEAD~1"
log "  bash $DEPLOY_DIR/scripts/deploy.sh"
log ""
