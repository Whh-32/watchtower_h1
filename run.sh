#!/bin/bash

# Watchtower Quick Start Script

set -e

echo "🛡️  Watchtower - Bug Bounty Asset Discovery & Monitoring"
echo "=========================================================="
echo ""

# Check if HackerOne token is set
if [ -z "$HACKERONE_TOKEN" ] && [ ! -f ".hackerone_token" ]; then
    echo "⚠️  Warning: HACKERONE_TOKEN not set!"
    echo "   Please set it with: export HACKERONE_TOKEN='your-token'"
    echo "   Or create a file: echo 'your-token' > .hackerone_token"
    echo ""
fi

# Check if subfinder is installed
if ! command -v subfinder &> /dev/null; then
    echo "⚠️  Warning: subfinder not found in PATH"
    echo "   Install it with: go install -v github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest"
    echo "   Or run: make install-subfinder"
    echo ""
fi

# Build the application
echo "📦 Building application..."
go build -o watchtower main.go

echo ""
echo "✅ Build successful!"
echo ""
echo "🚀 Starting Watchtower..."
echo "   Web interface will be available at: http://localhost:${WEB_PORT:-8080}"
echo ""

# Run the application
./watchtower
