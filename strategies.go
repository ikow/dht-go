package main

import (
	"context"
	"crypto/rand"
	"log"
	mathrand "math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// initStrategies initializes all discovery strategies
func (c *EnhancedDHTCrawler) initStrategies() {
	c.strategies["bootstrap"] = &BootstrapStrategy{}
	c.strategies["aggressive"] = &AggressiveStrategy{}
	c.strategies["random"] = &RandomStrategy{}
	c.strategies["adaptive"] = &AdaptiveStrategy{}
	c.strategies["burst"] = &BurstStrategy{}
}

// BootstrapStrategy implements systematic bootstrap node querying
type BootstrapStrategy struct {
	stopChan chan struct{}
}

func (s *BootstrapStrategy) Name() string {
	return "bootstrap"
}

func (s *BootstrapStrategy) Start(ctx context.Context, crawler *EnhancedDHTCrawler) error {
	s.stopChan = make(chan struct{})
	
	go func() {
		// Initial bootstrap
		s.bootstrap(crawler)
		
		// Periodic re-bootstrap every 2 minutes
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopChan:
				return
			case <-ticker.C:
				s.bootstrap(crawler)
			}
		}
	}()
	
	return nil
}

func (s *BootstrapStrategy) Stop() error {
	if s.stopChan != nil {
		close(s.stopChan)
	}
	return nil
}

func (s *BootstrapStrategy) bootstrap(crawler *EnhancedDHTCrawler) {
	log.Printf("🌐 Bootstrap strategy: Querying %d bootstrap nodes", len(enhancedBootstrapNodes))
	
	for connID, conn := range crawler.connPool {
		nodeID := crawler.nodeIDs[connID]
		
		for _, nodeAddr := range enhancedBootstrapNodes {
			addr, err := net.ResolveUDPAddr("udp", nodeAddr)
			if err != nil {
				continue
			}
			
			// Rate limiting
			select {
			case <-crawler.rateLimiter:
			case <-crawler.ctx.Done():
				return
			}
			
			// Generate random target
			target := make([]byte, 20)
			rand.Read(target)
			
			// Send both find_node and get_peers
			crawler.sendFindNode(conn, addr, target, nodeID)
			atomic.AddInt64(&crawler.queryCount, 1)
			
			// Small delay between queries
			time.Sleep(10 * time.Millisecond)
			
			select {
			case <-crawler.rateLimiter:
			case <-crawler.ctx.Done():
				return
			}
			
			crawler.sendGetPeers(conn, addr, target, nodeID)
			atomic.AddInt64(&crawler.queryCount, 1)
		}
	}
}

// AggressiveStrategy implements high-frequency random querying
type AggressiveStrategy struct {
	stopChan chan struct{}
}

func (s *AggressiveStrategy) Name() string {
	return "aggressive"
}

func (s *AggressiveStrategy) Start(ctx context.Context, crawler *EnhancedDHTCrawler) error {
	s.stopChan = make(chan struct{})
	
	// Start multiple aggressive workers
	numWorkers := len(crawler.connPool)
	for i := 0; i < numWorkers; i++ {
		go s.aggressiveWorker(ctx, crawler, i)
	}
	
	return nil
}

func (s *AggressiveStrategy) Stop() error {
	if s.stopChan != nil {
		close(s.stopChan)
	}
	return nil
}

func (s *AggressiveStrategy) aggressiveWorker(ctx context.Context, crawler *EnhancedDHTCrawler, connID int) {
	conn := crawler.connPool[connID]
	nodeID := crawler.nodeIDs[connID]
	
	ticker := time.NewTicker(time.Second / 10) // 10 queries per second per worker
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.sendRandomQueries(crawler, conn, nodeID, 5) // 5 queries per tick
		}
	}
}

