-- Training data for fine-tuning
CREATE TABLE training_pairs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_type VARCHAR(50) NOT NULL, -- dictation, twitch, blog, agent_run
    input_text TEXT NOT NULL,
    output_text TEXT NOT NULL,
    quality_score FLOAT, -- for filtering, 0.0-1.0
    included_in_training BOOLEAN DEFAULT false,
    metadata JSONB, -- flexible storage for additional data
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Index for filtering by source type
CREATE INDEX idx_training_pairs_source_type ON training_pairs(source_type);

-- Index for filtering by training inclusion
CREATE INDEX idx_training_pairs_included ON training_pairs(included_in_training);

-- Index for quality score filtering
CREATE INDEX idx_training_pairs_quality ON training_pairs(quality_score) WHERE quality_score IS NOT NULL;
