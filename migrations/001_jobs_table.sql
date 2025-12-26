-- Migration: 001_jobs_table
-- Description: Create jobs table for persistent job storage
-- Created: 2025-01-01

-- Enable UUID extension if not exists (PostgreSQL)
-- For SQLite, UUIDs are stored as TEXT

CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY,
    job_type VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    input_payload JSONB,
    output_payload JSONB,
    model_used VARCHAR(200),
    hardware_target VARCHAR(50),
    error_message TEXT,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_job_type ON jobs(job_type);
CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_hardware_target ON jobs(hardware_target);

-- Comment: Job types include 'builder', 'debugger', 'reviewer', 'triager',
-- 'image_generation', 'image_analysis', 'alt_text_generation', 'blog_workflow'
-- Status values: 'pending', 'running', 'completed', 'failed', 'cancelled'
