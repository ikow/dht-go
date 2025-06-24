# Enhanced DHT Crawler

A **next-generation, high-performance DHT (Distributed Hash Table) crawler** designed for maximum hash discovery efficiency while maintaining intelligent resource management.

## 🚀 Key Features

### Performance Optimizations
- **Multi-connection Architecture**: Parallel UDP connections for maximum throughput
- **Advanced Buffer Pooling**: Zero-allocation message processing
- **Asynchronous File I/O**: Non-blocking hash storage with batch writes
- **Connection Pooling**: Optimized socket management with performance tuning
- **Rate Limiting**: Intelligent traffic control to prevent network saturation

### Resource Management
- **Adaptive Scaling**: Automatic adjustment based on CPU and memory usage
- **CPU-aware Scaling**: Configurable CPU usage limits (30% to 90%)
- **Memory Management**: Automatic garbage collection hints and memory limits
- **System Impact Control**: Designed to coexist with other applications

### Discovery Strategies
1. **Bootstrap Strategy**: Systematic querying of known DHT bootstrap nodes
2. **Aggressive Strategy**: High-frequency random discovery queries
3. **Random Strategy**: Intelligent querying of discovered nodes
4. **Adaptive Strategy**: Performance-based strategy adjustment
5. **Burst Strategy**: Periodic high-intensity discovery bursts

### Intelligent Features
- **Node Discovery & Caching**: Automatic discovery and intelligent reuse of DHT nodes
- **Efficiency Monitoring**: Real-time performance tracking and optimization
- **Graceful Shutdown**: Clean resource cleanup and final statistics
- **Multiple Discovery Sources**: Enhanced bootstrap node list with fallbacks

## 📊 Performance Modes

### 🔧 Low Resource Mode
- **Target**: Minimal system impact, background operation
- **CPU Usage**: 30% maximum
- **Memory**: 128MB limit
- **Rate**: 200 queries/second
- **Expected Performance**: 5-15 hashes/minute

### ⚖️ Balanced Mode (Default)
- **Target**: Optimal performance vs resource balance
- **CPU Usage**: 70% maximum
- **Memory**: 512MB limit  
- **Rate**: 1000 queries/second
- **Expected Performance**: 15-40 hashes/minute

### 🚀 High Performance Mode
- **Target**: Maximum hash discovery rate
- **CPU Usage**: 90% maximum
- **Memory**: 1GB limit
- **Rate**: 2000 queries/second
- **Expected Performance**: 30-80+ hashes/minute

## 🛠️ Installation & Usage

### Quick Start

```bash
# Build the enhanced crawler
./build_enhanced.sh

# Run with balanced mode (recommended)
./enhanced-dht-crawler

# Run in low resource mode
./enhanced-dht-crawler -mode low

# Maximum performance with burst mode
./enhanced-dht-crawler -mode high -burst
```

### Advanced Usage

```bash
# Custom resource limits
./enhanced-dht-crawler -cpu 0.5 -memory 256 -rate 500

# Disable adaptive scaling
./enhanced-dht-crawler -adaptive=false

# Custom statistics interval
./enhanced-dht-crawler -stats 30

# Show available options
./enhanced-dht-crawler -help

# Show detailed mode information
./enhanced-dht-crawler -modes
```

## 📈 Performance Comparison

| Metric | Original Crawler | Enhanced Crawler |
|--------|------------------|------------------|
| **Hash Discovery Rate** | ~22 hashes/min | **30-80+ hashes/min** |
| **Resource Usage** | Fixed high | **Adaptive (30-90%)** |
| **Memory Efficiency** | Basic | **Buffer pooling + GC optimization** |
| **Network Utilization** | Single strategy | **5 intelligent strategies** |
| **System Impact** | High | **Configurable (low/balanced/high)** |
| **Scalability** | Limited | **Multi-core aware** |
| **Monitoring** | Basic stats | **Real-time performance tracking** |

## 🧠 Advanced Features

### Adaptive Strategy System
The enhanced crawler continuously monitors its own performance and adjusts strategies in real-time:

- **High Efficiency Detected**: Increases query rate to recently successful nodes
- **Medium Efficiency**: Maintains balanced approach between bootstrap and discovered nodes
- **Low Efficiency**: Focuses on node discovery to expand the crawling network

### Burst Mode
Periodic high-intensity discovery sessions that temporarily increase the query rate by 3x for 30-second intervals, scheduled every 10-15 minutes.

