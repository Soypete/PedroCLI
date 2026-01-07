// Package database provides database connection and migration management for PedroCLI.
// It supports PostgreSQL backend only.
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
	_ "github.com/lib/pq" // PostgreSQL driver
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

// Config holds database configuration for PostgreSQL.
type Config struct {
	Host     string `json:"host"`     // PostgreSQL host
	Port     int    `json:"port"`     // PostgreSQL port
	Database string `json:"database"` // Database name
	User     string `json:"user"`     // PostgreSQL user
	Password string `json:"password"` // PostgreSQL password
	SSLMode  string `json:"ssl_mode"` // PostgreSQL SSL mode
}

// DefaultConfig returns default database configuration.
// Reads from DATABASE_URL or individual DB_* environment variables.
func DefaultConfig() *Config {
	// Check for DATABASE_URL environment variable
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		// TODO: Implement proper DATABASE_URL parsing
		// For now, return defaults - parsePostgresURL should be implemented
		return &Config{
			Host:     "localhost",
			Port:     5432,
			Database: "pedrocli",
			User:     "pedrocli",
			Password: "pedrocli",
			SSLMode:  "disable",
		}
	}

	// Fall back to individual environment variables
	return &Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnvInt("DB_PORT", 5432),
		Database: getEnv("DB_NAME", "pedrocli"),
		User:     getEnv("DB_USER", "pedrocli"),
		Password: getEnv("DB_PASSWORD", "pedrocli"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}
}

// getEnv returns an environment variable or default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt returns an integer environment variable or default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		// Simple parsing - could use strconv.Atoi but keeping it simple
		var i int
		if _, err := fmt.Sscanf(value, "%d", &i); err == nil {
			return i
		}
	}
	return defaultValue
}

// New creates a new PostgreSQL database connection.
func New(cfg *Config) (*DB, error) {
	// Build PostgreSQL connection string
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, sslMode,
	)

	db, err := sql.Open("postgres", connStr)
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
		driver:  "postgres",
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

	// Set the dialect to postgres
	if err := goose.SetDialect("postgres"); err != nil {
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
// TODO: This is part of the old custom migration system, now using goose
//
//nolint:unused // Kept for reference, may be used in future
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
//
//nolint:unused // Part of old custom migration system
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
//
//nolint:unused // Part of old custom migration system
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
//
//nolint:unused // Part of old custom migration system
func (d *DB) applyMigration(ctx context.Context, filename string) error {
	content, err := migrationsFS.ReadFile(filepath.Join("migrations", filename))
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	// Parse goose migration format to extract only the "Up" section
	sql := string(content)
	upSQL := extractGooseUpSection(sql)

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
//
//nolint:unused // Part of old custom migration system
func extractGooseUpSection(content string) string {
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
