package content

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

// StoreConfig configures content and version storage.
// Provides both PostgreSQL (for Web UI) and file-based (for CLI) options.
type StoreConfig struct {
	// DB is the PostgreSQL connection (optional)
	// If nil, file-based storage will be used
	DB *sql.DB

	// FileBaseDir is the base directory for file-based storage
	// Used when DB is nil
	// Defaults to ~/.pedrocli/content if not specified
	FileBaseDir string
}

// NewContentStore creates the appropriate content store based on config.
// Returns PostgresContentStore if DB is provided, otherwise FileContentStore.
func NewContentStore(cfg StoreConfig) (ContentStore, error) {
	if cfg.DB != nil {
		return NewPostgresContentStore(cfg.DB), nil
	}

	// Use file-based storage
	baseDir := cfg.FileBaseDir
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		baseDir = filepath.Join(home, ".pedrocli", "content")
	}

	return NewFileContentStore(baseDir)
}

// NewVersionStore creates the appropriate version store based on config.
// Returns PostgresVersionStore if DB is provided, otherwise FileVersionStore.
func NewVersionStore(cfg StoreConfig) (VersionStore, error) {
	if cfg.DB != nil {
		return NewPostgresVersionStore(cfg.DB), nil
	}

	// Use file-based storage
	baseDir := cfg.FileBaseDir
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		baseDir = filepath.Join(home, ".pedrocli", "content")
	}

	return NewFileVersionStore(baseDir)
}

// NewStores is a convenience function that creates both stores with the same config
func NewStores(cfg StoreConfig) (ContentStore, VersionStore, error) {
	contentStore, err := NewContentStore(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create content store: %w", err)
	}

	versionStore, err := NewVersionStore(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create version store: %w", err)
	}

	return contentStore, versionStore, nil
}
