-- +goose Up
-- OAuth tokens table for Notion and Google Calendar integration

CREATE TABLE IF NOT EXISTS oauth_tokens (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,              -- "notion", "google", "github"
    service TEXT NOT NULL,               -- "database", "calendar", etc.
    access_token TEXT NOT NULL,          -- The actual token (TODO: encrypt at rest)
    refresh_token TEXT,                  -- For OAuth2 refresh flow (NULL for API keys)
    token_type TEXT DEFAULT 'Bearer',    -- "Bearer", "Basic", etc.
    scope TEXT,                          -- Space-separated OAuth scopes
    expires_at TIMESTAMP,                 -- When token expires (NULL for non-expiring keys)
    last_refreshed TIMESTAMP,             -- When we last refreshed
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider, service)            -- One token per provider+service combo
);

CREATE INDEX IF NOT EXISTS idx_oauth_tokens_provider
    ON oauth_tokens(provider);

CREATE INDEX IF NOT EXISTS idx_oauth_tokens_expires_at
    ON oauth_tokens(expires_at);

-- +goose Down
DROP INDEX IF EXISTS idx_oauth_tokens_expires_at;
DROP INDEX IF EXISTS idx_oauth_tokens_provider;
DROP TABLE IF EXISTS oauth_tokens;
