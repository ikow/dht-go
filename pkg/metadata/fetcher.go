package metadata

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/storage"
)

// TorrentMetadata represents extracted torrent metadata
type TorrentMetadata struct {
	InfoHash     string     `json:"info_hash"`
	Name         string     `json:"name"`
	Files        []FileInfo `json:"files"`
	TotalSize    int64      `json:"total_size"`
	PieceCount   int        `json:"piece_count"`
	PieceLength  int64      `json:"piece_length"`
	Comment      string     `json:"comment"`
	CreatedBy    string     `json:"created_by"`
	CreationDate time.Time  `json:"creation_date"`
	Announce     []string   `json:"announce"`
	AnnounceList [][]string `json:"announce_list"`
	Encoding     string     `json:"encoding"`
	Private      bool       `json:"private"`
	Source       string     `json:"source"`
	Tags         []string   `json:"tags"`
	Categories   []string   `json:"categories"`
	FetchedAt    time.Time  `json:"fetched_at"`
	Success      bool       `json:"success"`
	Error        string     `json:"error,omitempty"`
}

// FileInfo represents individual file information
type FileInfo struct {
	Path   string `json:"path"`
	Length int64  `json:"length"`
}

// FetcherConfig holds configuration for metadata fetcher
type FetcherConfig struct {
	MaxConcurrentFetches int           // Maximum concurrent metadata fetches
	FetchTimeout         time.Duration // Timeout for each fetch operation
	MaxFileSize          int64         // Maximum file size to fetch metadata for
	EnableDHT            bool          // Enable DHT for metadata fetching
	EnableTrackers       bool          // Enable tracker announcements
	DataDir              string        // Directory for temporary data storage
	RateLimitPerSec      int           // Rate limit for metadata fetches
	RetryAttempts        int           // Number of retry attempts for failed fetches
	RetryDelay           time.Duration // Delay between retry attempts
}

// Fetcher handles metadata retrieval from torrent hashes
type Fetcher struct {
	config        *FetcherConfig
	client        *torrent.Client
	pendingHashes chan string
	metadataQueue chan *TorrentMetadata
	activeCount   int64
	successCount  int64
	failureCount  int64
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	rateLimiter   chan struct{}

	// Event handlers
	OnMetadataFetched func(*TorrentMetadata)
	OnFetchFailed     func(string, error)
}

// DefaultFetcherConfig returns a default configuration
func DefaultFetcherConfig() *FetcherConfig {
	return &FetcherConfig{
		MaxConcurrentFetches: 10,
		FetchTimeout:         30 * time.Second,
		MaxFileSize:          100 * 1024 * 1024, // 100MB
		EnableDHT:            true,
		EnableTrackers:       true,
		DataDir:              "./metadata_temp",
		RateLimitPerSec:      5, // Conservative rate limit
		RetryAttempts:        3,
		RetryDelay:           5 * time.Second,
	}
}

// NewFetcher creates a new metadata fetcher
func NewFetcher(config *FetcherConfig) (*Fetcher, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Configure torrent client
	clientConfig := torrent.NewDefaultClientConfig()
	clientConfig.DataDir = config.DataDir
	clientConfig.NoUpload = true // We only want to download metadata
	clientConfig.DisableWebseeds = true
	clientConfig.DisableTrackers = !config.EnableTrackers
	clientConfig.NoDHT = !config.EnableDHT
	clientConfig.DefaultStorage = storage.NewMMap(config.DataDir)

	// Create torrent client
	client, err := torrent.NewClient(clientConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create torrent client: %v", err)
	}

	fetcher := &Fetcher{
		config:        config,
		client:        client,
		pendingHashes: make(chan string, 1000),
		metadataQueue: make(chan *TorrentMetadata, 100),
		ctx:           ctx,
		cancel:        cancel,
		rateLimiter:   make(chan struct{}, config.RateLimitPerSec),
	}

	return fetcher, nil
}

// Start begins the metadata fetching process
func (f *Fetcher) Start() error {
	log.Println("🚀 Starting Metadata Fetcher")

	// Start rate limiter
	f.wg.Add(1)
	go f.rateLimiterLoop()

	// Start worker pool
	for i := 0; i < f.config.MaxConcurrentFetches; i++ {
		f.wg.Add(1)
		go f.fetchWorker(i)
	}

	// Start metadata processor
	f.wg.Add(1)
	go f.metadataProcessor()

	return nil
}

