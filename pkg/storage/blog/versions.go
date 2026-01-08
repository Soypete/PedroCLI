package blog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// VersionType represents the type of version
type VersionType string

const (
	VersionTypeAutoSnapshot VersionType = "auto_snapshot"
	VersionTypeManualSave   VersionType = "manual_save"
	VersionTypePhaseResult  VersionType = "phase_result"
)

// Section represents a section of a blog post
type Section struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Order   int    `json:"order"`
}

// PostVersion represents a snapshot of a blog post at a specific version
type PostVersion struct {
	ID            uuid.UUID   `json:"id"`
	PostID        uuid.UUID   `json:"post_id"`
	VersionNumber int         `json:"version_number"`
	VersionType   VersionType `json:"version_type"`
	Status        PostStatus  `json:"status"`
	Phase         string      `json:"phase,omitempty"`

	// Content at this version
	PostTitle        string    `json:"post_title,omitempty"` // The blog post title at this version
	Title            string    `json:"title,omitempty"`        // Section title (if applicable)
	RawTranscription string    `json:"raw_transcription,omitempty"`
	Outline          string    `json:"outline,omitempty"`
	Sections         []Section `json:"sections,omitempty"`
	FullContent      string    `json:"full_content,omitempty"`

	// Metadata
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	ChangeNotes string    `json:"change_notes,omitempty"`
}

// VersionDiff represents the difference between two versions
type VersionDiff struct {
	PostID        uuid.UUID       `json:"post_id"`
	FromVersion   int             `json:"from_version"`
	ToVersion     int             `json:"to_version"`
	TitleChanged  bool            `json:"title_changed"`
	ContentDiff   string          `json:"content_diff"` // Simple diff for now
	FromVersionAt time.Time       `json:"from_version_at"`
	ToVersionAt   time.Time       `json:"to_version_at"`
	Changes       []VersionChange `json:"changes"`
}

// VersionChange represents a specific change between versions
type VersionChange struct {
	Field    string `json:"field"`
	OldValue string `json:"old_value,omitempty"`
	NewValue string `json:"new_value,omitempty"`
}

// VersionStore handles blog post version database operations
type VersionStore struct {
	db *sql.DB
}

// NewVersionStore creates a new version store
func NewVersionStore(db *sql.DB) *VersionStore {
	return &VersionStore{db: db}
}

// CreateVersion saves a new version of a blog post
func (s *VersionStore) CreateVersion(ctx context.Context, v *PostVersion) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	if v.CreatedBy == "" {
		v.CreatedBy = "system"
	}

	// Marshal sections to JSON
	var sectionsJSON []byte
	if len(v.Sections) > 0 {
		var err error
		sectionsJSON, err = json.Marshal(v.Sections)
		if err != nil {
			return fmt.Errorf("failed to marshal sections: %w", err)
		}
	}

	query := `
		INSERT INTO blog_post_versions (
			id, post_id, version_number, version_type, status, phase,
			post_title, title, raw_transcription, outline, sections, full_content,
			created_by, change_notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING created_at
	`

	err := s.db.QueryRowContext(
		ctx,
		query,
		v.ID, v.PostID, v.VersionNumber, v.VersionType, v.Status, v.Phase,
		v.PostTitle, v.Title, v.RawTranscription, v.Outline, sectionsJSON, v.FullContent,
		v.CreatedBy, v.ChangeNotes,
	).Scan(&v.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create version: %w", err)
	}

	return nil
}

// GetVersion retrieves a specific version of a blog post
func (s *VersionStore) GetVersion(ctx context.Context, postID uuid.UUID, versionNumber int) (*PostVersion, error) {
	query := `
		SELECT id, post_id, version_number, version_type, status, phase,
		       post_title, title, raw_transcription, outline, sections, full_content,
		       created_by, created_at, change_notes
		FROM blog_post_versions
		WHERE post_id = $1 AND version_number = $2
	`

	version := &PostVersion{}
	var sectionsJSON []byte

	err := s.db.QueryRowContext(ctx, query, postID, versionNumber).Scan(
		&version.ID, &version.PostID, &version.VersionNumber, &version.VersionType,
		&version.Status, &version.Phase, &version.PostTitle, &version.Title, &version.RawTranscription,
		&version.Outline, &sectionsJSON, &version.FullContent, &version.CreatedBy,
		&version.CreatedAt, &version.ChangeNotes,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("version %d not found for post %s", versionNumber, postID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get version: %w", err)
	}

	// Unmarshal sections if present
	if sectionsJSON != nil {
		if err := json.Unmarshal(sectionsJSON, &version.Sections); err != nil {
			return nil, fmt.Errorf("failed to unmarshal sections: %w", err)
		}
	}

	return version, nil
}

// ListVersions returns all versions for a blog post, ordered by version number
func (s *VersionStore) ListVersions(ctx context.Context, postID uuid.UUID) ([]*PostVersion, error) {
	query := `
		SELECT id, post_id, version_number, version_type, status, phase,
		       post_title, title, raw_transcription, outline, sections, full_content,
		       created_by, created_at, change_notes
		FROM blog_post_versions
		WHERE post_id = $1
		ORDER BY version_number DESC
	`

	rows, err := s.db.QueryContext(ctx, query, postID)
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}
	defer rows.Close()

	var versions []*PostVersion
	for rows.Next() {
		version := &PostVersion{}
		var sectionsJSON []byte

		err := rows.Scan(
			&version.ID, &version.PostID, &version.VersionNumber, &version.VersionType,
			&version.Status, &version.Phase, &version.PostTitle, &version.Title, &version.RawTranscription,
			&version.Outline, &sectionsJSON, &version.FullContent, &version.CreatedBy,
			&version.CreatedAt, &version.ChangeNotes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan version: %w", err)
		}

		// Unmarshal sections if present
		if sectionsJSON != nil {
			if err := json.Unmarshal(sectionsJSON, &version.Sections); err != nil {
				return nil, fmt.Errorf("failed to unmarshal sections: %w", err)
			}
		}

		versions = append(versions, version)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating versions: %w", err)
	}

	return versions, nil
}

