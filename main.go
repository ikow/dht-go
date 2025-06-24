package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeebo/bencode"
)

// Configuration structure
type CrawlerConfig struct {
	TurboMode     bool
	PortCount     int
	WorkerCount   int
	NodeCount     int
	QueryRate     int
	StatsInterval int
}

// Multi-threaded DHT crawler with aggressive parallelization
type MultiThreadDHTCrawler struct {
	connections []*net.UDPConn
	nodeIDs     [][]byte
	hashCount   int64
	queryCount  int64
	startTime   time.Time
	outputFile  *os.File
	hashSet     sync.Map // Deduplication
	messageChan chan MessageJob
	workerCount int
	portCount   int
	wg          sync.WaitGroup
	shutdown    chan bool
	config      *CrawlerConfig
}

// Simple single-threaded DHT crawler
type SimpleDHTCrawler struct {
	conn       *net.UDPConn
	nodeID     []byte
	hashCount  int64
	queryCount int64
	startTime  time.Time
	outputFile *os.File
	hashSet    sync.Map
	shutdown   chan bool
	config     *CrawlerConfig
}

type MessageJob struct {
	data []byte
	addr *net.UDPAddr
	conn *net.UDPConn
}

// KRPC message structures
type KRPCMessage struct {
	T string                 `bencode:"t"`
	Y string                 `bencode:"y"`
	Q string                 `bencode:"q,omitempty"`
	A map[string]interface{} `bencode:"a,omitempty"`
	R map[string]interface{} `bencode:"r,omitempty"`
	E []interface{}          `bencode:"e,omitempty"`
}

// Enhanced bootstrap nodes list
var bootstrapNodes = []string{
	"router.bittorrent.com:6881",
	"dht.transmissionbt.com:6881",
	"router.utorrent.com:6881",
	"dht.aelitis.com:6881",
	"node.bitcomet.com:6881",
	"dht.libtorrent.org:25401",
	"bt1.archive.org:6881",
	"bt2.archive.org:6881",
}

func main() {
	// Command line flags
	var (
		turboMode     = flag.Bool("turbo", true, "Enable turbo mode (multi-threaded)")
		portCount     = flag.Int("ports", 8, "Number of UDP ports (turbo mode only)")
		workerCount   = flag.Int("workers", 16, "Number of worker threads (turbo mode only)")
		nodeCount     = flag.Int("nodes", 8, "Number of virtual node IDs")
		queryRate     = flag.Int("rate", 50, "Queries per interval")
		statsInterval = flag.Int("stats", 15, "Statistics display interval (seconds)")
		showHelp      = flag.Bool("help", false, "Show help message")
	)

	flag.Parse()

	if *showHelp {
		showUsage()
		return
	}

	config := &CrawlerConfig{
		TurboMode:     *turboMode,
		PortCount:     *portCount,
		WorkerCount:   *workerCount,
		NodeCount:     *nodeCount,
		QueryRate:     *queryRate,
		StatsInterval: *statsInterval,
	}

	if config.TurboMode {
		log.Println("🚀 Starting Multi-Threaded DHT Crawler - TURBO MODE")
		runTurboMode(config)
	} else {
		log.Println("🚀 Starting Simple DHT Crawler - NORMAL MODE")
		runNormalMode(config)
	}
}

func showUsage() {
	fmt.Println(`
DHT Crawler - High-Performance Torrent Hash Discovery

USAGE:
  go run main.go [OPTIONS]

MODES:
  Normal Mode:  Single-threaded, lower resource usage
  Turbo Mode:   Multi-threaded, maximum performance (default)

OPTIONS:
  -turbo        Enable turbo mode (default: true)
  -ports N      Number of UDP ports for turbo mode (default: 8)
  -workers N    Number of worker threads for turbo mode (default: 16)
  -nodes N      Number of virtual node IDs (default: 8)
  -rate N       Queries per interval (default: 50)
  -stats N      Statistics interval in seconds (default: 15)
  -help         Show this help message

EXAMPLES:
  Normal mode:
    go run main.go -turbo=false

  Turbo mode with custom settings:
    go run main.go -turbo=true -ports=12 -workers=24 -nodes=12

  Light turbo mode:
    go run main.go -ports=4 -workers=8 -nodes=4 -rate=25

  Maximum performance:
    go run main.go -ports=16 -workers=32 -nodes=16 -rate=100
`)
}

