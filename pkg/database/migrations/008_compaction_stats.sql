-- Migration: Add compaction statistics table
-- Created: 2026-01-05
-- Description: Track context window compaction events for monitoring and optimization

-- +migrate Up
CREATE TABLE IF NOT EXISTS compaction_stats (
    id SERIAL PRIMARY KEY,
    job_id VARCHAR(255) NOT NULL,
    inference_round INTEGER NOT NULL,
    model_name VARCHAR(255) NOT NULL,
    context_limit INTEGER NOT NULL,
    tokens_before INTEGER NOT NULL,
    tokens_after INTEGER NOT NULL,
    rounds_compacted INTEGER NOT NULL,
    rounds_kept INTEGER NOT NULL,
    compaction_time_ms INTEGER NOT NULL,
    threshold_hit BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Indexes for efficient querying
    INDEX idx_compaction_job_id (job_id),
    INDEX idx_compaction_created_at (created_at),
    INDEX idx_compaction_threshold_hit (threshold_hit)
);

-- Comment on table
COMMENT ON TABLE compaction_stats IS 'Statistics for context window compaction events';

-- Comments on columns
COMMENT ON COLUMN compaction_stats.job_id IS 'ID of the job that triggered compaction';
COMMENT ON COLUMN compaction_stats.inference_round IS 'Which inference round triggered compaction';
COMMENT ON COLUMN compaction_stats.model_name IS 'Model being used (for tokenization method tracking)';
COMMENT ON COLUMN compaction_stats.context_limit IS 'Total context window size in tokens';
COMMENT ON COLUMN compaction_stats.tokens_before IS 'Token count before compaction';
COMMENT ON COLUMN compaction_stats.tokens_after IS 'Token count after compaction';
COMMENT ON COLUMN compaction_stats.rounds_compacted IS 'Number of rounds that were summarized';
COMMENT ON COLUMN compaction_stats.rounds_kept IS 'Number of recent rounds kept in full';
COMMENT ON COLUMN compaction_stats.compaction_time_ms IS 'Time taken to perform compaction in milliseconds';
COMMENT ON COLUMN compaction_stats.threshold_hit IS 'Whether the 75% threshold was exceeded';

-- +migrate Down
DROP TABLE IF NOT EXISTS compaction_stats;
