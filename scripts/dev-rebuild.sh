#!/bin/bash

# Development rebuild script - rebuilds code without rebuilding Docker images
# Usage: ./scripts/dev-rebuild.sh

set -e

echo "🔨 Rebuilding application..."

# Build frontend
echo "📦 Building React frontend..."
cd web
npm run build
cd ..

# Build Go backend
echo "⚙️  Building Go backend..."
docker exec binance-trading-bot sh -c "cd /app && go build -o trading-bot main.go"

echo "✅ Build complete! Restarting services..."

# Restart containers
docker-compose restart trading-bot

echo "🚀 Application restarted successfully!"
echo "📊 Dashboard: http://localhost:8088"
