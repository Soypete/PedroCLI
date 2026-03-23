package db

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UpsertArtifactParams struct {
	DocID         *uuid.UUID      `json:"doc_id"`
	FeedID        *uuid.UUID      `json:"feed_id"`
	ArtifactType  ArtifactType    `json:"artifact_type"`
	PromptVersion string          `json:"prompt_version"`
	Content       json.RawMessage `json:"content"`
	Model         string          `json:"model"`
	InputTokens   int32           `json:"input_tokens"`
	OutputTokens  int32           `json:"output_tokens"`
}

func (db *DB) UpsertArtifact(ctx context.Context, p UpsertArtifactParams) (Artifact, error) {
	var a Artifact
	err := db.conn.QueryRow(ctx,
		`INSERT INTO artifacts (doc_id, feed_id, artifact_type, prompt_version, content, model, input_tokens, output_tokens, generated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
		 ON CONFLICT (doc_id, artifact_type, prompt_version)
		 DO UPDATE SET content = EXCLUDED.content, model = EXCLUDED.model,
		              input_tokens = EXCLUDED.input_tokens, output_tokens = EXCLUDED.output_tokens, generated_at = now()
		 RETURNING *`,
		p.DocID, p.FeedID, p.ArtifactType, p.PromptVersion, p.Content, p.Model, p.InputTokens, p.OutputTokens,
	).Scan(&a.ID, &a.DocID, &a.FeedID, &a.ArtifactType, &a.PromptVersion, &a.Content,
		&a.Model, &a.InputTokens, &a.OutputTokens, &a.GeneratedAt, &a.CreatedAt)
	return a, err
}

func (db *DB) GetArtifactByDocAndType(ctx context.Context, docID uuid.UUID, artType ArtifactType) (Artifact, error) {
	var a Artifact
	err := db.conn.QueryRow(ctx,
		`SELECT * FROM artifacts WHERE doc_id = $1 AND artifact_type = $2 ORDER BY generated_at DESC LIMIT 1`,
		docID, artType,
	).Scan(&a.ID, &a.DocID, &a.FeedID, &a.ArtifactType, &a.PromptVersion, &a.Content,
		&a.Model, &a.InputTokens, &a.OutputTokens, &a.GeneratedAt, &a.CreatedAt)
	return a, err
}

func (db *DB) ListArtifactsByDocID(ctx context.Context, docID uuid.UUID) ([]Artifact, error) {
	rows, err := db.conn.Query(ctx,
		`SELECT * FROM artifacts WHERE doc_id = $1 ORDER BY artifact_type, generated_at DESC`, docID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Artifact, error) {
		var a Artifact
		err := row.Scan(&a.ID, &a.DocID, &a.FeedID, &a.ArtifactType, &a.PromptVersion, &a.Content,
			&a.Model, &a.InputTokens, &a.OutputTokens, &a.GeneratedAt, &a.CreatedAt)
		return a, err
	})
}
