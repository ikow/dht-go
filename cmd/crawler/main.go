package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"dht-crawler/pkg/dht"
	"dht-crawler/pkg/metadata"
)

// AppConfig holds the application configuration
type AppConfig struct {
	DHT      *dht.CrawlerConfig
	Metadata *metadata.FetcherConfig
	Mode     string
	Output   OutputConfig
}

// OutputConfig holds output configuration
type OutputConfig struct {
	HashesFile     string
	MetadataFile   string
	MetadataFormat string // json, csv, or both
	EnableStats    bool
}

func main() {
	// Command line flags
	var (
		mode              = flag.String("mode", "balanced", "Operation mode: low, balanced, high")
		enableMetadata    = flag.Bool("metadata", true, "Enable metadata fetching")
		outputDir         = flag.String("output", "./output", "Output directory")
		metadataFormat    = flag.String("format", "json", "Metadata format: json, csv, or both")
		maxConcurrentMeta = flag.Int("meta-concurrent", 5, "Max concurrent metadata fetches")
		metadataTimeout   = flag.Duration("meta-timeout", 30*time.Second, "Metadata fetch timeout")
		enableStats       = flag.Bool("stats", true, "Enable statistics logging")
		showHelp          = flag.Bool("help", false, "Show help")
	)

	flag.Parse()

	if *showHelp {
		showUsage()
		return
	}

	// Create output directory
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Configure application
	config := &AppConfig{
		Mode: *mode,
		Output: OutputConfig{
			HashesFile:     filepath.Join(*outputDir, fmt.Sprintf("hashes_%s.txt", time.Now().Format("2006-01-02_15-04-05"))),
			MetadataFile:   filepath.Join(*outputDir, fmt.Sprintf("metadata_%s", time.Now().Format("2006-01-02_15-04-05"))),
			MetadataFormat: *metadataFormat,
			EnableStats:    *enableStats,
		},
	}

	// Configure DHT crawler based on mode
	switch *mode {
	case "low":
		config.DHT = dht.LowResourceConfig()
		log.Println("🔧 Selected: Low Resource Mode")
	case "high":
		config.DHT = dht.HighPerformanceConfig()
		log.Println("🔧 Selected: High Performance Mode")
	case "balanced":
		fallthrough
	default:
		config.DHT = dht.DefaultConfig()
		log.Println("🔧 Selected: Balanced Mode")
	}

	// Configure metadata fetcher
	if *enableMetadata {
		config.Metadata = metadata.DefaultFetcherConfig()
		config.Metadata.MaxConcurrentFetches = *maxConcurrentMeta
		config.Metadata.FetchTimeout = *metadataTimeout
		config.Metadata.DataDir = filepath.Join(*outputDir, "temp_metadata")
		log.Printf("🔧 Metadata fetching enabled: %d concurrent, %v timeout",
			*maxConcurrentMeta, *metadataTimeout)
	}

	// Create and start application
	app, err := NewApp(config)
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start application
	if err := app.Start(); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}

	// Wait for shutdown signal
	<-sigChan
	log.Println("🛑 Received shutdown signal")

	// Graceful shutdown
	if err := app.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	// Display final statistics
	app.DisplayFinalStats()
}

// App represents the main application
type App struct {
	config          *AppConfig
	dhtCrawler      *dht.Crawler
	metadataFetcher *metadata.Fetcher
	hashesFile      *os.File
	metadataFile    *os.File
	stats           *Stats
}

// Stats holds application statistics
type Stats struct {
	StartTime       time.Time
	HashesFound     int64
	MetadataFetched int64
	MetadataFailed  int64
}

// NewApp creates a new application instance
func NewApp(config *AppConfig) (*App, error) {
	app := &App{
		config: config,
		stats:  &Stats{StartTime: time.Now()},
	}

	// Create DHT crawler
	crawler, err := dht.New(config.DHT)
	if err != nil {
		return nil, fmt.Errorf("failed to create DHT crawler: %v", err)
	}
	app.dhtCrawler = crawler

	// Create metadata fetcher if enabled
	if config.Metadata != nil {
		fetcher, err := metadata.NewFetcher(config.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to create metadata fetcher: %v", err)
		}
		app.metadataFetcher = fetcher
	}

	// Setup output files
	if err := app.setupOutput(); err != nil {
		return nil, fmt.Errorf("failed to setup output: %v", err)
	}

	// Setup event handlers
	app.setupEventHandlers()

	return app, nil
}

