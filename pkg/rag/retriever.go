package rag

import (
	"context"

	"github.com/google/uuid"
	"github.com/soypete/pedrocli/pkg/db"
)

// SearchScope constrains which docs/feeds to search within.
type SearchScope struct {
	FeedID *uuid.UUID
	DocID  *uuid.UUID
}

// Retriever performs full-text search over chunked documents.
type Retriever struct {
	db *db.DB
}

// NewRetriever creates a Retriever.
func NewRetriever(database *db.DB) *Retriever {
	return &Retriever{db: database}
}

// Search finds the top-10 most relevant chunks matching the query,
// scoped by feed or doc if specified.
func (r *Retriever) Search(ctx context.Context, query string, scope SearchScope) ([]db.ChunkResult, error) {
	if scope.DocID != nil {
		return r.db.SearchChunksByDoc(ctx, query, *scope.DocID)
	}
	if scope.FeedID != nil {
		return r.db.SearchChunksByFeed(ctx, query, *scope.FeedID)
	}
	return r.db.SearchChunks(ctx, query)
}
