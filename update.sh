#!/bin/bash
# Update script for Episod deployment
# Run this on your server after pushing changes to GitHub

set -e

echo "🔄 Updating Episod..."
cd ~/apps/episod

echo "📥 Pulling latest changes from GitHub..."
git pull origin main

echo "🐳 Rebuilding and restarting containers..."
docker compose -f docker-compose.prod.yml up -d --build

echo ""
echo "✅ Update complete!"
echo ""
echo "📊 Container status:"
docker compose -f docker-compose.prod.yml ps

echo ""
echo "💡 Useful commands:"
echo "  View logs:    docker compose -f docker-compose.prod.yml logs -f"
echo "  Restart:      docker compose -f docker-compose.prod.yml restart"
echo "  Stop:         docker compose -f docker-compose.prod.yml down"
echo "  Check health: curl https://video.xophie.ai/health"
