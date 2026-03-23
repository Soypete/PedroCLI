// Package database provides database connection and migration management for PedroCLI.
// It supports PostgreSQL backend only.
package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB represents a database connection with migration support.
type DB struct {
	*sql.DB
	pool     *pgxpool.Pool
	driver   string
	schema   string
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
	Schema   string `json:"schema"`   // PostgreSQL schema (default: pedrocli)
}

// DefaultConfig returns default database configuration.
// Reads from DATABASE_URL or individual DB_* environment variables.
// For Supabase, set DATABASE_URL to your Supabase connection string
// (e.g. postgres://postgres.<ref>:<password>@aws-0-us-west-1.pooler.supabase.com:6543/postgres).
func DefaultConfig() *Config {
	// Check for DATABASE_URL environment variable (preferred for Supabase)
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		cfg, err := ParseDatabaseURL(dbURL)
		if err == nil {
			return cfg
		}
		// Fall through to individual env vars on parse error
		fmt.Fprintf(os.Stderr, "Warning: failed to parse DATABASE_URL: %v, falling back to DB_* env vars\n", err)
	}

	// Fall back to individual environment variables
	return &Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnvInt("DB_PORT", 5432),
		Database: getEnv("DB_NAME", "pedrocli"),
		User:     getEnv("DB_USER", "pedrocli"),
		Password: getEnv("DB_PASSWORD", "pedrocli"),
		SSLMode:  getEnv("DB_SSLMODE", "require"),
		Schema:   getEnv("DB_SCHEMA", "pedrocli"),
	}
}

// ParseDatabaseURL parses a PostgreSQL connection URL into a Config.
// Supports standard postgres:// and postgresql:// URL formats used by Supabase.
func ParseDatabaseURL(rawURL string) (*Config, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid database URL: %w", err)
	}

	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return nil, fmt.Errorf("unsupported scheme %q, expected postgres or postgresql", u.Scheme)
	}

	cfg := &Config{
		Host:     u.Hostname(),
		Database: strings.TrimPrefix(u.Path, "/"),
		SSLMode:  "require", // Default to require for Supabase
	}

	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil {
			return nil, fmt.Errorf("invalid port %q: %w", u.Port(), err)
		}
		cfg.Port = port
	} else {
		cfg.Port = 5432
	}

	if u.User != nil {
		cfg.User = u.User.Username()
		if pw, ok := u.User.Password(); ok {
			cfg.Password = pw
		}
	}

	// Parse query parameters for sslmode override
	if sslmode := u.Query().Get("sslmode"); sslmode != "" {
		cfg.SSLMode = sslmode
	}

	// Schema from env var (URL doesn't carry schema; set DB_SCHEMA or default)
	cfg.Schema = getEnv("DB_SCHEMA", "pedrocli")

	return cfg, nil
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

// New creates a new PostgreSQL database connection backed by pgxpool.
// pgxpool's AfterConnect hook sets search_path on every connection so that
// all queries land in the correct schema even through Supabase's PgBouncer
// (which strips session-level startup parameters).
func New(cfg *Config) (*DB, error) {
	schema := cfg.Schema
	if schema == "" {
		schema = "pedrocli"
	}

	// Build the DSN: prefer DATABASE_URL env var, fall back to individual cfg fields.
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		sslMode := cfg.SSLMode
		if sslMode == "" {
			sslMode = "require"
		}
		connStr = fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, sslMode,
		)
	}

	poolCfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pool config: %w", err)
	}

	// AfterConnect runs after every new connection is established.
	// Setting search_path here guarantees it is applied regardless of PgBouncer
	// session-reset behaviour.
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET search_path TO "+schema)
		return err
	}

	// Conservative pool size for Supabase connection limits.
	poolCfg.MaxConns = 10
	poolCfg.MaxConnLifetime = 5 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// stdlib adapter makes the pgxpool usable as a standard *sql.DB so the rest
	// of the codebase (store, goose) does not need to change.
	sqlDB := stdlib.OpenDBFromPool(pool)

	// Ensure the pedrocli schema exists.
	if _, err := sqlDB.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS "+schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to create schema %q: %w", schema, err)
	}

	return &DB{
		DB:     sqlDB,
		pool:   pool,
		driver: "pgx",
		schema: schema,
	}, nil
}

// Driver returns the database driver name.
func (d *DB) Driver() string {
	return d.driver
}

// Migrate runs all pending database migrations using goose.
// search_path is set on every connection via the pgxpool AfterConnect hook,
// so migrations always land in the correct schema.
func (d *DB) Migrate(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.migrated {
		return nil
	}

	goose.SetBaseFS(migrationsFS)
	// Use a pedrocli-specific table so we don't conflict with other apps
	// sharing the same Supabase database (e.g. discord bot, chatbot).
	goose.SetTableName("pedrocli_goose_version")

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

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
