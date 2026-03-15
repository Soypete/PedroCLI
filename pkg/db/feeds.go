package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (db *DB) InsertFeed(ctx context.Context, f *Feed) error {
	return db.conn.QueryRow(ctx,
		`INSERT INTO feeds (url, title, feed_type, content_type, poll_interval, podcast_enabled, podcast_slug)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at, updated_at`,
		f.URL, f.Title, f.FeedType, f.ContentType, f.PollInterval, f.PodcastEnabled, f.PodcastSlug,
	).Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt)
}

func (db *DB) GetFeed(ctx context.Context, id uuid.UUID) (Feed, error) {
	var f Feed
	err := db.conn.QueryRow(ctx, `SELECT * FROM feeds WHERE id = $1`, id).
		Scan(&f.ID, &f.URL, &f.Title, &f.FeedType, &f.ContentType, &f.PollInterval,
			&f.LastPolledAt, &f.LastError, &f.PodcastEnabled, &f.PodcastSlug, &f.CreatedAt, &f.UpdatedAt)
	return f, err
}

func (db *DB) GetFeedByURL(ctx context.Context, url string) (Feed, error) {
	var f Feed
	err := db.conn.QueryRow(ctx, `SELECT * FROM feeds WHERE url = $1`, url).
		Scan(&f.ID, &f.URL, &f.Title, &f.FeedType, &f.ContentType, &f.PollInterval,
			&f.LastPolledAt, &f.LastError, &f.PodcastEnabled, &f.PodcastSlug, &f.CreatedAt, &f.UpdatedAt)
	return f, err
}

func (db *DB) ListFeeds(ctx context.Context) ([]Feed, error) {
	rows, err := db.conn.Query(ctx, `SELECT * FROM feeds ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Feed, error) {
		var f Feed
		err := row.Scan(&f.ID, &f.URL, &f.Title, &f.FeedType, &f.ContentType, &f.PollInterval,
			&f.LastPolledAt, &f.LastError, &f.PodcastEnabled, &f.PodcastSlug, &f.CreatedAt, &f.UpdatedAt)
		return f, err
	})
}

func (db *DB) UpdateFeedLastPolled(ctx context.Context, id uuid.UUID) error {
	_, err := db.conn.Exec(ctx, `UPDATE feeds SET last_polled_at = now(), last_error = NULL WHERE id = $1`, id)
	return err
}

func (db *DB) UpdateFeedLastError(ctx context.Context, id uuid.UUID, lastError string) error {
	_, err := db.conn.Exec(ctx, `UPDATE feeds SET last_error = $2 WHERE id = $1`, id, lastError)
	return err
}
