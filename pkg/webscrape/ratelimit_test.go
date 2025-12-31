package webscrape

import (
	"context"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter()
	if rl == nil {
		t.Fatal("NewRateLimiter() returned nil")
	}
	if rl.limiters == nil {
		t.Error("limiters map is nil")
	}
}

func TestRateLimiterSetLimit(t *testing.T) {
	rl := NewRateLimiter()
	rl.SetLimit("example.com", 5.0)

	// Check that we can get the bucket
	bucket := rl.getBucket("example.com")
	if bucket == nil {
		t.Fatal("getBucket returned nil")
	}
	if bucket.refillRate != 5.0 {
		t.Errorf("refillRate = %v, want 5.0", bucket.refillRate)
	}
}

func TestRateLimiterTryAcquire(t *testing.T) {
	rl := NewRateLimiter()
	rl.SetLimit("test.com", 2.0) // 2 requests per second

	// First request should succeed
	if !rl.TryAcquire("test.com") {
		t.Error("First TryAcquire should succeed")
	}

	// Second request should succeed
	if !rl.TryAcquire("test.com") {
		t.Error("Second TryAcquire should succeed")
	}

	// Third request may fail (depends on timing)
	// Just check that it doesn't panic
	_ = rl.TryAcquire("test.com")
}

func TestRateLimiterWait(t *testing.T) {
	rl := NewRateLimiter()
	rl.SetLimit("wait-test.com", 10.0) // High rate to avoid waiting

	ctx := context.Background()

	// Should not block with high rate
	start := time.Now()
	err := rl.Wait(ctx, "wait-test.com")
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Wait() error = %v", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("Wait took too long: %v", elapsed)
	}
}

func TestRateLimiterWaitCancellation(t *testing.T) {
	rl := NewRateLimiter()
	rl.SetLimit("cancel-test.com", 0.1) // Very low rate

	// Exhaust the bucket
	_ = rl.TryAcquire("cancel-test.com")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx, "cancel-test.com")
	if err == nil {
		t.Error("Wait() should return error on cancellation")
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://example.com/path", "example.com"},
		{"http://api.example.com:8080/api", "api.example.com"},
		{"https://GITHUB.COM/user/repo", "github.com"},
		{"invalid-url", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := ExtractDomain(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractDomain(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestRateLimiterWaitForURL(t *testing.T) {
	rl := NewRateLimiter()

	ctx := context.Background()

	// Test with valid URL
	err := rl.WaitForURL(ctx, "https://example.com/test")
	if err != nil {
		t.Errorf("WaitForURL() error = %v", err)
	}

	// Test with invalid URL (should not error)
	err = rl.WaitForURL(ctx, "not-a-url")
	if err != nil {
		t.Errorf("WaitForURL() with invalid URL should not error, got %v", err)
	}
}

func TestDefaultRateLimits(t *testing.T) {
	// Ensure default limits are set
	if DefaultRateLimits["github.com"] == 0 {
		t.Error("github.com should have a default rate limit")
	}
	if DefaultRateLimits["default"] == 0 {
		t.Error("default rate limit should be set")
	}
}
