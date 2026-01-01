// Package database provides database connection and migration management for PedroCLI.
// It supports both PostgreSQL and SQLite backends.
package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"           // PostgreSQL driver
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB represents a database connection with migration support.
type DB struct {
	*sql.DB
	driver   string
	connStr  string
	mu       sync.RWMutex
	migrated bool
}

// Config holds database configuration.
type Config struct {
	Driver   string `json:"driver"`   // "postgres" or "sqlite"
	Host     string `json:"host"`     // PostgreSQL host
	Port     int    `json:"port"`     // PostgreSQL port
	Database string `json:"database"` // Database name or SQLite file path
	User     string `json:"user"`     // PostgreSQL user
	Password string `json:"password"` // PostgreSQL password
	SSLMode  string `json:"ssl_mode"` // PostgreSQL SSL mode
}

// DefaultConfig returns default database configuration using SQLite.
func DefaultConfig() *Config {
	return &Config{
		Driver:   "sqlite",
		Database: "pedrocli.db",
	}
}

// New creates a new database connection.
func New(cfg *Config) (*DB, error) {
	var connStr string
	var driver string

	switch cfg.Driver {
	case "postgres", "postgresql":
		driver = "postgres"
		sslMode := cfg.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		connStr = fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, sslMode,
		)
	case "sqlite", "sqlite3", "":
		driver = "sqlite3"
		connStr = cfg.Database
		if connStr == "" {
			connStr = "pedrocli.db"
		}
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	db, err := sql.Open(driver, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{
		DB:      db,
		driver:  driver,
		connStr: connStr,
	}, nil
}

// Driver returns the database driver name.
func (d *DB) Driver() string {
	return d.driver
}

// Migrate runs all pending database migrations.
func (d *DB) Migrate(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.migrated {
		return nil
	}

	// Create migrations tracking table
	if err := d.createMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get list of migration files
	migrations, err := d.getMigrationFiles()
	if err != nil {
		return fmt.Errorf("failed to list migrations: %w", err)
	}

	// Get already applied migrations
	applied, err := d.getAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Apply pending migrations
	for _, migration := range migrations {
		if applied[migration] {
			continue
		}

		if err := d.applyMigration(ctx, migration); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", migration, err)
		}
	}

	d.migrated = true
	return nil
}

// createMigrationsTable creates the migrations tracking table if it doesn't exist.
func (d *DB) createMigrationsTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := d.ExecContext(ctx, query)
	return err
}

// getMigrationFiles returns a sorted list of migration file names.
func (d *DB) getMigrationFiles() ([]string, error) {
	var migrations []string

	err := fs.WalkDir(migrationsFS, "migrations", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".sql") {
			migrations = append(migrations, entry.Name())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(migrations)
	return migrations, nil
}

// getAppliedMigrations returns a map of already applied migration names.
func (d *DB) getAppliedMigrations(ctx context.Context) (map[string]bool, error) {
	applied := make(map[string]bool)

	rows, err := d.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

// applyMigration applies a single migration file.
func (d *DB) applyMigration(ctx context.Context, filename string) error {
	content, err := migrationsFS.ReadFile(filepath.Join("migrations", filename))
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	// Adapt SQL for SQLite if needed
	sql := string(content)
	if d.driver == "sqlite3" {
		sql = d.adaptSQLForSQLite(sql)
	}

	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Execute migration
	if _, err := tx.ExecContext(ctx, sql); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	// Record migration
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", filename); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return tx.Commit()
}

// adaptSQLForSQLite modifies PostgreSQL SQL to work with SQLite.
func (d *DB) adaptSQLForSQLite(sql string) string {
	// Replace UUID with TEXT
	sql = strings.ReplaceAll(sql, "UUID", "TEXT")
	// Replace JSONB with TEXT (SQLite stores JSON as text)
	sql = strings.ReplaceAll(sql, "JSONB", "TEXT")
	// Replace BIGINT with INTEGER
	sql = strings.ReplaceAll(sql, "BIGINT", "INTEGER")
	// Remove ON DELETE CASCADE (SQLite needs PRAGMA foreign_keys = ON)
	sql = strings.ReplaceAll(sql, "ON DELETE CASCADE", "")
	return sql
}

// NewUUID generates a new UUID.
func NewUUID() string {
	return uuid.New().String()
}

// NullString creates a sql.NullString from a string pointer.
func NullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

// NullTime creates a sql.NullTime from a time.Time pointer.
func NullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

// NullInt64 creates a sql.NullInt64 from an int64 pointer.
func NullInt64(i *int64) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *i, Valid: true}
}
