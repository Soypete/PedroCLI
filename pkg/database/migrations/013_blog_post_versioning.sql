-- +goose Up
-- Create blog_post_versions table for version history
CREATE TABLE IF NOT EXISTS blog_post_versions (
    id TEXT PRIMARY KEY,
    post_id TEXT NOT NULL REFERENCES blog_posts(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    version_type TEXT NOT NULL, -- 'auto_snapshot', 'manual_save', 'phase_result'
    status TEXT NOT NULL, -- Status at time of snapshot
    phase TEXT, -- Phase that created this version (if applicable)

    -- Content at this version
    post_title TEXT, -- The blog post title at this version
    title TEXT, -- Section title (if applicable)
    raw_transcription TEXT,
    outline TEXT,
    sections TEXT, -- JSON array of section objects: [{title, content, order}]
    full_content TEXT,

    -- Metadata
    created_by TEXT DEFAULT 'system',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_notes TEXT, -- User-provided notes for manual saves

    UNIQUE(post_id, version_number)
);

-- Indexes for querying versions
CREATE INDEX IF NOT EXISTS idx_blog_post_versions_post_id ON blog_post_versions(post_id);
CREATE INDEX IF NOT EXISTS idx_blog_post_versions_created_at ON blog_post_versions(created_at);
CREATE INDEX IF NOT EXISTS idx_blog_post_versions_type ON blog_post_versions(version_type);

-- Add version tracking to blog_posts table
ALTER TABLE blog_posts ADD COLUMN IF NOT EXISTS current_version INTEGER DEFAULT 1;

-- +goose Down
-- Remove version tracking from blog_posts
ALTER TABLE blog_posts DROP COLUMN IF EXISTS current_version;

-- Drop indexes
DROP INDEX IF EXISTS idx_blog_post_versions_type;
DROP INDEX IF EXISTS idx_blog_post_versions_created_at;
DROP INDEX IF EXISTS idx_blog_post_versions_post_id;

-- Drop versions table
DROP TABLE IF EXISTS blog_post_versions;
