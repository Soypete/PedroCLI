-- +goose Up
-- Migration: 010_update_jobs_schema
-- Description: Add missing columns to jobs table for full job tracking
-- Note: SQLite doesn't support IF NOT EXISTS with ALTER TABLE, so this migration
-- assumes a fresh run (goose tracks which migrations have run)

-- Add description column for human-readable job description
ALTER TABLE jobs ADD COLUMN description TEXT;

-- Add work_dir for job working directory
ALTER TABLE jobs ADD COLUMN work_dir VARCHAR(512);

-- Add context_dir for LLM context directory
ALTER TABLE jobs ADD COLUMN context_dir VARCHAR(512);

-- Store all prompts, responses, tool calls for debugging and replay
-- Format: [{"role": "user"|"assistant"|"tool_call"|"tool_result", "content": "...", "timestamp": "..."}]
-- SQLite TEXT with JSON content (JSONB not supported in SQLite)
ALTER TABLE jobs ADD COLUMN conversation_history TEXT DEFAULT '[]';

-- +goose Down
-- Note: SQLite doesn't support DROP COLUMN in older versions,
-- but modern SQLite (3.35+) does. This may fail on older SQLite.
-- For production, consider recreating the table without these columns.

ALTER TABLE jobs DROP COLUMN conversation_history;
ALTER TABLE jobs DROP COLUMN context_dir;
ALTER TABLE jobs DROP COLUMN work_dir;
ALTER TABLE jobs DROP COLUMN description;
