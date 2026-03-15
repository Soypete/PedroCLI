-- name: InsertJob :one
INSERT INTO study_jobs (job_type, priority, feed_id, doc_id, artifact_id, tts_job_id, payload, run_after)
VALUES ($1, $2, $3, $4, $5, $6, $7, coalesce($8, now()))
RETURNING *;

-- name: ClaimJob :one
UPDATE study_jobs
SET status = 'processing', attempts = attempts + 1, updated_at = now()
WHERE id = (
    SELECT id FROM study_jobs
    WHERE status = 'pending' AND run_after <= now()
    ORDER BY priority ASC, created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: UpdateJobStatus :exec
UPDATE study_jobs SET status = $2, updated_at = now() WHERE id = $1;

-- name: MarkJobDone :exec
UPDATE study_jobs SET status = 'done', updated_at = now() WHERE id = $1;

-- name: MarkJobError :exec
UPDATE study_jobs
SET status = CASE WHEN attempts >= max_attempts THEN 'error'::study_job_status ELSE 'pending'::study_job_status END,
    last_error = $2,
    run_after = now() + (interval '1 minute' * power(2, attempts)),
    updated_at = now()
WHERE id = $1;
