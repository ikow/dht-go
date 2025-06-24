#!/bin/bash

# Enhanced DHT Crawler Build Script
echo "🔨 Building Enhanced DHT Crawler..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21+ first."
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
REQUIRED_VERSION="1.21"

if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
    echo "❌ Go version $GO_VERSION is too old. Please install Go $REQUIRED_VERSION or later."
    exit 1
fi

echo "✅ Go version $GO_VERSION detected"

# Download dependencies
echo "📦 Downloading dependencies..."
go mod tidy

if [ $? -ne 0 ]; then
    echo "❌ Failed to download dependencies"
    exit 1
fi

# Build the enhanced crawler
echo "🔨 Building enhanced crawler..."
go build -o enhanced-dht-crawler enhanced_main.go enhanced_crawler.go strategies.go

if [ $? -eq 0 ]; then
    echo "✅ Enhanced DHT Crawler built successfully!"
    echo ""
    echo "🚀 Usage examples:"
    echo "  ./enhanced-dht-crawler                    # Balanced mode (recommended)"
    echo "  ./enhanced-dht-crawler -mode low          # Low resource usage"
    echo "  ./enhanced-dht-crawler -mode high -burst  # Maximum performance"
    echo "  ./enhanced-dht-crawler -help              # Show all options"
    echo ""
    echo "📁 Executable created: enhanced-dht-crawler"
else
    echo "❌ Build failed"
    exit 1
fi