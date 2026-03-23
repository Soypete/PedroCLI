CREATE TABLE chunks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    doc_id      UUID NOT NULL REFERENCES docs(id) ON DELETE CASCADE,
    chunk_index INT NOT NULL,
    chunk_hash  TEXT NOT NULL UNIQUE,
    text        TEXT NOT NULL DEFAULT '',
    token_count INT NOT NULL DEFAULT 0,
    start_time  DOUBLE PRECISION,
    end_time    DOUBLE PRECISION,
    tsv         TSVECTOR GENERATED ALWAYS AS (to_tsvector('english', text)) STORED,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_chunks_tsv ON chunks USING GIN (tsv);
CREATE UNIQUE INDEX idx_chunks_doc_index ON chunks (doc_id, chunk_index);
