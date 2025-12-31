-- +goose Up
-- Blog posts pipeline table
CREATE TABLE IF NOT EXISTS blog_posts (
    id TEXT PRIMARY KEY,
    title TEXT,
    status TEXT NOT NULL DEFAULT 'dictated', -- dictated, drafted, edited, published, public
    raw_transcription TEXT,
    transcription_duration_seconds INTEGER,
    writer_output TEXT,
    editor_output TEXT,
    final_content TEXT,
    newsletter_addendum TEXT, -- JSON blob
    notion_page_id TEXT,
    substack_url TEXT,
    paywall_until DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for querying by status
CREATE INDEX IF NOT EXISTS idx_blog_posts_status ON blog_posts(status);

-- Index for querying by created_at
CREATE INDEX IF NOT EXISTS idx_blog_posts_created_at ON blog_posts(created_at);

-- Index for paywall expiration queries
CREATE INDEX IF NOT EXISTS idx_blog_posts_paywall_until ON blog_posts(paywall_until);

-- +goose Down
DROP INDEX IF EXISTS idx_blog_posts_paywall_until;
DROP INDEX IF EXISTS idx_blog_posts_created_at;
DROP INDEX IF EXISTS idx_blog_posts_status;
DROP TABLE IF EXISTS blog_posts;