func runTurboMode(config *CrawlerConfig) {
	crawler, err := NewMultiThreadDHTCrawler(config)
	if err != nil {
		log.Fatalf("Failed to create turbo crawler: %v", err)
	}
	defer crawler.Close()

	crawler.Run()
}

func runNormalMode(config *CrawlerConfig) {
	crawler, err := NewSimpleDHTCrawler(config)
	if err != nil {
		log.Fatalf("Failed to create simple crawler: %v", err)
	}
	defer crawler.Close()

	crawler.Run()
}

// ============================================================================
// TURBO MODE IMPLEMENTATION
// ============================================================================

func NewMultiThreadDHTCrawler(config *CrawlerConfig) (*MultiThreadDHTCrawler, error) {
	// Create output file
	filename := fmt.Sprintf("hashes_turbo_%s.txt", time.Now().Format("2006-01-02"))
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	crawler := &MultiThreadDHTCrawler{
		connections: make([]*net.UDPConn, config.PortCount),
		nodeIDs:     make([][]byte, config.NodeCount),
		startTime:   time.Now(),
		outputFile:  file,
		messageChan: make(chan MessageJob, 10000), // Large buffer
		workerCount: config.WorkerCount,
		portCount:   config.PortCount,
		shutdown:    make(chan bool),
		config:      config,
	}

	// Create multiple UDP connections
	for i := 0; i < config.PortCount; i++ {
		conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 0})
		if err != nil {
			crawler.closeConnections()
			return nil, err
		}
		crawler.connections[i] = conn
	}

	// Generate multiple node IDs
	for i := 0; i < config.NodeCount; i++ {
		nodeID := make([]byte, 20)
		rand.Read(nodeID)
		crawler.nodeIDs[i] = nodeID
	}

	return crawler, nil
}

func (c *MultiThreadDHTCrawler) closeConnections() {
	for _, conn := range c.connections {
		if conn != nil {
			conn.Close()
		}
	}
}

func (c *MultiThreadDHTCrawler) Run() {
	log.Printf("✅ Multi-Threaded DHT Crawler started with %d ports, %d workers, %d node IDs",
		c.portCount, c.workerCount, len(c.nodeIDs))

	for i, conn := range c.connections {
		log.Printf("📡 UDP Listener %d: %s", i+1, conn.LocalAddr())
	}
	log.Printf("💾 Output file: %s", c.outputFile.Name())

	// Start worker pools
	c.startWorkerPool()

	// Start UDP listeners
	c.startUDPListeners()

	// Start statistics display
	go c.displayStats()

	// Bootstrap the DHT aggressively
	c.aggressiveBootstrap()

	// Start aggressive crawling
	c.startAggressiveCrawling()

	// Wait for shutdown
	<-c.shutdown
}

func (c *MultiThreadDHTCrawler) startWorkerPool() {
	for i := 0; i < c.workerCount; i++ {
		c.wg.Add(1)
		go func(workerID int) {
			defer c.wg.Done()
			log.Printf("🔧 Worker %d started", workerID)

			for job := range c.messageChan {
				c.processMessage(job.data, job.addr, job.conn)
			}
		}(i)
	}
}

func (c *MultiThreadDHTCrawler) startUDPListeners() {
	for i, conn := range c.connections {
		c.wg.Add(1)
		go func(connID int, connection *net.UDPConn) {
			defer c.wg.Done()
			log.Printf("👂 UDP Listener %d started", connID)

			buffer := make([]byte, 2048) // Larger buffer

			for {
				select {
				case <-c.shutdown:
					return
				default:
					connection.SetReadDeadline(time.Now().Add(1 * time.Second))
					n, addr, err := connection.ReadFromUDP(buffer)
					if err != nil {
						continue
					}

					// Send to worker pool
					select {
					case c.messageChan <- MessageJob{
						data: append([]byte(nil), buffer[:n]...),
						addr: addr,
						conn: connection,
					}:
					default:
						// Channel full, drop message
					}
				}
			}
		}(i, conn)
	}
}

