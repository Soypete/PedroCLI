CREATE TYPE tts_source_type AS ENUM ('artifact', 'doc_raw');
CREATE TYPE tts_status AS ENUM ('pending', 'processing', 'done', 'error');

CREATE TABLE tts_jobs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_type         tts_source_type NOT NULL,
    artifact_id         UUID REFERENCES artifacts(id) ON DELETE SET NULL,
    doc_id              UUID REFERENCES docs(id) ON DELETE SET NULL,
    content_key         TEXT NOT NULL DEFAULT '',
    model               TEXT NOT NULL DEFAULT 'qwen-tts',
    voice               TEXT NOT NULL DEFAULT 'default',
    speed               DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    status              tts_status NOT NULL DEFAULT 'pending',
    audio_path          TEXT,
    audio_url           TEXT,
    duration_sec        DOUBLE PRECISION,
    file_size_bytes     BIGINT,
    include_in_podcast  BOOLEAN NOT NULL DEFAULT true,
    podcast_guid        TEXT UNIQUE,
    queued_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_tts_source CHECK (
        (source_type = 'artifact' AND artifact_id IS NOT NULL) OR
        (source_type = 'doc_raw' AND doc_id IS NOT NULL)
    )
);
