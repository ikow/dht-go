package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeebo/bencode"
)

// KRPC message structures
type KRPCMessage struct {
	T string                 `bencode:"t"`
	Y string                 `bencode:"y"`
	Q string                 `bencode:"q,omitempty"`
	A map[string]interface{} `bencode:"a,omitempty"`
	R map[string]interface{} `bencode:"r,omitempty"`
	E []interface{}          `bencode:"e,omitempty"`
}

// EnhancedCrawlerConfig holds advanced configuration
type EnhancedCrawlerConfig struct {
	MaxCPUPercent     float64       // Maximum CPU usage percentage (0.0-1.0)
	MaxMemoryMB       int           // Maximum memory usage in MB
	MaxGoroutines     int           // Maximum number of goroutines
	AdaptiveScaling   bool          // Enable adaptive resource scaling
	BurstMode         bool          // Enable burst mode for maximum performance
	RateLimitPerSec   int           // Rate limit per second
	ConnPoolSize      int           // Connection pool size
	BufferPoolSize    int           // Buffer pool size
	StatsInterval     time.Duration // Statistics display interval
	FlushInterval     time.Duration // File flush interval
	NetworkTimeout    time.Duration // Network operation timeout
	DiscoveryStrategies []string    // Discovery strategies to use
}

// DefaultEnhancedConfig returns a balanced configuration
func DefaultEnhancedConfig() *EnhancedCrawlerConfig {
	numCPU := runtime.NumCPU()
	return &EnhancedCrawlerConfig{
		MaxCPUPercent:     0.7,                // Use max 70% CPU
		MaxMemoryMB:       512,                // Max 512MB RAM
		MaxGoroutines:     numCPU * 50,        // 50 goroutines per CPU core
		AdaptiveScaling:   true,
		BurstMode:         false,
		RateLimitPerSec:   1000,              // 1000 queries per second
		ConnPoolSize:      numCPU * 4,        // 4 connections per CPU core
		BufferPoolSize:    1024,
		StatsInterval:     15 * time.Second,
		FlushInterval:     5 * time.Second,
		NetworkTimeout:    3 * time.Second,
		DiscoveryStrategies: []string{"aggressive", "bootstrap", "random", "adaptive"},
	}
}

// HighPerformanceConfig returns a configuration optimized for maximum hash discovery
func HighPerformanceConfig() *EnhancedCrawlerConfig {
	numCPU := runtime.NumCPU()
	return &EnhancedCrawlerConfig{
		MaxCPUPercent:     0.9,                // Use up to 90% CPU
		MaxMemoryMB:       1024,               // Max 1GB RAM
		MaxGoroutines:     numCPU * 100,       // 100 goroutines per CPU core
		AdaptiveScaling:   true,
		BurstMode:         true,
		RateLimitPerSec:   2000,              // 2000 queries per second
		ConnPoolSize:      numCPU * 8,        // 8 connections per CPU core
		BufferPoolSize:    2048,
		StatsInterval:     10 * time.Second,
		FlushInterval:     3 * time.Second,
		NetworkTimeout:    2 * time.Second,
		DiscoveryStrategies: []string{"aggressive", "bootstrap", "random", "adaptive", "burst"},
	}
}

// LowResourceConfig returns a configuration that minimizes system impact
func LowResourceConfig() *EnhancedCrawlerConfig {
	numCPU := runtime.NumCPU()
	return &EnhancedCrawlerConfig{
		MaxCPUPercent:     0.3,                // Use max 30% CPU
		MaxMemoryMB:       128,                // Max 128MB RAM
		MaxGoroutines:     numCPU * 10,        // 10 goroutines per CPU core
		AdaptiveScaling:   true,
		BurstMode:         false,
		RateLimitPerSec:   200,               // 200 queries per second
		ConnPoolSize:      numCPU * 2,        // 2 connections per CPU core
		BufferPoolSize:    256,
		StatsInterval:     30 * time.Second,
		FlushInterval:     10 * time.Second,
		NetworkTimeout:    5 * time.Second,
		DiscoveryStrategies: []string{"bootstrap", "random"},
	}
}

