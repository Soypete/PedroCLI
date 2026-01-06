package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/soypete/pedrocli/pkg/storage"
)

// CompactionStatsStore implements storage.CompactionStatsStore for PostgreSQL
type CompactionStatsStore struct {
	db *sql.DB
}

// NewCompactionStatsStore creates a new compaction stats store
func NewCompactionStatsStore(db *sql.DB) *CompactionStatsStore {
	return &CompactionStatsStore{db: db}
}

// RecordCompaction records a compaction event
func (s *CompactionStatsStore) RecordCompaction(ctx context.Context, stats *storage.CompactionStats) error {
	query := `
		INSERT INTO compaction_stats (
			job_id, inference_round, model_name, context_limit,
			tokens_before, tokens_after, rounds_compacted, rounds_kept,
			compaction_time_ms, threshold_hit, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`

	err := s.db.QueryRowContext(
		ctx,
		query,
		stats.JobID,
		stats.InferenceRound,
		stats.ModelName,
		stats.ContextLimit,
		stats.TokensBefore,
		stats.TokensAfter,
		stats.RoundsCompacted,
		stats.RoundsKept,
		stats.CompactionTimeMs,
		stats.ThresholdHit,
		stats.CreatedAt,
	).Scan(&stats.ID)

	if err != nil {
		return fmt.Errorf("failed to record compaction stats: %w", err)
	}

	return nil
}

// GetJobCompactionStats retrieves all compaction stats for a job
func (s *CompactionStatsStore) GetJobCompactionStats(ctx context.Context, jobID string) ([]*storage.CompactionStats, error) {
	query := `
		SELECT
			id, job_id, inference_round, model_name, context_limit,
			tokens_before, tokens_after, rounds_compacted, rounds_kept,
			compaction_time_ms, threshold_hit, created_at
		FROM compaction_stats
		WHERE job_id = $1
		ORDER BY inference_round ASC
	`

	rows, err := s.db.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to query compaction stats: %w", err)
	}
	defer rows.Close()

	var stats []*storage.CompactionStats
	for rows.Next() {
		stat := &storage.CompactionStats{}
		err := rows.Scan(
			&stat.ID,
			&stat.JobID,
			&stat.InferenceRound,
			&stat.ModelName,
			&stat.ContextLimit,
			&stat.TokensBefore,
			&stat.TokensAfter,
			&stat.RoundsCompacted,
			&stat.RoundsKept,
			&stat.CompactionTimeMs,
			&stat.ThresholdHit,
			&stat.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan compaction stat: %w", err)
		}
		stats = append(stats, stat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating compaction stats: %w", err)
	}

	return stats, nil
}

// GetCompactionSummary gets summary statistics across all jobs
func (s *CompactionStatsStore) GetCompactionSummary(ctx context.Context) (*storage.CompactionSummary, error) {
	query := `
		WITH stats AS (
			SELECT
				COUNT(*) as total_compactions,
				AVG(tokens_before) as avg_tokens_before,
				AVG(tokens_after) as avg_tokens_after,
				AVG(compaction_time_ms) as avg_compaction_time,
				SUM(CASE WHEN threshold_hit THEN 1 ELSE 0 END) as threshold_hit_count
			FROM compaction_stats
		),
		most_compacted AS (
			SELECT job_id, COUNT(*) as compaction_count
			FROM compaction_stats
			GROUP BY job_id
			ORDER BY compaction_count DESC
			LIMIT 1
		)
		SELECT
			COALESCE(s.total_compactions, 0),
			COALESCE(s.avg_tokens_before, 0),
			COALESCE(s.avg_tokens_after, 0),
			COALESCE(s.avg_compaction_time, 0),
			COALESCE(s.threshold_hit_count, 0),
			COALESCE(mc.job_id, '')
		FROM stats s
		LEFT JOIN most_compacted mc ON true
	`

	summary := &storage.CompactionSummary{}
	err := s.db.QueryRowContext(ctx, query).Scan(
		&summary.TotalCompactions,
		&summary.AverageTokensBefore,
		&summary.AverageTokensAfter,
		&summary.AverageCompactionTime,
		&summary.ThresholdHitCount,
		&summary.MostCompactedJob,
	)

	if err == sql.ErrNoRows {
		// No stats yet, return empty summary
		return summary, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get compaction summary: %w", err)
	}

	return summary, nil
}
