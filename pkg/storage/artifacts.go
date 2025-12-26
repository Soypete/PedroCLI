package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ArtifactType represents the type of artifact.
type ArtifactType string

const (
	ArtifactTypeReferenceImage ArtifactType = "reference_image"
	ArtifactTypeGeneratedImage ArtifactType = "generated_image"
	ArtifactTypePrompt         ArtifactType = "prompt"
	ArtifactTypeAltText        ArtifactType = "alt_text"
	ArtifactTypeWorkflow       ArtifactType = "workflow"
	ArtifactTypeLog            ArtifactType = "log"
)

// Artifact represents a job artifact in the database.
type Artifact struct {
	ID               string          `json:"id"`
	JobID            string          `json:"job_id"`
	ArtifactType     ArtifactType    `json:"artifact_type"`
	FilePath         string          `json:"file_path"`
	OriginalFilename string          `json:"original_filename,omitempty"`
	MimeType         string          `json:"mime_type,omitempty"`
	FileSize         int64           `json:"file_size,omitempty"`
	Checksum         string          `json:"checksum,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

// ArtifactMetadata represents common metadata fields.
type ArtifactMetadata struct {
	Width       int                    `json:"width,omitempty"`
	Height      int                    `json:"height,omitempty"`
	Format      string                 `json:"format,omitempty"`
	Prompt      string                 `json:"prompt,omitempty"`
	Model       string                 `json:"model,omitempty"`
	Seed        int64                  `json:"seed,omitempty"`
	CFGScale    float64                `json:"cfg_scale,omitempty"`
	Steps       int                    `json:"steps,omitempty"`
	Sampler     string                 `json:"sampler,omitempty"`
	AltText     string                 `json:"alt_text,omitempty"`
	StylePreset string                 `json:"style_preset,omitempty"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// ArtifactStore provides database operations for artifacts.
type ArtifactStore struct {
	db *sql.DB
}

// NewArtifactStore creates a new artifact store.
func NewArtifactStore(db *sql.DB) *ArtifactStore {
	return &ArtifactStore{db: db}
}

// Create creates a new artifact.
func (s *ArtifactStore) Create(ctx context.Context, artifact *Artifact) error {
	query := `
		INSERT INTO job_artifacts (id, job_id, artifact_type, file_path, original_filename,
			mime_type, file_size, checksum, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	artifact.CreatedAt = time.Now()

	var fileSize sql.NullInt64
	if artifact.FileSize > 0 {
		fileSize = sql.NullInt64{Int64: artifact.FileSize, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, query,
		artifact.ID,
		artifact.JobID,
		artifact.ArtifactType,
		artifact.FilePath,
		nullString(artifact.OriginalFilename),
		nullString(artifact.MimeType),
		fileSize,
		nullString(artifact.Checksum),
		nullJSON(artifact.Metadata),
		artifact.CreatedAt,
	)
	return err
}

// Get retrieves an artifact by ID.
func (s *ArtifactStore) Get(ctx context.Context, id string) (*Artifact, error) {
	query := `
		SELECT id, job_id, artifact_type, file_path, original_filename,
			   mime_type, file_size, checksum, metadata, created_at
		FROM job_artifacts WHERE id = $1
	`
	artifact := &Artifact{}
	var originalFilename, mimeType, checksum, metadata sql.NullString
	var fileSize sql.NullInt64

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&artifact.ID,
		&artifact.JobID,
		&artifact.ArtifactType,
		&artifact.FilePath,
		&originalFilename,
		&mimeType,
		&fileSize,
		&checksum,
		&metadata,
		&artifact.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("artifact not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	if originalFilename.Valid {
		artifact.OriginalFilename = originalFilename.String
	}
	if mimeType.Valid {
		artifact.MimeType = mimeType.String
	}
	if fileSize.Valid {
		artifact.FileSize = fileSize.Int64
	}
	if checksum.Valid {
		artifact.Checksum = checksum.String
	}
	if metadata.Valid {
		artifact.Metadata = json.RawMessage(metadata.String)
	}

	return artifact, nil
}

// ListByJob retrieves all artifacts for a job.
func (s *ArtifactStore) ListByJob(ctx context.Context, jobID string) ([]*Artifact, error) {
	query := `
		SELECT id, job_id, artifact_type, file_path, original_filename,
			   mime_type, file_size, checksum, metadata, created_at
		FROM job_artifacts WHERE job_id = $1
		ORDER BY created_at ASC
	`
	return s.queryArtifacts(ctx, query, jobID)
}

// ListByJobAndType retrieves artifacts for a job filtered by type.
func (s *ArtifactStore) ListByJobAndType(ctx context.Context, jobID string, artifactType ArtifactType) ([]*Artifact, error) {
	query := `
		SELECT id, job_id, artifact_type, file_path, original_filename,
			   mime_type, file_size, checksum, metadata, created_at
		FROM job_artifacts WHERE job_id = $1 AND artifact_type = $2
		ORDER BY created_at ASC
	`
	return s.queryArtifacts(ctx, query, jobID, artifactType)
}

// queryArtifacts executes a query and returns artifacts.
func (s *ArtifactStore) queryArtifacts(ctx context.Context, query string, args ...interface{}) ([]*Artifact, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artifacts []*Artifact
	for rows.Next() {
		artifact := &Artifact{}
		var originalFilename, mimeType, checksum, metadata sql.NullString
		var fileSize sql.NullInt64

		err := rows.Scan(
			&artifact.ID,
			&artifact.JobID,
			&artifact.ArtifactType,
			&artifact.FilePath,
			&originalFilename,
			&mimeType,
			&fileSize,
			&checksum,
			&metadata,
			&artifact.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if originalFilename.Valid {
			artifact.OriginalFilename = originalFilename.String
		}
		if mimeType.Valid {
			artifact.MimeType = mimeType.String
		}
		if fileSize.Valid {
			artifact.FileSize = fileSize.Int64
		}
		if checksum.Valid {
			artifact.Checksum = checksum.String
		}
		if metadata.Valid {
			artifact.Metadata = json.RawMessage(metadata.String)
		}

		artifacts = append(artifacts, artifact)
	}

	return artifacts, rows.Err()
}

// Delete deletes an artifact by ID.
func (s *ArtifactStore) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM job_artifacts WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

// DeleteByJob deletes all artifacts for a job.
func (s *ArtifactStore) DeleteByJob(ctx context.Context, jobID string) error {
	query := `DELETE FROM job_artifacts WHERE job_id = $1`
	_, err := s.db.ExecContext(ctx, query, jobID)
	return err
}

// Count returns the number of artifacts for a job.
func (s *ArtifactStore) Count(ctx context.Context, jobID string) (int, error) {
	query := `SELECT COUNT(*) FROM job_artifacts WHERE job_id = $1`
	var count int
	err := s.db.QueryRowContext(ctx, query, jobID).Scan(&count)
	return count, err
}
