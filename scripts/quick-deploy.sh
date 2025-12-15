#!/bin/bash

# Quick deployment script for development
# This restarts services after code changes without rebuilding images

set -e

echo "🚀 Quick deployment starting..."

# Stop services
echo "⏹️  Stopping services..."
docker-compose down

# Start services with build
echo "▶️  Starting services..."
docker-compose up -d

# Wait for services to be healthy
echo "⏳ Waiting for services to be ready..."
sleep 5

# Check status
echo "📊 Service status:"
docker-compose ps

echo "✅ Deployment complete!"
echo "📊 Dashboard: http://localhost:8088"
echo "💾 Database: localhost:5433"
