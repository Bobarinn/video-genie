#!/bin/bash

# Setup script for Episod

set -e

echo "Episod - Setup Script"
echo "====================="

# Check prerequisites
echo -e "\nChecking prerequisites..."

# Check Go
if ! command -v go &> /dev/null; then
    echo "✗ Go is not installed. Please install Go 1.22+"
    exit 1
fi
echo "✓ Go $(go version | awk '{print $3}')"

# Check Docker
if ! command -v docker &> /dev/null; then
    echo "⚠ Docker is not installed (optional, but recommended)"
else
    echo "✓ Docker $(docker --version | awk '{print $3}' | tr -d ',')"
fi

# Check FFmpeg
if ! command -v ffmpeg &> /dev/null; then
    echo "⚠ FFmpeg is not installed. Installing is required for video rendering."
    echo "  macOS: brew install ffmpeg"
    echo "  Ubuntu: apt-get install ffmpeg"
else
    echo "✓ FFmpeg $(ffmpeg -version | head -n1 | awk '{print $3}')"
fi

# Check PostgreSQL
if ! command -v psql &> /dev/null; then
    echo "⚠ PostgreSQL client (psql) is not installed"
else
    echo "✓ PostgreSQL client installed"
fi

# Setup .env file
echo -e "\nSetting up configuration..."
if [ ! -f .env ]; then
    echo "Creating .env from .env.example..."
    cp .env.example .env
    echo "✓ Created .env file"
    echo "⚠ Please edit .env and add your API keys!"
else
    echo "✓ .env file already exists"
fi

# Install Go dependencies
echo -e "\nInstalling Go dependencies..."
go mod download
go mod tidy
echo "✓ Dependencies installed"

# Create temp directory
echo -e "\nCreating temp directory..."
mkdir -p /tmp/episod
echo "✓ Temp directory created"

# Check if running with Docker
if command -v docker-compose &> /dev/null; then
    echo -e "\n========================================"
    echo "Setup complete! You can now:"
    echo ""
    echo "Option 1 - Run with Docker (recommended):"
    echo "  1. Edit .env with your API keys"
    echo "  2. Run: make docker-up"
    echo "  3. View logs: make docker-logs"
    echo ""
    echo "Option 2 - Run locally:"
    echo "  1. Start PostgreSQL and Redis"
    echo "  2. Run migrations: make migrate"
    echo "  3. Edit .env with your API keys"
    echo "  4. Run: make run"
    echo ""
    echo "Option 3 - Development mode:"
    echo "  1. Install air: go install github.com/cosmtrek/air@latest"
    echo "  2. Run: make dev"
    echo ""
    echo "Test the API:"
    echo "  ./scripts/test-api.sh"
    echo "========================================"
else
    echo -e "\n========================================"
    echo "Setup complete! Next steps:"
    echo ""
    echo "1. Start PostgreSQL and Redis"
    echo "2. Run migrations: make migrate"
    echo "3. Edit .env with your API keys"
    echo "4. Run: make run"
    echo ""
    echo "Or install Docker Compose for easier setup."
    echo "========================================"
fi
