package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	// Command line flags for enhanced crawler
	var (
		mode          = flag.String("mode", "balanced", "Crawler mode: low, balanced, high")
		maxCPU        = flag.Float64("cpu", 0.0, "Maximum CPU percentage (0.0-1.0, 0=auto)")
		maxMemoryMB   = flag.Int("memory", 0, "Maximum memory in MB (0=auto)")
		rateLimitSec  = flag.Int("rate", 0, "Rate limit per second (0=auto)")
		connections   = flag.Int("connections", 0, "Number of connections (0=auto)")
		burstMode     = flag.Bool("burst", false, "Enable burst mode")
		adaptiveMode  = flag.Bool("adaptive", true, "Enable adaptive scaling")
		statsInterval = flag.Int("stats", 15, "Statistics interval in seconds")
		strategies    = flag.String("strategies", "", "Comma-separated list of strategies (auto-detected)")
		showHelp      = flag.Bool("help", false, "Show help message")
		showModes     = flag.Bool("modes", false, "Show available modes")
	)

	flag.Parse()

	if *showHelp {
		showEnhancedUsage()
		return
	}

	if *showModes {
		showAvailableModes()
		return
	}

	// Select configuration based on mode
	var config *EnhancedCrawlerConfig
	switch *mode {
	case "low":
		config = LowResourceConfig()
		log.Println("🔧 Selected: Low Resource Mode - Minimal system impact")
	case "high":
		config = HighPerformanceConfig()
		log.Println("🔧 Selected: High Performance Mode - Maximum hash discovery")
	case "balanced":
		fallthrough
	default:
		config = DefaultEnhancedConfig()
		log.Println("🔧 Selected: Balanced Mode - Optimal balance of performance and resource usage")
	}

	// Apply command line overrides
	if *maxCPU > 0 {
		config.MaxCPUPercent = math.Min(*maxCPU, 1.0)
	}
	if *maxMemoryMB > 0 {
		config.MaxMemoryMB = *maxMemoryMB
	}
	if *rateLimitSec > 0 {
		config.RateLimitPerSec = *rateLimitSec
	}
	if *connections > 0 {
		config.ConnPoolSize = *connections
	}
	if *burstMode {
		config.BurstMode = true
	}
	config.AdaptiveScaling = *adaptiveMode
	config.StatsInterval = time.Duration(*statsInterval) * time.Second

	// Override strategies if specified
	if *strategies != "" {
		// Parse comma-separated strategies
		// For simplicity, keeping default for now
	}

	// Display configuration
	displayConfig(config)

	// Create and start crawler
	crawler, err := NewEnhancedDHTCrawler(config)
	if err != nil {
		log.Fatalf("❌ Failed to create enhanced crawler: %v", err)
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start crawler in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- crawler.Run()
	}()

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		log.Println("🛑 Received shutdown signal")
	case err := <-errChan:
		if err != nil {
			log.Printf("❌ Crawler error: %v", err)
		}
	}

	// Graceful shutdown
	crawler.Close()
	
	// Final statistics
	displayFinalStats(crawler)
}

func showEnhancedUsage() {
	fmt.Println(`
Enhanced DHT Crawler - High-Performance Torrent Hash Discovery
============================================================

USAGE:
  go run enhanced_main.go enhanced_crawler.go strategies.go [OPTIONS]

MODES:
  -mode low       Low resource mode (30% CPU, 128MB RAM, minimal impact)
  -mode balanced  Balanced mode (70% CPU, 512MB RAM, optimal performance) [DEFAULT]
  -mode high      High performance mode (90% CPU, 1GB RAM, maximum discovery)

CONFIGURATION:
  -cpu FLOAT      Maximum CPU percentage (0.0-1.0, overrides mode default)
  -memory INT     Maximum memory in MB (overrides mode default)
  -rate INT       Rate limit per second (overrides mode default)
  -connections INT Number of UDP connections (overrides mode default)
  -burst          Enable burst mode for periodic high-intensity discovery
  -adaptive       Enable adaptive scaling based on performance (default: true)
  -stats INT      Statistics display interval in seconds (default: 15)

INFORMATION:
  -modes          Show detailed information about available modes
  -help           Show this help message

EXAMPLES:

  Balanced mode (recommended for most users):
    go run enhanced_main.go enhanced_crawler.go strategies.go

  Low resource mode (minimal system impact):
    go run enhanced_main.go enhanced_crawler.go strategies.go -mode low

  High performance mode with burst:
    go run enhanced_main.go enhanced_crawler.go strategies.go -mode high -burst

  Custom configuration:
    go run enhanced_main.go enhanced_crawler.go strategies.go -cpu 0.5 -memory 256 -rate 500

  Quiet mode with longer stats interval:
    go run enhanced_main.go enhanced_crawler.go strategies.go -stats 60

FEATURES:
  ✅ Adaptive resource management
  ✅ Multiple discovery strategies
  ✅ Connection pooling and rate limiting
  ✅ Asynchronous file I/O
  ✅ Real-time performance monitoring
  ✅ Graceful shutdown and cleanup
  ✅ Memory-efficient buffer pooling
  ✅ Intelligent node discovery and caching

OUTPUT:
  Hashes are saved to: enhanced_hashes_YYYY-MM-DD_HH-MM-SS.txt
  Format: hash|timestamp|source
`)
}