func (c *MultiThreadDHTCrawler) aggressiveBootstrap() {
	log.Printf("📊 Aggressive bootstrapping with %d nodes...", len(bootstrapNodes))

	// Bootstrap from each connection with each node ID
	for _, conn := range c.connections {
		for _, nodeID := range c.nodeIDs {
			for _, node := range bootstrapNodes {
				addr, err := net.ResolveUDPAddr("udp", node)
				if err != nil {
					continue
				}

				// Send multiple types of queries per bootstrap node
				target := make([]byte, 20)
				rand.Read(target)

				c.sendFindNode(conn, addr, target, nodeID)
				c.sendGetPeers(conn, addr, target, nodeID)

				atomic.AddInt64(&c.queryCount, 2)
			}
		}
	}
}

func (c *MultiThreadDHTCrawler) startAggressiveCrawling() {
	// Multiple crawling goroutines with different strategies

	// Strategy 1: Random queries every 2 seconds
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-c.shutdown:
				return
			case <-ticker.C:
				c.sendRandomQueries(c.config.QueryRate) // Configurable query rate
			}
		}
	}()

	// Strategy 2: Bootstrap flooding every 30 seconds
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-c.shutdown:
				return
			case <-ticker.C:
				c.aggressiveBootstrap()
			}
		}
	}()

	// Strategy 3: Rapid fire queries every 1 second
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-c.shutdown:
				return
			case <-ticker.C:
				c.rapidFireQueries(c.config.QueryRate / 3) // Lower rate for rapid fire
			}
		}
	}()
}

func (c *MultiThreadDHTCrawler) sendRandomQueries(count int) {
	for i := 0; i < count; i++ {
		// Random connection and node ID
		conn := c.connections[i%len(c.connections)]
		nodeID := c.nodeIDs[i%len(c.nodeIDs)]

		// Random target
		target := make([]byte, 20)
		rand.Read(target)

		// Send to random bootstrap node
		node := bootstrapNodes[i%len(bootstrapNodes)]
		addr, err := net.ResolveUDPAddr("udp", node)
		if err != nil {
			continue
		}

		// Alternate query types
		if i%2 == 0 {
			c.sendGetPeers(conn, addr, target, nodeID)
		} else {
			c.sendFindNode(conn, addr, target, nodeID)
		}

		atomic.AddInt64(&c.queryCount, 1)
	}
}

func (c *MultiThreadDHTCrawler) rapidFireQueries(count int) {
	// Send queries to all bootstrap nodes in parallel
	var wg sync.WaitGroup

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			conn := c.connections[idx%len(c.connections)]
			nodeID := c.nodeIDs[idx%len(c.nodeIDs)]
			node := bootstrapNodes[idx%len(bootstrapNodes)]

			addr, err := net.ResolveUDPAddr("udp", node)
			if err != nil {
				return
			}

			target := make([]byte, 20)
			rand.Read(target)

			c.sendGetPeers(conn, addr, target, nodeID)
			atomic.AddInt64(&c.queryCount, 1)
		}(i)
	}

	wg.Wait()
}

