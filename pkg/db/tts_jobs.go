package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (db *DB) InsertTTSJob(ctx context.Context, t *TtsJob) error {
	return db.conn.QueryRow(ctx,
		`INSERT INTO tts_jobs (source_type, artifact_id, doc_id, content_key, model, voice, speed, include_in_podcast)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, status, queued_at, created_at`,
		t.SourceType, t.ArtifactID, t.DocID, t.ContentKey, t.Model, t.Voice, t.Speed, t.IncludeInPodcast,
	).Scan(&t.ID, &t.Status, &t.QueuedAt, &t.CreatedAt)
}

type UpdateTTSJobStatusParams struct {
	ID            uuid.UUID
	Status        TtsStatus
	AudioPath     *string
	AudioURL      *string
	DurationSec   *float64
	FileSizeBytes *int64
}

func (db *DB) UpdateTTSJobStatus(ctx context.Context, p UpdateTTSJobStatusParams) error {
	_, err := db.conn.Exec(ctx,
		`UPDATE tts_jobs
		 SET status = $2,
		     started_at = CASE WHEN $2 = 'processing' THEN now() ELSE started_at END,
		     completed_at = CASE WHEN $2 IN ('done', 'error') THEN now() ELSE completed_at END,
		     audio_path = coalesce($3, audio_path),
		     audio_url = coalesce($4, audio_url),
		     duration_sec = coalesce($5, duration_sec),
		     file_size_bytes = coalesce($6, file_size_bytes)
		 WHERE id = $1`,
		p.ID, p.Status, p.AudioPath, p.AudioURL, p.DurationSec, p.FileSizeBytes)
	return err
}

func (db *DB) ListPendingTTSJobs(ctx context.Context) ([]TtsJob, error) {
	rows, err := db.conn.Query(ctx, `SELECT * FROM tts_jobs WHERE status = 'pending' ORDER BY queued_at ASC`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanTtsJobRow)
}

func (db *DB) ListPodcastEpisodes(ctx context.Context) ([]TtsJob, error) {
	rows, err := db.conn.Query(ctx,
		`SELECT * FROM tts_jobs WHERE status = 'done' AND include_in_podcast = true ORDER BY completed_at DESC`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanTtsJobRow)
}

func scanTtsJobRow(row pgx.CollectableRow) (TtsJob, error) {
	var t TtsJob
	err := row.Scan(&t.ID, &t.SourceType, &t.ArtifactID, &t.DocID, &t.ContentKey, &t.Model, &t.Voice,
		&t.Speed, &t.Status, &t.AudioPath, &t.AudioURL, &t.DurationSec, &t.FileSizeBytes,
		&t.IncludeInPodcast, &t.PodcastGUID, &t.QueuedAt, &t.StartedAt, &t.CompletedAt, &t.CreatedAt)
	return t, err
}