func (s *AggressiveStrategy) sendRandomQueries(crawler *EnhancedDHTCrawler, conn *net.UDPConn, nodeID []byte, count int) {
	for i := 0; i < count; i++ {
		// Rate limiting
		select {
		case <-crawler.rateLimiter:
		case <-crawler.ctx.Done():
			return
		}
		
		// Random target
		target := make([]byte, 20)
		rand.Read(target)
		
		// Random bootstrap node
		nodeAddr := enhancedBootstrapNodes[i%len(enhancedBootstrapNodes)]
		addr, err := net.ResolveUDPAddr("udp", nodeAddr)
		if err != nil {
			continue
		}
		
		// Prefer get_peers for hash discovery
		if i%3 == 0 {
			crawler.sendFindNode(conn, addr, target, nodeID)
		} else {
			crawler.sendGetPeers(conn, addr, target, nodeID)
		}
		
		atomic.AddInt64(&crawler.queryCount, 1)
	}
}

// RandomStrategy implements intelligent random querying with discovered nodes
type RandomStrategy struct {
	stopChan chan struct{}
}

func (s *RandomStrategy) Name() string {
	return "random"
}

func (s *RandomStrategy) Start(ctx context.Context, crawler *EnhancedDHTCrawler) error {
	s.stopChan = make(chan struct{})
	
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopChan:
				return
			case <-ticker.C:
				s.randomQueries(crawler)
			}
		}
	}()
	
	return nil
}

func (s *RandomStrategy) Stop() error {
	if s.stopChan != nil {
		close(s.stopChan)
	}
	return nil
}

func (s *RandomStrategy) randomQueries(crawler *EnhancedDHTCrawler) {
	// Collect some discovered nodes
	var discoveredNodes []*net.UDPAddr
	
	crawler.discoveredNodes.Range(func(key, value interface{}) bool {
		nodeKey := key.(string)
		lastSeen := value.(time.Time)
		
		// Only use recently seen nodes
		if time.Since(lastSeen) < 30*time.Minute {
			if addr, err := net.ResolveUDPAddr("udp", nodeKey); err == nil {
				discoveredNodes = append(discoveredNodes, addr)
			}
		}
		
		return len(discoveredNodes) < 100 // Limit collection
	})
	
	log.Printf("🎲 Random strategy: Querying %d discovered nodes", len(discoveredNodes))
	
	// Query discovered nodes
	for i, addr := range discoveredNodes {
		if i >= 50 { // Limit queries per round
			break
		}
		
		connID := i % len(crawler.connPool)
		conn := crawler.connPool[connID]
		nodeID := crawler.nodeIDs[connID]
		
		// Rate limiting
		select {
		case <-crawler.rateLimiter:
		case <-crawler.ctx.Done():
			return
		}
		
		// Random target
		target := make([]byte, 20)
		rand.Read(target)
		
		// Send get_peers query
		crawler.sendGetPeers(conn, addr, target, nodeID)
		atomic.AddInt64(&crawler.queryCount, 1)
		
		// Small delay
		time.Sleep(50 * time.Millisecond)
	}
}

// AdaptiveStrategy adjusts behavior based on success rate and network conditions
type AdaptiveStrategy struct {
	stopChan         chan struct{}
	lastHashCount    int64
	lastQueryCount   int64
	lastCheckTime    time.Time
	currentEfficiency float64
}

func (s *AdaptiveStrategy) Name() string {
	return "adaptive"
}

func (s *AdaptiveStrategy) Start(ctx context.Context, crawler *EnhancedDHTCrawler) error {
	s.stopChan = make(chan struct{})
	s.lastCheckTime = time.Now()
	
	go func() {
		ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopChan:
				return
			case <-ticker.C:
				s.adaptBehavior(crawler)
			}
		}
	}()
	
	return nil
}

func (s *AdaptiveStrategy) Stop() error {
	if s.stopChan != nil {
		close(s.stopChan)
	}
	return nil
}

