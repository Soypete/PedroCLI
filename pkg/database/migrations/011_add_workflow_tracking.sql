-- Migration: Add workflow/phase tracking to jobs table
-- This enables phased agent execution with explicit phase tracking

-- Add workflow tracking columns
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS workflow_type VARCHAR(50);
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS current_phase VARCHAR(50);
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS phase_results JSONB DEFAULT '{}';
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS plan JSONB;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS phase_started_at TIMESTAMP;

-- Add index for querying jobs by workflow type and phase
CREATE INDEX IF NOT EXISTS idx_jobs_workflow_type ON jobs(workflow_type);
CREATE INDEX IF NOT EXISTS idx_jobs_current_phase ON jobs(current_phase);

-- Comment on columns for documentation
COMMENT ON COLUMN jobs.workflow_type IS 'Type of workflow: builder_phased, reviewer_phased, debugger_phased, etc.';
COMMENT ON COLUMN jobs.current_phase IS 'Current phase name: analyze, plan, implement, validate, deliver, etc.';
COMMENT ON COLUMN jobs.phase_results IS 'JSON object storing results from each completed phase';
COMMENT ON COLUMN jobs.plan IS 'Implementation plan from planning phase (for builder agents)';
COMMENT ON COLUMN jobs.phase_started_at IS 'Timestamp when current phase started';
