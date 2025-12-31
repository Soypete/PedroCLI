package webscrape

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
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
	Type       string        `json:"type"` // "sqlite", "memory"
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
	var err error

	switch cfg.Type {
	case "sqlite":
		store, err = NewSQLiteCache(cfg.Path)
	case "memory":
		store = NewMemoryCache(cfg.MaxSizeMB * 1024 * 1024)
	default:
		store = NewMemoryCache(cfg.MaxSizeMB * 1024 * 1024)
	}

	if err != nil {
		return nil, err
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

// SQLiteCache implements a SQLite-based cache
type SQLiteCache struct {
	db *sql.DB
}

// NewSQLiteCache creates a new SQLite cache
func NewSQLiteCache(path string) (*SQLiteCache, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// Create table if not exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS webscrape_cache (
			id TEXT PRIMARY KEY,
			url_hash TEXT UNIQUE,
			url TEXT,
			content_type TEXT,
			raw_content TEXT,
			clean_text TEXT,
			code_blocks TEXT,
			metadata TEXT,
			fetched_at DATETIME,
			expires_at DATETIME,
			created_at DATETIME
		);
		CREATE INDEX IF NOT EXISTS idx_cache_url_hash ON webscrape_cache(url_hash);
		CREATE INDEX IF NOT EXISTS idx_cache_expires ON webscrape_cache(expires_at);
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	return &SQLiteCache{db: db}, nil
}

func (s *SQLiteCache) Get(urlHash string) (*CacheEntry, error) {
	row := s.db.QueryRow(`
		SELECT id, url_hash, url, content_type, raw_content, clean_text,
		       code_blocks, metadata, fetched_at, expires_at, created_at
		FROM webscrape_cache
		WHERE url_hash = ?
	`, urlHash)

	var entry CacheEntry
	var codeBlocks, metadata sql.NullString
	var fetchedAt, expiresAt, createdAt string

	err := row.Scan(
		&entry.ID, &entry.URLHash, &entry.URL, &entry.ContentType,
		&entry.RawContent, &entry.CleanText, &codeBlocks, &metadata,
		&fetchedAt, &expiresAt, &createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	entry.FetchedAt, _ = time.Parse(time.RFC3339, fetchedAt)
	entry.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	entry.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

	if codeBlocks.Valid {
		entry.CodeBlocks = []byte(codeBlocks.String)
	}
	if metadata.Valid {
		entry.Metadata = []byte(metadata.String)
	}

	return &entry, nil
}

func (s *SQLiteCache) Set(entry *CacheEntry) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO webscrape_cache
		(id, url_hash, url, content_type, raw_content, clean_text,
		 code_blocks, metadata, fetched_at, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		entry.ID, entry.URLHash, entry.URL, entry.ContentType,
		entry.RawContent, entry.CleanText,
		string(entry.CodeBlocks), string(entry.Metadata),
		entry.FetchedAt.Format(time.RFC3339),
		entry.ExpiresAt.Format(time.RFC3339),
		entry.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *SQLiteCache) Delete(urlHash string) error {
	_, err := s.db.Exec("DELETE FROM webscrape_cache WHERE url_hash = ?", urlHash)
	return err
}

func (s *SQLiteCache) Cleanup() error {
	_, err := s.db.Exec("DELETE FROM webscrape_cache WHERE expires_at < ?", time.Now().Format(time.RFC3339))
	return err
}

func (s *SQLiteCache) Close() error {
	return s.db.Close()
}