func (s *AdaptiveStrategy) adaptBehavior(crawler *EnhancedDHTCrawler) {
	currentHashCount := atomic.LoadInt64(&crawler.hashCount)
	currentQueryCount := atomic.LoadInt64(&crawler.queryCount)
	currentTime := time.Now()
	
	// Calculate recent efficiency
	hashDelta := currentHashCount - s.lastHashCount
	queryDelta := currentQueryCount - s.lastQueryCount
	timeDelta := currentTime.Sub(s.lastCheckTime)
	
	if queryDelta > 0 {
		s.currentEfficiency = float64(hashDelta) / float64(queryDelta)
	}
	
	log.Printf("🧠 Adaptive strategy: Recent efficiency %.4f hashes/query (%.1f hashes/min)", 
		s.currentEfficiency, float64(hashDelta)/timeDelta.Minutes())
	
	// Adapt strategy based on efficiency
	if s.currentEfficiency > 0.01 { // High efficiency - increase activity
		s.highEfficiencyMode(crawler)
	} else if s.currentEfficiency > 0.001 { // Medium efficiency - balanced approach
		s.balancedMode(crawler)
	} else { // Low efficiency - focus on discovery
		s.discoveryMode(crawler)
	}
	
	// Update tracking variables
	s.lastHashCount = currentHashCount
	s.lastQueryCount = currentQueryCount
	s.lastCheckTime = currentTime
}

func (s *AdaptiveStrategy) highEfficiencyMode(crawler *EnhancedDHTCrawler) {
	log.Println("🚀 Adaptive: High efficiency detected - increasing query rate")
	
	// Focus on recently successful nodes
	var successfulNodes []*net.UDPAddr
	crawler.discoveredNodes.Range(func(key, value interface{}) bool {
		nodeKey := key.(string)
		lastSeen := value.(time.Time)
		
		// Very recent nodes
		if time.Since(lastSeen) < 5*time.Minute {
			if addr, err := net.ResolveUDPAddr("udp", nodeKey); err == nil {
				successfulNodes = append(successfulNodes, addr)
			}
		}
		return len(successfulNodes) < 50
	})
	
	// Send more queries to successful nodes
	for i, addr := range successfulNodes {
		connID := i % len(crawler.connPool)
		s.queryNode(crawler, connID, addr, 3) // 3 queries per node
	}
}

func (s *AdaptiveStrategy) balancedMode(crawler *EnhancedDHTCrawler) {
	log.Println("⚖️ Adaptive: Balanced mode - maintaining current strategy")
	
	// Mix of bootstrap and discovered nodes
	for i := 0; i < 10; i++ {
		connID := i % len(crawler.connPool)
		
		if i%2 == 0 {
			// Query bootstrap nodes
			nodeAddr := enhancedBootstrapNodes[i%len(enhancedBootstrapNodes)]
			if addr, err := net.ResolveUDPAddr("udp", nodeAddr); err == nil {
				s.queryNode(crawler, connID, addr, 1)
			}
		} else {
			// Query discovered nodes
			var found bool
			crawler.discoveredNodes.Range(func(key, value interface{}) bool {
				nodeKey := key.(string)
				if addr, err := net.ResolveUDPAddr("udp", nodeKey); err == nil {
					s.queryNode(crawler, connID, addr, 1)
					found = true
					return false // Stop after first found
				}
				return true
			})
			
			if !found {
				// Fallback to bootstrap
				nodeAddr := enhancedBootstrapNodes[i%len(enhancedBootstrapNodes)]
				if addr, err := net.ResolveUDPAddr("udp", nodeAddr); err == nil {
					s.queryNode(crawler, connID, addr, 1)
				}
			}
		}
	}
}

