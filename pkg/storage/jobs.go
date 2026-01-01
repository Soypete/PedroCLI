// Package storage provides persistent storage for jobs and artifacts.
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// JobStatus represents the status of a job.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// JobType represents the type of job.
type JobType string

const (
	JobTypeBuilder           JobType = "builder"
	JobTypeDebugger          JobType = "debugger"
	JobTypeReviewer          JobType = "reviewer"
	JobTypeTriager           JobType = "triager"
	JobTypeImageGeneration   JobType = "image_generation"
	JobTypeImageAnalysis     JobType = "image_analysis"
	JobTypeAltTextGeneration JobType = "alt_text_generation"
	JobTypeBlogWorkflow      JobType = "blog_workflow"
)

// Job represents a job in the database.
type Job struct {
	ID             string          `json:"id"`
	JobType        JobType         `json:"job_type"`
	Status         JobStatus       `json:"status"`
	InputPayload   json.RawMessage `json:"input_payload,omitempty"`
	OutputPayload  json.RawMessage `json:"output_payload,omitempty"`
	ModelUsed      string          `json:"model_used,omitempty"`
	HardwareTarget string          `json:"hardware_target,omitempty"`
	ErrorMessage   string          `json:"error_message,omitempty"`
	StartedAt      *time.Time      `json:"started_at,omitempty"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// JobInput represents common input fields for jobs.
type JobInput struct {
	Description     string                 `json:"description,omitempty"`
	Prompt          string                 `json:"prompt,omitempty"`
	ReferenceImages []string               `json:"reference_images,omitempty"`
	StylePreset     string                 `json:"style_preset,omitempty"`
	BlogPostID      string                 `json:"blog_post_id,omitempty"`
	Width           int                    `json:"width,omitempty"`
	Height          int                    `json:"height,omitempty"`
	Model           string                 `json:"model,omitempty"`
	ExtraParams     map[string]interface{} `json:"extra_params,omitempty"`
}

// JobOutput represents common output fields for jobs.
type JobOutput struct {
	GeneratedImages []string               `json:"generated_images,omitempty"`
	AltText         string                 `json:"alt_text,omitempty"`
	Analysis        string                 `json:"analysis,omitempty"`
	Prompt          string                 `json:"prompt,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// JobStore provides database operations for jobs.
type JobStore struct {
	db *sql.DB
}

// NewJobStore creates a new job store.
func NewJobStore(db *sql.DB) *JobStore {
	return &JobStore{db: db}
}

// Create creates a new job.
func (s *JobStore) Create(ctx context.Context, job *Job) error {
	query := `
		INSERT INTO jobs (id, job_type, status, input_payload, model_used, hardware_target, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now
	if job.Status == "" {
		job.Status = JobStatusPending
	}

	_, err := s.db.ExecContext(ctx, query,
		job.ID,
		job.JobType,
		job.Status,
		job.InputPayload,
		nullString(job.ModelUsed),
		nullString(job.HardwareTarget),
		job.CreatedAt,
		job.UpdatedAt,
	)
	return err
}

// Get retrieves a job by ID.
func (s *JobStore) Get(ctx context.Context, id string) (*Job, error) {
	query := `
		SELECT id, job_type, status, input_payload, output_payload, model_used,
			   hardware_target, error_message, started_at, completed_at, created_at, updated_at
		FROM jobs WHERE id = $1
	`
	job := &Job{}
	var inputPayload, outputPayload sql.NullString
	var modelUsed, hardwareTarget, errorMessage sql.NullString
	var startedAt, completedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&job.ID,
		&job.JobType,
		&job.Status,
		&inputPayload,
		&outputPayload,
		&modelUsed,
		&hardwareTarget,
		&errorMessage,
		&startedAt,
		&completedAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	if inputPayload.Valid {
		job.InputPayload = json.RawMessage(inputPayload.String)
	}
	if outputPayload.Valid {
		job.OutputPayload = json.RawMessage(outputPayload.String)
	}
	if modelUsed.Valid {
		job.ModelUsed = modelUsed.String
	}
	if hardwareTarget.Valid {
		job.HardwareTarget = hardwareTarget.String
	}
	if errorMessage.Valid {
		job.ErrorMessage = errorMessage.String
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}

	return job, nil
}

// Update updates a job.
func (s *JobStore) Update(ctx context.Context, job *Job) error {
	query := `
		UPDATE jobs SET
			status = $2,
			output_payload = $3,
			model_used = $4,
			hardware_target = $5,
			error_message = $6,
			started_at = $7,
			completed_at = $8,
			updated_at = $9
		WHERE id = $1
	`
	job.UpdatedAt = time.Now()

	_, err := s.db.ExecContext(ctx, query,
		job.ID,
		job.Status,
		nullJSON(job.OutputPayload),
		nullString(job.ModelUsed),
		nullString(job.HardwareTarget),
		nullString(job.ErrorMessage),
		nullTime(job.StartedAt),
		nullTime(job.CompletedAt),
		job.UpdatedAt,
	)
	return err
}

// UpdateStatus updates just the status of a job.
func (s *JobStore) UpdateStatus(ctx context.Context, id string, status JobStatus, errorMsg string) error {
	query := `UPDATE jobs SET status = $2, error_message = $3, updated_at = $4 WHERE id = $1`
	now := time.Now()
	_, err := s.db.ExecContext(ctx, query, id, status, nullString(errorMsg), now)
	return err
}

// MarkStarted marks a job as started.
func (s *JobStore) MarkStarted(ctx context.Context, id string) error {
	now := time.Now()
	query := `UPDATE jobs SET status = $2, started_at = $3, updated_at = $4 WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id, JobStatusRunning, now, now)
	return err
}

