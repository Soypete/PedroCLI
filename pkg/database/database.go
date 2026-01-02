// Package database provides database connection and migration management for PedroCLI.
// It supports both PostgreSQL and SQLite backends.
package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"           // PostgreSQL driver
	_ "github.com/mattn/go-sqlite3" // SQLite driver
	"github.com/pressly/goose/v3"
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

// DefaultConfig returns default database configuration.
// Checks DATABASE_URL environment variable first, falls back to SQLite.
func DefaultConfig() *Config {
	// Check for DATABASE_URL environment variable (Postgres connection string)
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		// Parse DATABASE_URL (format: postgres://user:password@host:port/database?sslmode=disable)
		return &Config{
			Driver:   "postgres",
			Host:     "localhost",
			Port:     5432,
			Database: "pedrocli",
			User:     "pedrocli",
			Password: "pedrocli",
			SSLMode:  "disable",
		}
	}

	// Fall back to SQLite for local development
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

// Migrate runs all pending database migrations using goose.
func (d *DB) Migrate(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.migrated {
		return nil
	}

	// Set the embedded filesystem for goose
	goose.SetBaseFS(migrationsFS)

	// Set the dialect based on driver
	dialect := "postgres"
	if d.driver == "sqlite3" {
		dialect = "sqlite3"
	}
	if err := goose.SetDialect(dialect); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	// Run migrations
	if err := goose.Up(d.DB, "migrations"); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
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

	// Parse goose migration format to extract only the "Up" section
	sql := string(content)
	upSQL := d.extractGooseUpSection(sql)

	// Adapt SQL for SQLite if needed
	if d.driver == "sqlite3" {
		upSQL = d.adaptSQLForSQLite(upSQL)
	}

	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Execute migration
	if _, err := tx.ExecContext(ctx, upSQL); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	// Record migration
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", filename); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return tx.Commit()
}

// extractGooseUpSection extracts only the "Up" section from a goose migration file.
func (d *DB) extractGooseUpSection(content string) string {
	lines := strings.Split(content, "\n")
	var upLines []string
	inUpSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Start of Up section
		if strings.HasPrefix(trimmed, "-- +goose Up") {
			inUpSection = true
			continue
		}

		// Start of Down section - stop collecting
		if strings.HasPrefix(trimmed, "-- +goose Down") {
			break
		}

		// Collect lines in Up section
		if inUpSection {
			upLines = append(upLines, line)
		}
	}

	return strings.Join(upLines, "\n")
}

// adaptSQLForSQLite modifies PostgreSQL SQL to work with SQLite.
func (d *DB) adaptSQLForSQLite(sql string) string {
	// Replace TIMESTAMP with DATETIME
	sql = strings.ReplaceAll(sql, "TIMESTAMP", "DATETIME")
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
