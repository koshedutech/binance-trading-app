#!/bin/sh
# Development entrypoint script
# Builds and runs the application inside the container

set -e

echo "=== Development Mode ==="

# Skip frontend build if SKIP_FRONTEND=1 or if dist/index.html exists and is recent
if [ "$SKIP_FRONTEND" = "1" ] || [ "$SKIP_FRONTEND" = "true" ]; then
    echo "Skipping frontend build (SKIP_FRONTEND=$SKIP_FRONTEND)"
elif [ -f "/app/web/dist/index.html" ]; then
    echo "Frontend already built, skipping (use SKIP_FRONTEND=0 to force rebuild)"
else
    echo "Building frontend..."
    cd /app/web
    npm install --silent 2>/dev/null || npm install
    npm run build
    echo "Frontend built successfully"
fi

echo "Building Go application..."
cd /app
go build -o trading-bot main.go
echo "Go application built successfully"

echo "Starting application..."
exec ./trading-bot
