#!/bin/bash
#
# Automated PostgreSQL Database Backup Script
# Scheduled to run via cron: 0 2 * * * /opt/trading-bot/scripts/backup.sh
#
# Features:
# - Automated daily backups at 2 AM
# - Compression with gzip
# - 7-day retention policy
# - Optional Oracle Object Storage upload
# - Logging of backup operations

set -e

# Configuration
BACKUP_DIR="/opt/trading-bot/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
DATE_ONLY=$(date +%Y%m%d)
BACKUP_FILE="$BACKUP_DIR/backup_$TIMESTAMP.sql"
LOG_FILE="/opt/trading-bot/logs/backup.log"
RETENTION_DAYS=7
DOCKER_COMPOSE_DIR="/opt/trading-bot"

# Ensure directories exist
mkdir -p "$BACKUP_DIR"
mkdir -p "$(dirname "$LOG_FILE")"

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

log "=========================================="
log "Starting database backup"
log "=========================================="

# Check if docker compose is running
if ! docker compose -f "$DOCKER_COMPOSE_DIR/docker-compose.yml" ps postgres | grep -q "Up"; then
    log "ERROR: PostgreSQL container is not running"
    exit 1
fi

# Create backup
log "Creating database dump..."
if docker compose -f "$DOCKER_COMPOSE_DIR/docker-compose.yml" exec -T postgres pg_dump \
    -U trading_bot \
    -d trading_bot \
    -F p \
    --verbose \
    > "$BACKUP_FILE" 2>> "$LOG_FILE"; then
    log "Database dump created successfully"
else
    log "ERROR: Failed to create database dump"
    exit 1
fi

# Verify backup file was created
if [ ! -f "$BACKUP_FILE" ]; then
    log "ERROR: Backup file was not created"
    exit 1
fi

BACKUP_SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
log "Backup file size: $BACKUP_SIZE"

# Compress backup
log "Compressing backup..."
if gzip "$BACKUP_FILE"; then
    BACKUP_FILE="$BACKUP_FILE.gz"
    COMPRESSED_SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
    log "Backup compressed successfully. Compressed size: $COMPRESSED_SIZE"
else
    log "ERROR: Failed to compress backup"
    exit 1
fi

# Remove old backups (retention policy)
log "Enforcing 7-day retention policy..."
DELETED_COUNT=0
while IFS= read -r old_backup; do
    log "Deleting old backup: $(basename "$old_backup")"
    rm -f "$old_backup"
    ((DELETED_COUNT++))
done < <(find "$BACKUP_DIR" -name "backup_*.sql.gz" -mtime +$RETENTION_DAYS)

if [ $DELETED_COUNT -gt 0 ]; then
    log "Deleted $DELETED_COUNT old backup(s)"
else
    log "No old backups to delete"
fi

# Optional: Upload to Oracle Object Storage (Always Free: 20GB)
# Uncomment and configure if you have Oracle CLI set up
# log "Uploading backup to Oracle Object Storage..."
# if oci os object put \
#     --bucket-name trading-bot-backups \
#     --file "$BACKUP_FILE" \
#     --object-name "database/backup_$DATE_ONLY.sql.gz" \
#     >> "$LOG_FILE" 2>&1; then
#     log "Backup uploaded to Object Storage successfully"
# else
#     log "WARNING: Failed to upload to Object Storage (not critical)"
# fi

# Summary
log "=========================================="
log "Backup completed successfully!"
log "Location: $BACKUP_FILE"
log "Size: $COMPRESSED_SIZE"
log "Retention: ${RETENTION_DAYS} days"
log "=========================================="