// EnhancedDHTCrawler represents the optimized DHT crawler
type EnhancedDHTCrawler struct {
	config         *EnhancedCrawlerConfig
	ctx            context.Context
	cancel         context.CancelFunc
	
	// Networking
	connPool       []*net.UDPConn
	bufferPool     sync.Pool
	
	// Node management
	nodeIDs        [][]byte
	discoveredNodes sync.Map // ip:port -> last_seen
	
	// Hash tracking
	hashSet        sync.Map
	hashCount      int64
	queryCount     int64
	
	// File operations
	outputFile     *os.File
	writeQueue     chan string
	
	// Performance monitoring
	startTime      time.Time
	lastCPUCheck   time.Time
	currentCPU     float64
	currentMemMB   int
	
	// Rate limiting
	rateLimiter    chan struct{}
	
	// Strategies
	strategies     map[string]DiscoveryStrategy
	
	wg             sync.WaitGroup
}

// DiscoveryStrategy interface for different discovery approaches
type DiscoveryStrategy interface {
	Start(ctx context.Context, crawler *EnhancedDHTCrawler) error
	Stop() error
	Name() string
}

// Enhanced bootstrap nodes with more reliable sources
var enhancedBootstrapNodes = []string{
	// Primary DHT bootstrap nodes
	"router.bittorrent.com:6881",
	"dht.transmissionbt.com:6881",
	"router.utorrent.com:6881",
	"dht.aelitis.com:6881",
	
	// Additional reliable nodes
	"node.bitcomet.com:6881",
	"dht.libtorrent.org:25401",
	"bt1.archive.org:6881",
	"bt2.archive.org:6881",
	
	// Public tracker DHT nodes
	"tracker.opentrackr.org:1337",
	"9.rarbg.com:2810",
	"tracker.openbittorrent.com:6969",
	"tracker.publicbt.com:80",
}

// NewEnhancedDHTCrawler creates a new enhanced DHT crawler
func NewEnhancedDHTCrawler(config *EnhancedCrawlerConfig) (*EnhancedDHTCrawler, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create output file with timestamp
	filename := fmt.Sprintf("enhanced_hashes_%s.txt", time.Now().Format("2006-01-02_15-04-05"))
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create output file: %v", err)
	}
	
	crawler := &EnhancedDHTCrawler{
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
		connPool:    make([]*net.UDPConn, config.ConnPoolSize),
		nodeIDs:     make([][]byte, config.ConnPoolSize),
		outputFile:  file,
		writeQueue:  make(chan string, 10000), // Large write buffer
		startTime:   time.Now(),
		rateLimiter: make(chan struct{}, config.RateLimitPerSec),
		strategies:  make(map[string]DiscoveryStrategy),
	}
	
	// Initialize buffer pool
	crawler.bufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 2048)
		},
	}
	
	// Create connection pool
	if err := crawler.initConnectionPool(); err != nil {
		crawler.Close()
		return nil, fmt.Errorf("failed to initialize connection pool: %v", err)
	}
	
	// Generate node IDs
	for i := range crawler.nodeIDs {
		nodeID := make([]byte, 20)
		rand.Read(nodeID)
		crawler.nodeIDs[i] = nodeID
	}
	
	// Initialize discovery strategies
	crawler.initStrategies()
	
	// Initialize rate limiter
	go crawler.rateLimiterLoop()
	
	// Start file writer
	go crawler.fileWriterLoop()
	
	// Start resource monitor
	if config.AdaptiveScaling {
		go crawler.resourceMonitorLoop()
	}
	
	return crawler, nil
}

