package storage

import (
	"context"
	"time"
)

// CompactionStats represents statistics about context window compaction
type CompactionStats struct {
	ID               int       `json:"id"`
	JobID            string    `json:"job_id"`
	InferenceRound   int       `json:"inference_round"`
	ModelName        string    `json:"model_name"`
	ContextLimit     int       `json:"context_limit"`
	TokensBefore     int       `json:"tokens_before"`
	TokensAfter      int       `json:"tokens_after"`
	RoundsCompacted  int       `json:"rounds_compacted"`
	RoundsKept       int       `json:"rounds_kept"`
	CompactionTimeMs int       `json:"compaction_time_ms"`
	ThresholdHit     bool      `json:"threshold_hit"`
	CreatedAt        time.Time `json:"created_at"`
}

// CompactionStatsStore handles database operations for compaction statistics
type CompactionStatsStore interface {
	// RecordCompaction records a compaction event
	RecordCompaction(ctx context.Context, stats *CompactionStats) error

	// GetJobCompactionStats retrieves all compaction stats for a job
	GetJobCompactionStats(ctx context.Context, jobID string) ([]*CompactionStats, error)

	// GetCompactionSummary gets summary statistics across all jobs
	GetCompactionSummary(ctx context.Context) (*CompactionSummary, error)
}

// CompactionSummary provides aggregate statistics
type CompactionSummary struct {
	TotalCompactions      int     `json:"total_compactions"`
	AverageTokensBefore   float64 `json:"average_tokens_before"`
	AverageTokensAfter    float64 `json:"average_tokens_after"`
	AverageCompactionTime float64 `json:"average_compaction_time_ms"`
	ThresholdHitCount     int     `json:"threshold_hit_count"`
	MostCompactedJob      string  `json:"most_compacted_job"`
}
