// Package database provides SQLite-based persistent storage for repository management.
package database

// Schema contains the SQL schema for the repository management database
const Schema = `
-- Managed repositories
CREATE TABLE IF NOT EXISTS managed_repos (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    owner TEXT NOT NULL,
    repo_name TEXT NOT NULL,
    local_path TEXT NOT NULL,
    default_branch TEXT DEFAULT 'main',
    project_type TEXT DEFAULT 'unknown',
    hooks_config TEXT, -- JSON blob
    last_fetched DATETIME,
    last_operation TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider, owner, repo_name)
);

-- Index for common queries
CREATE INDEX IF NOT EXISTS idx_managed_repos_provider_owner
    ON managed_repos(provider, owner);

CREATE INDEX IF NOT EXISTS idx_managed_repos_local_path
    ON managed_repos(local_path);

-- Repository operations history
CREATE TABLE IF NOT EXISTS repo_operations (
    id TEXT PRIMARY KEY,
    repo_id TEXT NOT NULL REFERENCES managed_repos(id) ON DELETE CASCADE,
    operation_type TEXT NOT NULL,
    ref_before TEXT,
    ref_after TEXT,
    details TEXT, -- JSON blob
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_repo_operations_repo_id
    ON repo_operations(repo_id);

CREATE INDEX IF NOT EXISTS idx_repo_operations_created_at
    ON repo_operations(created_at);

-- Tracked pull requests
CREATE TABLE IF NOT EXISTS tracked_prs (
    id TEXT PRIMARY KEY,
    repo_id TEXT NOT NULL REFERENCES managed_repos(id) ON DELETE CASCADE,
    pr_number INTEGER NOT NULL,
    branch_name TEXT NOT NULL,
    base_branch TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT,
    status TEXT NOT NULL DEFAULT 'open',
    local_commit_hash TEXT,
    remote_commit_hash TEXT,
    html_url TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    merged_at DATETIME,
    UNIQUE(repo_id, pr_number)
);

CREATE INDEX IF NOT EXISTS idx_tracked_prs_repo_id
    ON tracked_prs(repo_id);

CREATE INDEX IF NOT EXISTS idx_tracked_prs_status
    ON tracked_prs(status);

-- Hook execution runs
CREATE TABLE IF NOT EXISTS hook_runs (
    id TEXT PRIMARY KEY,
    repo_id TEXT NOT NULL REFERENCES managed_repos(id) ON DELETE CASCADE,
    hook_type TEXT NOT NULL,
    triggered_by TEXT NOT NULL,
    passed INTEGER NOT NULL DEFAULT 0,
    results TEXT, -- JSON blob
    agent_feedback TEXT,
    duration_ms INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_hook_runs_repo_id
    ON hook_runs(repo_id);

CREATE INDEX IF NOT EXISTS idx_hook_runs_created_at
    ON hook_runs(created_at);

-- Repository jobs (agent tasks on repos)
CREATE TABLE IF NOT EXISTS repo_jobs (
    id TEXT PRIMARY KEY,
    repo_id TEXT NOT NULL REFERENCES managed_repos(id) ON DELETE CASCADE,
    job_type TEXT NOT NULL,
    branch_name TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    input_payload TEXT, -- JSON blob
    output_payload TEXT, -- JSON blob
    validation_attempts INTEGER DEFAULT 0,
    last_validation_result TEXT, -- JSON blob
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_repo_jobs_repo_id
    ON repo_jobs(repo_id);

CREATE INDEX IF NOT EXISTS idx_repo_jobs_status
    ON repo_jobs(status);

-- Git credentials (encrypted tokens stored separately)
CREATE TABLE IF NOT EXISTS git_credentials (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL UNIQUE,
    credential_type TEXT NOT NULL,
    ssh_key_path TEXT,
    username TEXT,
    token_hash TEXT, -- For validation, not the actual token
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

// MigrationV1 contains the initial migration
const MigrationV1 = Schema

// MigrationV2 adds OAuth tokens table for Notion and Google Calendar integration
const MigrationV2 = `
-- OAuth tokens for external service integrations
CREATE TABLE IF NOT EXISTS oauth_tokens (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,              -- "notion", "google", "github"
    service TEXT NOT NULL,               -- "database", "calendar", etc.
    access_token TEXT NOT NULL,          -- The actual token (TODO: encrypt at rest)
    refresh_token TEXT,                  -- For OAuth2 refresh flow (NULL for API keys)
    token_type TEXT DEFAULT 'Bearer',    -- "Bearer", "Basic", etc.
    scope TEXT,                          -- Space-separated OAuth scopes
    expires_at DATETIME,                 -- When token expires (NULL for non-expiring keys)
    last_refreshed DATETIME,             -- When we last refreshed
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider, service)            -- One token per provider+service combo
);

CREATE INDEX IF NOT EXISTS idx_oauth_tokens_provider
    ON oauth_tokens(provider);

CREATE INDEX IF NOT EXISTS idx_oauth_tokens_expires_at
    ON oauth_tokens(expires_at);
`

// Migrations contains all database migrations in order
var Migrations = []string{
	MigrationV1,
	MigrationV2,
}