func (c *MultiThreadDHTCrawler) sendFindNode(conn *net.UDPConn, addr *net.UDPAddr, target, nodeID []byte) {
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

func (c *MultiThreadDHTCrawler) sendGetPeers(conn *net.UDPConn, addr *net.UDPAddr, infoHash, nodeID []byte) {
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

func (c *MultiThreadDHTCrawler) sendMessage(conn *net.UDPConn, addr *net.UDPAddr, msg *KRPCMessage) {
	data, err := bencode.EncodeBytes(msg)
	if err != nil {
		return
	}

	conn.WriteToUDP(data, addr)
}

func (c *MultiThreadDHTCrawler) processMessage(data []byte, addr *net.UDPAddr, fromConn *net.UDPConn) {
	var msg KRPCMessage
	if err := bencode.DecodeBytes(data, &msg); err != nil {
		return
	}

	// Process announce_peer queries (highest value)
	if msg.Y == "q" && msg.Q == "announce_peer" {
		if infoHashInt, ok := msg.A["info_hash"]; ok {
			if infoHashStr, ok := infoHashInt.(string); ok && len(infoHashStr) == 20 {
				c.processInfoHash([]byte(infoHashStr))
			}
		}
	}

	// Process get_peers queries
	if msg.Y == "q" && msg.Q == "get_peers" {
		if infoHashInt, ok := msg.A["info_hash"]; ok {
			if infoHashStr, ok := infoHashInt.(string); ok && len(infoHashStr) == 20 {
				c.processInfoHash([]byte(infoHashStr))
			}
		}
	}

	// Process responses with nodes - discover new nodes to query
	if msg.Y == "r" && msg.R != nil {
		if nodesInt, ok := msg.R["nodes"]; ok {
			if nodesStr, ok := nodesInt.(string); ok {
				c.processNodes([]byte(nodesStr), fromConn)
			}
		}

		// Process responses with peers
		if peersInt, ok := msg.R["values"]; ok {
			// New peers discovered - this indicates active torrents
			c.processDiscoveredPeers(peersInt)
		}
	}
}

func (c *MultiThreadDHTCrawler) processInfoHash(infoHash []byte) {
	if len(infoHash) != 20 {
		return
	}

	hashStr := hex.EncodeToString(infoHash)

	// Check for duplicates
	if _, exists := c.hashSet.LoadOrStore(hashStr, true); exists {
		return // Already seen this hash
	}

	// Write to file (thread-safe)
	entry := fmt.Sprintf("%s|%s\n", hashStr, time.Now().Format("2006-01-02 15:04:05"))
	c.outputFile.WriteString(entry)
	c.outputFile.Sync()

	atomic.AddInt64(&c.hashCount, 1)

	log.Printf("🎯 Hash discovered: %s (Total: %d)", hashStr, atomic.LoadInt64(&c.hashCount))
}

func (c *MultiThreadDHTCrawler) processNodes(nodes []byte, fromConn *net.UDPConn) {
	// Each node is 26 bytes: 20 bytes ID + 4 bytes IP + 2 bytes port
	for i := 0; i+26 <= len(nodes); i += 26 {
		ip := net.IP(nodes[i+20 : i+24])
		port := int(uint16(nodes[i+24])<<8 | uint16(nodes[i+25]))

		if port > 0 && port < 65536 && !ip.IsLoopback() {
			addr := &net.UDPAddr{IP: ip, Port: port}

			// Send queries to discovered nodes (spread across different node IDs)
			go func(targetAddr *net.UDPAddr) {
				for j := 0; j < 3; j++ { // Multiple queries per discovered node
					nodeID := c.nodeIDs[j%len(c.nodeIDs)]
					target := make([]byte, 20)
					rand.Read(target)

					c.sendGetPeers(fromConn, targetAddr, target, nodeID)
					atomic.AddInt64(&c.queryCount, 1)

					time.Sleep(100 * time.Millisecond) // Small delay
				}
			}(addr)
		}
	}
}

func (c *MultiThreadDHTCrawler) processDiscoveredPeers(peers interface{}) {
	// This indicates active torrent sharing - we can use this for more targeted queries
	// Implementation could be enhanced to track popular torrents
}

func (c *MultiThreadDHTCrawler) displayStats() {
	ticker := time.NewTicker(time.Duration(c.config.StatsInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.shutdown:
			return
		case <-ticker.C:
			c.printDetailedStats()
		}
	}
}

func (c *MultiThreadDHTCrawler) printDetailedStats() {
	uptime := time.Since(c.startTime)
	hashCount := atomic.LoadInt64(&c.hashCount)
	queryCount := atomic.LoadInt64(&c.queryCount)

	hashRate := float64(hashCount) / uptime.Minutes()
	queryRate := float64(queryCount) / uptime.Minutes()

	fmt.Println("================================================================================")
	fmt.Println("🚀 MULTI-THREADED DHT CRAWLER - TURBO MODE")
	fmt.Println("================================================================================")
	fmt.Printf("⏱️  Uptime: %v\n", uptime.Round(time.Second))
	fmt.Printf("🎯 Hashes Discovered: %d (%.2f/min)\n", hashCount, hashRate)
	fmt.Printf("🔍 Queries Sent: %d (%.2f/min)\n", queryCount, queryRate)
	fmt.Printf("📡 UDP Ports: %d | 🔧 Workers: %d | 🆔 Node IDs: %d\n",
		c.portCount, c.workerCount, len(c.nodeIDs))
	fmt.Printf("💾 Output File: %s\n", c.outputFile.Name())
	fmt.Printf("⚡ Efficiency: %.3f hashes/query\n", float64(hashCount)/float64(queryCount))
	fmt.Println("================================================================================")
}

func (c *MultiThreadDHTCrawler) Close() {
	close(c.shutdown)

	log.Println("🛑 Shutting down turbo crawler...")

	// Close connections
	c.closeConnections()

	// Close message channel
	close(c.messageChan)

	// Wait for workers to finish
	c.wg.Wait()

	if c.outputFile != nil {
		c.outputFile.Close()
	}

	log.Println("✅ Turbo crawler stopped")
}

// ============================================================================
// NORMAL MODE IMPLEMENTATION
// ============================================================================

func NewSimpleDHTCrawler(config *CrawlerConfig) (*SimpleDHTCrawler, error) {
	// Generate random node ID
	nodeID := make([]byte, 20)
	rand.Read(nodeID)

	// Create UDP connection
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 0})
	if err != nil {
		return nil, err
	}

	// Create output file
	filename := fmt.Sprintf("hashes_normal_%s.txt", time.Now().Format("2006-01-02"))
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &SimpleDHTCrawler{
		conn:       conn,
		nodeID:     nodeID,
		startTime:  time.Now(),
		outputFile: file,
		shutdown:   make(chan bool),
		config:     config,
	}, nil
}

