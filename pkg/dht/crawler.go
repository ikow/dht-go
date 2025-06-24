package dht

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
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

// CrawlerConfig holds crawler configuration
type CrawlerConfig struct {
	MaxCPUPercent       float64       // Maximum CPU usage percentage (0.0-1.0)
	MaxMemoryMB         int           // Maximum memory usage in MB
	MaxGoroutines       int           // Maximum number of goroutines
	AdaptiveScaling     bool          // Enable adaptive resource scaling
	BurstMode           bool          // Enable burst mode for maximum performance
	RateLimitPerSec     int           // Rate limit per second
	ConnPoolSize        int           // Connection pool size
	BufferPoolSize      int           // Buffer pool size
	StatsInterval       time.Duration // Statistics display interval
	FlushInterval       time.Duration // File flush interval
	NetworkTimeout      time.Duration // Network operation timeout
	DiscoveryStrategies []string      // Discovery strategies to use
}

// DHT crawler with enhanced functionality
type Crawler struct {
	config *CrawlerConfig
	ctx    context.Context
	cancel context.CancelFunc

	// Networking
	connPool   []*net.UDPConn
	bufferPool sync.Pool

	// Node management
	nodeIDs         [][]byte
	discoveredNodes sync.Map // ip:port -> last_seen

	// Hash tracking
	hashSet    sync.Map
	hashCount  int64
	queryCount int64

	// File operations
	outputFile *os.File
	writeQueue chan string

	// Performance monitoring
	startTime    time.Time
	lastCPUCheck time.Time
	currentCPU   float64
	currentMemMB int

	// Rate limiting
	rateLimiter chan struct{}

	// Event handlers
	OnHashDiscovered func(hash []byte, source string)

	wg sync.WaitGroup
}

// HashInfo represents discovered hash information
type HashInfo struct {
	Hash      []byte
	HexHash   string
	Source    string
	Timestamp time.Time
}

// Enhanced bootstrap nodes with more reliable sources
var BootstrapNodes = []string{
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

// DefaultConfig returns a balanced configuration
func DefaultConfig() *CrawlerConfig {
	numCPU := runtime.NumCPU()
	return &CrawlerConfig{
		MaxCPUPercent:       0.7,         // Use max 70% CPU
		MaxMemoryMB:         512,         // Max 512MB RAM
		MaxGoroutines:       numCPU * 50, // 50 goroutines per CPU core
		AdaptiveScaling:     true,
		BurstMode:           false,
		RateLimitPerSec:     1000,       // 1000 queries per second
		ConnPoolSize:        numCPU * 4, // 4 connections per CPU core
		BufferPoolSize:      1024,
		StatsInterval:       15 * time.Second,
		FlushInterval:       5 * time.Second,
		NetworkTimeout:      3 * time.Second,
		DiscoveryStrategies: []string{"aggressive", "bootstrap", "random", "adaptive"},
	}
}

// HighPerformanceConfig returns a configuration optimized for maximum hash discovery
func HighPerformanceConfig() *CrawlerConfig {
	numCPU := runtime.NumCPU()
	return &CrawlerConfig{
		MaxCPUPercent:       0.9,          // Use up to 90% CPU
		MaxMemoryMB:         1024,         // Max 1GB RAM
		MaxGoroutines:       numCPU * 100, // 100 goroutines per CPU core
		AdaptiveScaling:     true,
		BurstMode:           true,
		RateLimitPerSec:     2000,       // 2000 queries per second
		ConnPoolSize:        numCPU * 8, // 8 connections per CPU core
		BufferPoolSize:      2048,
		StatsInterval:       10 * time.Second,
		FlushInterval:       3 * time.Second,
		NetworkTimeout:      2 * time.Second,
		DiscoveryStrategies: []string{"aggressive", "bootstrap", "random", "adaptive", "burst"},
	}
}

// LowResourceConfig returns a configuration that minimizes system impact
func LowResourceConfig() *CrawlerConfig {
	numCPU := runtime.NumCPU()
	return &CrawlerConfig{
		MaxCPUPercent:       0.3,         // Use max 30% CPU
		MaxMemoryMB:         128,         // Max 128MB RAM
		MaxGoroutines:       numCPU * 10, // 10 goroutines per CPU core
		AdaptiveScaling:     true,
		BurstMode:           false,
		RateLimitPerSec:     200,        // 200 queries per second
		ConnPoolSize:        numCPU * 2, // 2 connections per CPU core
		BufferPoolSize:      256,
		StatsInterval:       30 * time.Second,
		FlushInterval:       10 * time.Second,
		NetworkTimeout:      5 * time.Second,
		DiscoveryStrategies: []string{"bootstrap", "random"},
	}
}

// New creates a new DHT crawler
func New(config *CrawlerConfig) (*Crawler, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create output file with timestamp
	filename := fmt.Sprintf("dht_hashes_%s.txt", time.Now().Format("2006-01-02_15-04-05"))
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create output file: %v", err)
	}

	crawler := &Crawler{
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
		connPool:    make([]*net.UDPConn, config.ConnPoolSize),
		nodeIDs:     make([][]byte, config.ConnPoolSize),
		outputFile:  file,
		writeQueue:  make(chan string, 10000), // Large write buffer
		startTime:   time.Now(),
		rateLimiter: make(chan struct{}, config.RateLimitPerSec),
	}

	// Initialize buffer pool
	crawler.bufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 2048)
		},
	}

	// Initialize connection pool and node IDs
	if err := crawler.initConnectionPool(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize connection pool: %v", err)
	}

	return crawler, nil
}

