package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (db *DB) InsertDoc(ctx context.Context, d *Doc) error {
	return db.conn.QueryRow(ctx,
		`INSERT INTO docs (feed_id, guid, source_url, content_hash, title, author, published_at, raw_content, content_type, meta)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, version, is_latest, ingest_status, created_at, updated_at`,
		d.FeedID, d.GUID, d.SourceURL, d.ContentHash, d.Title, d.Author, d.PublishedAt,
		d.RawContent, d.ContentType, d.Meta,
	).Scan(&d.ID, &d.Version, &d.IsLatest, &d.IngestStatus, &d.CreatedAt, &d.UpdatedAt)
}

func (db *DB) GetDoc(ctx context.Context, id uuid.UUID) (Doc, error) {
	var d Doc
	err := db.conn.QueryRow(ctx, `SELECT * FROM docs WHERE id = $1`, id).
		Scan(&d.ID, &d.FeedID, &d.GUID, &d.SourceURL, &d.ContentHash, &d.Title, &d.Author,
			&d.PublishedAt, &d.RawContent, &d.ContentType, &d.Meta, &d.Version, &d.SupersededBy,
			&d.IsLatest, &d.IngestStatus, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func (db *DB) GetDocByContentHash(ctx context.Context, hash string) (Doc, error) {
	var d Doc
	err := db.conn.QueryRow(ctx, `SELECT * FROM docs WHERE content_hash = $1`, hash).
		Scan(&d.ID, &d.FeedID, &d.GUID, &d.SourceURL, &d.ContentHash, &d.Title, &d.Author,
			&d.PublishedAt, &d.RawContent, &d.ContentType, &d.Meta, &d.Version, &d.SupersededBy,
			&d.IsLatest, &d.IngestStatus, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func (db *DB) ListDocsByFeed(ctx context.Context, feedID uuid.UUID, limit, offset int32) ([]Doc, error) {
	rows, err := db.conn.Query(ctx,
		`SELECT * FROM docs WHERE feed_id = $1 AND is_latest = true ORDER BY published_at DESC LIMIT $2 OFFSET $3`,
		feedID, limit, offset)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Doc, error) {
		var d Doc
		err := row.Scan(&d.ID, &d.FeedID, &d.GUID, &d.SourceURL, &d.ContentHash, &d.Title, &d.Author,
			&d.PublishedAt, &d.RawContent, &d.ContentType, &d.Meta, &d.Version, &d.SupersededBy,
			&d.IsLatest, &d.IngestStatus, &d.CreatedAt, &d.UpdatedAt)
		return d, err
	})
}

func (db *DB) UpdateDocIngestStatus(ctx context.Context, id uuid.UUID, status IngestStatus) error {
	_, err := db.conn.Exec(ctx, `UPDATE docs SET ingest_status = $2 WHERE id = $1`, id, status)
	return err
}

func (db *DB) MarkDocSuperseded(ctx context.Context, id uuid.UUID, supersededBy uuid.UUID) error {
	_, err := db.conn.Exec(ctx, `UPDATE docs SET is_latest = false, superseded_by = $2 WHERE id = $1`, id, supersededBy)
	return err
}
