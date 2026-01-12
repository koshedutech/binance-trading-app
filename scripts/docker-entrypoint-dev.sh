#!/bin/sh
# Development entrypoint script
# Builds and runs the application inside the container

set -e

echo "=== Development Mode ==="

# Skip frontend build if SKIP_FRONTEND=1, or use existing if dist exists (unless SKIP_FRONTEND=0)
if [ "$SKIP_FRONTEND" = "1" ] || [ "$SKIP_FRONTEND" = "true" ]; then
    echo "Skipping frontend build (SKIP_FRONTEND=$SKIP_FRONTEND)"
elif [ "$SKIP_FRONTEND" = "0" ]; then
    echo "Force rebuilding frontend (SKIP_FRONTEND=0)..."
    cd /app/web
    npm install --silent 2>/dev/null || npm install
    npm run build
    echo "Frontend built successfully"
elif [ -f "/app/web/dist/index.html" ]; then
    echo "Frontend already built, skipping (set SKIP_FRONTEND=0 to force rebuild)"
else
    echo "Building frontend..."
    cd /app/web
    npm install --silent 2>/dev/null || npm install
    npm run build
    echo "Frontend built successfully"
fi

echo "Building Go application..."
cd /app
# Build to /tmp to avoid WSL2 volume mount issues causing segfaults
go build -o /tmp/trading-bot main.go
echo "Go application built successfully"

echo "Starting application..."
exec /tmp/trading-bot