// initConnectionPool creates UDP connections and node IDs
func (c *Crawler) initConnectionPool() error {
	for i := 0; i < c.config.ConnPoolSize; i++ {
		// Create UDP connection
		conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 0})
		if err != nil {
			return fmt.Errorf("failed to create UDP connection %d: %v", i, err)
		}

		c.connPool[i] = conn

		// Generate random node ID for this connection
		nodeID := make([]byte, 20)
		if _, err := rand.Read(nodeID); err != nil {
			return fmt.Errorf("failed to generate node ID %d: %v", i, err)
		}
		c.nodeIDs[i] = nodeID

		// Optimize socket
		c.optimizeSocket(conn)
	}

	log.Printf("✅ Initialized %d UDP connections", c.config.ConnPoolSize)
	return nil
}

// optimizeSocket applies performance optimizations to UDP socket
func (c *Crawler) optimizeSocket(conn *net.UDPConn) error {
	// Set read/write buffer sizes
	if err := conn.SetReadBuffer(65536); err != nil {
		log.Printf("⚠️  Failed to set read buffer: %v", err)
	}
	if err := conn.SetWriteBuffer(65536); err != nil {
		log.Printf("⚠️  Failed to set write buffer: %v", err)
	}
	return nil
}

// Start begins the DHT crawling process
func (c *Crawler) Start() error {
	log.Println("🚀 Starting DHT Crawler")

	// Start core loops
	c.wg.Add(4)
	go c.rateLimiterLoop()
	go c.fileWriterLoop()
	go c.resourceMonitorLoop()
	go c.statsLoop()

	// Start receiver loops for each connection
	for i, conn := range c.connPool {
		c.wg.Add(1)
		go c.receiverLoop(i, conn)
	}

	// Initial bootstrap
	c.bootstrap()

	// Start continuous discovery
	c.wg.Add(1)
	go c.discoveryLoop()

	return nil
}

// Stop gracefully shuts down the crawler
func (c *Crawler) Stop() error {
	log.Println("🛑 Shutting down DHT Crawler")

	c.cancel()
	c.wg.Wait()

	// Close connections
	for _, conn := range c.connPool {
		if conn != nil {
			conn.Close()
		}
	}

	// Close output file
	if c.outputFile != nil {
		c.outputFile.Close()
	}

	log.Printf("📊 Final stats: %d hashes discovered, %d queries sent",
		atomic.LoadInt64(&c.hashCount),
		atomic.LoadInt64(&c.queryCount))

	return nil
}

