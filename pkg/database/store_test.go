package database

import (
	"testing"
)

// NOTE: SQLite support is deprecated in favor of PostgreSQL.
// These tests are skipped because migrations now use PostgreSQL-specific features (JSONB).
// Use PostgreSQL for all database operations.

func TestNewSQLiteStore(t *testing.T) {
	t.Skip("SQLite is deprecated: migrations use PostgreSQL-specific JSONB type")
}

func TestNewSQLiteStoreWithoutAutoMigration(t *testing.T) {
	t.Skip("SQLite is deprecated: migrations use PostgreSQL-specific JSONB type")
}

func TestMigrationRollback(t *testing.T) {
	t.Skip("SQLite is deprecated: migrations use PostgreSQL-specific JSONB type")
}

func TestMigrationRedo(t *testing.T) {
	t.Skip("SQLite is deprecated: migrations use PostgreSQL-specific JSONB type")
}

func TestMigrationReset(t *testing.T) {
	t.Skip("SQLite is deprecated: migrations use PostgreSQL-specific JSONB type")
}

func TestTablesExist(t *testing.T) {
	t.Skip("SQLite is deprecated: migrations use PostgreSQL-specific JSONB type")
}

func TestDatabasePath(t *testing.T) {
	t.Skip("SQLite is deprecated: migrations use PostgreSQL-specific JSONB type")
}

func TestSaveAndGetRepo(t *testing.T) {
	t.Skip("SQLite is deprecated: migrations use PostgreSQL-specific JSONB type")
}

func TestIdempotentMigrations(t *testing.T) {
	t.Skip("SQLite is deprecated: migrations use PostgreSQL-specific JSONB type")
}
