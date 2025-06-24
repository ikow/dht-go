# DHT Crawler Project Makefile

.PHONY: all build clean test run help deps install

# Default target
all: build

# Build the main application
build:
	@echo "🔨 Building DHT Crawler..."
	@mkdir -p bin
	go build -o bin/dht-crawler cmd/crawler/main.go

# Install dependencies
deps:
	@echo "📦 Installing dependencies..."
	go mod tidy
	go mod download

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf bin/
	rm -rf output/
	rm -rf metadata_temp/
	rm -f *.txt *.json

# Run tests
test:
	@echo "🧪 Running tests..."
	go test ./...

# Run the application with default settings
run: build
	@echo "🚀 Running DHT Crawler..."
	./bin/dht-crawler

# Run in low resource mode
run-low: build
	@echo "🚀 Running DHT Crawler (Low Resource Mode)..."
	./bin/dht-crawler -mode low

# Run in high performance mode
run-high: build
	@echo "🚀 Running DHT Crawler (High Performance Mode)..."
	./bin/dht-crawler -mode high

# Run without metadata fetching
run-no-meta: build
	@echo "🚀 Running DHT Crawler (No Metadata)..."
	./bin/dht-crawler -metadata=false

# Install to system
install: build
	@echo "📥 Installing to system..."
	sudo cp bin/dht-crawler /usr/local/bin/

# Format code
fmt:
	@echo "📝 Formatting code..."
	go fmt ./...

# Lint code
lint:
	@echo "🔍 Linting code..."
	golangci-lint run

# Create output directory
output-dir:
	@mkdir -p output

# Show help
help:
	@echo "DHT Crawler Project"
	@echo "==================="
	@echo ""
	@echo "Available targets:"
	@echo "  build      - Build the application"
	@echo "  deps       - Install/update dependencies"
	@echo "  clean      - Clean build artifacts"
	@echo "  test       - Run tests"
	@echo "  run        - Run with default settings"
	@echo "  run-low    - Run in low resource mode"
	@echo "  run-high   - Run in high performance mode"
	@echo "  run-no-meta - Run without metadata fetching"
	@echo "  install    - Install to system (/usr/local/bin)"
	@echo "  fmt        - Format code"
	@echo "  lint       - Lint code"
	@echo "  help       - Show this help"
	@echo ""
	@echo "Usage examples:"
	@echo "  make build && ./bin/dht-crawler -help"
	@echo "  make run-high"
	@echo "  make clean build run" 