# 🔥 ULTRA-AGGRESSIVE DHT Honeypot Crawler

An **ultra-high-performance** Go-based DHT (Distributed Hash Table) honeypot crawler designed to discover and collect torrent hashes from the BitTorrent DHT network at **maximum speed**.

## 🚀 Features

- **🍯 DHT Honeypot Strategy**: Acts as multiple virtual DHT nodes to attract maximum queries
- **⚡ Ultra-Aggressive Collection**: 200+ queries/second with 1000 concurrent workers  
- **💎 Multi-Pattern Discovery**: Uses 5+ different hash generation patterns
- **🌐 Enhanced Bootstrap**: Connects to 12+ bootstrap nodes with multi-ID flooding
- **🔍 Smart Hash Detection**: Extracts hashes from get_peers, announce_peer, AND find_node queries
- **🎯 Real-time Statistics**: Live monitoring with hourly rates and performance metrics
- **🕷️ Active Node Discovery**: Continuously discovers new DHT nodes for maximum coverage
- **⭐ Hash Validation**: Filters out invalid hashes (all-zeros, all-FF, etc.)
- **🛡️ Graceful Shutdown**: Saves all discovered hashes to timestamped files

## 🚀 Performance Results

### Multi-Threaded Turbo Mode (Latest)
- **Discovery Rate**: ~22 hashes/minute
- **Architecture**: 8 UDP ports, 16 workers, 8 virtual node IDs
- **42 hashes** discovered in under 2 minutes
- **Multiple concurrent strategies** for maximum efficiency

### Evolution of Performance
1. **First Implementation**: 0-2 hashes in 17+ minutes
2. **Simple UDP Approach**: 269 hashes in 13 minutes (~20.7/min)
3. **Multi-Threaded Turbo**: 42 hashes in 2 minutes (~22/min)

## 🏗️ Architecture

### Current Implementation (Turbo Mode)
- **8 UDP Listeners**: Multiple ports for maximum network capacity
- **16 Worker Threads**: Parallel message processing
- **8 Virtual Node IDs**: Enhanced DHT network presence
- **3 Concurrent Strategies**:
  - Random queries (50 every 2 seconds)
  - Bootstrap flooding (every 30 seconds) 
  - Rapid fire queries (20 parallel every 1 second)

### Key Technical Features
- Direct UDP implementation of KRPC protocol
- Bencode message encoding/decoding
- Thread-safe hash deduplication
- Aggressive bootstrapping with 8+ bootstrap nodes
- Real-time statistics and monitoring

## 📊 Usage

### Quick Start
```bash
# Install dependencies
go mod tidy

# Run the turbo crawler
go run main.go
```

### Configuration
The crawler can be configured by modifying these constants in `main.go`:
```go
portCount := 8     // Number of UDP ports
workerCount := 16  // Worker threads for processing
nodeCount := 8     // Virtual node identities
```

## 📁 Output

Hashes are saved to `hashes_turbo_YYYY-MM-DD.txt` with format:
```
hash|timestamp
27934dbb71508307e266d379078b8cc639c08284|2025-06-10 22:44:04
```

## 🛠️ Technical Details

### DHT Protocol Implementation
- **KRPC Messages**: Proper bencode encoding
- **Query Types**: `find_node`, `get_peers`, `announce_peer`
- **Bootstrap Nodes**: 8 reliable DHT bootstrap servers
- **Node Discovery**: Automatic discovery and querying of new nodes

### Multi-Threading Strategy
- **Worker Pool Pattern**: Efficient message processing
- **Multiple UDP Sockets**: Parallel network I/O
- **Concurrent Query Strategies**: Different approaches running simultaneously
- **Thread-Safe Operations**: Atomic counters and sync.Map for deduplication

## 🎯 Why It Works

1. **Direct Protocol Implementation**: No complex library overhead
2. **Proper DHT Citizenship**: Responds correctly to DHT queries
3. **Aggressive Querying**: Multiple strategies maximize discovery
4. **Parallel Processing**: Multi-threading handles high message volume
5. **Smart Bootstrapping**: Establishes presence across DHT network

## 📈 Performance Monitoring

The crawler displays real-time statistics every 15 seconds:
```
🚀 MULTI-THREADED DHT CRAWLER - TURBO MODE
================================================================================
⏱️  Uptime: 2m0s
🎯 Hashes Discovered: 42 (22.00/min)
🔍 Queries Sent: 2,400 (1200.00/min)
📡 UDP Ports: 8 | 🔧 Workers: 16 | 🆔 Node IDs: 8
💾 Output File: hashes_turbo_2025-06-10.txt
⚡ Efficiency: 0.018 hashes/query
================================================================================
```

