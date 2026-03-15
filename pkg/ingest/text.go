// Package ingest provides text extraction and audio transcription clients.
package ingest

import (
	"context"
	"fmt"
	"time"

	readability "github.com/go-shiori/go-readability"
)

// TextExtractor fetches a URL and extracts clean article text.
type TextExtractor struct {
	timeout time.Duration
}

// NewTextExtractor creates a TextExtractor with a 30-second timeout.
func NewTextExtractor() *TextExtractor {
	return &TextExtractor{
		timeout: 30 * time.Second,
	}
}

// Extract fetches the given URL and returns clean plain-text content
// plus the article title.
func (te *TextExtractor) Extract(_ context.Context, url string) (title, text string, err error) {
	article, err := readability.FromURL(url, te.timeout)
	if err != nil {
		return "", "", fmt.Errorf("readability: %w", err)
	}
	return article.Title, article.TextContent, nil
}
