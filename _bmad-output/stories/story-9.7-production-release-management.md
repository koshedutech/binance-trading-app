# Story 9.7: Production Release Management with Rollback Capability

**Story ID:** INFRA-9.7
**Epic:** Epic 9 - Entry Signal Quality Improvements & Infrastructure
**Priority:** P1 (High - Operational Safety)
**Estimated Effort:** 8-12 hours
**Author:** Claude Code Agent
**Status:** Ready for Implementation
**Created:** 2026-01-14
**Depends On:** Story 9.6 (Shared Redis Infrastructure)

---

## Problem Statement

### Current Pain Points

1. **No Version History**: Production builds use `latest` tag, overwriting previous working versions
2. **No Rollback Capability**: If new production code fails, no quick way to restore previous working state
3. **Data Loss on Rollback**: Even if we had old images, the database/cache state wouldn't match
4. **Trading Continuity Risk**: Active orders need protection - can't afford extended downtime during emergencies

### Business Impact

- **Downtime = Lost Trades**: Every minute without working autopilot means missed opportunities or unprotected positions
- **No Safety Net**: Breaking production means scrambling to debug instead of quickly reverting
- **Manual Recovery**: Currently requires manual intervention to restore a working state

---

## Solution Overview

### Release Management System

```
┌─────────────────────────────────────────────────────────────────┐
│                    PRODUCTION RELEASES                           │
├─────────────────────────────────────────────────────────────────┤
│  Image: binance-bot:prod-003 (current)                          │
│  Image: binance-bot:prod-002 "Story 9.5 - Trend Filters"        │
│  Image: binance-bot:prod-001 "Initial stable release"           │
├─────────────────────────────────────────────────────────────────┤
│  releases/                                                       │
│  ├── manifest.json          # Master index                       │
│  ├── prod-001/                                                   │
│  │   ├── release-info.json  # Metadata                          │
│  │   └── volumes/                                                │
│  │       ├── postgres-data.tar.gz                               │
│  │       └── redis-shared-data.tar.gz                           │
│  ├── prod-002/                                                   │
│  │   └── ...                                                     │
│  └── prod-003/                                                   │
│       └── ...                                                    │
└─────────────────────────────────────────────────────────────────┘
```

### Key Features

| Feature | Description |
|---------|-------------|
| **Sequential Versioning** | `prod-001`, `prod-002`, `prod-003`... |
| **Keep Last 3 Releases** | Auto-cleanup of older releases |
| **Volume Snapshots** | PostgreSQL + Redis backed up per release |
| **Config Preservation** | Settings files saved with each release |
| **Quick Rollback** | Restore any of last 3 releases in ~60 seconds |
| **Optional Redis Restore** | Choose whether to restore cache or use current |

---

## Goals

1. **Version History**: Every production release tagged and preserved
2. **Complete Snapshots**: Each release = image + database + cache + configs
3. **Quick Recovery**: Rollback to working state in under 60 seconds
4. **Selective Restore**: Option to keep current Redis or restore historical
5. **Metadata Tracking**: Know what each release contains

---

## Implementation Phases

### Phase 1: Folder Structure & Manifest (LOW RISK)
*Estimated: 1 hour*
*Dependencies: None*

#### Task 1.1: Create releases directory structure
```bash
releases/
├── manifest.json
└── .gitkeep
```

#### Task 1.2: Add releases/ to .gitignore
```
# Production releases (large volume backups)
releases/
!releases/.gitkeep
```

#### Task 1.3: Define manifest.json schema
```json
{
  "schema_version": 1,
  "current_release": "prod-003",
  "max_releases": 3,
  "releases": [
    {
      "version": "prod-003",
      "created": "2026-01-14T15:30:00Z",
      "git_commit": "7e835aec",
      "git_branch": "main",
      "description": "Story 9.5 - Enhanced Trend Validation Filters",
      "docker_image": "binance-bot:prod-003",
      "status": "active",
      "volumes": {
        "postgres": "volumes/postgres-data.tar.gz",
        "redis": "volumes/redis-shared-data.tar.gz"
      },
      "configs": [
        "configs/default-settings.json",
        "configs/autopilot_settings.json"
      ]
    }
  ]
}
```

---

### Phase 2: Release Script - Create New Release (MEDIUM RISK)
*Estimated: 3 hours*
*Dependencies: Phase 1*

#### Task 2.1: Create prod-release.sh script

File: `scripts/prod-release.sh`

```bash
#!/bin/bash
# Production Release Management Script
#
# Usage:
#   ./scripts/prod-release.sh --new "Description of this release"
#   ./scripts/prod-release.sh --list
#   ./scripts/prod-release.sh --rollback prod-002
#   ./scripts/prod-release.sh --rollback prod-002 --restore-redis
#   ./scripts/prod-release.sh --info prod-002
```

