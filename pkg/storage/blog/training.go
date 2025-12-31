package blog

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SourceType represents the source of training data
type SourceType string

const (
	SourceDictation SourceType = "dictation"
	SourceTwitch    SourceType = "twitch"
	SourceBlog      SourceType = "blog"
	SourceAgentRun  SourceType = "agent_run"
)

// TrainingPair represents a training data pair for fine-tuning
type TrainingPair struct {
	ID                 uuid.UUID              `json:"id"`
	SourceType         SourceType             `json:"source_type"`
	InputText          string                 `json:"input_text"`
	OutputText         string                 `json:"output_text"`
	QualityScore       *float64               `json:"quality_score,omitempty"`
	IncludedInTraining bool                   `json:"included_in_training"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
}

// TrainingStore handles training pair database operations
type TrainingStore struct {
	db *sql.DB
}

// NewTrainingStore creates a new training store
func NewTrainingStore(db *sql.DB) *TrainingStore {
	return &TrainingStore{db: db}
}

// Create creates a new training pair
func (s *TrainingStore) Create(pair *TrainingPair) error {
	if pair.ID == uuid.Nil {
		pair.ID = uuid.New()
	}

	var metadataJSON []byte
	if pair.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(pair.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	query := `
		INSERT INTO training_pairs (
			id, source_type, input_text, output_text,
			quality_score, included_in_training, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at
	`

	err := s.db.QueryRow(
		query,
		pair.ID, pair.SourceType, pair.InputText, pair.OutputText,
		pair.QualityScore, pair.IncludedInTraining, metadataJSON,
	).Scan(&pair.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create training pair: %w", err)
	}

	return nil
}

// Get retrieves a training pair by ID
func (s *TrainingStore) Get(id uuid.UUID) (*TrainingPair, error) {
	query := `
		SELECT id, source_type, input_text, output_text,
		       quality_score, included_in_training, metadata, created_at
		FROM training_pairs
		WHERE id = $1
	`

	pair := &TrainingPair{}
	var metadataJSON []byte

	err := s.db.QueryRow(query, id).Scan(
		&pair.ID, &pair.SourceType, &pair.InputText, &pair.OutputText,
		&pair.QualityScore, &pair.IncludedInTraining, &metadataJSON,
		&pair.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("training pair not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get training pair: %w", err)
	}

	if metadataJSON != nil {
		if err := json.Unmarshal(metadataJSON, &pair.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return pair, nil
}

// Update updates a training pair
func (s *TrainingStore) Update(pair *TrainingPair) error {
	var metadataJSON []byte
	if pair.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(pair.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	query := `
		UPDATE training_pairs SET
			source_type = $2,
			input_text = $3,
			output_text = $4,
			quality_score = $5,
			included_in_training = $6,
			metadata = $7
		WHERE id = $1
	`

	result, err := s.db.Exec(
		query,
		pair.ID, pair.SourceType, pair.InputText, pair.OutputText,
		pair.QualityScore, pair.IncludedInTraining, metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to update training pair: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("training pair not found: %s", pair.ID)
	}

	return nil
}

// SetIncluded marks a training pair as included/excluded from training
func (s *TrainingStore) SetIncluded(id uuid.UUID, included bool) error {
	query := `UPDATE training_pairs SET included_in_training = $2 WHERE id = $1`
	result, err := s.db.Exec(query, id, included)
	if err != nil {
		return fmt.Errorf("failed to update training inclusion: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("training pair not found: %s", id)
	}

	return nil
}

// ListFilters represents filters for listing training pairs
type ListFilters struct {
	SourceType         *SourceType
	IncludedInTraining *bool
	MinQualityScore    *float64
	Limit              int
	Offset             int
}

// List returns training pairs with optional filters
func (s *TrainingStore) List(filters ListFilters) ([]*TrainingPair, error) {
	query := `
		SELECT id, source_type, input_text, output_text,
		       quality_score, included_in_training, metadata, created_at
		FROM training_pairs
		WHERE 1=1
	`
	args := []interface{}{}
	argNum := 1

	if filters.SourceType != nil {
		query += fmt.Sprintf(" AND source_type = $%d", argNum)
		args = append(args, *filters.SourceType)
		argNum++
	}

	if filters.IncludedInTraining != nil {
		query += fmt.Sprintf(" AND included_in_training = $%d", argNum)
		args = append(args, *filters.IncludedInTraining)
		argNum++
	}

	if filters.MinQualityScore != nil {
		query += fmt.Sprintf(" AND quality_score >= $%d", argNum)
		args = append(args, *filters.MinQualityScore)
		argNum++
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
		return nil, fmt.Errorf("failed to list training pairs: %w", err)
	}
	defer rows.Close()

	var pairs []*TrainingPair
	for rows.Next() {
		pair := &TrainingPair{}
		var metadataJSON []byte

		err := rows.Scan(
			&pair.ID, &pair.SourceType, &pair.InputText, &pair.OutputText,
			&pair.QualityScore, &pair.IncludedInTraining, &metadataJSON,
			&pair.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan training pair: %w", err)
		}

		if metadataJSON != nil {
			if err := json.Unmarshal(metadataJSON, &pair.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		pairs = append(pairs, pair)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating training pairs: %w", err)
	}

	return pairs, nil
}

// Delete deletes a training pair
func (s *TrainingStore) Delete(id uuid.UUID) error {
	query := `DELETE FROM training_pairs WHERE id = $1`
	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete training pair: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("training pair not found: %s", id)
	}

	return nil
}

// GetStats returns statistics about training pairs
func (s *TrainingStore) GetStats() (map[string]interface{}, error) {
	query := `
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN included_in_training THEN 1 ELSE 0 END) as included,
			SUM(CASE WHEN source_type = 'dictation' THEN 1 ELSE 0 END) as from_dictation,
			SUM(CASE WHEN source_type = 'twitch' THEN 1 ELSE 0 END) as from_twitch,
			SUM(CASE WHEN source_type = 'blog' THEN 1 ELSE 0 END) as from_blog,
			SUM(CASE WHEN source_type = 'agent_run' THEN 1 ELSE 0 END) as from_agent_run,
			AVG(quality_score) as avg_quality_score
		FROM training_pairs
	`

	var total, included, fromDictation, fromTwitch, fromBlog, fromAgentRun int
	var avgQuality sql.NullFloat64

	err := s.db.QueryRow(query).Scan(
		&total, &included, &fromDictation, &fromTwitch,
		&fromBlog, &fromAgentRun, &avgQuality,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get training stats: %w", err)
	}

	stats := map[string]interface{}{
		"total":          total,
		"included":       included,
		"from_dictation": fromDictation,
		"from_twitch":    fromTwitch,
		"from_blog":      fromBlog,
		"from_agent_run": fromAgentRun,
		"avg_quality":    nil,
	}

	if avgQuality.Valid {
		stats["avg_quality"] = avgQuality.Float64
	}

	return stats, nil
}
