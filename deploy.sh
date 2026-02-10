#!/bin/bash
# Quick deployment script for Hetzner server
# Run this script on your server after cloning the repository

set -e  # Exit on any error

echo "🚀 Episod - Deployment Script"
echo "================================================"

# Check if .env file exists
if [ ! -f .env ]; then
    echo "❌ Error: .env file not found!"
    echo "Please create .env file with your configuration."
    echo "You can copy from .env.example: cp .env.example .env"
    exit 1
fi

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "❌ Error: Docker is not installed!"
    exit 1
fi

# Check if Docker Compose is installed
if ! docker compose version &> /dev/null; then
    echo "❌ Error: Docker Compose is not installed!"
    exit 1
fi

# Check if FFmpeg is installed
if ! command -v ffmpeg &> /dev/null; then
    echo "⚠️  Warning: FFmpeg is not installed!"
    echo "Install it with: apt install ffmpeg"
    read -p "Continue anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

echo ""
echo "📦 Building and starting Docker containers..."
docker compose -f docker-compose.prod.yml up -d --build

echo ""
echo "⏳ Waiting for services to be healthy..."
sleep 10

# Check if services are running
if docker compose -f docker-compose.prod.yml ps | grep -q "Up"; then
    echo "✅ Services are running!"
else
    echo "❌ Error: Services failed to start!"
    echo "Check logs with: docker compose -f docker-compose.prod.yml logs"
    exit 1
fi

echo ""
echo "🔍 Testing API health endpoint..."
for i in {1..10}; do
    if curl -s http://localhost:8080/health > /dev/null 2>&1; then
        echo "✅ API is responding!"
        break
    fi
    if [ $i -eq 10 ]; then
        echo "❌ API health check failed after 10 attempts"
        echo "Check logs with: docker compose -f docker-compose.prod.yml logs api"
        exit 1
    fi
    echo "   Attempt $i/10... waiting 3s"
    sleep 3
done

echo ""
echo "✅ Deployment successful!"
echo ""
echo "📋 Next steps:"
echo "1. Configure Nginx reverse proxy (see DEPLOYMENT.md Step 7)"
echo "2. Obtain SSL certificate with Certbot (see DEPLOYMENT.md Step 8)"
echo "3. Test your API at https://yourdomain.com/health"
echo ""
echo "📊 View logs: docker compose -f docker-compose.prod.yml logs -f"
echo "🔄 Restart: docker compose -f docker-compose.prod.yml restart"
echo "🛑 Stop: docker compose -f docker-compose.prod.yml down"
echo ""