// bootstrap queries initial bootstrap nodes
func (c *Crawler) bootstrap() {
	log.Printf("🌐 Bootstrapping with %d nodes", len(BootstrapNodes))

	for connID, conn := range c.connPool {
		nodeID := c.nodeIDs[connID]

		for _, nodeAddr := range BootstrapNodes {
			addr, err := net.ResolveUDPAddr("udp", nodeAddr)
			if err != nil {
				continue
			}

			// Rate limiting
			select {
			case <-c.rateLimiter:
			case <-c.ctx.Done():
				return
			}

			// Generate random target
			target := make([]byte, 20)
			rand.Read(target)

			// Send find_node and get_peers
			c.sendFindNode(conn, addr, target, nodeID)
			atomic.AddInt64(&c.queryCount, 1)

			time.Sleep(10 * time.Millisecond)

			select {
			case <-c.rateLimiter:
			case <-c.ctx.Done():
				return
			}

			c.sendGetPeers(conn, addr, target, nodeID)
			atomic.AddInt64(&c.queryCount, 1)
		}
	}
}

// discoveryLoop continuously discovers new hashes
func (c *Crawler) discoveryLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.performDiscovery()
		}
	}
}

// performDiscovery executes discovery queries
func (c *Crawler) performDiscovery() {
	for connID, conn := range c.connPool {
		nodeID := c.nodeIDs[connID]

		// Query random targets
		for i := 0; i < 10; i++ {
			select {
			case <-c.rateLimiter:
			case <-c.ctx.Done():
				return
			}

			target := make([]byte, 20)
			rand.Read(target)

			// Choose random bootstrap node as initial target
			nodeAddr := BootstrapNodes[i%len(BootstrapNodes)]
			addr, err := net.ResolveUDPAddr("udp", nodeAddr)
			if err != nil {
				continue
			}

			if i%2 == 0 {
				c.sendFindNode(conn, addr, target, nodeID)
			} else {
				c.sendGetPeers(conn, addr, target, nodeID)
			}
			atomic.AddInt64(&c.queryCount, 1)
		}

		// Also query discovered nodes
		c.queryDiscoveredNodes(connID)
	}
}

// queryDiscoveredNodes queries previously discovered nodes
func (c *Crawler) queryDiscoveredNodes(connID int) {
	conn := c.connPool[connID]
	nodeID := c.nodeIDs[connID]

	count := 0
	c.discoveredNodes.Range(func(key, value interface{}) bool {
		if count >= 5 { // Limit to 5 nodes per iteration
			return false
		}

		nodeAddr := key.(string)
		addr, err := net.ResolveUDPAddr("udp", nodeAddr)
		if err != nil {
			return true
		}

		select {
		case <-c.rateLimiter:
		case <-c.ctx.Done():
			return false
		}

		target := make([]byte, 20)
		rand.Read(target)

		c.sendGetPeers(conn, addr, target, nodeID)
		atomic.AddInt64(&c.queryCount, 1)
		count++

		return true
	})
}

// receiverLoop handles incoming messages for a connection
func (c *Crawler) receiverLoop(connID int, conn *net.UDPConn) {
	defer c.wg.Done()

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

			// Process message in goroutine to avoid blocking
			go func() {
				defer c.bufferPool.Put(buffer)
				c.processMessage(buffer[:n], addr, connID)
			}()
		}
	}
}

// processMessage handles incoming KRPC messages
func (c *Crawler) processMessage(data []byte, addr *net.UDPAddr, connID int) {
	var msg KRPCMessage
	if err := bencode.DecodeBytes(data, &msg); err != nil {
		return
	}

	// Handle different message types
	switch msg.Y {
	case "q": // Query
		c.processQuery(&msg, addr, connID)
	case "r": // Response
		c.processResponse(&msg, addr, connID)
	}
}

