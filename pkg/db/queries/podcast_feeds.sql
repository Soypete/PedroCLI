-- name: InsertPodcastFeed :one
INSERT INTO podcast_feeds (feed_id, title, description, author, image_url, language)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetPodcastFeedByFeedID :one
SELECT * FROM podcast_feeds WHERE feed_id = $1;

-- name: ListPodcastFeeds :many
SELECT * FROM podcast_feeds ORDER BY created_at DESC;
