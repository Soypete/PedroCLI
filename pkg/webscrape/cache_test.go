package webscrape

import (
	"testing"
	"time"
)

func TestMemoryCache(t *testing.T) {
	cache := NewMemoryCache(1024 * 1024) // 1MB

	t.Run("Set and Get", func(t *testing.T) {
		entry := &CacheEntry{
			ID:        "test-1",
			URLHash:   "hash123",
			URL:       "https://example.com",
			CleanText: "test content",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
		}

		err := cache.Set(entry)
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		got, err := cache.Get("hash123")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got == nil {
			t.Fatal("Get() returned nil")
		}
		if got.URL != entry.URL {
			t.Errorf("URL = %q, want %q", got.URL, entry.URL)
		}
	})

	t.Run("Get non-existent", func(t *testing.T) {
		got, err := cache.Get("nonexistent")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got != nil {
			t.Error("Get() should return nil for non-existent key")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		entry := &CacheEntry{
			ID:        "test-2",
			URLHash:   "hash456",
			URL:       "https://example.com/2",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
		}

		_ = cache.Set(entry)
		err := cache.Delete("hash456")
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		got, _ := cache.Get("hash456")
		if got != nil {
			t.Error("Entry should be deleted")
		}
	})

	t.Run("Cleanup removes expired", func(t *testing.T) {
		entry := &CacheEntry{
			ID:        "test-3",
			URLHash:   "hash789",
			URL:       "https://example.com/3",
			ExpiresAt: time.Now().Add(-1 * time.Hour), // Already expired
			CreatedAt: time.Now(),
		}

		_ = cache.Set(entry)
		err := cache.Cleanup()
		if err != nil {
			t.Fatalf("Cleanup() error = %v", err)
		}

		got, _ := cache.Get("hash789")
		if got != nil {
			t.Error("Expired entry should be cleaned up")
		}
	})
}

func TestCacheIntegration(t *testing.T) {
	cfg := &CacheConfig{
		Enabled:    true,
		Type:       "memory",
		DefaultTTL: 1 * time.Hour,
		MaxSizeMB:  10,
	}

	cache, err := NewCache(cfg)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}
	defer cache.Close()

	t.Run("Set and Get FetchResult", func(t *testing.T) {
		result := &FetchResult{
			URL:         "https://example.com/test",
			StatusCode:  200,
			ContentType: "text/html",
			CleanText:   "Hello World",
			CodeBlocks: []CodeBlock{
				{Language: "go", Code: "func main() {}"},
			},
			FetchedAt: time.Now(),
		}

		err := cache.Set("https://example.com/test", result)
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		got, found := cache.Get("https://example.com/test")
		if !found {
			t.Fatal("Get() should find cached result")
		}
		if got.URL != result.URL {
			t.Errorf("URL = %q, want %q", got.URL, result.URL)
		}
		if got.CleanText != result.CleanText {
			t.Errorf("CleanText = %q, want %q", got.CleanText, result.CleanText)
		}
		if len(got.CodeBlocks) != 1 {
			t.Errorf("CodeBlocks count = %d, want 1", len(got.CodeBlocks))
		}
	})

	t.Run("Get non-existent returns false", func(t *testing.T) {
		_, found := cache.Get("https://nonexistent.com")
		if found {
			t.Error("Get() should return false for non-existent URL")
		}
	})
}

func TestCacheDisabled(t *testing.T) {
	cfg := &CacheConfig{
		Enabled: false,
	}

	cache, err := NewCache(cfg)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	// Operations should not error on disabled cache
	err = cache.Set("url", &FetchResult{})
	if err != nil {
		t.Errorf("Set() on disabled cache should not error: %v", err)
	}

	_, found := cache.Get("url")
	if found {
		t.Error("Get() on disabled cache should return false")
	}

	err = cache.Close()
	if err != nil {
		t.Errorf("Close() on disabled cache should not error: %v", err)
	}
}

func TestHashURL(t *testing.T) {
	// Same URL should produce same hash
	hash1 := hashURL("https://example.com/test")
	hash2 := hashURL("https://example.com/test")
	if hash1 != hash2 {
		t.Error("Same URL should produce same hash")
	}

	// Different URLs should produce different hashes
	hash3 := hashURL("https://example.com/other")
	if hash1 == hash3 {
		t.Error("Different URLs should produce different hashes")
	}

	// Hash should be consistent length
	if len(hash1) != 16 {
		t.Errorf("Hash length = %d, want 16", len(hash1))
	}
}