### Resource Monitoring
Real-time tracking of:
- CPU usage estimation
- Memory consumption
- Goroutine count
- Network efficiency
- Discovery rate trends

## 📁 Output Format

Hashes are saved to timestamped files: `enhanced_hashes_YYYY-MM-DD_HH-MM-SS.txt`

Format: `hash|timestamp|source`

Example:
```
a1b2c3d4e5f6789012345678901234567890abcd|2025-06-17 14:30:25|announce_peer
ef123456789abcdef123456789abcdef12345678|2025-06-17 14:30:26|get_peers
```

## 🔧 Configuration Options

### Resource Limits
- `-cpu FLOAT`: Maximum CPU percentage (0.0-1.0)
- `-memory INT`: Maximum memory in MB
- `-rate INT`: Rate limit per second
- `-connections INT`: Number of UDP connections

### Behavior Control
- `-burst`: Enable burst mode
- `-adaptive`: Enable adaptive scaling (default: true)
- `-stats INT`: Statistics display interval in seconds

### Modes
- `-mode low`: Low resource impact
- `-mode balanced`: Optimal balance (default)
- `-mode high`: Maximum performance

## 🎯 Optimization Tips

### For Maximum Hash Discovery
```bash
./enhanced-dht-crawler -mode high -burst -connections 16 -rate 3000
```

### For Server Environments
```bash
./enhanced-dht-crawler -mode low -stats 60 -adaptive=true
```

### For Research/Analysis
```bash
./enhanced-dht-crawler -mode balanced -burst -stats 10
```

## 📊 Real-time Statistics

The crawler displays comprehensive statistics every 15 seconds (configurable):

```
================================================================================
🚀 ENHANCED DHT CRAWLER - OPTIMIZED MODE
================================================================================
⏱️  Uptime: 5m30s
🎯 Hashes Discovered: 127 (23.27/min)
🔍 Queries Sent: 8,432 (1534.91/min)
🌐 Discovered Nodes: 1,847
📡 Connections: 8 | 🔧 Goroutines: 156
💾 Output File: enhanced_hashes_2025-06-17_14-25-30.txt
📊 CPU: 68.2% | Memory: 347MB
⚡ Efficiency: 0.015 hashes/query
🎛️  Rate Limit: 1000/sec | Strategies: [aggressive bootstrap random adaptive]
================================================================================
```

## ⚠️ Important Notes

### System Requirements
- **Go 1.21+** required
- **Network Access**: UDP port availability for DHT communication
- **Memory**: Minimum 128MB available RAM
- **CPU**: Multi-core systems recommended for optimal performance

### Network Considerations
- The crawler participates in the DHT network as a legitimate node
- It only **observes** hash announcements, doesn't download content
- Respects network protocols and implements proper rate limiting
- Uses enhanced bootstrap nodes for better network connectivity

### Legal & Ethical Use
- This tool is for **educational and research purposes**
- Users are responsible for compliance with local laws
- The crawler only collects publicly announced hash identifiers
- No personal information or actual torrent content is accessed

## 🔍 Troubleshooting

### Low Hash Discovery Rate
- Try increasing the rate limit: `-rate 2000`
- Enable burst mode: `-burst`
- Switch to high performance mode: `-mode high`
- Check network connectivity and firewall settings

### High Resource Usage
- Use low resource mode: `-mode low`
- Reduce rate limit: `-rate 200`
- Lower CPU limit: `-cpu 0.3`
- Reduce memory limit: `-memory 128`

### Network Issues
- Ensure UDP ports are not blocked by firewall
- Try different bootstrap nodes if connectivity is poor
- Check if your ISP blocks DHT traffic
- Verify internet connectivity to DHT network

## 🆚 Comparison with Original Crawler

### Architecture Improvements
- **Original**: Single-threaded with basic optimizations
- **Enhanced**: Multi-threaded with advanced resource management

### Discovery Efficiency
- **Original**: Fixed query strategies
- **Enhanced**: Adaptive strategies based on real-time performance

### Resource Management
- **Original**: No resource limits or monitoring
- **Enhanced**: Configurable limits with adaptive scaling

### User Experience
- **Original**: Basic statistics and limited configuration
- **Enhanced**: Comprehensive monitoring and extensive customization

## 🚧 Future Enhancements

Potential improvements for future versions:
- Machine learning-based strategy optimization
- Geographic node distribution awareness
- Content popularity tracking and analysis
- Advanced network topology mapping
- Integration with external DHT monitoring services

---

**Enhanced DHT Crawler** - Maximizing hash discovery efficiency while maintaining intelligent resource management.