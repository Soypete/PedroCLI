CREATE TYPE artifact_type AS ENUM ('summary', 'faq', 'study_guide', 'timeline', 'digest');

CREATE TABLE artifacts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    doc_id          UUID REFERENCES docs(id) ON DELETE CASCADE,
    feed_id         UUID REFERENCES feeds(id) ON DELETE CASCADE,
    artifact_type   artifact_type NOT NULL,
    prompt_version  TEXT NOT NULL DEFAULT 'v1',
    content         JSONB NOT NULL DEFAULT '{}',
    model           TEXT NOT NULL DEFAULT '',
    input_tokens    INT NOT NULL DEFAULT 0,
    output_tokens   INT NOT NULL DEFAULT 0,
    generated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_artifacts_doc_type_version
    ON artifacts (doc_id, artifact_type, prompt_version) NULLS NOT DISTINCT;
CREATE INDEX idx_artifacts_content ON artifacts USING GIN (content);
