-- name: SearchChunks :many
SELECT c.id, c.doc_id, c.chunk_index, c.text, c.start_time, c.end_time,
       ts_rank_cd(c.tsv, query) AS rank,
       d.title AS doc_title, d.source_url, d.published_at
FROM chunks c
JOIN docs d ON d.id = c.doc_id,
websearch_to_tsquery('english', @query::text) query
WHERE d.is_latest = true
  AND c.tsv @@ query
ORDER BY rank DESC
LIMIT 10;

-- name: SearchChunksByFeed :many
SELECT c.id, c.doc_id, c.chunk_index, c.text, c.start_time, c.end_time,
       ts_rank_cd(c.tsv, query) AS rank,
       d.title AS doc_title, d.source_url, d.published_at
FROM chunks c
JOIN docs d ON d.id = c.doc_id,
websearch_to_tsquery('english', @query::text) query
WHERE d.feed_id = @feed_id
  AND d.is_latest = true
  AND c.tsv @@ query
ORDER BY rank DESC
LIMIT 10;

-- name: SearchChunksByDoc :many
SELECT c.id, c.doc_id, c.chunk_index, c.text, c.start_time, c.end_time,
       ts_rank_cd(c.tsv, query) AS rank,
       d.title AS doc_title, d.source_url, d.published_at
FROM chunks c
JOIN docs d ON d.id = c.doc_id,
websearch_to_tsquery('english', @query::text) query
WHERE c.doc_id = @doc_id
  AND d.is_latest = true
  AND c.tsv @@ query
ORDER BY rank DESC
LIMIT 10;