func (c *SimpleDHTCrawler) Run() {
	log.Printf("✅ Simple DHT Crawler started on %s", c.conn.LocalAddr())
	log.Printf("💾 Output file: %s", c.outputFile.Name())

	// Start statistics display
	go c.displayStats()

	// Start receiving responses
	go c.receiveLoop()

	// Bootstrap the DHT
	c.bootstrap()

	// Start crawling loop
	c.crawlLoop()
}

func (c *SimpleDHTCrawler) bootstrap() {
	log.Println("📊 Bootstrapping DHT network...")

	for _, node := range bootstrapNodes {
		addr, err := net.ResolveUDPAddr("udp", node)
		if err != nil {
			log.Printf("Failed to resolve %s: %v", node, err)
			continue
		}

		// Send find_node query to bootstrap
		target := make([]byte, 20)
		rand.Read(target)
		c.sendFindNode(addr, target)
		atomic.AddInt64(&c.queryCount, 1)
	}
}

func (c *SimpleDHTCrawler) crawlLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.shutdown:
			return
		case <-ticker.C:
			// Send random queries to discover more nodes and hashes
			for i := 0; i < c.config.QueryRate/10; i++ { // Reduced rate for normal mode
				c.sendRandomQuery()
			}
		}
	}
}

func (c *SimpleDHTCrawler) sendRandomQuery() {
	// Generate random target
	target := make([]byte, 20)
	rand.Read(target)

	// Send to bootstrap nodes
	for _, node := range bootstrapNodes {
		addr, err := net.ResolveUDPAddr("udp", node)
		if err != nil {
			continue
		}

		// Alternate between find_node and get_peers
		if atomic.LoadInt64(&c.queryCount)%2 == 0 {
			c.sendFindNode(addr, target)
		} else {
			c.sendGetPeers(addr, target)
		}

		atomic.AddInt64(&c.queryCount, 1)
		break // Only send to one node at a time in normal mode
	}
}

func (c *SimpleDHTCrawler) sendFindNode(addr *net.UDPAddr, target []byte) {
	transactionID := make([]byte, 2)
	rand.Read(transactionID)

	msg := KRPCMessage{
		T: string(transactionID),
		Y: "q",
		Q: "find_node",
		A: map[string]interface{}{
			"id":     string(c.nodeID),
			"target": string(target),
		},
	}

	c.sendMessage(addr, &msg)
}

func (c *SimpleDHTCrawler) sendGetPeers(addr *net.UDPAddr, infoHash []byte) {
	transactionID := make([]byte, 2)
	rand.Read(transactionID)

	msg := KRPCMessage{
		T: string(transactionID),
		Y: "q",
		Q: "get_peers",
		A: map[string]interface{}{
			"id":        string(c.nodeID),
			"info_hash": string(infoHash),
		},
	}

	c.sendMessage(addr, &msg)
}

func (c *SimpleDHTCrawler) sendMessage(addr *net.UDPAddr, msg *KRPCMessage) {
	data, err := bencode.EncodeBytes(msg)
	if err != nil {
		return
	}

	c.conn.WriteToUDP(data, addr)
}

func (c *SimpleDHTCrawler) receiveLoop() {
	buffer := make([]byte, 1024)

	for {
		select {
		case <-c.shutdown:
			return
		default:
			c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, addr, err := c.conn.ReadFromUDP(buffer)
			if err != nil {
				continue
			}

			c.processMessage(buffer[:n], addr)
		}
	}
}

