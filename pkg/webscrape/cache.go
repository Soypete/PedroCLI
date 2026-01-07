package webscrape

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CacheStore defines the interface for cache storage
type CacheStore interface {
	Get(urlHash string) (*CacheEntry, error)
	Set(entry *CacheEntry) error
	Delete(urlHash string) error
	Cleanup() error // Remove expired entries
	Close() error
}

// Cache provides caching for fetch results
type Cache struct {
	store      CacheStore
	defaultTTL time.Duration
	enabled    bool
}

// CacheConfig configures the cache behavior
type CacheConfig struct {
	Enabled    bool          `json:"enabled"`
	Type       string        `json:"type"` // "memory" only
	Path       string        `json:"path,omitempty"`
	DefaultTTL time.Duration `json:"default_ttl"`
	MaxSizeMB  int64         `json:"max_size_mb"`
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Enabled:    true,
		Type:       "memory",
		DefaultTTL: 1 * time.Hour,
		MaxSizeMB:  100,
	}
}

// NewCache creates a new cache with the given configuration
func NewCache(cfg *CacheConfig) (*Cache, error) {
	if !cfg.Enabled {
		return &Cache{enabled: false}, nil
	}

	var store CacheStore

	// Only memory cache is supported
	if cfg.Type == "" || cfg.Type == "memory" {
		store = NewMemoryCache(cfg.MaxSizeMB * 1024 * 1024)
	} else {
		// Return error for unsupported cache types
		return nil, fmt.Errorf("unsupported cache type: %s (only 'memory' is supported)", cfg.Type)
	}

	return &Cache{
		store:      store,
		defaultTTL: cfg.DefaultTTL,
		enabled:    true,
	}, nil
}

// Get retrieves a cached result for the given URL
func (c *Cache) Get(url string) (*FetchResult, bool) {
	if !c.enabled || c.store == nil {
		return nil, false
	}

	urlHash := hashURL(url)
	entry, err := c.store.Get(urlHash)
	if err != nil || entry == nil {
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		_ = c.store.Delete(urlHash)
		return nil, false
	}

	// Reconstruct FetchResult
	result := &FetchResult{
		URL:         entry.URL,
		ContentType: entry.ContentType,
		RawHTML:     entry.RawContent,
		CleanText:   entry.CleanText,
		FetchedAt:   entry.FetchedAt,
	}

	// Decode code blocks
	if len(entry.CodeBlocks) > 0 {
		_ = json.Unmarshal(entry.CodeBlocks, &result.CodeBlocks)
	}

	return result, true
}

// Set stores a fetch result in the cache
func (c *Cache) Set(url string, result *FetchResult) error {
	return c.SetWithTTL(url, result, c.defaultTTL)
}

// SetWithTTL stores a fetch result with a specific TTL
func (c *Cache) SetWithTTL(url string, result *FetchResult, ttl time.Duration) error {
	if !c.enabled || c.store == nil {
		return nil
	}

	// Encode code blocks
	codeBlocksJSON, _ := json.Marshal(result.CodeBlocks)

	entry := &CacheEntry{
		ID:          uuid.New().String(),
		URLHash:     hashURL(url),
		URL:         url,
		ContentType: result.ContentType,
		RawContent:  result.RawHTML,
		CleanText:   result.CleanText,
		CodeBlocks:  codeBlocksJSON,
		FetchedAt:   result.FetchedAt,
		ExpiresAt:   time.Now().Add(ttl),
		CreatedAt:   time.Now(),
	}

	return c.store.Set(entry)
}

// Delete removes a cached entry
func (c *Cache) Delete(url string) error {
	if !c.enabled || c.store == nil {
		return nil
	}
	return c.store.Delete(hashURL(url))
}

// Cleanup removes expired entries
func (c *Cache) Cleanup() error {
	if !c.enabled || c.store == nil {
		return nil
	}
	return c.store.Cleanup()
}

// Close closes the cache
func (c *Cache) Close() error {
	if c.store != nil {
		return c.store.Close()
	}
	return nil
}

// hashURL creates a hash of the URL for use as cache key
func hashURL(url string) string {
	h := sha256.New()
	h.Write([]byte(url))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// MemoryCache implements an in-memory cache
type MemoryCache struct {
	data        map[string]*CacheEntry
	maxSize     int64
	currentSize int64
	mu          sync.RWMutex
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(maxSize int64) *MemoryCache {
	return &MemoryCache{
		data:    make(map[string]*CacheEntry),
		maxSize: maxSize,
	}
}

func (m *MemoryCache) Get(urlHash string) (*CacheEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.data[urlHash]
	if !exists {
		return nil, nil
	}
	return entry, nil
}

func (m *MemoryCache) Set(entry *CacheEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Estimate size
	entrySize := int64(len(entry.RawContent) + len(entry.CleanText) + len(entry.CodeBlocks))

	// Evict if necessary
	for m.currentSize+entrySize > m.maxSize && len(m.data) > 0 {
		// Simple eviction: remove oldest entry
		var oldestKey string
		var oldestTime time.Time
		for key, e := range m.data {
			if oldestKey == "" || e.CreatedAt.Before(oldestTime) {
				oldestKey = key
				oldestTime = e.CreatedAt
			}
		}
		if oldestKey != "" {
			oldEntry := m.data[oldestKey]
			m.currentSize -= int64(len(oldEntry.RawContent) + len(oldEntry.CleanText) + len(oldEntry.CodeBlocks))
			delete(m.data, oldestKey)
		}
	}

	m.data[entry.URLHash] = entry
	m.currentSize += entrySize
	return nil
}

func (m *MemoryCache) Delete(urlHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry, exists := m.data[urlHash]; exists {
		m.currentSize -= int64(len(entry.RawContent) + len(entry.CleanText) + len(entry.CodeBlocks))
		delete(m.data, urlHash)
	}
	return nil
}

func (m *MemoryCache) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, entry := range m.data {
		if now.After(entry.ExpiresAt) {
			m.currentSize -= int64(len(entry.RawContent) + len(entry.CleanText) + len(entry.CodeBlocks))
			delete(m.data, key)
		}
	}
	return nil
}

func (m *MemoryCache) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = nil
	return nil
}
