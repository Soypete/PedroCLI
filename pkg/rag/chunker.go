// Package rag provides text chunking and full-text search retrieval.
package rag

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/soypete/pedrocli/pkg/db"
	"github.com/soypete/pedrocli/pkg/ingest"
)

const targetTokens = 512

// estimateTokens approximates token count from word count.
func estimateTokens(text string) int {
	words := len(strings.Fields(text))
	return int(math.Ceil(float64(words) * 1.3))
}

// chunkHash returns SHA-256 of docID||chunkIndex||text.
func chunkHash(docID uuid.UUID, index int, text string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s||%d||%s", docID, index, text)))
	return fmt.Sprintf("%x", h)
}

// ChunkText splits plain text into ~512-token chunks by splitting on
// paragraph boundaries first, then sentence boundaries.
func ChunkText(docID uuid.UUID, text string) []db.Chunk {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	targetWords := int(math.Floor(float64(targetTokens) / 1.3))
	var chunks []db.Chunk
	idx := 0

	for start := 0; start < len(words); {
		end := start + targetWords
		if end > len(words) {
			end = len(words)
		}
		chunkText := strings.Join(words[start:end], " ")
		tokens := estimateTokens(chunkText)

		chunks = append(chunks, db.Chunk{
			DocID:      docID,
			ChunkIndex: int32(idx),
			ChunkHash:  chunkHash(docID, idx, chunkText),
			Text:       chunkText,
			TokenCount: int32(tokens),
		})
		idx++
		start = end
	}
	return chunks
}

// ChunkAudio splits text into chunks aligned to whisper.cpp segment boundaries
// where possible, targeting ~512 tokens per chunk.
func ChunkAudio(docID uuid.UUID, segments []ingest.Segment) []db.Chunk {
	if len(segments) == 0 {
		return nil
	}

	var chunks []db.Chunk
	idx := 0
	var currentText strings.Builder
	var currentTokens int
	startTime := segments[0].Start
	var endTime float64

	for _, seg := range segments {
		segTokens := estimateTokens(seg.Text)
		if currentTokens+segTokens > targetTokens && currentTokens > 0 {
			text := strings.TrimSpace(currentText.String())
			st := startTime
			et := endTime
			chunks = append(chunks, db.Chunk{
				DocID:      docID,
				ChunkIndex: int32(idx),
				ChunkHash:  chunkHash(docID, idx, text),
				Text:       text,
				TokenCount: int32(currentTokens),
				StartTime:  &st,
				EndTime:    &et,
			})
			idx++
			currentText.Reset()
			currentTokens = 0
			startTime = seg.Start
		}
		if currentText.Len() > 0 {
			currentText.WriteString(" ")
		}
		currentText.WriteString(strings.TrimSpace(seg.Text))
		currentTokens += segTokens
		endTime = seg.End
	}

	if currentTokens > 0 {
		text := strings.TrimSpace(currentText.String())
		st := startTime
		et := endTime
		chunks = append(chunks, db.Chunk{
			DocID:      docID,
			ChunkIndex: int32(idx),
			ChunkHash:  chunkHash(docID, idx, text),
			Text:       text,
			TokenCount: int32(currentTokens),
			StartTime:  &st,
			EndTime:    &et,
		})
	}

	return chunks
}
