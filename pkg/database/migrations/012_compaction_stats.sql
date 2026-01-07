-- +goose Up
-- Migration: Add compaction statistics table
-- Created: 2026-01-05
-- Description: Track context window compaction events for monitoring and optimization

CREATE TABLE IF NOT EXISTS compaction_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id VARCHAR(255) NOT NULL,
    inference_round INTEGER NOT NULL,
    model_name VARCHAR(255) NOT NULL,
    context_limit INTEGER NOT NULL,
    tokens_before INTEGER NOT NULL,
    tokens_after INTEGER NOT NULL,
    rounds_compacted INTEGER NOT NULL,
    rounds_kept INTEGER NOT NULL,
    compaction_time_ms INTEGER NOT NULL,
    threshold_hit BOOLEAN NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_compaction_job_id ON compaction_stats(job_id);
CREATE INDEX IF NOT EXISTS idx_compaction_created_at ON compaction_stats(created_at);
CREATE INDEX IF NOT EXISTS idx_compaction_threshold_hit ON compaction_stats(threshold_hit);

-- +goose Down
DROP TABLE IF EXISTS compaction_stats;
