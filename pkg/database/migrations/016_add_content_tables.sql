-- +goose Up
-- Migration: Add content and content_versions tables for unified storage abstraction
-- Date: 2026-01-17
-- Related: PR #1 Unified Architecture Foundation

-- Content table stores all agent-generated content (blog, podcast, code)
CREATE TABLE IF NOT EXISTS content (
    id UUID PRIMARY KEY,
    type VARCHAR(50) NOT NULL,  -- 'blog', 'podcast', 'code'
    status VARCHAR(50) NOT NULL,  -- 'draft', 'in_progress', 'review', 'published'
    title TEXT NOT NULL,
    data JSONB NOT NULL DEFAULT '{}',  -- Flexible schema per content type
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_content_type ON content(type);
CREATE INDEX IF NOT EXISTS idx_content_status ON content(status);
CREATE INDEX IF NOT EXISTS idx_content_created_at ON content(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_content_type_status ON content(type, status);

-- JSONB GIN index for flexible data queries
CREATE INDEX IF NOT EXISTS idx_content_data_gin ON content USING GIN (data);

-- Content versions table stores phase snapshots
CREATE TABLE IF NOT EXISTS content_versions (
    id UUID PRIMARY KEY,
    content_id UUID NOT NULL REFERENCES content(id) ON DELETE CASCADE,
    phase VARCHAR(100) NOT NULL,  -- Phase name (e.g., "Outline", "Generate Sections")
    version_num INT NOT NULL,  -- Sequential version number
    snapshot JSONB NOT NULL DEFAULT '{}',  -- Phase-specific data snapshot
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Ensure unique version numbers per content
    UNIQUE(content_id, version_num)
);

-- Indexes for version queries
CREATE INDEX IF NOT EXISTS idx_content_versions_content_id ON content_versions(content_id);
CREATE INDEX IF NOT EXISTS idx_content_versions_phase ON content_versions(phase);
CREATE INDEX IF NOT EXISTS idx_content_versions_created_at ON content_versions(created_at DESC);

-- Comments for documentation
COMMENT ON TABLE content IS 'Stores all agent-generated content with flexible JSONB schema';
COMMENT ON COLUMN content.type IS 'Content type: blog, podcast, code';
COMMENT ON COLUMN content.status IS 'Workflow status: draft, in_progress, review, published';
COMMENT ON COLUMN content.data IS 'Flexible JSONB field for type-specific data';

COMMENT ON TABLE content_versions IS 'Stores version snapshots at each workflow phase';
COMMENT ON COLUMN content_versions.phase IS 'Workflow phase name (e.g., Outline, Sections)';
COMMENT ON COLUMN content_versions.version_num IS 'Sequential version number within content';
COMMENT ON COLUMN content_versions.snapshot IS 'Phase-specific data snapshot in JSONB';

-- +goose Down
DROP TABLE IF EXISTS content_versions;
DROP TABLE IF EXISTS content;
