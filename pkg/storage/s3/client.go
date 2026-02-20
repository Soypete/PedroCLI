package s3

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client provides S3-compatible object storage operations.
// Uses raw HTTP requests to avoid heavy SDK dependencies.
type Client struct {
	endpoint  string
	bucket    string
	accessKey string
	secretKey string
	useSSL    bool
	client    *http.Client
}

// Config holds S3 client configuration.
type Config struct {
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

// NewClient creates a new S3-compatible storage client.
func NewClient(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("s3 endpoint is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}

	return &Client{
		endpoint:  strings.TrimRight(cfg.Endpoint, "/"),
		bucket:    cfg.Bucket,
		accessKey: cfg.AccessKey,
		secretKey: cfg.SecretKey,
		useSSL:    cfg.UseSSL,
		client: &http.Client{
			Timeout: 5 * time.Minute, // Large files may take time
		},
	}, nil
}

// Upload stores data in S3 at the given key. Returns the object URL.
func (c *Client) Upload(ctx context.Context, key string, reader io.Reader, contentType string) (string, error) {
	objectURL := c.objectURL(key)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, objectURL, reader)
	if err != nil {
		return "", fmt.Errorf("failed to create upload request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	c.signRequest(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return objectURL, nil
}

// Download retrieves an object from S3. Caller must close the returned ReadCloser.
func (c *Client) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	objectURL := c.objectURL(key)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, objectURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	c.signRequest(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// Delete removes an object from S3.
func (c *Client) Delete(ctx context.Context, key string) error {
	objectURL := c.objectURL(key)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, objectURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	c.signRequest(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete failed with status %d", resp.StatusCode)
	}

	return nil
}

// Exists checks if an object exists in S3.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	objectURL := c.objectURL(key)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, objectURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create head request: %w", err)
	}

	c.signRequest(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("head request failed: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// objectURL builds the full URL for an object key.
func (c *Client) objectURL(key string) string {
	scheme := "http"
	if c.useSSL {
		scheme = "https"
	}
	// Path-style URL: http://endpoint/bucket/key
	return fmt.Sprintf("%s://%s/%s/%s", scheme, c.endpointHost(), c.bucket, url.PathEscape(key))
}

// endpointHost strips the scheme from the endpoint if present.
func (c *Client) endpointHost() string {
	host := c.endpoint
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	return host
}

// signRequest adds authentication headers for S3-compatible APIs.
// Uses simple access key auth suitable for MinIO/Longhorn.
func (c *Client) signRequest(req *http.Request) {
	if c.accessKey != "" && c.secretKey != "" {
		// For MinIO/Longhorn, basic auth or access key auth works
		req.SetBasicAuth(c.accessKey, c.secretKey)
	}
}