// GetLatestVersion returns the most recent version for a blog post
func (s *VersionStore) GetLatestVersion(ctx context.Context, postID uuid.UUID) (*PostVersion, error) {
	query := `
		SELECT id, post_id, version_number, version_type, status, phase,
		       post_title, title, raw_transcription, outline, sections, full_content,
		       created_by, created_at, change_notes
		FROM blog_post_versions
		WHERE post_id = $1
		ORDER BY version_number DESC
		LIMIT 1
	`

	version := &PostVersion{}
	var sectionsJSON []byte

	err := s.db.QueryRowContext(ctx, query, postID).Scan(
		&version.ID, &version.PostID, &version.VersionNumber, &version.VersionType,
		&version.Status, &version.Phase, &version.PostTitle, &version.Title, &version.RawTranscription,
		&version.Outline, &sectionsJSON, &version.FullContent, &version.CreatedBy,
		&version.CreatedAt, &version.ChangeNotes,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No versions yet, not an error
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest version: %w", err)
	}

	// Unmarshal sections if present
	if sectionsJSON != nil {
		if err := json.Unmarshal(sectionsJSON, &version.Sections); err != nil {
			return nil, fmt.Errorf("failed to unmarshal sections: %w", err)
		}
	}

	return version, nil
}

// GetNextVersionNumber returns the next version number for a blog post
func (s *VersionStore) GetNextVersionNumber(ctx context.Context, postID uuid.UUID) (int, error) {
	latest, err := s.GetLatestVersion(ctx, postID)
	if err != nil {
		return 0, err
	}
	if latest == nil {
		return 1, nil
	}
	return latest.VersionNumber + 1, nil
}

// DiffVersions compares two versions and returns the differences
func (s *VersionStore) DiffVersions(ctx context.Context, postID uuid.UUID, v1, v2 int) (*VersionDiff, error) {
	// Get both versions
	version1, err := s.GetVersion(ctx, postID, v1)
	if err != nil {
		return nil, fmt.Errorf("failed to get version %d: %w", v1, err)
	}

	version2, err := s.GetVersion(ctx, postID, v2)
	if err != nil {
		return nil, fmt.Errorf("failed to get version %d: %w", v2, err)
	}

	diff := &VersionDiff{
		PostID:        postID,
		FromVersion:   v1,
		ToVersion:     v2,
		FromVersionAt: version1.CreatedAt,
		ToVersionAt:   version2.CreatedAt,
		Changes:       []VersionChange{},
	}

	// Compare fields
	if version1.PostTitle != version2.PostTitle {
		diff.TitleChanged = true
		diff.Changes = append(diff.Changes, VersionChange{
			Field:    "post_title",
			OldValue: version1.PostTitle,
			NewValue: version2.PostTitle,
		})
	}

	if version1.Title != version2.Title {
		diff.Changes = append(diff.Changes, VersionChange{
			Field:    "title",
			OldValue: version1.Title,
			NewValue: version2.Title,
		})
	}

	if version1.Status != version2.Status {
		diff.Changes = append(diff.Changes, VersionChange{
			Field:    "status",
			OldValue: string(version1.Status),
			NewValue: string(version2.Status),
		})
	}

	if version1.FullContent != version2.FullContent {
		diff.Changes = append(diff.Changes, VersionChange{
			Field: "full_content",
			// Don't include full content in change, just mark it changed
		})
		// Simple line-based diff for content
		diff.ContentDiff = generateSimpleDiff(version1.FullContent, version2.FullContent)
	}

	return diff, nil
}

// DeleteVersions deletes all versions for a blog post (cascade on post delete handles this)
func (s *VersionStore) DeleteVersions(ctx context.Context, postID uuid.UUID) error {
	query := `DELETE FROM blog_post_versions WHERE post_id = $1`
	_, err := s.db.ExecContext(ctx, query, postID)
	if err != nil {
		return fmt.Errorf("failed to delete versions: %w", err)
	}
	return nil
}

// generateSimpleDiff creates a simple diff between two text strings
// This is a basic implementation - could be enhanced with proper diff library
func generateSimpleDiff(old, new string) string {
	if old == new {
		return "No changes"
	}

	oldLen := len(old)
	newLen := len(new)

	if oldLen == 0 {
		return fmt.Sprintf("Added %d characters", newLen)
	}
	if newLen == 0 {
		return fmt.Sprintf("Deleted %d characters", oldLen)
	}

	diff := newLen - oldLen
	if diff > 0 {
		return fmt.Sprintf("Added %d characters (total: %d → %d)", diff, oldLen, newLen)
	} else if diff < 0 {
		return fmt.Sprintf("Removed %d characters (total: %d → %d)", -diff, oldLen, newLen)
	}

	return fmt.Sprintf("Modified content (%d characters)", newLen)
}