#### Task 2.2: Implement --new command

```bash
# --new "Description"
# 1. Stop production containers
# 2. Get current git commit hash
# 3. Calculate next version number
# 4. Backup current PostgreSQL volume
# 5. Backup current shared Redis data
# 6. Copy current config files
# 7. Build new Docker image with version tag
# 8. Update manifest.json
# 9. Cleanup old releases (keep last 3)
# 10. Start production with new image
# 11. Tag image as :latest as well
```

#### Task 2.3: Volume backup functions

```bash
backup_postgres_volume() {
    local release_dir="$1"
    echo "Backing up PostgreSQL data..."

    # Stop postgres to ensure consistent backup
    docker-compose -f docker-compose.prod.yml stop postgres

    # Create backup using docker
    docker run --rm \
        -v binance-trading-app_postgres_prod_data:/data:ro \
        -v "$(pwd)/${release_dir}/volumes":/backup \
        alpine tar czf /backup/postgres-data.tar.gz -C /data .

    # Restart postgres
    docker-compose -f docker-compose.prod.yml start postgres
}

backup_redis_volume() {
    local release_dir="$1"
    echo "Backing up shared Redis data..."

    # Redis supports live backup via BGSAVE
    docker exec binance-bot-redis redis-cli BGSAVE
    sleep 2  # Wait for save to complete

    # Copy the RDB file
    docker run --rm \
        -v binance-infra_redis-data:/data:ro \
        -v "$(pwd)/${release_dir}/volumes":/backup \
        alpine tar czf /backup/redis-shared-data.tar.gz -C /data .
}
```

#### Task 2.4: Cleanup old releases

```bash
cleanup_old_releases() {
    local max_releases=3
    local releases_dir="releases"

    # Get list of releases sorted by version number
    local releases=($(ls -d ${releases_dir}/prod-* 2>/dev/null | sort -V))
    local count=${#releases[@]}

    if [ $count -gt $max_releases ]; then
        local to_delete=$((count - max_releases))
        for ((i=0; i<to_delete; i++)); do
            local old_release="${releases[$i]}"
            local old_version=$(basename "$old_release")

            echo "Removing old release: $old_version"

            # Remove Docker image
            docker rmi "binance-bot:${old_version}" 2>/dev/null || true

            # Remove release folder
            rm -rf "$old_release"
        done
    fi
}
```

---

### Phase 3: Release Script - List & Info (LOW RISK)
*Estimated: 1 hour*
*Dependencies: Phase 2*

#### Task 3.1: Implement --list command

```bash
# --list
# Display formatted table of all releases

list_releases() {
    local manifest="releases/manifest.json"

    if [ ! -f "$manifest" ]; then
        echo "No releases found. Create one with: $0 --new \"Description\""
        exit 0
    fi

    echo ""
    echo "=== Production Releases ==="
    echo ""
    printf "%-12s %-20s %-10s %-40s\n" "VERSION" "DATE" "STATUS" "DESCRIPTION"
    printf "%-12s %-20s %-10s %-40s\n" "-------" "----" "------" "-----------"

    # Parse manifest and display releases
    jq -r '.releases[] | "\(.version)|\(.created)|\(.status)|\(.description)"' "$manifest" | \
    while IFS='|' read -r version created status description; do
        created_short=$(echo "$created" | cut -d'T' -f1)
        printf "%-12s %-20s %-10s %-40s\n" "$version" "$created_short" "$status" "$description"
    done

    echo ""
    echo "Current: $(jq -r '.current_release' "$manifest")"
    echo ""
}
```

#### Task 3.2: Implement --info command

```bash
# --info prod-002
# Display detailed information about a specific release

show_release_info() {
    local version="$1"
    local info_file="releases/${version}/release-info.json"

    if [ ! -f "$info_file" ]; then
        echo "Error: Release $version not found"
        exit 1
    fi

    echo ""
    echo "=== Release: $version ==="
    echo ""
    jq '.' "$info_file"
    echo ""

    # Show backup sizes
    echo "Backup Sizes:"
    ls -lh "releases/${version}/volumes/" 2>/dev/null || echo "  No volume backups"
    echo ""
}
```

---

### Phase 4: Release Script - Rollback (MEDIUM RISK)
*Estimated: 3 hours*
*Dependencies: Phase 2, Phase 3*

#### Task 4.1: Implement --rollback command

```bash
# --rollback prod-002 [--restore-redis]
# 1. Verify target release exists
# 2. Stop production containers
# 3. Restore PostgreSQL volume from backup
# 4. Optionally restore Redis from backup
# 5. Restore config files
# 6. Update manifest.json (set target as current)
# 7. Start production with target image
```

