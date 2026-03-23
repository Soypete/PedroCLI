-- name: InsertFeed :one
INSERT INTO feeds (url, title, feed_type, content_type, poll_interval, podcast_enabled, podcast_slug)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetFeed :one
SELECT * FROM feeds WHERE id = $1;

-- name: GetFeedByURL :one
SELECT * FROM feeds WHERE url = $1;

-- name: ListFeeds :many
SELECT * FROM feeds ORDER BY created_at DESC;

-- name: UpdateFeedLastPolled :exec
UPDATE feeds SET last_polled_at = now(), last_error = NULL WHERE id = $1;

-- name: UpdateFeedLastError :exec
UPDATE feeds SET last_error = $2 WHERE id = $1;
