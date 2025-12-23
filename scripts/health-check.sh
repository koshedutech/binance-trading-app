#!/bin/bash
#
# Health Check Script for Trading Bot Monitoring
# Scheduled to run via cron: */5 * * * * /opt/trading-bot/scripts/health-check.sh
#
# Features:
# - Verifies Docker containers are running
# - Checks application health endpoint
# - Checks database connectivity
# - Auto-restarts services if they fail
# - Sends alerts via Telegram if configured
# - Logs health status

set -e

# Configuration
DOCKER_COMPOSE_DIR="/opt/trading-bot"
HEALTH_ENDPOINT="http://localhost:8090/api/health"
LOG_FILE="/opt/trading-bot/logs/health-check.log"
ALERT_LOG="/opt/trading-bot/logs/health-alerts.log"
TIMEOUT=10
MAX_RETRIES=3
TELEGRAM_BOT_TOKEN="${TELEGRAM_BOT_TOKEN:-}"
TELEGRAM_CHAT_ID="${TELEGRAM_CHAT_ID:-}"

# Ensure log directory exists
mkdir -p "$(dirname "$LOG_FILE")"

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE"
}

# Alert function (Telegram if configured)
alert() {
    local message=$1
    local severity=$2

    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [$severity] $message" >> "$ALERT_LOG"

    if [ -n "$TELEGRAM_BOT_TOKEN" ] && [ -n "$TELEGRAM_CHAT_ID" ]; then
        curl -s -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
            -d "chat_id=${TELEGRAM_CHAT_ID}" \
            -d "text=⚠️ Trading Bot Alert [$severity]\n\n$message" \
            > /dev/null 2>&1 || true
    fi
}

log "========== Health Check Started =========="

# Flag for tracking if we had any issues
HEALTH_OK=true

# Function: Check if Docker containers are running
check_containers() {
    log "Checking Docker containers status..."

    if ! docker compose -f "$DOCKER_COMPOSE_DIR/docker-compose.yml" ps | grep -q "postgres.*Up"; then
        log "ERROR: PostgreSQL container is not running"
        alert "PostgreSQL container is down!" "ERROR"
        HEALTH_OK=false
        return 1
    fi
    log "✓ PostgreSQL container is running"

    if ! docker compose -f "$DOCKER_COMPOSE_DIR/docker-compose.yml" ps | grep -q "trading-bot.*Up"; then
        log "ERROR: Trading Bot container is not running"
        alert "Trading Bot container is down!" "ERROR"
        HEALTH_OK=false
        return 1
    fi
    log "✓ Trading Bot container is running"

    return 0
}

# Function: Check health endpoint
check_health_endpoint() {
    log "Checking application health endpoint..."

    local retry_count=0
    local http_code=0

    while [ $retry_count -lt $MAX_RETRIES ]; do
        http_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time $TIMEOUT "$HEALTH_ENDPOINT" 2>/dev/null || echo "000")

        if [ "$http_code" = "200" ]; then
            log "✓ Health endpoint returned 200 OK"
            return 0
        fi

        log "WARNING: Health endpoint returned HTTP $http_code (attempt $((retry_count + 1))/$MAX_RETRIES)"
        retry_count=$((retry_count + 1))

        if [ $retry_count -lt $MAX_RETRIES ]; then
            sleep 2
        fi
    done

    log "ERROR: Health endpoint failed after $MAX_RETRIES attempts (HTTP $http_code)"
    alert "Health endpoint check failed (HTTP $http_code)" "ERROR"
    HEALTH_OK=false
    return 1
}

# Function: Check database connectivity
check_database() {
    log "Checking database connectivity..."

    if docker compose -f "$DOCKER_COMPOSE_DIR/docker-compose.yml" exec -T postgres \
        psql -U trading_bot -d trading_bot -c "SELECT NOW();" > /dev/null 2>&1; then
        log "✓ Database connectivity OK"
        return 0
    else
        log "ERROR: Database connectivity check failed"
        alert "Database connectivity check failed" "ERROR"
        HEALTH_OK=false
        return 1
    fi
}

# Function: Restart services if needed
restart_services() {
    if [ "$HEALTH_OK" = false ]; then
        log "Health check failed, attempting to restart services..."
        alert "Attempting to restart trading bot services..." "WARNING"

        if docker compose -f "$DOCKER_COMPOSE_DIR/docker-compose.yml" \
            -f "$DOCKER_COMPOSE_DIR/docker-compose.oracle.yml" \
            up -d >> "$LOG_FILE" 2>&1; then
            log "Services restarted successfully"
            alert "Services restarted successfully" "INFO"
            sleep 5

            # Verify restart was successful
            if check_containers && check_health_endpoint; then
                log "✓ Services recovered successfully"
                HEALTH_OK=true
                return 0
            else
                log "ERROR: Services still unhealthy after restart"
                alert "Services still unhealthy after restart" "CRITICAL"
                return 1
            fi
        else
            log "ERROR: Failed to restart services"
            alert "Failed to restart services" "CRITICAL"
            return 1
        fi
    fi

    return 0
}

# Function: Get system metrics
get_metrics() {
    log "System metrics:"

    # CPU usage
    cpu_usage=$(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d'%' -f1 || echo "N/A")
    log "  CPU Usage: $cpu_usage%"

    # Memory usage
    mem_usage=$(free | grep Mem | awk '{printf("%.1f", $3/$2 * 100.0)}' || echo "N/A")
    log "  Memory Usage: ${mem_usage}%"

    # Disk usage
    disk_usage=$(df /opt/trading-bot | tail -1 | awk '{print $5}' || echo "N/A")
    log "  Disk Usage: $disk_usage"

    # Docker stats
    log "  Docker container stats:"
    docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}" >> "$LOG_FILE" 2>&1 || true
}

# Main execution
{
    check_containers || true
    check_health_endpoint || true
    check_database || true
    restart_services || true
    get_metrics

    if [ "$HEALTH_OK" = true ]; then
        log "✓ All health checks passed"
        log "========== Health Check Completed (OK) =========="
        exit 0
    else
        log "✗ Health check completed with issues"
        log "========== Health Check Completed (FAILED) =========="
        exit 1
    fi
}