// initConnectionPool creates and initializes the UDP connection pool
func (c *EnhancedDHTCrawler) initConnectionPool() error {
	for i := 0; i < c.config.ConnPoolSize; i++ {
		conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 0})
		if err != nil {
			// Close already created connections
			for j := 0; j < i; j++ {
				if c.connPool[j] != nil {
					c.connPool[j].Close()
				}
			}
			return err
		}
		
		// Set socket options for better performance
		if err := c.optimizeSocket(conn); err != nil {
			log.Printf("Warning: Failed to optimize socket %d: %v", i, err)
		}
		
		c.connPool[i] = conn
		
		// Start receiver for this connection
		go c.receiverLoop(i, conn)
	}
	
	log.Printf("✅ Connection pool initialized with %d connections", c.config.ConnPoolSize)
	return nil
}

// optimizeSocket applies performance optimizations to UDP sockets
func (c *EnhancedDHTCrawler) optimizeSocket(conn *net.UDPConn) error {
	// Set socket buffer sizes
	if err := conn.SetReadBuffer(1024 * 1024); err != nil { // 1MB read buffer
		return err
	}
	if err := conn.SetWriteBuffer(1024 * 1024); err != nil { // 1MB write buffer
		return err
	}
	return nil
}

// rateLimiterLoop manages the rate limiting for network operations
func (c *EnhancedDHTCrawler) rateLimiterLoop() {
	ticker := time.NewTicker(time.Second / time.Duration(c.config.RateLimitPerSec))
	defer ticker.Stop()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			select {
			case c.rateLimiter <- struct{}{}:
			default:
				// Rate limiter is full, continue
			}
		}
	}
}

// fileWriterLoop handles asynchronous file writing to reduce blocking
func (c *EnhancedDHTCrawler) fileWriterLoop() {
	flushTicker := time.NewTicker(c.config.FlushInterval)
	defer flushTicker.Stop()
	
	var pendingWrites []string
	
	for {
		select {
		case <-c.ctx.Done():
			// Flush remaining writes
			for _, entry := range pendingWrites {
				c.outputFile.WriteString(entry)
			}
			if len(pendingWrites) > 0 {
				c.outputFile.Sync()
			}
			return
			
		case entry := <-c.writeQueue:
			pendingWrites = append(pendingWrites, entry)
			
			// Batch write for performance
			if len(pendingWrites) >= 100 {
				c.flushWrites(pendingWrites)
				pendingWrites = pendingWrites[:0]
			}
			
		case <-flushTicker.C:
			if len(pendingWrites) > 0 {
				c.flushWrites(pendingWrites)
				pendingWrites = pendingWrites[:0]
			}
		}
	}
}

// flushWrites performs batch writing to file
func (c *EnhancedDHTCrawler) flushWrites(entries []string) {
	for _, entry := range entries {
		c.outputFile.WriteString(entry)
	}
	c.outputFile.Sync()
}

// resourceMonitorLoop monitors system resources and adjusts performance
func (c *EnhancedDHTCrawler) resourceMonitorLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.checkResources()
		}
	}
}

// checkResources monitors CPU and memory usage
func (c *EnhancedDHTCrawler) checkResources() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	currentMemMB := int(m.Alloc / 1024 / 1024)
	
	// Simple CPU estimation based on goroutine count and time
	numGoroutines := runtime.NumGoroutine()
	cpuEstimate := float64(numGoroutines) / float64(c.config.MaxGoroutines)
	
	c.currentMemMB = currentMemMB
	c.currentCPU = cpuEstimate
	
	// Adjust rate limiting based on resource usage
	if currentMemMB > c.config.MaxMemoryMB || cpuEstimate > c.config.MaxCPUPercent {
		// Reduce rate limit
		newRate := int(float64(c.config.RateLimitPerSec) * 0.8)
		if newRate < 50 {
			newRate = 50 // Minimum rate
		}
		log.Printf("⚠️  High resource usage detected. Reducing rate to %d/sec", newRate)
		
		// Force garbage collection if memory is high
		if currentMemMB > c.config.MaxMemoryMB {
			runtime.GC()
		}
	}
}