// MarkCompleted marks a job as completed with output.
func (s *JobStore) MarkCompleted(ctx context.Context, id string, output json.RawMessage) error {
	now := time.Now()
	query := `UPDATE jobs SET status = $2, output_payload = $3, completed_at = $4, updated_at = $5 WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id, JobStatusCompleted, nullJSON(output), now, now)
	return err
}

// MarkFailed marks a job as failed with an error message.
func (s *JobStore) MarkFailed(ctx context.Context, id string, errorMsg string) error {
	now := time.Now()
	query := `UPDATE jobs SET status = $2, error_message = $3, completed_at = $4, updated_at = $5 WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id, JobStatusFailed, errorMsg, now, now)
	return err
}

// List retrieves jobs with optional filtering.
func (s *JobStore) List(ctx context.Context, opts ListJobsOptions) ([]*Job, error) {
	query := `
		SELECT id, job_type, status, input_payload, output_payload, model_used,
			   hardware_target, error_message, started_at, completed_at, created_at, updated_at
		FROM jobs WHERE 1=1
	`
	var args []interface{}
	argIdx := 1

	if opts.JobType != "" {
		query += fmt.Sprintf(" AND job_type = $%d", argIdx)
		args = append(args, opts.JobType)
		argIdx++
	}
	if opts.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, opts.Status)
		argIdx++
	}
	if opts.HardwareTarget != "" {
		query += fmt.Sprintf(" AND hardware_target = $%d", argIdx)
		args = append(args, opts.HardwareTarget)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, opts.Limit)
		argIdx++
	}
	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, opts.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		job := &Job{}
		var inputPayload, outputPayload sql.NullString
		var modelUsed, hardwareTarget, errorMessage sql.NullString
		var startedAt, completedAt sql.NullTime

		err := rows.Scan(
			&job.ID,
			&job.JobType,
			&job.Status,
			&inputPayload,
			&outputPayload,
			&modelUsed,
			&hardwareTarget,
			&errorMessage,
			&startedAt,
			&completedAt,
			&job.CreatedAt,
			&job.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if inputPayload.Valid {
			job.InputPayload = json.RawMessage(inputPayload.String)
		}
		if outputPayload.Valid {
			job.OutputPayload = json.RawMessage(outputPayload.String)
		}
		if modelUsed.Valid {
			job.ModelUsed = modelUsed.String
		}
		if hardwareTarget.Valid {
			job.HardwareTarget = hardwareTarget.String
		}
		if errorMessage.Valid {
			job.ErrorMessage = errorMessage.String
		}
		if startedAt.Valid {
			job.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			job.CompletedAt = &completedAt.Time
		}

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// Delete deletes a job by ID.
func (s *JobStore) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM jobs WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

// ListJobsOptions provides filtering options for listing jobs.
type ListJobsOptions struct {
	JobType        JobType
	Status         JobStatus
	HardwareTarget string
	Limit          int
	Offset         int
}

// Helper functions for nullable fields
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func nullJSON(data json.RawMessage) sql.NullString {
	if len(data) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: string(data), Valid: true}
}