## 🔧 Requirements

- Go 1.24+
- Network connectivity (works best on VPS/public IP)
- UDP port availability

## 📦 Dependencies

```go
require github.com/zeebo/bencode v1.0.0
```

## 🎉 Success Factors

This implementation succeeds where others fail by:

1. **Simplicity**: Direct UDP implementation vs complex libraries
2. **Parallelization**: Multiple threads and connections
3. **Network Presence**: Multiple virtual identities
4. **Aggressive Discovery**: High query rate and multiple strategies
5. **Proper Protocol**: Correct KRPC message handling

The key insight: **Simple, correct protocol implementation with aggressive parallelization beats complex theoretical optimizations.**

## How It Works

The DHT crawler operates by:

1. **Joining the DHT Network**: Connects to well-known bootstrap nodes
2. **Listening for Queries**: Intercepts DHT traffic for `get_peers` and `announce_peer` messages
3. **Extracting Hashes**: Parses incoming queries to extract 20-byte torrent info hashes
4. **Deduplicating**: Maintains an in-memory set to avoid duplicate entries
5. **Real-time Display**: Shows newly discovered hashes and statistics
6. **Persistent Storage**: Saves all unique hashes to a timestamped file on exit

## Installation

### Prerequisites
- Go 1.21 or higher
- Network connectivity (UDP port 6881 by default)

### Build from Source
```bash
git clone <your-repo>
cd dht-go
go mod download
go build .
```

## Usage

### Basic Usage
```bash
./dht-crawler
```

The crawler will:
- Start listening on UDP port 6881
- Bootstrap to the DHT network
- Begin collecting hashes automatically
- Display statistics every 10 seconds
- Save results to `dht_hashes_<timestamp>.txt` on exit

### Stopping the Crawler
- Press `Ctrl+C` for graceful shutdown
- The crawler will save all discovered hashes before exiting

## Configuration

The crawler uses the following default settings:

```go
const (
    DefaultPort        = 6881           // UDP port to listen on
    DefaultWorkers     = 100            // Maximum concurrent workers
    DefaultTimeout     = 30 * time.Second  // Query timeout
    DefaultMaxResults  = 1000000        // Maximum hashes to collect
    DefaultStatsInterval = 10 * time.Second // Statistics display interval
)
```

To modify these settings, edit the constants in `main.go` and rebuild.

## Output

### Console Output
The crawler displays:
- Newly discovered hashes in real-time
- Periodic statistics including:
  - Uptime
  - Total queries processed
  - Unique hashes discovered
  - Network nodes contacted
  - Discovery rate (hashes/second)

### File Output
On exit, all unique hashes are saved to a file named `dht_hashes_<timestamp>.txt`, with one hash per line in hexadecimal format.

Example output file content:
```
a1b2c3d4e5f6789012345678901234567890abcd
ef123456789abcdef123456789abcdef12345678
...
```

## Network Requirements

- **Port Access**: Requires UDP port 6881 (configurable)
- **Firewall**: Ensure the port is not blocked by firewalls
- **Internet Connection**: Needs access to DHT bootstrap nodes
- **Bandwidth**: Minimal bandwidth usage (primarily UDP packets)

## Performance

The crawler is optimized for speed and can discover thousands of hashes per hour depending on:
- Network connectivity quality
- DHT network activity levels
- Geographic location
- Time of day (peak BitTorrent usage times)

Typical performance metrics:
- **Queries/second**: 10-100+ depending on network activity
- **Memory usage**: Minimal (primarily hash storage)
- **CPU usage**: Low (mostly network I/O)

## Security Considerations

- The crawler only **observes** DHT traffic and doesn't download any actual torrent content
- It participates in the DHT network as a passive node
- No personal information is collected or stored
- All operations are read-only from a content perspective

## Troubleshooting

### Common Issues

1. **"Permission denied" on port 6881**
   - Run with elevated privileges or change the port
   - Ensure no other application is using the port

2. **No hashes discovered**
   - Check firewall settings
   - Verify internet connectivity
   - Try running during peak hours for more DHT activity

3. **Low discovery rate**
   - Normal for new nodes - discovery rate increases over time
   - Consider running for longer periods
   - Check network connectivity quality

### Debug Mode
For verbose logging, you can modify the log level in the code or add debug flags.

## Legal Notice

This tool is for educational and research purposes. Users are responsible for ensuring compliance with local laws and regulations regarding BitTorrent and DHT network participation.

## Contributing

Contributions are welcome! Please feel free to submit pull requests or open issues for bugs and feature requests.

## License

[Specify your license here] 