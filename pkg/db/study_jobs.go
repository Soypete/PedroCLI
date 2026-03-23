package db

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type InsertJobParams struct {
	JobType    StudyJobType
	Priority   int32
	FeedID     *uuid.UUID
	DocID      *uuid.UUID
	ArtifactID *uuid.UUID
	TtsJobID   *uuid.UUID
	Payload    json.RawMessage
	RunAfter   *time.Time
}

func (db *DB) InsertJob(ctx context.Context, p InsertJobParams) (StudyJob, error) {
	var j StudyJob
	err := db.conn.QueryRow(ctx,
		`INSERT INTO study_jobs (job_type, priority, feed_id, doc_id, artifact_id, tts_job_id, payload, run_after)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, coalesce($8, now()))
		 RETURNING *`,
		p.JobType, p.Priority, p.FeedID, p.DocID, p.ArtifactID, p.TtsJobID, p.Payload, p.RunAfter,
	).Scan(&j.ID, &j.JobType, &j.Status, &j.Priority, &j.FeedID, &j.DocID, &j.ArtifactID,
		&j.TtsJobID, &j.Payload, &j.Attempts, &j.MaxAttempts, &j.LastError, &j.RunAfter,
		&j.CreatedAt, &j.UpdatedAt)
	return j, err
}

func (db *DB) ClaimJob(ctx context.Context) (StudyJob, error) {
	var j StudyJob
	err := db.conn.QueryRow(ctx,
		`UPDATE study_jobs
		 SET status = 'processing', attempts = attempts + 1, updated_at = now()
		 WHERE id = (
		     SELECT id FROM study_jobs
		     WHERE status = 'pending' AND run_after <= now()
		     ORDER BY priority ASC, created_at ASC
		     LIMIT 1
		     FOR UPDATE SKIP LOCKED
		 ) RETURNING *`,
	).Scan(&j.ID, &j.JobType, &j.Status, &j.Priority, &j.FeedID, &j.DocID, &j.ArtifactID,
		&j.TtsJobID, &j.Payload, &j.Attempts, &j.MaxAttempts, &j.LastError, &j.RunAfter,
		&j.CreatedAt, &j.UpdatedAt)
	return j, err
}

func (db *DB) UpdateJobStatus(ctx context.Context, id uuid.UUID, status StudyJobStatus) error {
	_, err := db.conn.Exec(ctx, `UPDATE study_jobs SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	return err
}

func (db *DB) MarkJobDone(ctx context.Context, id uuid.UUID) error {
	_, err := db.conn.Exec(ctx, `UPDATE study_jobs SET status = 'done', updated_at = now() WHERE id = $1`, id)
	return err
}

func (db *DB) MarkJobError(ctx context.Context, id uuid.UUID, lastError string) error {
	_, err := db.conn.Exec(ctx,
		`UPDATE study_jobs
		 SET status = CASE WHEN attempts >= max_attempts THEN 'error'::study_job_status ELSE 'pending'::study_job_status END,
		     last_error = $2,
		     run_after = now() + (interval '1 minute' * power(2, attempts)),
		     updated_at = now()
		 WHERE id = $1`, id, lastError)
	return err
}
