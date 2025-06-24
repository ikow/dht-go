# DHT Crawler with Metadata Fetching

A high-performance DHT (Distributed Hash Table) crawler that discovers torrent hashes and automatically fetches detailed metadata. Built with Go for maximum efficiency and scalability.

## 🚀 Features

### DHT Crawling
- **Multi-connection Architecture**: Utilizes multiple UDP connections for parallel crawling
- **Adaptive Resource Management**: Automatically adjusts performance based on system resources
- **Three Operation Modes**: Low, Balanced, and High performance configurations
- **Discovery Strategies**: Multiple strategies for optimal hash discovery
- **Real-time Statistics**: Live monitoring of discovery rates and system usage

### Metadata Fetching
- **Automatic Metadata Retrieval**: Fetches detailed torrent metadata for discovered hashes
- **Concurrent Processing**: Configurable concurrent metadata fetches
- **Comprehensive Information**: Extracts file lists, sizes, names, and torrent properties
- **Smart Categorization**: Automatically categorizes content by type
- **Multiple Output Formats**: JSON and CSV output support

### System Management
- **Resource Monitoring**: CPU and memory usage tracking
- **Rate Limiting**: Configurable rate limits to prevent network congestion
- **Graceful Shutdown**: Clean shutdown with statistics summary
- **Error Handling**: Robust error handling and retry mechanisms

## 📁 Project Structure

```
dht-go/
├── cmd/
│   └── crawler/
│       └── main.go          # Main application
├── pkg/
│   ├── dht/
│   │   └── crawler.go       # DHT crawler implementation
│   └── metadata/
│       └── fetcher.go       # Metadata fetching implementation
├── go.mod                   # Go module definition
├── go.sum                   # Go module checksums
├── Makefile                 # Build automation
└── README_RESTRUCTURED.md   # This documentation
```

## 🛠 Installation

### Prerequisites
- Go 1.21 or later
- 2GB+ available RAM (for high performance mode)
- Stable internet connection

### Building from Source

```bash
# Clone the repository
git clone <repository-url>
cd dht-go

# Install dependencies
make deps

# Build the application
make build

# Or build and run in one step
make run
```

## 📖 Usage

### Basic Usage

```bash
# Run with default balanced mode
./bin/dht-crawler

# Run with custom output directory
./bin/dht-crawler -output ./my_results

# Show all available options
./bin/dht-crawler -help
```

### Operation Modes

#### Balanced Mode (Default)
```bash
./bin/dht-crawler -mode balanced
```
- **CPU Usage**: ~70%
- **Memory**: ~512MB
- **Connections**: 4 per CPU core
- **Rate Limit**: 1000 queries/second
- **Best For**: Most users, optimal balance of performance and resource usage

#### Low Resource Mode
```bash
./bin/dht-crawler -mode low
```
- **CPU Usage**: ~30%
- **Memory**: ~128MB
- **Connections**: 2 per CPU core
- **Rate Limit**: 200 queries/second
- **Best For**: Background operation, shared systems, low-power devices

#### High Performance Mode
```bash
./bin/dht-crawler -mode high
```
- **CPU Usage**: ~90%
- **Memory**: ~1GB
- **Connections**: 8 per CPU core
- **Rate Limit**: 2000 queries/second
- **Best For**: Dedicated systems, maximum discovery speed

### Advanced Configuration

```bash
# Custom metadata fetching settings
./bin/dht-crawler \
  -meta-concurrent 10 \
  -meta-timeout 45s \
  -format both

# Disable metadata fetching for hash-only discovery
./bin/dht-crawler -metadata=false

# Custom output location with CSV format
./bin/dht-crawler \
  -output ./results \
  -format csv
```

### Make Commands

```bash
# Build and run variations
make run              # Default balanced mode
make run-low          # Low resource mode
make run-high         # High performance mode
make run-no-meta      # Hash discovery only

# Development commands
make deps             # Install/update dependencies
make clean            # Clean build artifacts
make test             # Run tests
make fmt              # Format code
make lint             # Lint code (requires golangci-lint)
```

## 📊 Output Files

