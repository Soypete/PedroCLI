package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (db *DB) InsertPodcastFeed(ctx context.Context, pf *PodcastFeed) error {
	return db.conn.QueryRow(ctx,
		`INSERT INTO podcast_feeds (feed_id, title, description, author, image_url, language)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at, updated_at`,
		pf.FeedID, pf.Title, pf.Description, pf.Author, pf.ImageURL, pf.Language,
	).Scan(&pf.ID, &pf.CreatedAt, &pf.UpdatedAt)
}

func (db *DB) GetPodcastFeedByFeedID(ctx context.Context, feedID uuid.UUID) (PodcastFeed, error) {
	var pf PodcastFeed
	err := db.conn.QueryRow(ctx, `SELECT * FROM podcast_feeds WHERE feed_id = $1`, feedID).
		Scan(&pf.ID, &pf.FeedID, &pf.Title, &pf.Description, &pf.Author, &pf.ImageURL,
			&pf.Language, &pf.CreatedAt, &pf.UpdatedAt)
	return pf, err
}

func (db *DB) ListPodcastFeeds(ctx context.Context) ([]PodcastFeed, error) {
	rows, err := db.conn.Query(ctx, `SELECT * FROM podcast_feeds ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (PodcastFeed, error) {
		var pf PodcastFeed
		err := row.Scan(&pf.ID, &pf.FeedID, &pf.Title, &pf.Description, &pf.Author, &pf.ImageURL,
			&pf.Language, &pf.CreatedAt, &pf.UpdatedAt)
		return pf, err
	})
}
