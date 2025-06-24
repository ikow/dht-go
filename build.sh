#!/bin/bash

# DHT Crawler Build Script

echo "Building DHT Hash Crawler..."

# Clean previous builds
rm -f dht-crawler

# Build the application
go build -o dht-crawler .

if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
    echo ""
    echo "Usage:"
    echo "  ./dht-crawler     - Run the crawler"
    echo "  ./build.sh run    - Build and run immediately"
    echo ""
    
    # If 'run' argument is provided, start the crawler
    if [ "$1" = "run" ]; then
        echo "Starting DHT crawler..."
        ./dht-crawler
    fi
else
    echo "❌ Build failed!"
    exit 1
fi 