#!/bin/sh
# Don't use set -e, we want to continue even if some installations fail

echo "🚀 Watchtower - Starting up..."

# Set up Go environment
export GOPATH=/root/go
export PATH="/root/go/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

# Check if HackerOne token is set
if [ -z "$HACKERONE_TOKEN" ] && [ ! -f "/app/.hackerone_token" ]; then
    echo "⚠️  WARNING: HACKERONE_TOKEN not set!"
    echo "   Please set it via environment variable or mount .hackerone_token file"
    echo "   The application will start but scans will fail without a valid token"
fi

# Ensure data directory exists
mkdir -p /app/data

# Check if subfinder is installed
if ! command -v subfinder &> /dev/null; then
    echo "📦 Installing subfinder..."
    if go install -v github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest 2>&1; then
        if [ -f "/root/go/bin/subfinder" ]; then
            cp /root/go/bin/subfinder /usr/local/bin/ 2>/dev/null || true
            echo "✅ subfinder installed successfully"
        fi
    else
        echo "⚠️  Failed to install subfinder (domain discovery will be limited)"
    fi
fi

# Check if httpx is installed
if ! command -v httpx &> /dev/null; then
    echo "📦 Installing httpx..."
    if go install -v github.com/projectdiscovery/httpx/cmd/httpx@latest 2>&1; then
        if [ -f "/root/go/bin/httpx" ]; then
            cp /root/go/bin/httpx /usr/local/bin/ 2>/dev/null || true
            echo "✅ httpx installed successfully"
        fi
    else
        echo "⚠️  Failed to install httpx (domain enrichment will be limited)"
    fi
fi

# Verify tools are available
echo "🔍 Verifying dependencies..."
if command -v subfinder &> /dev/null; then
    echo "✅ subfinder: $(subfinder -version 2>&1 | head -1 || echo 'installed')"
else
    echo "⚠️  subfinder: NOT FOUND (domain discovery will be limited)"
fi

if command -v httpx &> /dev/null; then
    echo "✅ httpx: $(httpx -version 2>&1 | head -1 || echo 'installed')"
else
    echo "⚠️  httpx: NOT FOUND (domain enrichment will be limited)"
fi

echo "🌐 Starting Watchtower..."
echo "   Web interface will be available at: http://localhost:${WEB_PORT:-8080}"
echo ""

# Execute the main command
exec "$@"
