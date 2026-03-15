-- name: InsertTTSJob :one
INSERT INTO tts_jobs (source_type, artifact_id, doc_id, content_key, model, voice, speed, include_in_podcast)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateTTSJobStatus :exec
UPDATE tts_jobs
SET status = $2, started_at = CASE WHEN $2 = 'processing' THEN now() ELSE started_at END,
    completed_at = CASE WHEN $2 IN ('done', 'error') THEN now() ELSE completed_at END,
    audio_path = coalesce($3, audio_path),
    audio_url = coalesce($4, audio_url),
    duration_sec = coalesce($5, duration_sec),
    file_size_bytes = coalesce($6, file_size_bytes)
WHERE id = $1;

-- name: ListPendingTTSJobs :many
SELECT * FROM tts_jobs WHERE status = 'pending' ORDER BY queued_at ASC;

-- name: ListPodcastEpisodes :many
SELECT * FROM tts_jobs
WHERE status = 'done' AND include_in_podcast = true
ORDER BY completed_at DESC;