// receiverLoop handles incoming UDP messages for a specific connection
func (c *EnhancedDHTCrawler) receiverLoop(connID int, conn *net.UDPConn) {
	log.Printf("👂 Receiver %d started on %s", connID, conn.LocalAddr())
	
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			buffer := c.bufferPool.Get().([]byte)
			
			conn.SetReadDeadline(time.Now().Add(c.config.NetworkTimeout))
			n, addr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				c.bufferPool.Put(buffer)
				continue
			}
			
			// Process message in a separate goroutine to avoid blocking
			go func(data []byte, remoteAddr *net.UDPAddr) {
				defer c.bufferPool.Put(buffer)
				c.processMessage(data[:n], remoteAddr, connID)
			}(buffer, addr)
		}
	}
}

// processMessage handles incoming DHT messages
func (c *EnhancedDHTCrawler) processMessage(data []byte, addr *net.UDPAddr, connID int) {
	var msg KRPCMessage
	if err := bencode.DecodeBytes(data, &msg); err != nil {
		return
	}
	
	// Record this node as discovered
	nodeKey := fmt.Sprintf("%s:%d", addr.IP.String(), addr.Port)
	c.discoveredNodes.Store(nodeKey, time.Now())
	
	// Process different message types
	switch msg.Y {
	case "q": // Query
		c.processQuery(&msg, addr, connID)
	case "r": // Response
		c.processResponse(&msg, addr, connID)
	}
}

// processQuery handles incoming DHT queries
func (c *EnhancedDHTCrawler) processQuery(msg *KRPCMessage, addr *net.UDPAddr, connID int) {
	switch msg.Q {
	case "announce_peer":
		// This is the most valuable - someone is announcing they have a torrent
		if infoHashInt, ok := msg.A["info_hash"]; ok {
			if infoHashStr, ok := infoHashInt.(string); ok && len(infoHashStr) == 20 {
				c.recordInfoHash([]byte(infoHashStr), "announce_peer")
			}
		}
		
	case "get_peers":
		// Someone is looking for peers for a torrent
		if infoHashInt, ok := msg.A["info_hash"]; ok {
			if infoHashStr, ok := infoHashInt.(string); ok && len(infoHashStr) == 20 {
				c.recordInfoHash([]byte(infoHashStr), "get_peers")
			}
		}
		
		// Send a response to encourage more queries
		c.sendGetPeersResponse(msg, addr, connID)
	}
}

// processResponse handles incoming DHT responses
func (c *EnhancedDHTCrawler) processResponse(msg *KRPCMessage, addr *net.UDPAddr, connID int) {
	if msg.R == nil {
		return
	}
	
	// Process discovered nodes
	if nodesInt, ok := msg.R["nodes"]; ok {
		if nodesStr, ok := nodesInt.(string); ok {
			c.processDiscoveredNodes([]byte(nodesStr), connID)
		}
	}
	
	// Process peer information
	if valuesInt, ok := msg.R["values"]; ok {
		// This indicates active torrent sharing
		c.processActiveContent(valuesInt)
	}
}

// recordInfoHash records a discovered torrent hash
func (c *EnhancedDHTCrawler) recordInfoHash(infoHash []byte, source string) {
	if len(infoHash) != 20 {
		return
	}
	
	hashStr := hex.EncodeToString(infoHash)
	
	// Check for duplicates
	if _, exists := c.hashSet.LoadOrStore(hashStr, true); exists {
		return
	}
	
	// Queue for writing
	entry := fmt.Sprintf("%s|%s|%s\n", hashStr, time.Now().Format("2006-01-02 15:04:05"), source)
	select {
	case c.writeQueue <- entry:
		atomic.AddInt64(&c.hashCount, 1)
		log.Printf("🎯 Hash discovered [%s]: %s (Total: %d)", source, hashStr, atomic.LoadInt64(&c.hashCount))
	default:
		// Write queue is full, drop this hash to avoid blocking
		log.Printf("⚠️  Write queue full, dropping hash: %s", hashStr)
	}
}

