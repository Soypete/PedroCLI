CREATE TYPE ingest_status AS ENUM ('pending', 'processing', 'done', 'error');

CREATE TABLE docs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    feed_id         UUID NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
    guid            TEXT NOT NULL,
    source_url      TEXT NOT NULL DEFAULT '',
    content_hash    TEXT NOT NULL,
    title           TEXT NOT NULL DEFAULT '',
    author          TEXT NOT NULL DEFAULT '',
    published_at    TIMESTAMPTZ,
    raw_content     TEXT NOT NULL DEFAULT '',
    content_type    TEXT NOT NULL DEFAULT 'text',
    meta            JSONB NOT NULL DEFAULT '{}',
    version         INT NOT NULL DEFAULT 1,
    superseded_by   UUID REFERENCES docs(id),
    is_latest       BOOLEAN NOT NULL DEFAULT true,
    ingest_status   ingest_status NOT NULL DEFAULT 'pending',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_docs_content_hash ON docs (content_hash);
CREATE UNIQUE INDEX idx_docs_feed_guid_latest ON docs (feed_id, guid, is_latest) NULLS NOT DISTINCT;
CREATE INDEX idx_docs_meta ON docs USING GIN (meta);
CREATE INDEX idx_docs_fts ON docs USING GIN (to_tsvector('english', coalesce(title, '') || ' ' || coalesce(raw_content, '')));

CREATE TRIGGER trg_docs_updated_at
    BEFORE UPDATE ON docs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
