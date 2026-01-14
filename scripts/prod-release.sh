#!/bin/bash
#
# Production Release Management Script
# =====================================
# Manages versioned production releases with rollback capability.
#
# Usage:
#   ./scripts/prod-release.sh --new "Description of this release"
#   ./scripts/prod-release.sh --list
#   ./scripts/prod-release.sh --info prod-002
#   ./scripts/prod-release.sh --rollback prod-002
#   ./scripts/prod-release.sh --rollback prod-002 --restore-redis
#
# Features:
#   - Sequential versioning (prod-001, prod-002, ...)
#   - Keeps last 3 releases
#   - Backs up PostgreSQL + Redis + configs
#   - Quick rollback (~60 seconds)
#

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
RELEASES_DIR="${PROJECT_DIR}/releases"
MANIFEST_FILE="${RELEASES_DIR}/manifest.json"
MAX_RELEASES=3

# Docker compose project names (from 'name:' in compose files)
PROD_PROJECT="binance-prod"
INFRA_PROJECT="binance-infra"

# Volume names (project_volume format)
POSTGRES_VOLUME="${PROD_PROJECT}_postgres-data"
REDIS_VOLUME="${INFRA_PROJECT}_redis-data"

# Docker image name
DOCKER_IMAGE="binance-trading-bot"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ============================================================================
# Helper Functions
# ============================================================================

print_header() {
    echo ""
    echo -e "${BLUE}=== $1 ===${NC}"
    echo ""
}

print_success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

# Initialize releases directory and manifest
init_releases() {
    mkdir -p "$RELEASES_DIR"

    if [ ! -f "$MANIFEST_FILE" ]; then
        cat > "$MANIFEST_FILE" << 'EOF'
{
  "schema_version": 1,
  "current_release": null,
  "max_releases": 3,
  "releases": []
}
EOF
        print_info "Initialized releases manifest"
    fi
}

# Get next version number (pure bash - no jq)
get_next_version() {
    local max_num=0

    # Look for existing release directories
    for dir in "${RELEASES_DIR}"/prod-*; do
        if [ -d "$dir" ]; then
            local ver=$(basename "$dir")
            local num=$(echo "$ver" | sed 's/prod-0*//')
            if [ -n "$num" ] && [ "$num" -gt "$max_num" ] 2>/dev/null; then
                max_num=$num
            fi
        fi
    done

    local next_num=$((max_num + 1))
    printf "prod-%03d" "$next_num"
}

