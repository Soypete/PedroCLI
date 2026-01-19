-- +goose Up
-- Migration: 017_add_workspace_fields
-- Description: Add workspace_dir and work_dir fields to jobs table for HTTP Bridge workspace isolation

ALTER TABLE jobs ADD COLUMN IF NOT EXISTS work_dir TEXT;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS workspace_dir TEXT;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS context_dir TEXT;

-- Comment: work_dir is the main repository directory
-- workspace_dir is the isolated workspace directory for HTTP Bridge jobs (if not null)
-- context_dir is the LLM conversation storage directory

-- +goose Down
ALTER TABLE jobs DROP COLUMN IF EXISTS context_dir;
ALTER TABLE jobs DROP COLUMN IF EXISTS workspace_dir;
ALTER TABLE jobs DROP COLUMN IF EXISTS work_dir;
