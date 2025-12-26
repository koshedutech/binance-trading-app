#!/bin/sh
# Development entrypoint script
# Builds and runs the application inside the container

set -e

echo "=== Development Mode ==="
echo "Building frontend..."
cd /app/web
npm install --silent 2>/dev/null || npm install
npm run build
echo "Frontend built successfully"

echo "Building Go application..."
cd /app
go build -o trading-bot main.go
echo "Go application built successfully"

echo "Starting application..."
exec ./trading-bot