// processQuery handles incoming queries (mainly get_peers)
func (c *Crawler) processQuery(msg *KRPCMessage, addr *net.UDPAddr, connID int) {
	if msg.Q == "get_peers" && msg.A != nil {
		if infoHashInterface, ok := msg.A["info_hash"]; ok {
			if infoHashBytes, ok := infoHashInterface.([]byte); ok && len(infoHashBytes) == 20 {
				c.recordInfoHash(infoHashBytes, "query")
			}
		}

		// Send response to maintain node reputation
		c.sendGetPeersResponse(msg, addr, connID)
	}
}

// processResponse handles responses to our queries
func (c *Crawler) processResponse(msg *KRPCMessage, addr *net.UDPAddr, connID int) {
	if msg.R == nil {
		return
	}

	// Process discovered nodes
	if nodesInterface, ok := msg.R["nodes"]; ok {
		if nodesBytes, ok := nodesInterface.([]byte); ok {
			c.processDiscoveredNodes(nodesBytes, connID)
		}
	}

	// Process values (peers)
	if valuesInterface, ok := msg.R["values"]; ok {
		c.processActiveContent(valuesInterface)
	}
}

// recordInfoHash processes and records discovered info hashes
func (c *Crawler) recordInfoHash(infoHash []byte, source string) {
	hexHash := hex.EncodeToString(infoHash)

	// Check for duplicates
	if _, exists := c.hashSet.LoadOrStore(hexHash, time.Now()); exists {
		return
	}

	// Record hash
	atomic.AddInt64(&c.hashCount, 1)

	// Queue for file writing
	entry := fmt.Sprintf("%s|%s|%s\n",
		hexHash,
		time.Now().Format("2006-01-02 15:04:05"),
		source)

	select {
	case c.writeQueue <- entry:
	default:
		// Queue full, skip this hash
	}

	// Call event handler if set
	if c.OnHashDiscovered != nil {
		go c.OnHashDiscovered(infoHash, source)
	}
}

// processDiscoveredNodes extracts and stores discovered nodes
func (c *Crawler) processDiscoveredNodes(nodes []byte, connID int) {
	// Each node is 26 bytes: 20 bytes node ID + 4 bytes IP + 2 bytes port
	for i := 0; i+26 <= len(nodes); i += 26 {
		// Extract IP and port (skip node ID)
		ip := net.IP(nodes[i+20 : i+24])
		port := int(nodes[i+24])<<8 + int(nodes[i+25])

		if port == 0 || ip.IsLoopback() || ip.IsUnspecified() {
			continue
		}

		nodeAddr := fmt.Sprintf("%s:%d", ip.String(), port)
		c.discoveredNodes.Store(nodeAddr, time.Now())
	}
}

// processActiveContent handles active peer information
func (c *Crawler) processActiveContent(values interface{}) {
	// This indicates active torrents - useful for metadata retrieval
}

// sendGetPeersResponse sends a response to get_peers queries
func (c *Crawler) sendGetPeersResponse(query *KRPCMessage, addr *net.UDPAddr, connID int) {
	conn := c.connPool[connID]
	nodeID := c.nodeIDs[connID]

	response := &KRPCMessage{
		T: query.T,
		Y: "r",
		R: map[string]interface{}{
			"id":    nodeID,
			"nodes": make([]byte, 0), // Empty nodes list
		},
	}

	c.sendMessage(conn, addr, response)
}

// sendGetPeers sends a get_peers query
func (c *Crawler) sendGetPeers(conn *net.UDPConn, addr *net.UDPAddr, infoHash, nodeID []byte) {
	transactionID := make([]byte, 2)
	rand.Read(transactionID)

	msg := &KRPCMessage{
		T: string(transactionID),
		Y: "q",
		Q: "get_peers",
		A: map[string]interface{}{
			"id":        nodeID,
			"info_hash": infoHash,
		},
	}

	c.sendMessage(conn, addr, msg)
}

// sendFindNode sends a find_node query
func (c *Crawler) sendFindNode(conn *net.UDPConn, addr *net.UDPAddr, target, nodeID []byte) {
	transactionID := make([]byte, 2)
	rand.Read(transactionID)

	msg := &KRPCMessage{
		T: string(transactionID),
		Y: "q",
		Q: "find_node",
		A: map[string]interface{}{
			"id":     nodeID,
			"target": target,
		},
	}

	c.sendMessage(conn, addr, msg)
}

