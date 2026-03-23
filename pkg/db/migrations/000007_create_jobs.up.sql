CREATE TYPE study_job_type AS ENUM (
    'ingest_text', 'ingest_audio', 'generate_artifact',
    'generate_tts', 'generate_digest', 'poll_feed'
);
CREATE TYPE study_job_status AS ENUM ('pending', 'processing', 'done', 'error', 'cancelled');

CREATE TABLE study_jobs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_type     study_job_type NOT NULL,
    status       study_job_status NOT NULL DEFAULT 'pending',
    priority     INT NOT NULL DEFAULT 5,
    feed_id      UUID REFERENCES feeds(id) ON DELETE SET NULL,
    doc_id       UUID REFERENCES docs(id) ON DELETE SET NULL,
    artifact_id  UUID REFERENCES artifacts(id) ON DELETE SET NULL,
    tts_job_id   UUID REFERENCES tts_jobs(id) ON DELETE SET NULL,
    payload      JSONB NOT NULL DEFAULT '{}',
    attempts     INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    last_error   TEXT,
    run_after    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_study_jobs_pending_priority
    ON study_jobs (priority ASC, created_at ASC) WHERE status = 'pending';
CREATE INDEX idx_study_jobs_run_after
    ON study_jobs (run_after) WHERE status = 'pending';

CREATE TRIGGER trg_study_jobs_updated_at
    BEFORE UPDATE ON study_jobs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE OR REPLACE FUNCTION notify_job_ready() RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('job_ready', NEW.job_type::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_study_jobs_notify
    AFTER INSERT ON study_jobs
    FOR EACH ROW EXECUTE FUNCTION notify_job_ready();
