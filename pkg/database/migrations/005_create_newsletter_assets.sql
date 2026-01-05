-- +goose Up
-- Newsletter assets (videos, events, links)
CREATE TABLE IF NOT EXISTS newsletter_assets (
    id TEXT PRIMARY KEY,
    asset_type TEXT NOT NULL, -- video, event, meetup, link, reading
    title TEXT NOT NULL,
    url TEXT,
    event_date TIMESTAMP,
    description TEXT,
    embed_code TEXT, -- for YouTube/Twitch embeds
    used_in_post TEXT REFERENCES blog_posts(id) ON DELETE SET NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for querying by asset type
CREATE INDEX IF NOT EXISTS idx_newsletter_assets_type ON newsletter_assets(asset_type);

-- Index for querying unused assets
CREATE INDEX IF NOT EXISTS idx_newsletter_assets_unused ON newsletter_assets(used_in_post);

-- Index for upcoming events
CREATE INDEX IF NOT EXISTS idx_newsletter_assets_event_date ON newsletter_assets(event_date);

-- +goose Down
DROP INDEX IF EXISTS idx_newsletter_assets_event_date;
DROP INDEX IF EXISTS idx_newsletter_assets_unused;
DROP INDEX IF EXISTS idx_newsletter_assets_type;
DROP TABLE IF EXISTS newsletter_assets;
