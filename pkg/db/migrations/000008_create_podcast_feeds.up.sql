CREATE TABLE podcast_feeds (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    feed_id     UUID NOT NULL UNIQUE REFERENCES feeds(id) ON DELETE CASCADE,
    title       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    author      TEXT NOT NULL DEFAULT 'SoypeteTech Study Engine',
    image_url   TEXT,
    language    TEXT NOT NULL DEFAULT 'en',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_podcast_feeds_updated_at
    BEFORE UPDATE ON podcast_feeds
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
