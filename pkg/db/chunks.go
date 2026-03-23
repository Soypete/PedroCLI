package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// InsertChunksBatch inserts chunks using individual INSERT statements through
// the Conn interface, which works for both pool and transaction contexts.
func (db *DB) InsertChunksBatch(ctx context.Context, chunks []Chunk) (int64, error) {
	var count int64
	for _, c := range chunks {
		_, err := db.conn.Exec(ctx,
			`INSERT INTO chunks (doc_id, chunk_index, chunk_hash, text, token_count, start_time, end_time)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			c.DocID, c.ChunkIndex, c.ChunkHash, c.Text, c.TokenCount, c.StartTime, c.EndTime)
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (db *DB) DeleteChunksByDocID(ctx context.Context, docID uuid.UUID) error {
	_, err := db.conn.Exec(ctx, `DELETE FROM chunks WHERE doc_id = $1`, docID)
	return err
}

func (db *DB) ListChunksByDocID(ctx context.Context, docID uuid.UUID) ([]Chunk, error) {
	rows, err := db.conn.Query(ctx,
		`SELECT id, doc_id, chunk_index, chunk_hash, text, token_count, start_time, end_time, created_at
		 FROM chunks WHERE doc_id = $1 ORDER BY chunk_index ASC`, docID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Chunk, error) {
		var c Chunk
		err := row.Scan(&c.ID, &c.DocID, &c.ChunkIndex, &c.ChunkHash, &c.Text, &c.TokenCount, &c.StartTime, &c.EndTime, &c.CreatedAt)
		return c, err
	})
}
