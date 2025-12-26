-- Blog posts pipeline table
CREATE TABLE blog_posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(500),
    status VARCHAR(50) NOT NULL DEFAULT 'dictated', -- dictated, drafted, edited, published, public
    raw_transcription TEXT,
    transcription_duration_seconds INTEGER,
    writer_output TEXT,
    editor_output TEXT,
    final_content TEXT,
    newsletter_addendum JSONB,
    notion_page_id VARCHAR(100),
    substack_url VARCHAR(500),
    paywall_until TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Index for querying by status
CREATE INDEX idx_blog_posts_status ON blog_posts(status);

-- Index for querying by created_at
CREATE INDEX idx_blog_posts_created_at ON blog_posts(created_at DESC);

-- Index for paywall expiration queries
CREATE INDEX idx_blog_posts_paywall_until ON blog_posts(paywall_until) WHERE paywall_until IS NOT NULL;

-- Trigger to automatically update updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_blog_posts_updated_at BEFORE UPDATE ON blog_posts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