// Stop gracefully shuts down the fetcher
func (f *Fetcher) Stop() error {
	log.Println("🛑 Shutting down Metadata Fetcher")

	f.cancel()
	f.wg.Wait()

	if f.client != nil {
		f.client.Close()
	}

	log.Printf("📊 Metadata fetch stats: %d success, %d failed",
		atomic.LoadInt64(&f.successCount),
		atomic.LoadInt64(&f.failureCount))

	return nil
}

// AddHash queues a hash for metadata fetching
func (f *Fetcher) AddHash(hash string) {
	select {
	case f.pendingHashes <- hash:
	default:
		// Queue full, skip this hash
	}
}

// AddHashes queues multiple hashes for metadata fetching
func (f *Fetcher) AddHashes(hashes []string) {
	for _, hash := range hashes {
		f.AddHash(hash)
	}
}

// rateLimiterLoop manages rate limiting for metadata fetches
func (f *Fetcher) rateLimiterLoop() {
	defer f.wg.Done()

	ticker := time.NewTicker(time.Second / time.Duration(f.config.RateLimitPerSec))
	defer ticker.Stop()

	for {
		select {
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			select {
			case f.rateLimiter <- struct{}{}:
			default:
			}
		}
	}
}

// fetchWorker processes metadata fetch requests
func (f *Fetcher) fetchWorker(workerID int) {
	defer f.wg.Done()

	for {
		select {
		case <-f.ctx.Done():
			return
		case hash := <-f.pendingHashes:
			// Rate limiting
			select {
			case <-f.rateLimiter:
			case <-f.ctx.Done():
				return
			}

			atomic.AddInt64(&f.activeCount, 1)
			metadata := f.fetchMetadata(hash, workerID)
			atomic.AddInt64(&f.activeCount, -1)

			if metadata.Success {
				atomic.AddInt64(&f.successCount, 1)
			} else {
				atomic.AddInt64(&f.failureCount, 1)
			}

			// Queue metadata for processing
			select {
			case f.metadataQueue <- metadata:
			default:
				// Queue full, skip
			}
		}
	}
}

// fetchMetadata attempts to fetch metadata for a given hash
func (f *Fetcher) fetchMetadata(hash string, workerID int) *TorrentMetadata {
	metadata := &TorrentMetadata{
		InfoHash:  hash,
		FetchedAt: time.Now(),
		Success:   false,
	}

	// Validate hash format
	hashBytes, err := hex.DecodeString(hash)
	if err != nil || len(hashBytes) != 20 {
		metadata.Error = "invalid hash format"
		return metadata
	}

	// Create magnet link
	magnetURI := fmt.Sprintf("magnet:?xt=urn:btih:%s", hash)

	// Add torrent to client
	t, err := f.client.AddMagnet(magnetURI)
	if err != nil {
		metadata.Error = fmt.Sprintf("failed to add magnet: %v", err)
		return metadata
	}

	// Ensure torrent is removed when done
	defer func() {
		t.Drop()
	}()

	// Wait for metadata with timeout
	metadataCtx, metadataCancel := context.WithTimeout(f.ctx, f.config.FetchTimeout)
	defer metadataCancel()

	select {
	case <-t.GotInfo():
		// Successfully got metadata
		info := t.Info()
		if info == nil {
			metadata.Error = "got info signal but info is nil"
			return metadata
		}

		return f.extractMetadata(t, metadata)

	case <-metadataCtx.Done():
		metadata.Error = "timeout waiting for metadata"
		return metadata
	}
}

// extractMetadata extracts detailed metadata from torrent
func (f *Fetcher) extractMetadata(t *torrent.Torrent, metadata *TorrentMetadata) *TorrentMetadata {
	info := t.Info()
	metaInfo := t.Metainfo()

	// Basic information
	metadata.Name = info.Name
	metadata.TotalSize = info.TotalLength()
	metadata.PieceCount = info.NumPieces()
	metadata.PieceLength = info.PieceLength

	// File information
	metadata.Files = make([]FileInfo, 0, len(info.Files))
	for _, file := range info.Files {
		metadata.Files = append(metadata.Files, FileInfo{
			Path:   file.DisplayPath(info),
			Length: file.Length,
		})
	}

	// Metadata from metainfo
	if metaInfo.Comment != "" {
		metadata.Comment = metaInfo.Comment
	}
	if metaInfo.CreatedBy != "" {
		metadata.CreatedBy = metaInfo.CreatedBy
	}
	if metaInfo.CreationDate > 0 {
		metadata.CreationDate = time.Unix(metaInfo.CreationDate, 0)
	}
	if metaInfo.Encoding != "" {
		metadata.Encoding = metaInfo.Encoding
	}

	// Announce information
	if metaInfo.Announce != "" {
		metadata.Announce = []string{metaInfo.Announce}
	}
	if len(metaInfo.AnnounceList) > 0 {
		metadata.AnnounceList = metaInfo.AnnounceList
		// Flatten announce list to single announce array
		for _, tier := range metaInfo.AnnounceList {
			metadata.Announce = append(metadata.Announce, tier...)
		}
	}

	// Check if private torrent
	if info.Private != nil {
		metadata.Private = *info.Private
	}

	// Extract additional fields if available
	// Note: Extra fields might not be available in all torrent library versions
	// This is a placeholder for potential future enhancements

	// Try to extract tags/categories from comment or name
	metadata.Tags = f.extractTags(metadata.Name, metadata.Comment)
	metadata.Categories = f.categorizeContent(metadata.Name, metadata.Files)

	metadata.Success = true
	return metadata
}

