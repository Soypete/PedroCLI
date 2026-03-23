-- name: InsertChunksBatch :copyfrom
INSERT INTO chunks (doc_id, chunk_index, chunk_hash, text, token_count, start_time, end_time)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: DeleteChunksByDocID :exec
DELETE FROM chunks WHERE doc_id = $1;

-- name: ListChunksByDocID :many
SELECT * FROM chunks WHERE doc_id = $1 ORDER BY chunk_index ASC;