# Get current git info
get_git_info() {
    local commit=$(git -C "$PROJECT_DIR" rev-parse --short HEAD 2>/dev/null || echo "unknown")
    local branch=$(git -C "$PROJECT_DIR" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
    echo "${commit}|${branch}"
}

# Read a value from release-info.json (simple grep-based)
read_release_info() {
    local file="$1"
    local key="$2"
    grep "\"${key}\"" "$file" 2>/dev/null | sed 's/.*: *"\([^"]*\)".*/\1/' | head -1
}

# ============================================================================
# Backup Functions
# ============================================================================

backup_postgres() {
    local release_dir="$1"
    local backup_file="${release_dir}/volumes/postgres-data.tar.gz"

    print_info "Backing up PostgreSQL data..."

    # Check if volume exists
    if ! docker volume inspect "$POSTGRES_VOLUME" &>/dev/null; then
        print_warning "PostgreSQL volume not found: $POSTGRES_VOLUME"
        return 1
    fi

    mkdir -p "${release_dir}/volumes"

    # Stop postgres for consistent backup
    print_info "Stopping PostgreSQL for consistent backup..."
    docker-compose -f "${PROJECT_DIR}/docker-compose.prod.yml" stop postgres 2>/dev/null || true
    sleep 2

    # Create backup
    docker run --rm \
        -v "${POSTGRES_VOLUME}:/data:ro" \
        -v "${release_dir}/volumes:/backup" \
        alpine tar czf /backup/postgres-data.tar.gz -C /data .

    # Restart postgres
    docker-compose -f "${PROJECT_DIR}/docker-compose.prod.yml" start postgres 2>/dev/null || true

    local size=$(du -h "$backup_file" | cut -f1)
    print_success "PostgreSQL backup created: $size"
}

backup_redis() {
    local release_dir="$1"
    local backup_file="${release_dir}/volumes/redis-shared-data.tar.gz"

    print_info "Backing up shared Redis data..."

    # Check if volume exists
    if ! docker volume inspect "$REDIS_VOLUME" &>/dev/null; then
        print_warning "Redis volume not found: $REDIS_VOLUME"
        return 0
    fi

    mkdir -p "${release_dir}/volumes"

    # Trigger Redis background save
    docker exec binance-bot-redis redis-cli BGSAVE 2>/dev/null || true
    sleep 3  # Wait for save to complete

    # Create backup
    docker run --rm \
        -v "${REDIS_VOLUME}:/data:ro" \
        -v "${release_dir}/volumes:/backup" \
        alpine tar czf /backup/redis-shared-data.tar.gz -C /data .

    local size=$(du -h "$backup_file" | cut -f1)
    print_success "Redis backup created: $size"
}

backup_configs() {
    local release_dir="$1"
    local configs_dir="${release_dir}/configs"

    print_info "Backing up configuration files..."

    mkdir -p "$configs_dir"

    # List of config files to backup
    local configs=("default-settings.json" "autopilot_settings.json" "config.json")

    for config in "${configs[@]}"; do
        if [ -f "${PROJECT_DIR}/${config}" ]; then
            cp "${PROJECT_DIR}/${config}" "${configs_dir}/${config}"
            print_success "Backed up: $config"
        fi
    done
}

# ============================================================================
# Restore Functions
# ============================================================================

restore_postgres() {
    local release_dir="$1"
    local backup_file="${release_dir}/volumes/postgres-data.tar.gz"

    if [ ! -f "$backup_file" ]; then
        print_error "PostgreSQL backup not found: $backup_file"
        return 1
    fi

    print_info "Restoring PostgreSQL data..."

    # Stop postgres
    docker-compose -f "${PROJECT_DIR}/docker-compose.prod.yml" stop postgres 2>/dev/null || true
    sleep 2

    # Clear and restore
    docker run --rm \
        -v "${POSTGRES_VOLUME}:/data" \
        -v "${backup_file}:/backup.tar.gz:ro" \
        alpine sh -c "rm -rf /data/* && tar xzf /backup.tar.gz -C /data"

    print_success "PostgreSQL restored"
}

restore_redis() {
    local release_dir="$1"
    local backup_file="${release_dir}/volumes/redis-shared-data.tar.gz"

    if [ ! -f "$backup_file" ]; then
        print_warning "Redis backup not found, skipping Redis restore"
        return 0
    fi

    print_info "Restoring shared Redis data..."
    print_warning "This will affect BOTH dev and prod environments!"

    # Stop Redis
    docker-compose -f "${PROJECT_DIR}/docker-compose.infra.yml" stop redis 2>/dev/null || true
    sleep 2

    # Clear and restore
    docker run --rm \
        -v "${REDIS_VOLUME}:/data" \
        -v "${backup_file}:/backup.tar.gz:ro" \
        alpine sh -c "rm -rf /data/* && tar xzf /backup.tar.gz -C /data"

    # Start Redis
    docker-compose -f "${PROJECT_DIR}/docker-compose.infra.yml" start redis 2>/dev/null || true

    print_success "Redis restored"
}

restore_configs() {
    local release_dir="$1"
    local configs_dir="${release_dir}/configs"

    if [ ! -d "$configs_dir" ]; then
        print_warning "No config backups found"
        return 0
    fi

    print_info "Restoring configuration files..."

    for config in "$configs_dir"/*; do
        if [ -f "$config" ]; then
            local filename=$(basename "$config")
            cp "$config" "${PROJECT_DIR}/${filename}"
            print_success "Restored: $filename"
        fi
    done
}

# ============================================================================
# Cleanup Functions
# ============================================================================

cleanup_old_releases() {
    print_info "Checking for old releases to clean up..."

    # Get sorted list of release directories
    local releases=($(ls -d ${RELEASES_DIR}/prod-* 2>/dev/null | sort -V))
    local count=${#releases[@]}

    if [ $count -le $MAX_RELEASES ]; then
        print_info "No cleanup needed ($count releases, max $MAX_RELEASES)"
        return 0
    fi

    local to_delete=$((count - MAX_RELEASES))

    for ((i=0; i<to_delete; i++)); do
        local old_release="${releases[$i]}"
        local old_version=$(basename "$old_release")

        print_warning "Removing old release: $old_version"

        # Remove Docker image
        docker rmi "${DOCKER_IMAGE}:${old_version}" 2>/dev/null || true

        # Remove release folder
        rm -rf "$old_release"

        print_success "Removed: $old_version"
    done
}

# ============================================================================
# Manifest Functions (pure bash - no jq)
# ============================================================================

# Update manifest with new release
update_manifest_add_release() {
    local version="$1"
    local timestamp="$2"
    local git_commit="$3"
    local git_branch="$4"
    local description="$5"
    local docker_image="$6"

    # Create new manifest
    cat > "$MANIFEST_FILE" << EOF
{
  "schema_version": 1,
  "current_release": "${version}",
  "max_releases": 3,
  "releases": [
EOF

    # Add existing releases (mark as previous)
    local first=true
    for dir in $(ls -d ${RELEASES_DIR}/prod-* 2>/dev/null | sort -V); do
        local ver=$(basename "$dir")
        local info_file="${dir}/release-info.json"

        if [ -f "$info_file" ]; then
            local ts=$(read_release_info "$info_file" "created")
            local commit=$(read_release_info "$info_file" "git_commit")
            local branch=$(read_release_info "$info_file" "git_branch")
            local desc=$(read_release_info "$info_file" "description")
            local img=$(read_release_info "$info_file" "docker_image")

            local status="previous"
            if [ "$ver" = "$version" ]; then
                status="active"
            fi

            if [ "$first" = true ]; then
                first=false
            else
                echo "," >> "$MANIFEST_FILE"
            fi

            cat >> "$MANIFEST_FILE" << EOF
    {
      "version": "${ver}",
      "created": "${ts}",
      "git_commit": "${commit}",
      "git_branch": "${branch}",
      "description": "${desc}",
      "docker_image": "${img}",
      "status": "${status}"
    }
EOF
        fi
    done

    # Close releases array and JSON
    cat >> "$MANIFEST_FILE" << EOF

  ]
}
EOF
}

# Update manifest for rollback (set active release)
update_manifest_set_active() {
    local active_version="$1"

    # Create new manifest
    cat > "$MANIFEST_FILE" << EOF
{
  "schema_version": 1,
  "current_release": "${active_version}",
  "max_releases": 3,
  "releases": [
EOF

    # Add all releases
    local first=true
    for dir in $(ls -d ${RELEASES_DIR}/prod-* 2>/dev/null | sort -V); do
        local ver=$(basename "$dir")
        local info_file="${dir}/release-info.json"

        if [ -f "$info_file" ]; then
            local ts=$(read_release_info "$info_file" "created")
            local commit=$(read_release_info "$info_file" "git_commit")
            local branch=$(read_release_info "$info_file" "git_branch")
            local desc=$(read_release_info "$info_file" "description")
            local img=$(read_release_info "$info_file" "docker_image")

            local status="previous"
            if [ "$ver" = "$active_version" ]; then
                status="active"
            fi

            if [ "$first" = true ]; then
                first=false
            else
                echo "," >> "$MANIFEST_FILE"
            fi

            cat >> "$MANIFEST_FILE" << EOF
    {
      "version": "${ver}",
      "created": "${ts}",
      "git_commit": "${commit}",
      "git_branch": "${branch}",
      "description": "${desc}",
      "docker_image": "${img}",
      "status": "${status}"
    }
EOF
        fi
    done

    # Close releases array and JSON
    cat >> "$MANIFEST_FILE" << EOF

  ]
}
EOF
}

# ============================================================================
# Command: --new
# ============================================================================

cmd_new() {
    local description="$1"

    if [ -z "$description" ]; then
        print_error "Description required. Usage: $0 --new \"Description\""
        exit 1
    fi

    print_header "Creating New Production Release"

    # Initialize if needed
    init_releases

    # Get version and git info
    local version=$(get_next_version)
    local git_info=$(get_git_info)
    local git_commit=$(echo "$git_info" | cut -d'|' -f1)
    local git_branch=$(echo "$git_info" | cut -d'|' -f2)
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    local release_dir="${RELEASES_DIR}/${version}"

    echo "Version:     $version"
    echo "Git Commit:  $git_commit"
    echo "Git Branch:  $git_branch"
    echo "Description: $description"
    echo ""

    # Create release directory
    mkdir -p "$release_dir"

    # Step 1: Backup current state (if production exists)
    if docker volume inspect "$POSTGRES_VOLUME" &>/dev/null; then
        print_header "Step 1: Backing Up Current State"
        backup_postgres "$release_dir"
        backup_redis "$release_dir"
        backup_configs "$release_dir"
    else
        print_info "No existing production volume found, skipping backups"
        mkdir -p "${release_dir}/volumes"
        mkdir -p "${release_dir}/configs"
    fi

    # Step 2: Build new Docker image
    print_header "Step 2: Building Docker Image"
    print_info "Building ${DOCKER_IMAGE}:${version}..."

    docker build -t "${DOCKER_IMAGE}:${version}" "$PROJECT_DIR"
    docker tag "${DOCKER_IMAGE}:${version}" "${DOCKER_IMAGE}:latest"

    print_success "Built: ${DOCKER_IMAGE}:${version}"
    print_success "Tagged: ${DOCKER_IMAGE}:latest"

    # Step 3: Create release info
    print_header "Step 3: Creating Release Metadata"

    cat > "${release_dir}/release-info.json" << EOF
{
  "version": "${version}",
  "created": "${timestamp}",
  "git_commit": "${git_commit}",
  "git_branch": "${git_branch}",
  "description": "${description}",
  "docker_image": "${DOCKER_IMAGE}:${version}",
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
EOF

    print_success "Created release-info.json"

    # Step 4: Update manifest
    update_manifest_add_release "$version" "$timestamp" "$git_commit" "$git_branch" "$description" "${DOCKER_IMAGE}:${version}"
    print_success "Manifest updated"

    # Step 5: Cleanup old releases
    print_header "Step 4: Cleanup Old Releases"
    cleanup_old_releases

    # Step 6: Restart production with new image
    print_header "Step 5: Starting Production"

    cd "$PROJECT_DIR"
    docker-compose -f docker-compose.prod.yml down 2>/dev/null || true
    docker-compose -f docker-compose.prod.yml up -d

    # Wait for health check
    print_info "Waiting for production to be healthy..."
    sleep 10

    if curl -s "http://localhost:8095/health" | grep -q "ok\|healthy"; then
        print_success "Production is healthy"
    else
        print_warning "Health check not confirmed, please verify manually"
    fi

    print_header "Release Complete"
    echo ""
    echo "Version:     $version"
    echo "Image:       ${DOCKER_IMAGE}:${version}"
    echo "Description: $description"
    echo ""
    print_success "Production release $version created successfully!"
}

# ============================================================================
# Command: --list
# ============================================================================

cmd_list() {
    print_header "Production Releases"

    # Count releases
    local release_count=$(ls -d ${RELEASES_DIR}/prod-* 2>/dev/null | wc -l)

    if [ "$release_count" -eq 0 ]; then
        echo "No releases found. Create one with:"
        echo "  $0 --new \"Description\""
        exit 0
    fi

    # Print header
    printf "${BLUE}%-12s %-12s %-10s %-10s %-40s${NC}\n" "VERSION" "DATE" "COMMIT" "STATUS" "DESCRIPTION"
    printf "%-12s %-12s %-10s %-10s %-40s\n" "-------" "----" "------" "------" "-----------"

    # Print releases (sorted, newest first)
    for dir in $(ls -d ${RELEASES_DIR}/prod-* 2>/dev/null | sort -Vr); do
        local ver=$(basename "$dir")
        local info_file="${dir}/release-info.json"

        if [ -f "$info_file" ]; then
            local created=$(read_release_info "$info_file" "created")
            local commit=$(read_release_info "$info_file" "git_commit")
            local status=$(read_release_info "$info_file" "status")
            local desc=$(read_release_info "$info_file" "description")

            local created_short=$(echo "$created" | cut -d'T' -f1)

            # Color based on status
            if [ "$status" = "active" ]; then
                printf "${GREEN}%-12s${NC} %-12s %-10s ${GREEN}%-10s${NC} %-40s\n" "$ver" "$created_short" "$commit" "$status" "$desc"
            else
                printf "%-12s %-12s %-10s %-10s %-40s\n" "$ver" "$created_short" "$commit" "$status" "$desc"
            fi
        fi
    done

    echo ""
    # Get current from manifest or find active
    local current="none"
    if [ -f "$MANIFEST_FILE" ]; then
        current=$(grep '"current_release"' "$MANIFEST_FILE" | sed 's/.*: *"\([^"]*\)".*/\1/' | head -1)
    fi
    echo -e "Current active: ${GREEN}${current}${NC}"
    echo ""
}

# ============================================================================
# Command: --info
# ============================================================================

cmd_info() {
    local version="$1"

    if [ -z "$version" ]; then
        print_error "Version required. Usage: $0 --info prod-002"
        exit 1
    fi

    local info_file="${RELEASES_DIR}/${version}/release-info.json"

    if [ ! -f "$info_file" ]; then
        print_error "Release not found: $version"
        exit 1
    fi

    print_header "Release: $version"

    # Display release info
    echo "Details:"
    echo "  Created:     $(read_release_info "$info_file" "created")"
    echo "  Git Commit:  $(read_release_info "$info_file" "git_commit")"
    echo "  Git Branch:  $(read_release_info "$info_file" "git_branch")"
    echo "  Status:      $(read_release_info "$info_file" "status")"
    echo "  Description: $(read_release_info "$info_file" "description")"
    echo "  Docker:      $(read_release_info "$info_file" "docker_image")"

    echo ""
    echo "Backup Sizes:"

    local volumes_dir="${RELEASES_DIR}/${version}/volumes"
    if [ -d "$volumes_dir" ]; then
        ls -lh "$volumes_dir" 2>/dev/null | tail -n +2 | awk '{print "  " $9 ": " $5}'
    else
        echo "  No volume backups"
    fi

    local configs_dir="${RELEASES_DIR}/${version}/configs"
    echo ""
    echo "Config Files:"
    if [ -d "$configs_dir" ]; then
        ls "$configs_dir" 2>/dev/null | while read f; do echo "  $f"; done
    else
        echo "  No config backups"
    fi

    echo ""
}

# ============================================================================
# Command: --rollback
# ============================================================================

cmd_rollback() {
    local version="$1"
    local restore_redis="$2"

    if [ -z "$version" ]; then
        print_error "Version required. Usage: $0 --rollback prod-002 [--restore-redis]"
        exit 1
    fi

    local release_dir="${RELEASES_DIR}/${version}"

    if [ ! -d "$release_dir" ]; then
        print_error "Release not found: $version"
        echo ""
        echo "Available releases:"
        cmd_list
        exit 1
    fi

    # Confirmation
    print_header "ROLLBACK CONFIRMATION"

    echo "Target Release: $version"
    echo ""

    local info_file="${release_dir}/release-info.json"
    if [ -f "$info_file" ]; then
        echo "Release Info:"
        echo "  Created:     $(read_release_info "$info_file" "created")"
        echo "  Git Commit:  $(read_release_info "$info_file" "git_commit")"
        echo "  Description: $(read_release_info "$info_file" "description")"
    fi

    echo ""
    echo "Actions to perform:"
    echo "  - Stop production containers"
    echo "  - Restore PostgreSQL from backup"
    if [ "$restore_redis" = "true" ]; then
        echo -e "  - ${YELLOW}Restore Redis from backup (affects dev too!)${NC}"
    else
        echo "  - Keep current Redis (NOT restoring)"
    fi
    echo "  - Restore configuration files"
    echo "  - Start production with: ${DOCKER_IMAGE}:${version}"
    echo ""

    read -p "Continue with rollback? (yes/no): " confirm

    if [ "$confirm" != "yes" ]; then
        print_warning "Rollback cancelled"
        exit 0
    fi

    # Perform rollback
    print_header "Performing Rollback to $version"

    # Step 1: Stop production
    print_info "Stopping production..."
    cd "$PROJECT_DIR"
    docker-compose -f docker-compose.prod.yml down 2>/dev/null || true

    # Step 2: Restore PostgreSQL
    print_header "Restoring PostgreSQL"
    restore_postgres "$release_dir"

    # Step 3: Optionally restore Redis
    if [ "$restore_redis" = "true" ]; then
        print_header "Restoring Redis"
        restore_redis "$release_dir"
    else
        print_info "Keeping current Redis state"
    fi

    # Step 4: Restore configs
    print_header "Restoring Configurations"
    restore_configs "$release_dir"

    # Step 5: Tag the version as latest and start
    print_header "Starting Production with $version"

    docker tag "${DOCKER_IMAGE}:${version}" "${DOCKER_IMAGE}:latest" 2>/dev/null || true

    docker-compose -f docker-compose.prod.yml up -d

    # Step 6: Update manifest
    update_manifest_set_active "$version"

    # Also update the release-info.json status
    if [ -f "$info_file" ]; then
        sed -i 's/"status": "previous"/"status": "active"/' "$info_file"
    fi

    # Wait for health
    print_info "Waiting for production to be healthy..."
    sleep 10

    if curl -s "http://localhost:8095/health" | grep -q "ok\|healthy"; then
        print_success "Production is healthy"
    else
        print_warning "Health check not confirmed, please verify manually"
    fi

    print_header "Rollback Complete"
    echo ""
    echo "Production is now running: $version"
    echo ""
    print_success "Rollback to $version completed successfully!"
}

# ============================================================================
# Command: --help
# ============================================================================

cmd_help() {
    echo ""
    echo "Production Release Management"
    echo "=============================="
    echo ""
    echo "Usage:"
    echo "  $0 --new \"Description\"              Create a new production release"
    echo "  $0 --list                            List all releases"
    echo "  $0 --info prod-XXX                   Show details of a release"
    echo "  $0 --rollback prod-XXX               Rollback to a release (keep current Redis)"
    echo "  $0 --rollback prod-XXX --restore-redis  Rollback with Redis restore"
    echo "  $0 --help                            Show this help"
    echo ""
    echo "Examples:"
    echo "  $0 --new \"Story 9.5 - Trend Filters\""
    echo "  $0 --rollback prod-002"
    echo "  $0 --rollback prod-001 --restore-redis"
    echo ""
    echo "Emergency Recovery:"
    echo "  1. $0 --list                         # See available releases"
    echo "  2. $0 --rollback prod-XXX            # Restore working version"
    echo "  Trading resumes in ~60 seconds"
    echo ""
}

# ============================================================================
# Main
# ============================================================================

main() {
    cd "$PROJECT_DIR"

    case "${1:-}" in
        --new)
            cmd_new "$2"
            ;;
        --list)
            cmd_list
            ;;
        --info)
            cmd_info "$2"
            ;;
        --rollback)
            local restore_redis="false"
            if [ "${3:-}" = "--restore-redis" ]; then
                restore_redis="true"
            fi
            cmd_rollback "$2" "$restore_redis"
            ;;
        --help|-h|"")
            cmd_help
            ;;
        *)
            print_error "Unknown command: $1"
            cmd_help
            exit 1
            ;;
    esac
}

main "$@"
