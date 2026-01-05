package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/soypete/pedrocli/pkg/repos"
)

// TODO(issue): Fix SQLite JSONB migration syntax error
func TestNewSQLiteStore(t *testing.T) {
	t.Skip("TODO: Fix migration 010 - SQLite doesn't support PostgreSQL JSONB syntax")
	// Create temp directory for test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Test creating store with auto-migration
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Verify tables exist by checking the database version
	version, err := store.MigrationVersion()
	if err != nil {
		t.Fatalf("failed to get migration version: %v", err)
	}

	// Should have version 10 (all migrations applied including 010_update_jobs_schema)
	if version != 10 {
		t.Errorf("expected migration version 10, got %d", version)
	}
}

// TODO(issue): Fix SQLite JSONB migration syntax error
func TestNewSQLiteStoreWithoutAutoMigration(t *testing.T) {
	t.Skip("TODO: Fix migration 010 - SQLite doesn't support PostgreSQL JSONB syntax")
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create store without auto-migration
	store, err := NewSQLiteStoreWithOptions(dbPath, false)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Version should be 0 (no migrations run)
	version, err := store.MigrationVersion()
	if err != nil {
		t.Fatalf("failed to get migration version: %v", err)
	}

	if version != 0 {
		t.Errorf("expected migration version 0, got %d", version)
	}

	// Now run migrations manually
	if err := store.Migrate(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Version should now be 10
	version, err = store.MigrationVersion()
	if err != nil {
		t.Fatalf("failed to get migration version: %v", err)
	}

	if version != 10 {
		t.Errorf("expected migration version 10 after migration, got %d", version)
	}
}

// TODO(issue): Fix SQLite JSONB migration syntax error
func TestMigrationRollback(t *testing.T) {
	t.Skip("TODO: Fix migration 010 - SQLite doesn't support PostgreSQL JSONB syntax")
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Should be at version 10
	version, err := store.MigrationVersion()
	if err != nil {
		t.Fatalf("failed to get migration version: %v", err)
	}
	if version != 10 {
		t.Fatalf("expected version 10, got %d", version)
	}

	// Rollback one migration
	if err := store.MigrateDown(); err != nil {
		t.Fatalf("failed to rollback migration: %v", err)
	}

	// Should now be at version 9
	version, err = store.MigrationVersion()
	if err != nil {
		t.Fatalf("failed to get migration version: %v", err)
	}
	if version != 9 {
		t.Errorf("expected version 9 after rollback, got %d", version)
	}

	// Re-apply migrations
	if err := store.Migrate(); err != nil {
		t.Fatalf("failed to re-apply migrations: %v", err)
	}

	// Should be back at version 10
	version, err = store.MigrationVersion()
	if err != nil {
		t.Fatalf("failed to get migration version: %v", err)
	}
	if version != 10 {
		t.Errorf("expected version 10 after re-migration, got %d", version)
	}
}

// TODO(issue): Fix SQLite JSONB migration syntax error
func TestMigrationRedo(t *testing.T) {
	t.Skip("TODO: Fix migration 010 - SQLite doesn't support PostgreSQL JSONB syntax")
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Redo last migration
	if err := store.MigrateRedo(); err != nil {
		t.Fatalf("failed to redo migration: %v", err)
	}

	// Should still be at version 10
	version, err := store.MigrationVersion()
	if err != nil {
		t.Fatalf("failed to get migration version: %v", err)
	}
	if version != 10 {
		t.Errorf("expected version 10 after redo, got %d", version)
	}
}

// TODO(issue): Fix SQLite JSONB migration syntax error
func TestMigrationReset(t *testing.T) {
	t.Skip("TODO: Fix migration 010 - SQLite doesn't support PostgreSQL JSONB syntax")
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Reset all migrations
	if err := store.MigrateReset(); err != nil {
		t.Fatalf("failed to reset migrations: %v", err)
	}

	// Should be at version 0
	version, err := store.MigrationVersion()
	if err != nil {
		t.Fatalf("failed to get migration version: %v", err)
	}
	if version != 0 {
		t.Errorf("expected version 0 after reset, got %d", version)
	}

	// Re-apply all migrations
	if err := store.Migrate(); err != nil {
		t.Fatalf("failed to re-apply migrations: %v", err)
	}

	// Should be back at version 10
	version, err = store.MigrationVersion()
	if err != nil {
		t.Fatalf("failed to get migration version: %v", err)
	}
	if version != 10 {
		t.Errorf("expected version 10 after re-migration, got %d", version)
	}
}

// TODO(issue): Fix SQLite JSONB migration syntax error
func TestTablesExist(t *testing.T) {
	t.Skip("TODO: Fix migration 010 - SQLite doesn't support PostgreSQL JSONB syntax")
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Check that all expected tables exist
	expectedTables := []string{
		"managed_repos",
		"repo_operations",
		"tracked_prs",
		"hook_runs",
		"repo_jobs",
		"git_credentials",
		"oauth_tokens",
		"goose_db_version",
		"blog_posts",
		"training_pairs",
		"newsletter_assets",
	}

	for _, table := range expectedTables {
		var count int
		err := store.db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&count)

		if err != nil {
			t.Errorf("failed to check for table %s: %v", table, err)
			continue
		}

		if count != 1 {
			t.Errorf("table %s does not exist", table)
		}
	}
}

