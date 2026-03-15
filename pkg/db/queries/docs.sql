-- name: InsertDoc :one
INSERT INTO docs (feed_id, guid, source_url, content_hash, title, author, published_at, raw_content, content_type, meta)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetDoc :one
SELECT * FROM docs WHERE id = $1;

-- name: GetDocByContentHash :one
SELECT * FROM docs WHERE content_hash = $1;

-- name: ListDocsByFeed :many
SELECT * FROM docs
WHERE feed_id = $1 AND is_latest = true
ORDER BY published_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateDocIngestStatus :exec
UPDATE docs SET ingest_status = $2 WHERE id = $1;

-- name: MarkDocSuperseded :exec
UPDATE docs
SET is_latest = false, superseded_by = $2
WHERE id = $1;
