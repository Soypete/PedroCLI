-- +goose Up
-- Training data for fine-tuning
CREATE TABLE IF NOT EXISTS training_pairs (
    id TEXT PRIMARY KEY,
    source_type TEXT NOT NULL, -- dictation, twitch, blog, agent_run
    input_text TEXT NOT NULL,
    output_text TEXT NOT NULL,
    quality_score REAL, -- for filtering, 0.0-1.0
    included_in_training INTEGER DEFAULT 0,
    metadata TEXT, -- JSON blob for flexible storage
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for filtering by source type
CREATE INDEX IF NOT EXISTS idx_training_pairs_source_type ON training_pairs(source_type);

-- Index for filtering by training inclusion
CREATE INDEX IF NOT EXISTS idx_training_pairs_included ON training_pairs(included_in_training);

-- Index for quality score filtering
CREATE INDEX IF NOT EXISTS idx_training_pairs_quality ON training_pairs(quality_score);

-- +goose Down
DROP INDEX IF EXISTS idx_training_pairs_quality;
DROP INDEX IF EXISTS idx_training_pairs_included;
DROP INDEX IF EXISTS idx_training_pairs_source_type;
DROP TABLE IF EXISTS training_pairs;
