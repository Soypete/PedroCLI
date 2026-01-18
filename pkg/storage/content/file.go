package content

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

// FileContentStore implements ContentStore using JSON files
// Used for CLI mode - stores content in ~/.pedrocli/content/
type FileContentStore struct {
	baseDir string
	mu      sync.RWMutex
}

// NewFileContentStore creates a new file-based content store
func NewFileContentStore(baseDir string) (ContentStore, error) {
	// Expand home directory if needed
	if baseDir[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		baseDir = filepath.Join(home, baseDir[2:])
	}

	// Create base directory and subdirectories
	dirs := []string{
		baseDir,
		filepath.Join(baseDir, "blog"),
		filepath.Join(baseDir, "podcast"),
		filepath.Join(baseDir, "code"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return &FileContentStore{baseDir: baseDir}, nil
}

// Create stores new content as a JSON file
func (s *FileContentStore) Create(ctx context.Context, content *Content) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate
	if content.ID == uuid.Nil {
		content.ID = uuid.New()
	}

	// Determine subdirectory based on content type
	subdir := string(content.Type)
	if subdir == "" {
		subdir = "blog" // Default
	}

	// Create file path
	filename := filepath.Join(s.baseDir, subdir, fmt.Sprintf("%s.json", content.ID))

	// Marshal to JSON
	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Get retrieves content by ID
func (s *FileContentStore) Get(ctx context.Context, id uuid.UUID) (*Content, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Search across all subdirectories
	subdirs := []string{"blog", "podcast", "code"}

	for _, subdir := range subdirs {
		filename := filepath.Join(s.baseDir, subdir, fmt.Sprintf("%s.json", id))

		data, err := os.ReadFile(filename)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Try next subdirectory
			}
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		var content Content
		if err := json.Unmarshal(data, &content); err != nil {
			return nil, fmt.Errorf("failed to unmarshal content: %w", err)
		}

		return &content, nil
	}

	return nil, fmt.Errorf("content not found: %s", id)
}

// Update modifies existing content
func (s *FileContentStore) Update(ctx context.Context, content *Content) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Determine subdirectory
	subdir := string(content.Type)
	if subdir == "" {
		subdir = "blog"
	}

	filename := filepath.Join(s.baseDir, subdir, fmt.Sprintf("%s.json", content.ID))

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("content not found: %s", content.ID)
	}

	// Marshal and write
	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// List retrieves content matching filter
func (s *FileContentStore) List(ctx context.Context, filter Filter) ([]*Content, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Content

	// Determine which subdirectories to search
	subdirs := []string{"blog", "podcast", "code"}
	if filter.Type != nil {
		subdirs = []string{string(*filter.Type)}
	}

	// Walk through directories
	for _, subdir := range subdirs {
		dirPath := filepath.Join(s.baseDir, subdir)

		entries, err := os.ReadDir(dirPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to read directory: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}

			filename := filepath.Join(dirPath, entry.Name())
			data, err := os.ReadFile(filename)
			if err != nil {
				continue // Skip unreadable files
			}

			var content Content
			if err := json.Unmarshal(data, &content); err != nil {
				continue // Skip invalid JSON
			}

			// Apply filters
			if filter.Status != nil && content.Status != *filter.Status {
				continue
			}

			results = append(results, &content)
		}
	}

	return results, nil
}

// Delete removes content
func (s *FileContentStore) Delete(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Search across all subdirectories
	subdirs := []string{"blog", "podcast", "code"}

	for _, subdir := range subdirs {
		filename := filepath.Join(s.baseDir, subdir, fmt.Sprintf("%s.json", id))

		if err := os.Remove(filename); err != nil {
			if os.IsNotExist(err) {
				continue // Try next subdirectory
			}
			return fmt.Errorf("failed to delete file: %w", err)
		}

		return nil // Successfully deleted
	}

	return fmt.Errorf("content not found: %s", id)
}

// FileVersionStore implements VersionStore using JSON files
type FileVersionStore struct {
	baseDir string
	mu      sync.RWMutex
}

// NewFileVersionStore creates a new file-based version store
func NewFileVersionStore(baseDir string) (VersionStore, error) {
	// Expand home directory if needed
	if baseDir[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		baseDir = filepath.Join(home, baseDir[2:])
	}

	// Create versions directory
	versionsDir := filepath.Join(baseDir, "versions")
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create versions directory: %w", err)
	}

	return &FileVersionStore{baseDir: versionsDir}, nil
}

// SaveVersion stores a version snapshot
func (s *FileVersionStore) SaveVersion(ctx context.Context, version *Version) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create content-specific directory
	contentDir := filepath.Join(s.baseDir, version.ContentID.String())
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		return fmt.Errorf("failed to create content directory: %w", err)
	}

	// Create version file
	filename := filepath.Join(contentDir, fmt.Sprintf("%d.json", version.VersionNum))

	data, err := json.MarshalIndent(version, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal version: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write version file: %w", err)
	}

	return nil
}

// GetVersion retrieves a specific version
func (s *FileVersionStore) GetVersion(ctx context.Context, contentID uuid.UUID, versionNum int) (*Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filename := filepath.Join(s.baseDir, contentID.String(), fmt.Sprintf("%d.json", versionNum))

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("version not found: %s v%d", contentID, versionNum)
		}
		return nil, fmt.Errorf("failed to read version file: %w", err)
	}

	var version Version
	if err := json.Unmarshal(data, &version); err != nil {
		return nil, fmt.Errorf("failed to unmarshal version: %w", err)
	}

	return &version, nil
}

// ListVersions retrieves all versions for content
func (s *FileVersionStore) ListVersions(ctx context.Context, contentID uuid.UUID) ([]*Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	contentDir := filepath.Join(s.baseDir, contentID.String())

	entries, err := os.ReadDir(contentDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Version{}, nil // No versions yet
		}
		return nil, fmt.Errorf("failed to read versions directory: %w", err)
	}

	var versions []*Version

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filename := filepath.Join(contentDir, entry.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			continue
		}

		var version Version
		if err := json.Unmarshal(data, &version); err != nil {
			continue
		}

		versions = append(versions, &version)
	}

	return versions, nil
}

// DeleteVersions removes all versions for content
func (s *FileVersionStore) DeleteVersions(ctx context.Context, contentID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	contentDir := filepath.Join(s.baseDir, contentID.String())

	if err := os.RemoveAll(contentDir); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete versions: %w", err)
	}

	return nil
}