#### Task 4.2: Volume restore functions

```bash
restore_postgres_volume() {
    local release_dir="$1"
    local backup_file="${release_dir}/volumes/postgres-data.tar.gz"

    if [ ! -f "$backup_file" ]; then
        echo "Error: PostgreSQL backup not found: $backup_file"
        exit 1
    fi

    echo "Restoring PostgreSQL data from $release_dir..."

    # Stop postgres
    docker-compose -f docker-compose.prod.yml stop postgres

    # Clear existing data and restore
    docker run --rm \
        -v binance-trading-app_postgres_prod_data:/data \
        -v "$(pwd)/${backup_file}":/backup.tar.gz:ro \
        alpine sh -c "rm -rf /data/* && tar xzf /backup.tar.gz -C /data"

    # Start postgres
    docker-compose -f docker-compose.prod.yml start postgres

    echo "PostgreSQL restored successfully"
}

restore_redis_volume() {
    local release_dir="$1"
    local backup_file="${release_dir}/volumes/redis-shared-data.tar.gz"

    if [ ! -f "$backup_file" ]; then
        echo "Warning: Redis backup not found: $backup_file"
        echo "Skipping Redis restore"
        return 0
    fi

    echo "Restoring shared Redis data from $release_dir..."

    # Stop Redis
    docker-compose -f docker-compose.infra.yml stop redis

    # Clear existing data and restore
    docker run --rm \
        -v binance-infra_redis-data:/data \
        -v "$(pwd)/${backup_file}":/backup.tar.gz:ro \
        alpine sh -c "rm -rf /data/* && tar xzf /backup.tar.gz -C /data"

    # Start Redis
    docker-compose -f docker-compose.infra.yml start redis

    echo "Redis restored successfully"
}
```

#### Task 4.3: Config restore function

```bash
restore_configs() {
    local release_dir="$1"
    local configs_dir="${release_dir}/configs"

    if [ ! -d "$configs_dir" ]; then
        echo "Warning: No config backups found"
        return 0
    fi

    echo "Restoring configuration files..."

    # Restore each config file
    for config in "$configs_dir"/*; do
        local filename=$(basename "$config")
        cp "$config" "./$filename"
        echo "  Restored: $filename"
    done
}
```

#### Task 4.4: Rollback confirmation

```bash
confirm_rollback() {
    local version="$1"
    local restore_redis="$2"

    echo ""
    echo "=== ROLLBACK CONFIRMATION ==="
    echo ""
    echo "Target release: $version"
    echo "Restore PostgreSQL: YES"
    echo "Restore Redis: $( [ "$restore_redis" = "true" ] && echo "YES" || echo "NO (using current)" )"
    echo "Restore Configs: YES"
    echo ""
    echo "WARNING: This will:"
    echo "  - Stop production containers"
    echo "  - Replace current PostgreSQL data"
    [ "$restore_redis" = "true" ] && echo "  - Replace current Redis data"
    echo "  - Switch to Docker image: binance-bot:$version"
    echo ""
    read -p "Continue? (yes/no): " confirm

    if [ "$confirm" != "yes" ]; then
        echo "Rollback cancelled"
        exit 0
    fi
}
```

---

### Phase 5: Documentation (LOW RISK)
*Estimated: 1 hour*
*Dependencies: Phase 4*

#### Task 5.1: Create quick reference doc

File: `docs/production-releases.md`

```markdown
# Production Release Management

## Quick Reference

### Create New Release
./scripts/prod-release.sh --new "Description of changes"

### List All Releases
./scripts/prod-release.sh --list

### View Release Details
./scripts/prod-release.sh --info prod-002

### Rollback (Keep Current Redis)
./scripts/prod-release.sh --rollback prod-002

### Rollback (Restore Redis Too)
./scripts/prod-release.sh --rollback prod-002 --restore-redis

## Emergency Recovery

If production breaks:
1. Run: ./scripts/prod-release.sh --list
2. Pick a working release
3. Run: ./scripts/prod-release.sh --rollback prod-XXX
4. Trading resumes in ~60 seconds

## When to Restore Redis

| Scenario | Restore Redis? |
|----------|---------------|
| Code bug, cache structure unchanged | NO |
| Cache key structure changed | YES |
| Unknown issue | Try NO first, then YES |
```

#### Task 5.2: Update CLAUDE.md with release commands

Add section to CLAUDE.md about production releases.

---

## Acceptance Criteria

### AC9.7.1: Folder Structure
- [ ] `releases/` directory created
- [ ] `releases/` added to `.gitignore`
- [ ] `manifest.json` schema defined and working

