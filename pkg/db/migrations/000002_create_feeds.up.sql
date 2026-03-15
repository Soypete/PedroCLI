CREATE TABLE feeds (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    url         TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    feed_type   TEXT NOT NULL DEFAULT 'rss' CHECK (feed_type IN ('rss', 'atom')),
    content_type TEXT NOT NULL DEFAULT 'text' CHECK (content_type IN ('text', 'audio', 'mixed')),
    poll_interval INTERVAL NOT NULL DEFAULT '15 minutes',
    last_polled_at TIMESTAMPTZ,
    last_error  TEXT,
    podcast_enabled BOOLEAN NOT NULL DEFAULT false,
    podcast_slug TEXT UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_feeds_url ON feeds (url);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_feeds_updated_at
    BEFORE UPDATE ON feeds
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
