#!/bin/bash
# ============================================================================
# Binance Trading Bot - Build Script for Linux/Mac
# ============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN} Binance Trading Bot - Build Script${NC}"
echo -e "${GREEN}========================================${NC}"

# Change to project root
cd "$PROJECT_ROOT"

# Detect OS and Architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo -e "${RED}Unsupported architecture: $ARCH${NC}"; exit 1 ;;
esac

echo -e "${YELLOW}Detected: ${OS}/${ARCH}${NC}"

# Build version from git (if available)
VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

echo -e "${YELLOW}Version: ${VERSION}${NC}"
echo -e "${YELLOW}Build Time: ${BUILD_TIME}${NC}"

# Create dist directory
DIST_DIR="$PROJECT_ROOT/dist"
mkdir -p "$DIST_DIR"

# Build frontend first
echo -e "\n${YELLOW}Building frontend...${NC}"
cd "$PROJECT_ROOT/web"
if [ -f "package.json" ]; then
    npm install
    npm run build
    echo -e "${GREEN}Frontend built successfully!${NC}"
else
    echo -e "${RED}Warning: Frontend package.json not found${NC}"
fi
cd "$PROJECT_ROOT"

# Build backend
echo -e "\n${YELLOW}Building backend for ${OS}/${ARCH}...${NC}"

OUTPUT_NAME="trading-bot"
if [ "$OS" = "windows" ]; then
    OUTPUT_NAME="trading-bot.exe"
fi

LDFLAGS="-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH go build \
    -ldflags "$LDFLAGS" \
    -o "$DIST_DIR/$OUTPUT_NAME" \
    .

echo -e "${GREEN}Backend built successfully!${NC}"

# Copy required files
echo -e "\n${YELLOW}Copying distribution files...${NC}"
cp -r "$PROJECT_ROOT/web/dist" "$DIST_DIR/web/" 2>/dev/null || mkdir -p "$DIST_DIR/web/dist"
cp "$PROJECT_ROOT/.env.example" "$DIST_DIR/" 2>/dev/null || true
cp "$PROJECT_ROOT/config.json.example" "$DIST_DIR/" 2>/dev/null || true

# Create start script
cat > "$DIST_DIR/start.sh" << 'EOF'
#!/bin/bash
# Start the trading bot

# Check for .env file
if [ ! -f ".env" ]; then
    echo "Warning: .env file not found!"
    echo "Please copy .env.example to .env and configure it."
    exit 1
fi

# Load environment variables
set -a
source .env
set +a

# Start the bot
./trading-bot
EOF
chmod +x "$DIST_DIR/start.sh"

echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN} Build Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo -e "Output: ${DIST_DIR}"
echo -e "Binary: ${OUTPUT_NAME}"
echo -e ""
echo -e "To run:"
echo -e "  1. cd ${DIST_DIR}"
echo -e "  2. cp .env.example .env"
echo -e "  3. Edit .env with your settings"
echo -e "  4. ./start.sh"