// processDiscoveredNodes processes nodes discovered in DHT responses
func (c *EnhancedDHTCrawler) processDiscoveredNodes(nodes []byte, connID int) {
	// Each node is 26 bytes: 20 bytes ID + 4 bytes IP + 2 bytes port
	for i := 0; i+26 <= len(nodes); i += 26 {
		ip := net.IP(nodes[i+20 : i+24])
		port := int(uint16(nodes[i+24])<<8 | uint16(nodes[i+25]))
		
		if port > 0 && port < 65536 && !ip.IsLoopback() && !ip.IsMulticast() {
			nodeKey := fmt.Sprintf("%s:%d", ip.String(), port)
			
			// Only query nodes we haven't seen recently
			if lastSeen, exists := c.discoveredNodes.Load(nodeKey); !exists || 
				time.Since(lastSeen.(time.Time)) > 10*time.Minute {
				
				addr := &net.UDPAddr{IP: ip, Port: port}
				c.discoveredNodes.Store(nodeKey, time.Now())
				
				// Send queries to newly discovered nodes
				go c.queryDiscoveredNode(addr, connID)
			}
		}
	}
}

// queryDiscoveredNode sends queries to a newly discovered node
func (c *EnhancedDHTCrawler) queryDiscoveredNode(addr *net.UDPAddr, connID int) {
	// Rate limiting
	select {
	case <-c.rateLimiter:
	case <-c.ctx.Done():
		return
	}
	
	conn := c.connPool[connID]
	nodeID := c.nodeIDs[connID]
	
	// Generate random target for discovery
	target := make([]byte, 20)
	rand.Read(target)
	
	// Send get_peers query (more likely to return info hashes)
	c.sendGetPeers(conn, addr, target, nodeID)
	atomic.AddInt64(&c.queryCount, 1)
}

// processActiveContent processes information about active content sharing
func (c *EnhancedDHTCrawler) processActiveContent(values interface{}) {
	// This could be enhanced to track peer popularity and target high-activity torrents
	// For now, just note that we've seen active content
}

// sendGetPeersResponse sends a response to get_peers queries to encourage more traffic
func (c *EnhancedDHTCrawler) sendGetPeersResponse(query *KRPCMessage, addr *net.UDPAddr, connID int) {
	conn := c.connPool[connID]
	nodeID := c.nodeIDs[connID]
	
	// Send a basic response with our node ID
	response := KRPCMessage{
		T: query.T,
		Y: "r",
		R: map[string]interface{}{
			"id":    string(nodeID),
			"token": "aa", // Simple token
			"nodes": "",   // No nodes to report
		},
	}
	
	c.sendMessage(conn, addr, &response)
}

// sendGetPeers sends a get_peers query
func (c *EnhancedDHTCrawler) sendGetPeers(conn *net.UDPConn, addr *net.UDPAddr, infoHash, nodeID []byte) {
	transactionID := make([]byte, 2)
	rand.Read(transactionID)
	
	msg := KRPCMessage{
		T: string(transactionID),
		Y: "q",
		Q: "get_peers",
		A: map[string]interface{}{
			"id":        string(nodeID),
			"info_hash": string(infoHash),
		},
	}
	
	c.sendMessage(conn, addr, &msg)
}

// sendFindNode sends a find_node query
func (c *EnhancedDHTCrawler) sendFindNode(conn *net.UDPConn, addr *net.UDPAddr, target, nodeID []byte) {
	transactionID := make([]byte, 2)
	rand.Read(transactionID)
	
	msg := KRPCMessage{
		T: string(transactionID),
		Y: "q",
		Q: "find_node",
		A: map[string]interface{}{
			"id":     string(nodeID),
			"target": string(target),
		},
	}
	
	c.sendMessage(conn, addr, &msg)
}

// sendMessage sends a KRPC message
func (c *EnhancedDHTCrawler) sendMessage(conn *net.UDPConn, addr *net.UDPAddr, msg *KRPCMessage) {
	data, err := bencode.EncodeBytes(msg)
	if err != nil {
		return
	}
	
	conn.SetWriteDeadline(time.Now().Add(c.config.NetworkTimeout))
	conn.WriteToUDP(data, addr)
}

