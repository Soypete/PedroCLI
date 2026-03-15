package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (db *DB) SearchChunks(ctx context.Context, query string) ([]ChunkResult, error) {
	rows, err := db.conn.Query(ctx,
		`SELECT c.id, c.doc_id, c.chunk_index, c.text, c.start_time, c.end_time,
		        ts_rank_cd(c.tsv, q) AS rank,
		        d.title AS doc_title, d.source_url, d.published_at
		 FROM chunks c
		 JOIN docs d ON d.id = c.doc_id,
		 websearch_to_tsquery('english', $1) q
		 WHERE d.is_latest = true AND c.tsv @@ q
		 ORDER BY rank DESC LIMIT 10`, query)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanChunkResult)
}

func (db *DB) SearchChunksByFeed(ctx context.Context, query string, feedID uuid.UUID) ([]ChunkResult, error) {
	rows, err := db.conn.Query(ctx,
		`SELECT c.id, c.doc_id, c.chunk_index, c.text, c.start_time, c.end_time,
		        ts_rank_cd(c.tsv, q) AS rank,
		        d.title AS doc_title, d.source_url, d.published_at
		 FROM chunks c
		 JOIN docs d ON d.id = c.doc_id,
		 websearch_to_tsquery('english', $1) q
		 WHERE d.feed_id = $2 AND d.is_latest = true AND c.tsv @@ q
		 ORDER BY rank DESC LIMIT 10`, query, feedID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanChunkResult)
}

func (db *DB) SearchChunksByDoc(ctx context.Context, query string, docID uuid.UUID) ([]ChunkResult, error) {
	rows, err := db.conn.Query(ctx,
		`SELECT c.id, c.doc_id, c.chunk_index, c.text, c.start_time, c.end_time,
		        ts_rank_cd(c.tsv, q) AS rank,
		        d.title AS doc_title, d.source_url, d.published_at
		 FROM chunks c
		 JOIN docs d ON d.id = c.doc_id,
		 websearch_to_tsquery('english', $1) q
		 WHERE c.doc_id = $2 AND d.is_latest = true AND c.tsv @@ q
		 ORDER BY rank DESC LIMIT 10`, query, docID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanChunkResult)
}

func scanChunkResult(row pgx.CollectableRow) (ChunkResult, error) {
	var cr ChunkResult
	err := row.Scan(&cr.ID, &cr.DocID, &cr.ChunkIndex, &cr.Text, &cr.StartTime, &cr.EndTime,
		&cr.Rank, &cr.DocTitle, &cr.SourceURL, &cr.PublishedAt)
	return cr, err
}