// extractTags attempts to extract tags from name and comment
func (f *Fetcher) extractTags(name, comment string) []string {
	tags := make([]string, 0)

	// Common patterns for tags
	tagPatterns := map[string]string{
		"1080p":  "quality",
		"720p":   "quality",
		"480p":   "quality",
		"4K":     "quality",
		"BluRay": "source",
		"WEB-DL": "source",
		"HDTV":   "source",
		"DVD":    "source",
		"x264":   "codec",
		"x265":   "codec",
		"H.264":  "codec",
		"H.265":  "codec",
		"HEVC":   "codec",
		"AAC":    "audio",
		"MP3":    "audio",
		"FLAC":   "audio",
		"DTS":    "audio",
	}

	text := name + " " + comment
	for pattern, category := range tagPatterns {
		if contains(text, pattern) {
			tags = append(tags, category+":"+pattern)
		}
	}

	return tags
}

// categorizeContent attempts to categorize based on file extensions and names
func (f *Fetcher) categorizeContent(name string, files []FileInfo) []string {
	categories := make([]string, 0)

	// Count file types
	videoCount := 0
	audioCount := 0
	imageCount := 0
	documentCount := 0

	for _, file := range files {
		ext := getFileExtension(file.Path)
		switch ext {
		case ".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v":
			videoCount++
		case ".mp3", ".flac", ".wav", ".aac", ".ogg", ".m4a", ".wma":
			audioCount++
		case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp":
			imageCount++
		case ".pdf", ".doc", ".docx", ".txt", ".epub", ".mobi":
			documentCount++
		}
	}

	// Determine primary category
	if videoCount > 0 {
		categories = append(categories, "video")
		if contains(name, "movie") || contains(name, "film") {
			categories = append(categories, "movie")
		} else if contains(name, "S0") || contains(name, "E0") || contains(name, "season") {
			categories = append(categories, "tv")
		}
	}
	if audioCount > 0 {
		categories = append(categories, "audio")
		if contains(name, "album") || audioCount > 5 {
			categories = append(categories, "music")
		}
	}
	if imageCount > 0 {
		categories = append(categories, "image")
	}
	if documentCount > 0 {
		categories = append(categories, "document")
	}

	// Software/Games
	if contains(name, "game") || contains(name, ".exe") || contains(name, "setup") {
		categories = append(categories, "software")
	}

	return categories
}

// metadataProcessor handles processed metadata
func (f *Fetcher) metadataProcessor() {
	defer f.wg.Done()

	for {
		select {
		case <-f.ctx.Done():
			return
		case metadata := <-f.metadataQueue:
			// Call event handlers
			if metadata.Success && f.OnMetadataFetched != nil {
				f.OnMetadataFetched(metadata)
			} else if !metadata.Success && f.OnFetchFailed != nil {
				f.OnFetchFailed(metadata.InfoHash, fmt.Errorf(metadata.Error))
			}
		}
	}
}

// GetStats returns current fetcher statistics
func (f *Fetcher) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"active_fetches": atomic.LoadInt64(&f.activeCount),
		"success_count":  atomic.LoadInt64(&f.successCount),
		"failure_count":  atomic.LoadInt64(&f.failureCount),
		"pending_queue":  len(f.pendingHashes),
		"metadata_queue": len(f.metadataQueue),
	}
}

// Helper functions
func contains(text, substr string) bool {
	return len(text) >= len(substr) &&
		(text == substr ||
			text[:len(substr)] == substr ||
			text[len(text)-len(substr):] == substr ||
			findInString(text, substr))
}

func findInString(text, substr string) bool {
	for i := 0; i <= len(text)-len(substr); i++ {
		if text[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func getFileExtension(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[i:]
		}
		if filename[i] == '/' || filename[i] == '\\' {
			break
		}
	}
	return ""
}
