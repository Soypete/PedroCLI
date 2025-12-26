-- Newsletter assets (videos, events, links)
CREATE TABLE newsletter_assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_type VARCHAR(50) NOT NULL, -- video, event, meetup, link, reading
    title VARCHAR(500) NOT NULL,
    url VARCHAR(500),
    event_date TIMESTAMP,
    description TEXT,
    embed_code TEXT, -- for YouTube/Twitch embeds
    used_in_post UUID REFERENCES blog_posts(id) ON DELETE SET NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Index for querying by asset type
CREATE INDEX idx_newsletter_assets_type ON newsletter_assets(asset_type);

-- Index for querying unused assets
CREATE INDEX idx_newsletter_assets_unused ON newsletter_assets(used_in_post) WHERE used_in_post IS NULL;

-- Index for upcoming events
CREATE INDEX idx_newsletter_assets_event_date ON newsletter_assets(event_date) WHERE event_date IS NOT NULL;