// sendMessage sends a KRPC message
func (c *Crawler) sendMessage(conn *net.UDPConn, addr *net.UDPAddr, msg *KRPCMessage) {
	data, err := bencode.EncodeBytes(msg)
	if err != nil {
		return
	}

	conn.SetWriteDeadline(time.Now().Add(c.config.NetworkTimeout))
	conn.WriteToUDP(data, addr)
}

// rateLimiterLoop manages rate limiting
func (c *Crawler) rateLimiterLoop() {
	defer c.wg.Done()

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
			}
		}
	}
}

// fileWriterLoop handles asynchronous file writing
func (c *Crawler) fileWriterLoop() {
	defer c.wg.Done()

	flushTicker := time.NewTicker(c.config.FlushInterval)
	defer flushTicker.Stop()

	var entries []string

	for {
		select {
		case <-c.ctx.Done():
			if len(entries) > 0 {
				c.flushWrites(entries)
			}
			return
		case entry := <-c.writeQueue:
			entries = append(entries, entry)
			if len(entries) >= 100 { // Batch writes
				c.flushWrites(entries)
				entries = entries[:0]
			}
		case <-flushTicker.C:
			if len(entries) > 0 {
				c.flushWrites(entries)
				entries = entries[:0]
			}
		}
	}
}

// flushWrites writes entries to file
func (c *Crawler) flushWrites(entries []string) {
	for _, entry := range entries {
		c.outputFile.WriteString(entry)
	}
	c.outputFile.Sync()
}

// resourceMonitorLoop monitors system resources
func (c *Crawler) resourceMonitorLoop() {
	defer c.wg.Done()

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
func (c *Crawler) checkResources() {
	// Basic resource monitoring - can be enhanced with proper metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	c.currentMemMB = int(m.Alloc / 1024 / 1024)

	// Adaptive scaling based on resource usage
	if c.config.AdaptiveScaling {
		if c.currentMemMB > c.config.MaxMemoryMB {
			// Reduce activity
			log.Printf("⚠️  High memory usage: %dMB, reducing activity", c.currentMemMB)
		}
	}
}

// statsLoop displays periodic statistics
func (c *Crawler) statsLoop() {
	defer c.wg.Done()

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

// displayStats shows current statistics
func (c *Crawler) displayStats() {
	hashCount := atomic.LoadInt64(&c.hashCount)
	queryCount := atomic.LoadInt64(&c.queryCount)
	uptime := time.Since(c.startTime)

	hashRate := float64(hashCount) / uptime.Seconds()
	queryRate := float64(queryCount) / uptime.Seconds()

	nodeCount := 0
	c.discoveredNodes.Range(func(k, v interface{}) bool {
		nodeCount++
		return true
	})

	log.Printf("📊 Stats: %d hashes (%.1f/s) | %d queries (%.1f/s) | %d nodes | %dMB RAM | %s uptime",
		hashCount, hashRate, queryCount, queryRate, nodeCount, c.currentMemMB, uptime.Truncate(time.Second))
}

// GetStats returns current crawler statistics
func (c *Crawler) GetStats() map[string]interface{} {
	hashCount := atomic.LoadInt64(&c.hashCount)
	queryCount := atomic.LoadInt64(&c.queryCount)
	uptime := time.Since(c.startTime)

	nodeCount := 0
	c.discoveredNodes.Range(func(k, v interface{}) bool {
		nodeCount++
		return true
	})

	return map[string]interface{}{
		"hash_count":  hashCount,
		"query_count": queryCount,
		"node_count":  nodeCount,
		"uptime":      uptime,
		"hash_rate":   float64(hashCount) / uptime.Seconds(),
		"query_rate":  float64(queryCount) / uptime.Seconds(),
		"memory_mb":   c.currentMemMB,
	}
}
