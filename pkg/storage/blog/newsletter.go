package blog

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AssetType represents the type of newsletter asset
type AssetType string

const (
	AssetVideo   AssetType = "video"
	AssetEvent   AssetType = "event"
	AssetMeetup  AssetType = "meetup"
	AssetLink    AssetType = "link"
	AssetReading AssetType = "reading"
)

// NewsletterAsset represents an asset for the newsletter section
type NewsletterAsset struct {
	ID          uuid.UUID  `json:"id"`
	AssetType   AssetType  `json:"asset_type"`
	Title       string     `json:"title"`
	URL         string     `json:"url,omitempty"`
	EventDate   *time.Time `json:"event_date,omitempty"`
	Description string     `json:"description,omitempty"`
	EmbedCode   string     `json:"embed_code,omitempty"`
	UsedInPost  *uuid.UUID `json:"used_in_post,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// NewsletterStore handles newsletter asset database operations
type NewsletterStore struct {
	db *sql.DB
}

// NewNewsletterStore creates a new newsletter store
func NewNewsletterStore(db *sql.DB) *NewsletterStore {
	return &NewsletterStore{db: db}
}

// Create creates a new newsletter asset
func (s *NewsletterStore) Create(asset *NewsletterAsset) error {
	if asset.ID == uuid.Nil {
		asset.ID = uuid.New()
	}

	query := `
		INSERT INTO newsletter_assets (
			id, asset_type, title, url, event_date,
			description, embed_code, used_in_post
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at
	`

	err := s.db.QueryRow(
		query,
		asset.ID, asset.AssetType, asset.Title, asset.URL, asset.EventDate,
		asset.Description, asset.EmbedCode, asset.UsedInPost,
	).Scan(&asset.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create newsletter asset: %w", err)
	}

	return nil
}

// Get retrieves a newsletter asset by ID
func (s *NewsletterStore) Get(id uuid.UUID) (*NewsletterAsset, error) {
	query := `
		SELECT id, asset_type, title, url, event_date,
		       description, embed_code, used_in_post, created_at
		FROM newsletter_assets
		WHERE id = $1
	`

	asset := &NewsletterAsset{}
	err := s.db.QueryRow(query, id).Scan(
		&asset.ID, &asset.AssetType, &asset.Title, &asset.URL, &asset.EventDate,
		&asset.Description, &asset.EmbedCode, &asset.UsedInPost, &asset.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("newsletter asset not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get newsletter asset: %w", err)
	}

	return asset, nil
}

// Update updates a newsletter asset
func (s *NewsletterStore) Update(asset *NewsletterAsset) error {
	query := `
		UPDATE newsletter_assets SET
			asset_type = $2,
			title = $3,
			url = $4,
			event_date = $5,
			description = $6,
			embed_code = $7,
			used_in_post = $8
		WHERE id = $1
	`

	result, err := s.db.Exec(
		query,
		asset.ID, asset.AssetType, asset.Title, asset.URL, asset.EventDate,
		asset.Description, asset.EmbedCode, asset.UsedInPost,
	)

	if err != nil {
		return fmt.Errorf("failed to update newsletter asset: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("newsletter asset not found: %s", asset.ID)
	}

	return nil
}

// MarkAsUsed marks an asset as used in a specific blog post
func (s *NewsletterStore) MarkAsUsed(assetID, postID uuid.UUID) error {
	query := `UPDATE newsletter_assets SET used_in_post = $2 WHERE id = $1`
	result, err := s.db.Exec(query, assetID, postID)
	if err != nil {
		return fmt.Errorf("failed to mark asset as used: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("newsletter asset not found: %s", assetID)
	}

	return nil
}

// AssetFilters represents filters for listing assets
type AssetFilters struct {
	AssetType  *AssetType
	OnlyUnused bool
	Limit      int
	Offset     int
}

// List returns newsletter assets with optional filters
func (s *NewsletterStore) List(filters AssetFilters) ([]*NewsletterAsset, error) {
	query := `
		SELECT id, asset_type, title, url, event_date,
		       description, embed_code, used_in_post, created_at
		FROM newsletter_assets
		WHERE 1=1
	`
	args := []interface{}{}
	argNum := 1

	if filters.AssetType != nil {
		query += fmt.Sprintf(" AND asset_type = $%d", argNum)
		args = append(args, *filters.AssetType)
		argNum++
	}

	if filters.OnlyUnused {
		query += " AND used_in_post IS NULL"
	}

	query += " ORDER BY created_at DESC"

	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, filters.Limit)
		argNum++
	}

	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, filters.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list newsletter assets: %w", err)
	}
	defer rows.Close()

	var assets []*NewsletterAsset
	for rows.Next() {
		asset := &NewsletterAsset{}
		err := rows.Scan(
			&asset.ID, &asset.AssetType, &asset.Title, &asset.URL, &asset.EventDate,
			&asset.Description, &asset.EmbedCode, &asset.UsedInPost, &asset.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan newsletter asset: %w", err)
		}

		assets = append(assets, asset)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating newsletter assets: %w", err)
	}

	return assets, nil
}

// GetUpcomingEvents returns events that haven't happened yet
func (s *NewsletterStore) GetUpcomingEvents() ([]*NewsletterAsset, error) {
	query := `
		SELECT id, asset_type, title, url, event_date,
		       description, embed_code, used_in_post, created_at
		FROM newsletter_assets
		WHERE asset_type IN ('event', 'meetup')
		  AND event_date IS NOT NULL
		  AND event_date > NOW()
		  AND used_in_post IS NULL
		ORDER BY event_date ASC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get upcoming events: %w", err)
	}
	defer rows.Close()

	var assets []*NewsletterAsset
	for rows.Next() {
		asset := &NewsletterAsset{}
		err := rows.Scan(
			&asset.ID, &asset.AssetType, &asset.Title, &asset.URL, &asset.EventDate,
			&asset.Description, &asset.EmbedCode, &asset.UsedInPost, &asset.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan newsletter asset: %w", err)
		}

		assets = append(assets, asset)
	}

	return assets, rows.Err()
}

// Delete deletes a newsletter asset
func (s *NewsletterStore) Delete(id uuid.UUID) error {
	query := `DELETE FROM newsletter_assets WHERE id = $1`
	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete newsletter asset: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("newsletter asset not found: %s", id)
	}

	return nil
}
