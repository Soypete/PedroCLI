-- +goose Up
-- Migration: 010_update_jobs_schema
-- Description: Add missing columns to jobs table for full job tracking

-- Add description column for human-readable job description
ALTER TABLE jobs ADD COLUMN description TEXT;

-- Add work_dir for job working directory
ALTER TABLE jobs ADD COLUMN work_dir VARCHAR(512);

-- Add context_dir for LLM context directory
ALTER TABLE jobs ADD COLUMN context_dir VARCHAR(512);

-- Store all prompts, responses, tool calls for debugging and replay
-- Format: [{"role": "user"|"assistant"|"tool_call"|"tool_result", "content": "...", "timestamp": "..."}]
ALTER TABLE jobs ADD COLUMN conversation_history TEXT DEFAULT '[]';

-- +goose Down

ALTER TABLE jobs DROP COLUMN conversation_history;
ALTER TABLE jobs DROP COLUMN context_dir;
ALTER TABLE jobs DROP COLUMN work_dir;
ALTER TABLE jobs DROP COLUMN description;