func showAvailableModes() {
	numCPU := runtime.NumCPU()
	
	fmt.Printf(`
Available Crawler Modes
======================

🔧 LOW RESOURCE MODE (-mode low)
  Target Use Case: Background operation, minimal system impact
  CPU Usage:       30%% (%.1f%% of %d cores)
  Memory Limit:    128 MB
  Connections:     %d (2 per CPU core)
  Rate Limit:      200 queries/second
  Goroutines:      ~%d (10 per CPU core)
  Strategies:      Bootstrap, Random
  Burst Mode:      Disabled
  
  Ideal for: Running alongside other applications, servers with limited resources

⚖️ BALANCED MODE (-mode balanced) [DEFAULT]
  Target Use Case: Optimal balance of performance and resource usage
  CPU Usage:       70%% (%.1f%% of %d cores)
  Memory Limit:    512 MB
  Connections:     %d (4 per CPU core)
  Rate Limit:      1000 queries/second
  Goroutines:      ~%d (50 per CPU core)
  Strategies:      Aggressive, Bootstrap, Random, Adaptive
  Burst Mode:      Optional

  Ideal for: General purpose hash discovery, dedicated crawler systems

🚀 HIGH PERFORMANCE MODE (-mode high)
  Target Use Case: Maximum hash discovery rate
  CPU Usage:       90%% (%.1f%% of %d cores)
  Memory Limit:    1024 MB
  Connections:     %d (8 per CPU core)
  Rate Limit:      2000 queries/second
  Goroutines:      ~%d (100 per CPU core)
  Strategies:      All strategies including Burst
  Burst Mode:      Enabled

  Ideal for: Dedicated crawler machines, research purposes, maximum data collection

🧠 ADAPTIVE FEATURES (All Modes)
  - Automatic resource scaling based on system performance
  - Dynamic strategy adjustment based on discovery efficiency
  - Intelligent rate limiting to prevent network congestion
  - Memory management with garbage collection hints
  - Connection pooling for optimal network utilization

📊 PERFORMANCE EXPECTATIONS
  Low Mode:      ~5-15 hashes/minute, <30%% CPU usage
  Balanced Mode: ~15-40 hashes/minute, ~70%% CPU usage
  High Mode:     ~30-80+ hashes/minute, ~90%% CPU usage

  Note: Actual performance varies based on network conditions,
  DHT activity levels, and system specifications.
`, 
		0.3*100, numCPU,
		numCPU*2, numCPU*10,
		0.7*100, numCPU,
		numCPU*4, numCPU*50,
		0.9*100, numCPU,
		numCPU*8, numCPU*100)
}

func displayConfig(config *EnhancedCrawlerConfig) {
	fmt.Println("================================================================================")
	fmt.Println("🔧 ENHANCED DHT CRAWLER CONFIGURATION")
	fmt.Println("================================================================================")
	fmt.Printf("💻 System: %d CPU cores, %d goroutines active\n", runtime.NumCPU(), runtime.NumGoroutine())
	fmt.Printf("🎛️  CPU Limit: %.1f%% | Memory Limit: %d MB\n", config.MaxCPUPercent*100, config.MaxMemoryMB)
	fmt.Printf("🌐 Connections: %d | Rate Limit: %d/sec\n", config.ConnPoolSize, config.RateLimitPerSec)
	fmt.Printf("📊 Stats Interval: %v | Adaptive Scaling: %v\n", config.StatsInterval, config.AdaptiveScaling)
	fmt.Printf("💥 Burst Mode: %v | Strategies: %v\n", config.BurstMode, config.DiscoveryStrategies)
	fmt.Printf("⏱️  Network Timeout: %v | File Flush: %v\n", config.NetworkTimeout, config.FlushInterval)
	fmt.Println("================================================================================")
}

func displayFinalStats(crawler *EnhancedDHTCrawler) {
	uptime := time.Since(crawler.startTime)
	hashCount := atomic.LoadInt64(&crawler.hashCount)
	queryCount := atomic.LoadInt64(&crawler.queryCount)
	
	// Count discovered nodes
	nodeCount := 0
	crawler.discoveredNodes.Range(func(_, _ interface{}) bool {
		nodeCount++
		return true
	})
	
	fmt.Println("\n================================================================================")
	fmt.Println("📊 FINAL STATISTICS")
	fmt.Println("================================================================================")
	fmt.Printf("⏱️  Total Runtime: %v\n", uptime.Round(time.Second))
	fmt.Printf("🎯 Total Hashes Discovered: %d\n", hashCount)
	fmt.Printf("🔍 Total Queries Sent: %d\n", queryCount)
	fmt.Printf("🌐 Nodes Discovered: %d\n", nodeCount)
	fmt.Printf("📈 Average Hash Rate: %.2f hashes/minute\n", float64(hashCount)/uptime.Minutes())
	fmt.Printf("📈 Average Query Rate: %.2f queries/minute\n", float64(queryCount)/uptime.Minutes())
	fmt.Printf("⚡ Overall Efficiency: %.4f hashes/query\n", float64(hashCount)/math.Max(float64(queryCount), 1))
	fmt.Printf("💾 Output File: %s\n", crawler.outputFile.Name())
	
	// Performance tier assessment
	hashesPerMin := float64(hashCount) / uptime.Minutes()
	var performanceTier string
	switch {
	case hashesPerMin >= 50:
		performanceTier = "🏆 EXCELLENT (50+ hashes/min)"
	case hashesPerMin >= 25:
		performanceTier = "🥇 VERY GOOD (25-50 hashes/min)"
	case hashesPerMin >= 10:
		performanceTier = "🥈 GOOD (10-25 hashes/min)"
	case hashesPerMin >= 5:
		performanceTier = "🥉 FAIR (5-10 hashes/min)"
	default:
		performanceTier = "📊 BASELINE (<5 hashes/min)"
	}
	
	fmt.Printf("🏅 Performance Tier: %s\n", performanceTier)
	fmt.Println("================================================================================")
	fmt.Println("✅ Enhanced DHT Crawler shutdown complete")
}