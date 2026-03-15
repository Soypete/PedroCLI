DROP TRIGGER IF EXISTS trg_study_jobs_notify ON study_jobs;
DROP FUNCTION IF EXISTS notify_job_ready();
DROP TRIGGER IF EXISTS trg_study_jobs_updated_at ON study_jobs;
DROP TABLE IF EXISTS study_jobs;
DROP TYPE IF EXISTS study_job_status;
DROP TYPE IF EXISTS study_job_type;
