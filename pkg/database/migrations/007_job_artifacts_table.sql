-- +goose Up
-- Migration: 007_job_artifacts_table
-- Description: Create job_artifacts table for storing job-related files

CREATE TABLE IF NOT EXISTS job_artifacts (
    id UUID PRIMARY KEY,
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    artifact_type VARCHAR(50) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    original_filename VARCHAR(255),
    mime_type VARCHAR(100),
    file_size BIGINT,
    checksum VARCHAR(64),
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for artifact lookups
CREATE INDEX IF NOT EXISTS idx_job_artifacts_job_id ON job_artifacts(job_id);
CREATE INDEX IF NOT EXISTS idx_job_artifacts_type ON job_artifacts(artifact_type);
CREATE INDEX IF NOT EXISTS idx_job_artifacts_created_at ON job_artifacts(created_at DESC);

-- Comment: Artifact types include:
-- 'reference_image' - Input reference images for style analysis
-- 'generated_image' - Output generated images
-- 'prompt' - Text prompts used for generation
-- 'alt_text' - Generated alt text for images
-- 'workflow' - ComfyUI workflow JSON
-- 'log' - Execution logs

-- +goose Down
DROP TABLE IF EXISTS job_artifacts;
