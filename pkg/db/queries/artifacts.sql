-- name: UpsertArtifact :one
INSERT INTO artifacts (doc_id, feed_id, artifact_type, prompt_version, content, model, input_tokens, output_tokens, generated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
ON CONFLICT (doc_id, artifact_type, prompt_version)
DO UPDATE SET
    content = EXCLUDED.content,
    model = EXCLUDED.model,
    input_tokens = EXCLUDED.input_tokens,
    output_tokens = EXCLUDED.output_tokens,
    generated_at = now()
RETURNING *;

-- name: GetArtifactByDocAndType :one
SELECT * FROM artifacts
WHERE doc_id = $1 AND artifact_type = $2
ORDER BY generated_at DESC LIMIT 1;

-- name: ListArtifactsByDocID :many
SELECT * FROM artifacts WHERE doc_id = $1 ORDER BY artifact_type, generated_at DESC;
