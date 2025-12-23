#!/bin/bash
# ============================================================================
# Binance Trading Bot - Cross-Platform Build Script
# Builds binaries for Windows, Linux, and macOS
# ============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo -e "${GREEN}================================================${NC}"
echo -e "${GREEN} Binance Trading Bot - Cross-Platform Build${NC}"
echo -e "${GREEN}================================================${NC}"

# Change to project root
cd "$PROJECT_ROOT"

# Build version from git (if available)
VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

echo -e "${YELLOW}Version: ${VERSION}${NC}"
echo -e "${YELLOW}Build Time: ${BUILD_TIME}${NC}"

# Create dist directory
DIST_DIR="$PROJECT_ROOT/dist"
rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

# Build frontend first
echo -e "\n${BLUE}Step 1: Building frontend...${NC}"
cd "$PROJECT_ROOT/web"
if [ -f "package.json" ]; then
    npm install --silent
    npm run build
    echo -e "${GREEN}Frontend built successfully!${NC}"
else
    echo -e "${RED}Warning: Frontend package.json not found${NC}"
fi
cd "$PROJECT_ROOT"

# Define platforms
PLATFORMS=(
    "windows/amd64"
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
)

LDFLAGS="-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -s -w"

echo -e "\n${BLUE}Step 2: Building binaries for all platforms...${NC}"

for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS="${PLATFORM%/*}"
    GOARCH="${PLATFORM#*/}"

    OUTPUT_NAME="trading-bot"
    if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME="trading-bot.exe"
    fi

    OUTPUT_DIR="$DIST_DIR/${GOOS}-${GOARCH}"
    mkdir -p "$OUTPUT_DIR"

    echo -e "${YELLOW}Building for ${GOOS}/${GOARCH}...${NC}"

    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags "$LDFLAGS" \
        -o "$OUTPUT_DIR/$OUTPUT_NAME" \
        . 2>&1 || {
            echo -e "${RED}Failed to build for ${GOOS}/${GOARCH}${NC}"
            continue
        }

    # Copy distribution files
    cp -r "$PROJECT_ROOT/web/dist" "$OUTPUT_DIR/web/" 2>/dev/null || mkdir -p "$OUTPUT_DIR/web/dist"
    cp "$PROJECT_ROOT/.env.example" "$OUTPUT_DIR/" 2>/dev/null || true
    cp "$PROJECT_ROOT/config.json.example" "$OUTPUT_DIR/" 2>/dev/null || true

    # Create appropriate start script
    if [ "$GOOS" = "windows" ]; then
        cat > "$OUTPUT_DIR/start.bat" << 'WINEOF'
@echo off
if not exist ".env" (
    echo Warning: .env file not found!
    echo Please copy .env.example to .env and configure it.
    exit /b 1
)
trading-bot.exe
WINEOF
    else
        cat > "$OUTPUT_DIR/start.sh" << 'UNIXEOF'
#!/bin/bash
if [ ! -f ".env" ]; then
    echo "Warning: .env file not found!"
    echo "Please copy .env.example to .env and configure it."
    exit 1
fi
set -a; source .env; set +a
./trading-bot
UNIXEOF
        chmod +x "$OUTPUT_DIR/start.sh"
        chmod +x "$OUTPUT_DIR/trading-bot"
    fi

    # Create archive
    ARCHIVE_NAME="trading-bot-${VERSION}-${GOOS}-${GOARCH}"
    echo -e "${YELLOW}Creating archive: ${ARCHIVE_NAME}...${NC}"

    cd "$DIST_DIR"
    if [ "$GOOS" = "windows" ]; then
        zip -r "${ARCHIVE_NAME}.zip" "${GOOS}-${GOARCH}" -x "*.DS_Store" > /dev/null
    else
        tar -czf "${ARCHIVE_NAME}.tar.gz" "${GOOS}-${GOARCH}"
    fi
    cd "$PROJECT_ROOT"

    echo -e "${GREEN}Built: ${GOOS}/${GOARCH}${NC}"
done

# Create checksums
echo -e "\n${BLUE}Step 3: Creating checksums...${NC}"
cd "$DIST_DIR"
sha256sum *.zip *.tar.gz 2>/dev/null > checksums.txt || true
cd "$PROJECT_ROOT"

echo -e "\n${GREEN}================================================${NC}"
echo -e "${GREEN} Build Complete!${NC}"
echo -e "${GREEN}================================================${NC}"
echo -e "Output directory: ${DIST_DIR}"
echo -e ""
echo -e "Archives created:"
ls -la "$DIST_DIR"/*.zip "$DIST_DIR"/*.tar.gz 2>/dev/null || echo "No archives found"
echo -e ""
echo -e "Checksums saved to: ${DIST_DIR}/checksums.txt"
