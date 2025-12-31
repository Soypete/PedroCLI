package webscrape

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"
)

// RateLimiter manages rate limiting per domain
type RateLimiter struct {
	limiters map[string]*tokenBucket
	defaults float64
	mu       sync.RWMutex
}

// tokenBucket implements a simple token bucket rate limiter
type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter with default limits
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*tokenBucket),
		defaults: DefaultRateLimits["default"],
	}

	// Initialize with default limits for known domains
	for domain, rate := range DefaultRateLimits {
		if domain != "default" {
			rl.SetLimit(domain, rate)
		}
	}

	return rl
}

// SetLimit sets the rate limit for a specific domain
func (r *RateLimiter) SetLimit(domain string, reqPerSecond float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.limiters[domain] = &tokenBucket{
		tokens:     reqPerSecond, // Start with full bucket
		maxTokens:  reqPerSecond,
		refillRate: reqPerSecond,
		lastRefill: time.Now(),
	}
}

// Wait blocks until a request can be made to the given domain
func (r *RateLimiter) Wait(ctx context.Context, domain string) error {
	bucket := r.getBucket(domain)

	for {
		bucket.mu.Lock()

		// Refill tokens based on elapsed time
		now := time.Now()
		elapsed := now.Sub(bucket.lastRefill).Seconds()
		bucket.tokens += elapsed * bucket.refillRate
		if bucket.tokens > bucket.maxTokens {
			bucket.tokens = bucket.maxTokens
		}
		bucket.lastRefill = now

		// Check if we can proceed
		if bucket.tokens >= 1 {
			bucket.tokens--
			bucket.mu.Unlock()
			return nil
		}

		// Calculate wait time
		waitTime := time.Duration((1 - bucket.tokens) / bucket.refillRate * float64(time.Second))
		bucket.mu.Unlock()

		// Wait for either context cancellation or wait time
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue to try again
		}
	}
}

// TryAcquire attempts to acquire a request slot without blocking
func (r *RateLimiter) TryAcquire(domain string) bool {
	bucket := r.getBucket(domain)

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens += elapsed * bucket.refillRate
	if bucket.tokens > bucket.maxTokens {
		bucket.tokens = bucket.maxTokens
	}
	bucket.lastRefill = now

	// Check if we can proceed
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}

	return false
}

// getBucket returns the rate limiter bucket for a domain
func (r *RateLimiter) getBucket(domain string) *tokenBucket {
	r.mu.RLock()
	bucket, exists := r.limiters[domain]
	r.mu.RUnlock()

	if exists {
		return bucket
	}

	// Create a new bucket with default rate
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if bucket, exists = r.limiters[domain]; exists {
		return bucket
	}

	bucket = &tokenBucket{
		tokens:     r.defaults,
		maxTokens:  r.defaults,
		refillRate: r.defaults,
		lastRefill: time.Now(),
	}
	r.limiters[domain] = bucket

	return bucket
}

// ExtractDomain extracts the domain from a URL string
func ExtractDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	host := parsed.Host
	// Remove port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	return strings.ToLower(host)
}

// WaitForURL is a convenience method that extracts the domain and waits
func (r *RateLimiter) WaitForURL(ctx context.Context, rawURL string) error {
	domain := ExtractDomain(rawURL)
	if domain == "" {
		return nil // Don't rate limit if we can't parse the URL
	}
	return r.Wait(ctx, domain)
}