### AC9.7.2: Create Release (--new)
- [ ] Creates versioned Docker image (binance-bot:prod-XXX)
- [ ] Backs up PostgreSQL volume
- [ ] Backs up shared Redis data
- [ ] Copies current config files
- [ ] Updates manifest.json
- [ ] Cleans up releases older than last 3
- [ ] Tags new image as :latest

### AC9.7.3: List Releases (--list)
- [ ] Shows all available releases in table format
- [ ] Displays version, date, status, description
- [ ] Indicates current active release

### AC9.7.4: Release Info (--info)
- [ ] Shows detailed info for specific release
- [ ] Displays git commit, branch, timestamp
- [ ] Shows backup file sizes

### AC9.7.5: Rollback (--rollback)
- [ ] Prompts for confirmation before proceeding
- [ ] Stops production containers
- [ ] Restores PostgreSQL from backup
- [ ] Optionally restores Redis (--restore-redis flag)
- [ ] Restores config files
- [ ] Starts production with target image
- [ ] Updates manifest.json current release

### AC9.7.6: Cleanup
- [ ] Automatically removes releases older than last 3
- [ ] Removes both Docker images and backup folders
- [ ] Logs what was removed

---

## Files to Create

| File | Description |
|------|-------------|
| `scripts/prod-release.sh` | Main release management script |
| `releases/.gitkeep` | Placeholder for releases directory |
| `docs/production-releases.md` | Quick reference documentation |

## Files to Modify

| File | Changes |
|------|---------|
| `.gitignore` | Add releases/ exclusion |
| `CLAUDE.md` | Add production release commands |

---

## Testing Strategy

### Manual Tests

#### Test 1: Create First Release
```bash
./scripts/prod-release.sh --new "Initial stable release"
# Verify:
# - releases/prod-001/ created
# - manifest.json created
# - Docker image binance-bot:prod-001 exists
# - Volume backups exist
```

#### Test 2: Create Multiple Releases
```bash
./scripts/prod-release.sh --new "Release 2"
./scripts/prod-release.sh --new "Release 3"
./scripts/prod-release.sh --new "Release 4"
# Verify:
# - Only prod-002, prod-003, prod-004 exist
# - prod-001 was cleaned up
# - Only 3 Docker images exist
```

#### Test 3: List Releases
```bash
./scripts/prod-release.sh --list
# Verify: Table shows all releases with correct info
```

#### Test 4: Rollback Without Redis
```bash
./scripts/prod-release.sh --rollback prod-002
# Verify:
# - Production runs with prod-002 image
# - PostgreSQL restored
# - Redis NOT touched
# - Current release updated in manifest
```

#### Test 5: Rollback With Redis
```bash
./scripts/prod-release.sh --rollback prod-002 --restore-redis
# Verify:
# - Production runs with prod-002 image
# - PostgreSQL restored
# - Redis restored
# - Current release updated in manifest
```

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| Backup corruption | Cannot restore | Verify backup integrity after creation |
| Volume naming mismatch | Wrong volume backed up | Use explicit volume names in script |
| Disk space | Large backups fill disk | Limit to 3 releases, warn if low space |
| Redis restore affects dev | Dev loses current cache | Warn user, document behavior |

---

## Usage Scenarios

### Scenario 1: Routine Production Update
```bash
# 1. Make code changes, test in dev
# 2. Create new production release
./scripts/prod-release.sh --new "Story 9.5 - Trend Filters"

# 3. Verify production is working
curl http://localhost:8095/health
```

### Scenario 2: Emergency Rollback
```bash
# Production is broken!

# 1. Check available releases
./scripts/prod-release.sh --list

# 2. Rollback to last known good
./scripts/prod-release.sh --rollback prod-002

# 3. Trading resumes in ~60 seconds
```

### Scenario 3: Cache Structure Changed
```bash
# New code changed Redis key structure
# Need full rollback including Redis

./scripts/prod-release.sh --rollback prod-002 --restore-redis
```

---

## Definition of Done

### Implementation
- [ ] All phases implemented
- [ ] All acceptance criteria met
- [ ] Documentation created

### Functional
- [ ] Can create new releases with description
- [ ] Can list all releases
- [ ] Can rollback to any of last 3 releases
- [ ] Optional Redis restore works
- [ ] Old releases auto-cleaned

### Testing
- [ ] All 5 manual tests pass
- [ ] Rollback tested with active trading

---

## Notes

- This system complements Story 9.6 (Active/Standby) for complete operational safety
- Redis backup is from shared infrastructure (affects both dev/prod if restored)
- User decides whether to restore Redis based on cache compatibility
- Releases folder can grow large - ensure adequate disk space