// setupOutput creates output files
func (a *App) setupOutput() error {
	// Create hashes file
	file, err := os.OpenFile(a.config.Output.HashesFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create hashes file: %v", err)
	}
	a.hashesFile = file

	// Write header to hashes file
	a.hashesFile.WriteString("# DHT Crawler Hash Discovery Log\n")
	a.hashesFile.WriteString("# Format: hash|timestamp|source\n")
	a.hashesFile.WriteString(fmt.Sprintf("# Started: %s\n", time.Now().Format("2006-01-02 15:04:05")))

	// Create metadata file if metadata fetching is enabled
	if a.metadataFetcher != nil {
		metadataFileName := a.config.Output.MetadataFile
		if a.config.Output.MetadataFormat == "json" || a.config.Output.MetadataFormat == "both" {
			metadataFileName += ".json"
		} else {
			metadataFileName += ".csv"
		}

		file, err := os.OpenFile(metadataFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to create metadata file: %v", err)
		}
		a.metadataFile = file

		// Write header for JSON array
		if a.config.Output.MetadataFormat == "json" || a.config.Output.MetadataFormat == "both" {
			a.metadataFile.WriteString("[\n")
		}
	}

	return nil
}

// setupEventHandlers configures event handlers for crawler and fetcher
func (a *App) setupEventHandlers() {
	// DHT crawler hash discovery handler
	a.dhtCrawler.OnHashDiscovered = func(hash []byte, source string) {
		hexHash := fmt.Sprintf("%x", hash)

		// Write to hashes file
		entry := fmt.Sprintf("%s|%s|%s\n",
			hexHash,
			time.Now().Format("2006-01-02 15:04:05"),
			source)

		a.hashesFile.WriteString(entry)
		a.stats.HashesFound++

		// Queue for metadata fetching if enabled
		if a.metadataFetcher != nil {
			a.metadataFetcher.AddHash(hexHash)
		}
	}

	// Metadata fetcher handlers
	if a.metadataFetcher != nil {
		a.metadataFetcher.OnMetadataFetched = func(meta *metadata.TorrentMetadata) {
			a.stats.MetadataFetched++

			// Write metadata to file
			if a.config.Output.MetadataFormat == "json" || a.config.Output.MetadataFormat == "both" {
				jsonData, err := json.MarshalIndent(meta, "", "  ")
				if err == nil {
					if a.stats.MetadataFetched > 1 {
						a.metadataFile.WriteString(",\n")
					}
					a.metadataFile.Write(jsonData)
				}
			}

			// Log successful metadata fetch
			log.Printf("📄 Metadata fetched: %s [%s] (%d files, %s)",
				meta.InfoHash[:12]+"...",
				meta.Name,
				len(meta.Files),
				formatBytes(meta.TotalSize))
		}

		a.metadataFetcher.OnFetchFailed = func(hash string, err error) {
			a.stats.MetadataFailed++
			log.Printf("❌ Metadata fetch failed for %s: %v", hash[:12]+"...", err)
		}
	}
}

// Start begins the application
func (a *App) Start() error {
	log.Println("🚀 Starting DHT Crawler Application")

	// Start DHT crawler
	if err := a.dhtCrawler.Start(); err != nil {
		return fmt.Errorf("failed to start DHT crawler: %v", err)
	}

	// Start metadata fetcher if enabled
	if a.metadataFetcher != nil {
		if err := a.metadataFetcher.Start(); err != nil {
			return fmt.Errorf("failed to start metadata fetcher: %v", err)
		}
	}

	// Start statistics logging if enabled
	if a.config.Output.EnableStats {
		go a.statsLoop()
	}

	return nil
}

// Stop gracefully shuts down the application
func (a *App) Stop() error {
	log.Println("🛑 Shutting down application")

	// Stop DHT crawler
	if a.dhtCrawler != nil {
		a.dhtCrawler.Stop()
	}

	// Stop metadata fetcher
	if a.metadataFetcher != nil {
		a.metadataFetcher.Stop()
	}

	// Close output files
	if a.hashesFile != nil {
		a.hashesFile.Close()
	}

	if a.metadataFile != nil {
		// Close JSON array
		if a.config.Output.MetadataFormat == "json" || a.config.Output.MetadataFormat == "both" {
			a.metadataFile.WriteString("\n]")
		}
		a.metadataFile.Close()
	}

	return nil
}

