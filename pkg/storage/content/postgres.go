package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PostgresContentStore implements ContentStore using PostgreSQL
// Used for Web UI mode - stores content in database for persistence and multi-user access
type PostgresContentStore struct {
	db *sql.DB
}

// NewPostgresContentStore creates a new PostgreSQL-based content store
func NewPostgresContentStore(db *sql.DB) ContentStore {
	return &PostgresContentStore{db: db}
}

// Create stores new content in the database
func (s *PostgresContentStore) Create(ctx context.Context, content *Content) error {
	// Generate ID if not set
	if content.ID == uuid.Nil {
		content.ID = uuid.New()
	}

	// Set timestamps
	now := time.Now()
	if content.CreatedAt.IsZero() {
		content.CreatedAt = now
	}
	content.UpdatedAt = now

	// Marshal data to JSON
	dataJSON, err := json.Marshal(content.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Insert into database
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO content (id, type, status, title, data, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, content.ID, content.Type, content.Status, content.Title,
		dataJSON, content.CreatedAt, content.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert content: %w", err)
	}

	return nil
}

// Get retrieves content by ID
func (s *PostgresContentStore) Get(ctx context.Context, id uuid.UUID) (*Content, error) {
	var content Content
	var dataJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT id, type, status, title, data, created_at, updated_at
		FROM content
		WHERE id = $1
	`, id).Scan(
		&content.ID,
		&content.Type,
		&content.Status,
		&content.Title,
		&dataJSON,
		&content.CreatedAt,
		&content.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("content not found: %s", id)
		}
		return nil, fmt.Errorf("failed to query content: %w", err)
	}

	// Unmarshal data
	if err := json.Unmarshal(dataJSON, &content.Data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return &content, nil
}

// Update modifies existing content
func (s *PostgresContentStore) Update(ctx context.Context, content *Content) error {
	// Update timestamp
	content.UpdatedAt = time.Now()

	// Marshal data
	dataJSON, err := json.Marshal(content.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Update database
	result, err := s.db.ExecContext(ctx, `
		UPDATE content
		SET type = $2, status = $3, title = $4, data = $5, updated_at = $6
		WHERE id = $1
	`, content.ID, content.Type, content.Status, content.Title, dataJSON, content.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update content: %w", err)
	}

	// Check if any rows were affected
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("content not found: %s", content.ID)
	}

	return nil
}

// List retrieves content matching filter
func (s *PostgresContentStore) List(ctx context.Context, filter Filter) ([]*Content, error) {
	// Build query based on filter
	query := `SELECT id, type, status, title, data, created_at, updated_at FROM content WHERE 1=1`
	args := []interface{}{}
	argNum := 1

	if filter.Type != nil {
		query += fmt.Sprintf(" AND type = $%d", argNum)
		args = append(args, *filter.Type)
		argNum++
	}

	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, *filter.Status)
	}

	query += " ORDER BY created_at DESC"

	// Execute query
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query content: %w", err)
	}
	defer rows.Close()

	var results []*Content

	for rows.Next() {
		var content Content
		var dataJSON []byte

		err := rows.Scan(
			&content.ID,
			&content.Type,
			&content.Status,
			&content.Title,
			&dataJSON,
			&content.CreatedAt,
			&content.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Unmarshal data
		if err := json.Unmarshal(dataJSON, &content.Data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal data: %w", err)
		}

		results = append(results, &content)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return results, nil
}

// Delete removes content
func (s *PostgresContentStore) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM content WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete content: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("content not found: %s", id)
	}

	return nil
}

// PostgresVersionStore implements VersionStore using PostgreSQL
type PostgresVersionStore struct {
	db *sql.DB
}

// NewPostgresVersionStore creates a new PostgreSQL-based version store
func NewPostgresVersionStore(db *sql.DB) VersionStore {
	return &PostgresVersionStore{db: db}
}

// SaveVersion stores a version snapshot
func (s *PostgresVersionStore) SaveVersion(ctx context.Context, version *Version) error {
	// Generate ID if not set
	if version.ID == uuid.Nil {
		version.ID = uuid.New()
	}

	// Set timestamp
	if version.CreatedAt.IsZero() {
		version.CreatedAt = time.Now()
	}

	// Marshal snapshot
	snapshotJSON, err := json.Marshal(version.Snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	// Insert into database
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO content_versions (id, content_id, phase, version_num, snapshot, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, version.ID, version.ContentID, version.Phase, version.VersionNum,
		snapshotJSON, version.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert version: %w", err)
	}

	return nil
}

// GetVersion retrieves a specific version
func (s *PostgresVersionStore) GetVersion(ctx context.Context, contentID uuid.UUID, versionNum int) (*Version, error) {
	var version Version
	var snapshotJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT id, content_id, phase, version_num, snapshot, created_at
		FROM content_versions
		WHERE content_id = $1 AND version_num = $2
	`, contentID, versionNum).Scan(
		&version.ID,
		&version.ContentID,
		&version.Phase,
		&version.VersionNum,
		&snapshotJSON,
		&version.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("version not found: %s v%d", contentID, versionNum)
		}
		return nil, fmt.Errorf("failed to query version: %w", err)
	}

	// Unmarshal snapshot
	if err := json.Unmarshal(snapshotJSON, &version.Snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	return &version, nil
}

// ListVersions retrieves all versions for content
func (s *PostgresVersionStore) ListVersions(ctx context.Context, contentID uuid.UUID) ([]*Version, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, content_id, phase, version_num, snapshot, created_at
		FROM content_versions
		WHERE content_id = $1
		ORDER BY version_num ASC
	`, contentID)

	if err != nil {
		return nil, fmt.Errorf("failed to query versions: %w", err)
	}
	defer rows.Close()

	var versions []*Version

	for rows.Next() {
		var version Version
		var snapshotJSON []byte

		err := rows.Scan(
			&version.ID,
			&version.ContentID,
			&version.Phase,
			&version.VersionNum,
			&snapshotJSON,
			&version.CreatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Unmarshal snapshot
		if err := json.Unmarshal(snapshotJSON, &version.Snapshot); err != nil {
			return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
		}

		versions = append(versions, &version)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return versions, nil
}

// DeleteVersions removes all versions for content
func (s *PostgresVersionStore) DeleteVersions(ctx context.Context, contentID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM content_versions WHERE content_id = $1`, contentID)
	if err != nil {
		return fmt.Errorf("failed to delete versions: %w", err)
	}

	return nil
}