func (c *SimpleDHTCrawler) processMessage(data []byte, addr *net.UDPAddr) {
	var msg KRPCMessage
	if err := bencode.DecodeBytes(data, &msg); err != nil {
		return
	}

	// Process announce_peer queries (most valuable for hash discovery)
	if msg.Y == "q" && msg.Q == "announce_peer" {
		if infoHashInt, ok := msg.A["info_hash"]; ok {
			if infoHashStr, ok := infoHashInt.(string); ok && len(infoHashStr) == 20 {
				c.processInfoHash([]byte(infoHashStr))
			}
		}
	}

	// Process get_peers queries
	if msg.Y == "q" && msg.Q == "get_peers" {
		if infoHashInt, ok := msg.A["info_hash"]; ok {
			if infoHashStr, ok := infoHashInt.(string); ok && len(infoHashStr) == 20 {
				c.processInfoHash([]byte(infoHashStr))
			}
		}
	}

	// Process responses with nodes
	if msg.Y == "r" && msg.R != nil {
		if nodesInt, ok := msg.R["nodes"]; ok {
			if nodesStr, ok := nodesInt.(string); ok {
				c.processNodes([]byte(nodesStr))
			}
		}
	}
}

func (c *SimpleDHTCrawler) processInfoHash(infoHash []byte) {
	if len(infoHash) != 20 {
		return
	}

	hashStr := hex.EncodeToString(infoHash)

	// Check for duplicates
	if _, exists := c.hashSet.LoadOrStore(hashStr, true); exists {
		return // Already seen this hash
	}

	// Write to file
	entry := fmt.Sprintf("%s|%s\n", hashStr, time.Now().Format("2006-01-02 15:04:05"))
	c.outputFile.WriteString(entry)
	c.outputFile.Sync()

	atomic.AddInt64(&c.hashCount, 1)

	log.Printf("🎯 Hash discovered: %s (Total: %d)", hashStr, atomic.LoadInt64(&c.hashCount))
}

func (c *SimpleDHTCrawler) processNodes(nodes []byte) {
	// Each node is 26 bytes: 20 bytes ID + 4 bytes IP + 2 bytes port
	for i := 0; i+26 <= len(nodes); i += 26 {
		ip := net.IP(nodes[i+20 : i+24])
		port := int(uint16(nodes[i+24])<<8 | uint16(nodes[i+25]))

		if port > 0 && port < 65536 && !ip.IsLoopback() {
			addr := &net.UDPAddr{IP: ip, Port: port}

			// Send single query to discovered node (conservative approach)
			target := make([]byte, 20)
			rand.Read(target)
			c.sendGetPeers(addr, target)
			atomic.AddInt64(&c.queryCount, 1)
		}
	}
}

func (c *SimpleDHTCrawler) displayStats() {
	ticker := time.NewTicker(time.Duration(c.config.StatsInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.shutdown:
			return
		case <-ticker.C:
			c.printStats()
		}
	}
}

func (c *SimpleDHTCrawler) printStats() {
	uptime := time.Since(c.startTime)
	hashCount := atomic.LoadInt64(&c.hashCount)
	queryCount := atomic.LoadInt64(&c.queryCount)

	hashRate := float64(hashCount) / uptime.Minutes()
	queryRate := float64(queryCount) / uptime.Minutes()

	fmt.Println("================================================================================")
	fmt.Println("📊 SIMPLE DHT CRAWLER - NORMAL MODE")
	fmt.Println("================================================================================")
	fmt.Printf("⏱️  Uptime: %v\n", uptime.Round(time.Second))
	fmt.Printf("🎯 Hashes Discovered: %d (%.2f/min)\n", hashCount, hashRate)
	fmt.Printf("🔍 Queries Sent: %d (%.2f/min)\n", queryCount, queryRate)
	fmt.Printf("🌐 Local Address: %s\n", c.conn.LocalAddr())
	fmt.Printf("💾 Output File: %s\n", c.outputFile.Name())
	fmt.Printf("⚡ Efficiency: %.3f hashes/query\n", float64(hashCount)/float64(queryCount))
	fmt.Println("================================================================================")
}

func (c *SimpleDHTCrawler) Close() {
	close(c.shutdown)

	log.Println("🛑 Shutting down simple crawler...")

	if c.outputFile != nil {
		c.outputFile.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}

	log.Println("✅ Simple crawler stopped")
}
