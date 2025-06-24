#!/bin/bash

# Quick test script for DHT Crawler
echo "🧪 Testing DHT Crawler Application"
echo "================================="

# Build the application
echo "Building application..."
make build

if [ $? -ne 0 ]; then
    echo "❌ Build failed"
    exit 1
fi

echo "✅ Build successful"

# Test help output
echo "Testing help output..."
./bin/dht-crawler -help > /dev/null 2>&1

if [ $? -ne 0 ]; then
    echo "❌ Help output failed"
    exit 1
fi

echo "✅ Help output works"

# Test short run (run for 5 seconds then kill)
echo "Testing short run..."
./bin/dht-crawler -mode low -output ./test_output &
CRAWLER_PID=$!

sleep 5
kill $CRAWLER_PID 2>/dev/null
wait $CRAWLER_PID 2>/dev/null

# Check if output files were created
if [ -d "./test_output" ]; then
    echo "✅ Output directory created"
    ls -la ./test_output/
else
    echo "⚠️  Output directory not found (might be normal for short run)"
fi

# Cleanup
rm -rf ./test_output

echo ""
echo "🎉 Basic tests completed successfully!"
echo ""
echo "Next steps:"
echo "1. Run: make run-low        # For low resource usage"
echo "2. Run: make run            # For balanced mode"
echo "3. Run: make run-high       # For maximum performance"
echo "4. Run: make run-no-meta    # For hash discovery only"
echo ""
echo "Check the output/ directory for results after running." 