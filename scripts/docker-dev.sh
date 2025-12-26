#!/bin/bash
# ============================================================================
# Binance Trading Bot - Docker Development Script
# ============================================================================
# Development workflow:
#   - Docker image built ONCE (contains Go, npm, build tools)
#   - Source code mounted via volumes
#   - Container builds app on startup
#   - To apply code changes, just restart container
#
# Usage:
#   ./scripts/docker-dev.sh              # Restart containers (rebuilds app)
#   ./scripts/docker-dev.sh --logs       # Just show logs
#   ./scripts/docker-dev.sh --down       # Stop containers
#   ./scripts/docker-dev.sh --build-image # Force rebuild Docker image (rare)
# ============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
COMPOSE_FILE="docker-compose.yml"
SERVICE_NAME="trading-bot"
LOGS_ONLY=false
DOWN_ONLY=false
DETACHED=false
BUILD_IMAGE=false

# Get script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --logs)
            LOGS_ONLY=true
            shift
            ;;
        --down)
            DOWN_ONLY=true
            shift
            ;;
        -d|--detached)
            DETACHED=true
            shift
            ;;
        --prod)
            COMPOSE_FILE="docker-compose.prod.yml"
            shift
            ;;
        --build-image)
            BUILD_IMAGE=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --logs         Just show logs"
            echo "  --down         Stop containers"
            echo "  -d, --detached Run in detached mode"
            echo "  --prod         Use production compose file"
            echo "  --build-image  Rebuild Docker image (rarely needed)"
            echo "  --help, -h     Show this help message"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

cd "$PROJECT_ROOT"

echo -e "${BLUE}╔══════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║           Binance Trading Bot - Docker Dev Script                ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${CYAN}Using compose file: ${COMPOSE_FILE}${NC}"
echo ""

# ============================================================================
# Handle --logs only
# ============================================================================
if [ "$LOGS_ONLY" = true ]; then
    echo -e "${YELLOW}Showing logs for ${SERVICE_NAME}...${NC}"
    docker-compose -f "$COMPOSE_FILE" logs -f "$SERVICE_NAME"
    exit 0
fi

# ============================================================================
# Handle --down only
# ============================================================================
if [ "$DOWN_ONLY" = true ]; then
    echo -e "${YELLOW}Stopping containers...${NC}"
    docker-compose -f "$COMPOSE_FILE" down
    echo -e "${GREEN}Containers stopped${NC}"
    exit 0
fi

# ============================================================================
# Build Docker image (only if --build-image flag or image doesn't exist)
# ============================================================================
IMAGE_EXISTS=$(docker images -q binance-trading-bot-trading-bot 2>/dev/null)

if [ "$BUILD_IMAGE" = true ] || [ -z "$IMAGE_EXISTS" ]; then
    echo -e "${YELLOW}Building Docker image (one-time setup)...${NC}"
    docker-compose -f "$COMPOSE_FILE" build
    echo -e "${GREEN}Docker image built${NC}"
else
    echo -e "${GREEN}Docker image exists, skipping build${NC}"
    echo -e "${CYAN}(use --build-image to force rebuild)${NC}"
fi

# ============================================================================
# Restart containers (app builds inside container)
# ============================================================================
echo -e "${YELLOW}Restarting containers...${NC}"
echo -e "${CYAN}(App will build inside container on startup)${NC}"
docker-compose -f "$COMPOSE_FILE" down 2>/dev/null || true

if [ "$DETACHED" = true ]; then
    docker-compose -f "$COMPOSE_FILE" up -d
    echo ""
    echo -e "${GREEN}Containers started in detached mode${NC}"
    echo -e "${BLUE}Development: http://localhost:8094${NC}"
    echo -e "${CYAN}Use --logs to see output${NC}"
else
    echo ""
    echo -e "${BLUE}Starting containers with logs...${NC}"
    echo -e "${BLUE}Development: http://localhost:8094${NC}"
    echo -e "${YELLOW}Press Ctrl+C to stop${NC}"
    echo ""
    docker-compose -f "$COMPOSE_FILE" up
fi