// Run starts the enhanced DHT crawler
func (c *EnhancedDHTCrawler) Run() error {
	log.Printf("🚀 Enhanced DHT Crawler started")
	log.Printf("📊 Config: %d connections, %d max goroutines, %.0f%% max CPU, %dMB max memory", 
		c.config.ConnPoolSize, c.config.MaxGoroutines, c.config.MaxCPUPercent*100, c.config.MaxMemoryMB)
	log.Printf("💾 Output file: %s", c.outputFile.Name())
	
	// Start discovery strategies
	for _, strategyName := range c.config.DiscoveryStrategies {
		if strategy, exists := c.strategies[strategyName]; exists {
			log.Printf("🔍 Starting discovery strategy: %s", strategyName)
			if err := strategy.Start(c.ctx, c); err != nil {
				log.Printf("❌ Failed to start strategy %s: %v", strategyName, err)
			}
		}
	}
	
	// Start statistics display
	go c.statsLoop()
	
	// Wait for context cancellation
	<-c.ctx.Done()
	
	log.Println("🛑 Shutting down Enhanced DHT Crawler...")
	return nil
}

// statsLoop displays periodic statistics
func (c *EnhancedDHTCrawler) statsLoop() {
	ticker := time.NewTicker(c.config.StatsInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.displayStats()
		}
	}
}

// displayStats shows current crawler statistics
func (c *EnhancedDHTCrawler) displayStats() {
	uptime := time.Since(c.startTime)
	hashCount := atomic.LoadInt64(&c.hashCount)
	queryCount := atomic.LoadInt64(&c.queryCount)
	
	hashRate := float64(hashCount) / uptime.Minutes()
	queryRate := float64(queryCount) / uptime.Minutes()
	
	// Count discovered nodes
	nodeCount := 0
	c.discoveredNodes.Range(func(_, _ interface{}) bool {
		nodeCount++
		return true
	})
	
	fmt.Println("================================================================================")
	fmt.Println("🚀 ENHANCED DHT CRAWLER - OPTIMIZED MODE")
	fmt.Println("================================================================================")
	fmt.Printf("⏱️  Uptime: %v\n", uptime.Round(time.Second))
	fmt.Printf("🎯 Hashes Discovered: %d (%.2f/min)\n", hashCount, hashRate)
	fmt.Printf("🔍 Queries Sent: %d (%.2f/min)\n", queryCount, queryRate)
	fmt.Printf("🌐 Discovered Nodes: %d\n", nodeCount)
	fmt.Printf("📡 Connections: %d | 🔧 Goroutines: %d\n", c.config.ConnPoolSize, runtime.NumGoroutine())
	fmt.Printf("💾 Output File: %s\n", c.outputFile.Name())
	fmt.Printf("📊 CPU: %.1f%% | Memory: %dMB\n", c.currentCPU*100, c.currentMemMB)
	fmt.Printf("⚡ Efficiency: %.3f hashes/query\n", float64(hashCount)/math.Max(float64(queryCount), 1))
	fmt.Printf("🎛️  Rate Limit: %d/sec | Strategies: %v\n", c.config.RateLimitPerSec, c.config.DiscoveryStrategies)
	fmt.Println("================================================================================")
}

// Close gracefully shuts down the crawler
func (c *EnhancedDHTCrawler) Close() error {
	log.Println("🛑 Shutting down Enhanced DHT Crawler...")
	
	// Cancel context to stop all goroutines
	c.cancel()
	
	// Stop all strategies
	for name, strategy := range c.strategies {
		log.Printf("🛑 Stopping strategy: %s", name)
		strategy.Stop()
	}
	
	// Close connections
	for i, conn := range c.connPool {
		if conn != nil {
			log.Printf("🔌 Closing connection %d", i)
			conn.Close()
		}
	}
	
	// Close channels
	close(c.writeQueue)
	close(c.rateLimiter)
	
	// Close output file
	if c.outputFile != nil {
		c.outputFile.Close()
	}
	
	log.Println("✅ Enhanced DHT Crawler stopped")
	return nil
}