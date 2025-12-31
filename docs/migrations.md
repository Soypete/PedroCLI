# Database Migrations

PedroCLI uses [goose](https://github.com/pressly/goose) for database migrations.

## Overview

Migrations are SQL files stored in `pkg/database/migrations/`. They run automatically when the database store is initialized, ensuring the database schema is always up to date.

## Migration Files

Migration files follow the naming convention:
```
{version}_{name}.sql
```

Example: `001_initial_schema.sql`, `002_oauth_tokens.sql`

Each file contains two sections:

```sql
-- +goose Up
-- SQL to apply the migration

-- +goose Down
-- SQL to rollback the migration
```

## Commands

### Run Migrations

```bash
# Run all pending migrations
pedrocli migrate up

# Or via make
make migrate-up
```

### Rollback

```bash
# Rollback last migration
pedrocli migrate down

# Rollback all migrations
pedrocli migrate reset

# Rollback and re-run last migration
pedrocli migrate redo
```

### Check Status

```bash
pedrocli migrate status
```

Output shows which migrations have been applied:

```
    Applied At                  Migration
    =======================================
    Mon Jan 15 10:30:00 2024 -- 001_initial_schema.sql
    Mon Jan 15 10:30:01 2024 -- 002_oauth_tokens.sql
    Pending                  -- 003_new_feature.sql
```

### Check Version

```bash
pedrocli migrate version
```

## Auto-Migration on Startup

By default, migrations run automatically when the SQLiteStore is initialized. This happens when the HTTP server starts or when any component uses the database.

To disable auto-migration and control it manually:

```go
// Create store without auto-migration
store, err := database.NewSQLiteStoreWithOptions(dbPath, false)
if err != nil {
    return err
}

// Manually run migrations when ready
if err := store.Migrate(); err != nil {
    return err
}
```

## Writing Migrations

### Best Practices

1. **Always write Down migrations** - Enable rollback for every Up migration

2. **Use IF NOT EXISTS / IF EXISTS** - Make migrations idempotent
   ```sql
   CREATE TABLE IF NOT EXISTS users (...);
   DROP TABLE IF EXISTS users;
   ```

3. **One logical change per migration** - Don't combine unrelated changes

4. **Never modify existing migrations** - Create new migrations instead

5. **Test rollbacks** - Always test `down` before merging

### Example Migration

```sql
-- pkg/database/migrations/003_add_users_table.sql

-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    name TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- +goose Down
DROP INDEX IF EXISTS idx_users_email;
DROP TABLE IF EXISTS users;
```

### Adding Columns

```sql
-- +goose Up
ALTER TABLE users ADD COLUMN avatar_url TEXT;

-- +goose Down
-- Note: SQLite has limited ALTER TABLE support
-- For SQLite, dropping columns requires recreating the table
```

### Data Migrations

For migrations that modify data:

```sql
-- +goose Up
UPDATE settings SET value = 'new_default' WHERE key = 'theme' AND value = '';

-- +goose Down
UPDATE settings SET value = '' WHERE key = 'theme' AND value = 'new_default';
```

## Current Migrations

### 001_initial_schema.sql

Creates the core tables:
- `managed_repos` - Managed repository information
- `repo_operations` - Repository operation history
- `tracked_prs` - Tracked pull requests
- `hook_runs` - Hook execution records
- `repo_jobs` - Agent job records
- `git_credentials` - Git credential storage

### 002_oauth_tokens.sql

Adds OAuth token storage:
- `oauth_tokens` - OAuth tokens for external services (Notion, Google Calendar, etc.)

## Troubleshooting

### Migration Failed

If a migration fails:

1. Check the error message
2. Fix the migration SQL
3. Run `pedrocli migrate redo` to retry

### Schema Mismatch

If database schema doesn't match migrations:

```bash
# Check current state
pedrocli migrate status

# For development, reset completely
make db-fresh
```

### Dirty Database

If goose reports "dirty database state":

```bash
# Check goose_db_version table
sqlite3 /var/pedro/repos/pedro.db "SELECT * FROM goose_db_version;"

# Manually mark as clean (use with caution)
sqlite3 /var/pedro/repos/pedro.db "UPDATE goose_db_version SET dirty = false WHERE version_id = X;"
```

## Development Workflow

### Adding a New Migration

1. Create a new SQL file in `pkg/database/migrations/`:
   ```
   003_add_new_feature.sql
   ```

2. Add Up and Down sections:
   ```sql
   -- +goose Up
   CREATE TABLE ...

   -- +goose Down
   DROP TABLE ...
   ```

3. Test the migration:
   ```bash
   make migrate-up
   make migrate-down
   make migrate-up
   ```

4. Test rollback:
   ```bash
   make migrate-redo
   ```

### Embedded Migrations

Migrations are embedded into the binary using Go's `embed` package. This means:

- No external migration files needed at runtime
- Migrations are versioned with the code
- Binary is self-contained

The embedding is done in `pkg/database/store.go`:

```go
//go:embed migrations/*.sql
var embedMigrations embed.FS
```