### Hash Discovery Log
**File**: `hashes_YYYY-MM-DD_HH-MM-SS.txt`
```
# DHT Crawler Hash Discovery Log
# Format: hash|timestamp|source
a1b2c3d4e5f6....|2024-01-15 14:30:25|query
b2c3d4e5f6a1....|2024-01-15 14:30:26|response
```

### Metadata JSON
**File**: `metadata_YYYY-MM-DD_HH-MM-SS.json`
```json
[
  {
    "info_hash": "a1b2c3d4e5f6789...",
    "name": "Example Movie 2024 1080p BluRay x264",
    "files": [
      {
        "path": "Example.Movie.2024.1080p.BluRay.x264.mkv",
        "length": 2147483648
      }
    ],
    "total_size": 2147483648,
    "piece_count": 1024,
    "piece_length": 2097152,
    "categories": ["video", "movie"],
    "tags": ["quality:1080p", "source:BluRay", "codec:x264"],
    "fetched_at": "2024-01-15T14:30:30Z",
    "success": true
  }
]
```

## 🔧 Configuration Options

| Flag | Default | Description |
|------|---------|-------------|
| `-mode` | `balanced` | Operation mode: `low`, `balanced`, `high` |
| `-metadata` | `true` | Enable metadata fetching |
| `-output` | `./output` | Output directory path |
| `-format` | `json` | Metadata format: `json`, `csv`, `both` |
| `-meta-concurrent` | `5` | Max concurrent metadata fetches |
| `-meta-timeout` | `30s` | Metadata fetch timeout |
| `-stats` | `true` | Enable statistics logging |

## 📈 Performance Metrics

### Typical Discovery Rates
- **Low Mode**: 50-200 hashes/hour
- **Balanced Mode**: 200-800 hashes/hour
- **High Mode**: 500-2000+ hashes/hour

*Rates vary based on network conditions, system performance, and DHT network activity*

### Resource Usage
- **CPU**: Scales with selected mode (30%-90%)
- **Memory**: 128MB-1GB depending on mode
- **Network**: 1-50 Mbps depending on activity
- **Disk I/O**: Minimal for hash logging, moderate for metadata

## 🛡 Safety Features

- **Rate Limiting**: Prevents network flooding
- **Resource Monitoring**: Automatic scaling based on system load
- **Graceful Shutdown**: Clean exit with Ctrl+C
- **Error Recovery**: Automatic retry for failed operations
- **Memory Management**: Efficient buffer pooling and cleanup

## 🐛 Troubleshooting

### Common Issues

#### Low Discovery Rate
```bash
# Try high performance mode
./bin/dht-crawler -mode high

# Check network connectivity
ping router.bittorrent.com
```

#### High Memory Usage
```bash
# Use low resource mode
./bin/dht-crawler -mode low

# Disable metadata fetching
./bin/dht-crawler -metadata=false
```

#### Metadata Fetch Failures
```bash
# Increase timeout
./bin/dht-crawler -meta-timeout 60s

# Reduce concurrent fetches
./bin/dht-crawler -meta-concurrent 3
```

### Debug Information

Enable verbose logging by modifying the log level in the source code or redirect output:

```bash
./bin/dht-crawler 2>&1 | tee crawler.log
```

## 🧪 Development

### Running Tests
```bash
make test
```

### Code Formatting
```bash
make fmt
```

### Adding New Features

1. **DHT Enhancements**: Modify `pkg/dht/crawler.go`
2. **Metadata Features**: Modify `pkg/metadata/fetcher.go`
3. **Application Logic**: Modify `cmd/crawler/main.go`

### Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Format code: `make fmt`
6. Submit a pull request

## 📄 License

This project is licensed under the MIT License. See LICENSE file for details.

## 🙏 Acknowledgments

- Built using the excellent [anacrolix/torrent](https://github.com/anacrolix/torrent) library
- DHT protocol implementation based on [BEP 5](http://bittorrent.org/beps/bep_0005.html)
- Bencode encoding/decoding via [zeebo/bencode](https://github.com/zeebo/bencode)

---

**⚠️ Legal Notice**: This tool is for educational and research purposes. Users are responsible for complying with local laws and regulations. The developers do not endorse or encourage piracy or copyright infringement. 