func (s *AdaptiveStrategy) discoveryMode(crawler *EnhancedDHTCrawler) {
	log.Println("🔍 Adaptive: Low efficiency - focusing on node discovery")
	
	// Focus on find_node queries to discover new nodes
	for i, nodeAddr := range enhancedBootstrapNodes {
		connID := i % len(crawler.connPool)
		conn := crawler.connPool[connID]
		nodeID := crawler.nodeIDs[connID]
		
		if addr, err := net.ResolveUDPAddr("udp", nodeAddr); err == nil {
			// Rate limiting
			select {
			case <-crawler.rateLimiter:
			case <-crawler.ctx.Done():
				return
			}
			
			// Multiple find_node queries with different targets
			for j := 0; j < 3; j++ {
				target := make([]byte, 20)
				rand.Read(target)
				
				crawler.sendFindNode(conn, addr, target, nodeID)
				atomic.AddInt64(&crawler.queryCount, 1)
				
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

func (s *AdaptiveStrategy) queryNode(crawler *EnhancedDHTCrawler, connID int, addr *net.UDPAddr, count int) {
	conn := crawler.connPool[connID]
	nodeID := crawler.nodeIDs[connID]
	
	for i := 0; i < count; i++ {
		// Rate limiting
		select {
		case <-crawler.rateLimiter:
		case <-crawler.ctx.Done():
			return
		}
		
		target := make([]byte, 20)
		rand.Read(target)
		
		crawler.sendGetPeers(conn, addr, target, nodeID)
		atomic.AddInt64(&crawler.queryCount, 1)
		
		if i < count-1 {
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// BurstStrategy implements periodic high-intensity bursts
type BurstStrategy struct {
	stopChan chan struct{}
}

func (s *BurstStrategy) Name() string {
	return "burst"
}

func (s *BurstStrategy) Start(ctx context.Context, crawler *EnhancedDHTCrawler) error {
	if !crawler.config.BurstMode {
		return nil // Burst mode disabled
	}
	
	s.stopChan = make(chan struct{})
	
	go func() {
		// Wait 2 minutes before first burst
		timer := time.NewTimer(2 * time.Minute)
		defer timer.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopChan:
				return
			case <-timer.C:
				s.executeBurst(crawler)
				// Next burst in 10-15 minutes (randomized)
				nextBurst := time.Duration(10+mathrand.Intn(5)) * time.Minute
				timer.Reset(nextBurst)
			}
		}
	}()
	
	return nil
}

func (s *BurstStrategy) Stop() error {
	if s.stopChan != nil {
		close(s.stopChan)
	}
	return nil
}

func (s *BurstStrategy) executeBurst(crawler *EnhancedDHTCrawler) {
	log.Println("💥 Burst strategy: Executing high-intensity burst")
	
	// Save current rate limit
	originalRate := crawler.config.RateLimitPerSec
	
	// Temporarily increase rate limit for burst
	burstRate := originalRate * 3
	
	// Create temporary rate limiter
	burstLimiter := make(chan struct{}, burstRate)
	
	// Fill burst limiter
	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(burstRate))
		defer ticker.Stop()
		
		for i := 0; i < 30; i++ { // 30 second burst
			select {
			case <-crawler.ctx.Done():
				return
			case <-ticker.C:
				select {
				case burstLimiter <- struct{}{}:
				default:
				}
			}
		}
		close(burstLimiter)
	}()
	
	// Execute burst queries
	var wg sync.WaitGroup
	
	for connID, conn := range crawler.connPool {
		wg.Add(1)
		go func(id int, connection *net.UDPConn) {
			defer wg.Done()
			s.burstWorker(crawler, id, connection, burstLimiter)
		}(connID, conn)
	}
	
	wg.Wait()
	log.Println("💥 Burst strategy: Burst completed")
}

func (s *BurstStrategy) burstWorker(crawler *EnhancedDHTCrawler, connID int, conn *net.UDPConn, limiter chan struct{}) {
	nodeID := crawler.nodeIDs[connID]
	
	for {
		// Use burst rate limiter
		select {
		case <-limiter:
		case <-crawler.ctx.Done():
			return
		}
		
		// If limiter is closed, stop
		if limiter == nil {
			return
		}
		
		// Random target
		target := make([]byte, 20)
		rand.Read(target)
		
		// Choose random method: bootstrap node vs discovered node
		if mathrand.Float32() < 0.3 { // 30% chance for bootstrap
			nodeAddr := enhancedBootstrapNodes[mathrand.Intn(len(enhancedBootstrapNodes))]
			if addr, err := net.ResolveUDPAddr("udp", nodeAddr); err == nil {
				crawler.sendGetPeers(conn, addr, target, nodeID)
				atomic.AddInt64(&crawler.queryCount, 1)
			}
		} else { // 70% chance for discovered nodes
			var targetAddr *net.UDPAddr
			
			crawler.discoveredNodes.Range(func(key, value interface{}) bool {
				nodeKey := key.(string)
				if addr, err := net.ResolveUDPAddr("udp", nodeKey); err == nil {
					targetAddr = addr
					return false // Stop at first valid node
				}
				return true
			})
			
			if targetAddr != nil {
				crawler.sendGetPeers(conn, targetAddr, target, nodeID)
				atomic.AddInt64(&crawler.queryCount, 1)
			}
		}
	}
}