// statsLoop displays periodic statistics
func (a *App) statsLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.displayStats()
		}
	}
}

// displayStats shows current statistics
func (a *App) displayStats() {
	uptime := time.Since(a.stats.StartTime)

	// Get DHT crawler stats
	dhtStats := a.dhtCrawler.GetStats()

	// Get metadata fetcher stats
	var metaStats map[string]interface{}
	if a.metadataFetcher != nil {
		metaStats = a.metadataFetcher.GetStats()
	}

	log.Printf("📊 Stats [%s uptime]:", uptime.Truncate(time.Second))
	log.Printf("   DHT: %d hashes (%.1f/s) | %d queries (%.1f/s) | %d nodes",
		dhtStats["hash_count"], dhtStats["hash_rate"],
		dhtStats["query_count"], dhtStats["query_rate"],
		dhtStats["node_count"])

	if metaStats != nil {
		log.Printf("   Metadata: %d success | %d failed | %d active | %d pending",
			metaStats["success_count"], metaStats["failure_count"],
			metaStats["active_fetches"], metaStats["pending_queue"])
	}

	log.Printf("   Memory: %dMB", dhtStats["memory_mb"])
}

// DisplayFinalStats shows final application statistics
func (a *App) DisplayFinalStats() {
	uptime := time.Since(a.stats.StartTime)

	log.Println("\n📊 Final Statistics:")
	log.Printf("   Runtime: %s", uptime.Truncate(time.Second))
	log.Printf("   Hashes discovered: %d", a.stats.HashesFound)
	log.Printf("   Metadata fetched: %d", a.stats.MetadataFetched)
	log.Printf("   Metadata failed: %d", a.stats.MetadataFailed)

	if a.stats.HashesFound > 0 {
		log.Printf("   Hash rate: %.2f hashes/minute",
			float64(a.stats.HashesFound)/(uptime.Minutes()))
	}

	if a.stats.MetadataFetched > 0 {
		successRate := float64(a.stats.MetadataFetched) /
			float64(a.stats.MetadataFetched+a.stats.MetadataFailed) * 100
		log.Printf("   Metadata success rate: %.1f%%", successRate)
	}

	log.Printf("   Output files:")
	log.Printf("     Hashes: %s", a.config.Output.HashesFile)
	if a.metadataFetcher != nil {
		log.Printf("     Metadata: %s", a.config.Output.MetadataFile)
	}
}

// formatBytes formats byte count as human readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// showUsage displays usage information
func showUsage() {
	fmt.Println(`
DHT Crawler with Metadata Fetching
==================================

USAGE:
  go run cmd/crawler/main.go [OPTIONS]

MODES:
  -mode low       Low resource mode (minimal CPU/memory usage)
  -mode balanced  Balanced mode (optimal performance/resource balance) [DEFAULT]
  -mode high      High performance mode (maximum discovery speed)

OPTIONS:
  -metadata              Enable metadata fetching (default: true)
  -output DIR            Output directory (default: ./output)
  -format FORMAT         Metadata format: json, csv, or both (default: json)
  -meta-concurrent N     Max concurrent metadata fetches (default: 5)
  -meta-timeout DURATION Metadata fetch timeout (default: 30s)
  -stats                 Enable statistics logging (default: true)
  -help                  Show this help

EXAMPLES:

  Basic usage (balanced mode with metadata):
    go run cmd/crawler/main.go

  High performance mode without metadata:
    go run cmd/crawler/main.go -mode high -metadata=false

  Custom output directory with more concurrent metadata fetches:
    go run cmd/crawler/main.go -output ./my_output -meta-concurrent 10

  Low resource mode with CSV metadata format:
    go run cmd/crawler/main.go -mode low -format csv

OUTPUT FILES:
  - hashes_YYYY-MM-DD_HH-MM-SS.txt   : Discovered torrent hashes
  - metadata_YYYY-MM-DD_HH-MM-SS.json: Detailed torrent metadata

FEATURES:
  ✅ High-performance DHT crawling
  ✅ Automatic metadata fetching
  ✅ Multiple output formats
  ✅ Resource usage monitoring
  ✅ Graceful shutdown
  ✅ Comprehensive logging
  ✅ Configurable concurrency
`)
}