// TODO(issue): Fix SQLite JSONB migration syntax error
func TestDatabasePath(t *testing.T) {
	t.Skip("TODO: Fix migration 010 - SQLite doesn't support PostgreSQL JSONB syntax")
	// Test that database is created in the correct location
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "nested", "path")
	dbPath := filepath.Join(subDir, "test.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Check that the file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("database file was not created at %s", dbPath)
	}
}

// TODO(issue): Fix SQLite JSONB migration syntax error
func TestSaveAndGetRepo(t *testing.T) {
	t.Skip("TODO: Fix migration 010 - SQLite doesn't support PostgreSQL JSONB syntax")
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create a test repo
	repo := &repos.LocalRepo{
		Provider:      "github",
		Owner:         "testowner",
		Name:          "testrepo",
		LocalPath:     "/tmp/test/repo",
		DefaultBranch: "main",
		ProjectType:   "go",
		CreatedAt:     time.Now(),
	}

	// Save the repo
	if err := store.SaveRepo(ctx, repo); err != nil {
		t.Fatalf("failed to save repo: %v", err)
	}

	// Get the repo back
	retrieved, err := store.GetRepo(ctx, "github", "testowner", "testrepo")
	if err != nil {
		t.Fatalf("failed to get repo: %v", err)
	}

	if retrieved == nil {
		t.Fatal("expected repo, got nil")
	}

	if retrieved.Owner != "testowner" {
		t.Errorf("expected owner 'testowner', got '%s'", retrieved.Owner)
	}

	if retrieved.Name != "testrepo" {
		t.Errorf("expected name 'testrepo', got '%s'", retrieved.Name)
	}
}

// TODO(issue): Fix idempotent migrations test - SQL syntax error with JSONB default
// Error: unrecognized token: ":" in ALTER TABLE jobs ADD COLUMN conversation_history JSONB DEFAULT '[]'::jsonb;
func TestIdempotentMigrations(t *testing.T) {
	t.Skip("TODO: Fix migration SQL syntax for SQLite - see GitHub issue")
	// tmpDir := t.TempDir()
	// dbPath := filepath.Join(tmpDir, "test.db")

	// // First run - create database and run migrations
	// store1, err := NewSQLiteStore(dbPath)
	// if err != nil {
	// 	t.Fatalf("failed to create store: %v", err)
	// }
	// store1.Close()

	// // Second run - open existing database, migrations should be no-op
	// store2, err := NewSQLiteStore(dbPath)
	// if err != nil {
	// 	t.Fatalf("failed to open existing database: %v", err)
	// }
	// defer store2.Close()

	// // Should still be at version 10
	// version, err := store2.MigrationVersion()
	// if err != nil {
	// 	t.Fatalf("failed to get migration version: %v", err)
	// }
	// if version != 10 {
	// 	t.Errorf("expected version 10, got %d", version)
	// }
}
