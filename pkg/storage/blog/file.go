package blog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// FileStorage implements BlogStorage using the filesystem
// Posts are stored as markdown files with JSON metadata
// Versions are stored as JSON files in a versions subdirectory
type FileStorage struct {
	outputDir string
	postsDir  string
	versDir   string
}

// NewFileStorage creates a new file-based storage
func NewFileStorage(outputDir string) (BlogStorage, error) {
	if outputDir == "" {
		outputDir = "./blog_output"
	}

	// Create directory structure
	postsDir := filepath.Join(outputDir, "posts")
	versDir := filepath.Join(outputDir, "versions")

	if err := os.MkdirAll(postsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create posts directory: %w", err)
	}

	if err := os.MkdirAll(versDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create versions directory: %w", err)
	}

	return &FileStorage{
		outputDir: outputDir,
		postsDir:  postsDir,
		versDir:   versDir,
	}, nil
}

// Post operations

func (s *FileStorage) CreatePost(ctx context.Context, post *BlogPost) error {
	if post.ID == uuid.Nil {
		post.ID = uuid.New()
	}
	if post.CreatedAt.IsZero() {
		post.CreatedAt = time.Now()
	}
	if post.UpdatedAt.IsZero() {
		post.UpdatedAt = time.Now()
	}

	return s.writePost(post)
}

func (s *FileStorage) GetPost(ctx context.Context, id uuid.UUID) (*BlogPost, error) {
	metaPath := filepath.Join(s.postsDir, fmt.Sprintf("%s.meta.json", id))

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("blog post not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read post metadata: %w", err)
	}

	var post BlogPost
	if err := json.Unmarshal(data, &post); err != nil {
		return nil, fmt.Errorf("failed to parse post metadata: %w", err)
	}

	return &post, nil
}

func (s *FileStorage) UpdatePost(ctx context.Context, post *BlogPost) error {
	// Check if post exists
	if _, err := s.GetPost(ctx, post.ID); err != nil {
		return err
	}

	post.UpdatedAt = time.Now()
	return s.writePost(post)
}

func (s *FileStorage) ListPosts(ctx context.Context, status PostStatus) ([]*BlogPost, error) {
	entries, err := os.ReadDir(s.postsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read posts directory: %w", err)
	}

	var posts []*BlogPost
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".meta.json") {
			continue
		}

		// Extract UUID from filename
		idStr := strings.TrimSuffix(entry.Name(), ".meta.json")
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue // Skip invalid filenames
		}

		post, err := s.GetPost(ctx, id)
		if err != nil {
			continue // Skip posts with read errors
		}

		// Filter by status if specified
		if status != "" && post.Status != status {
			continue
		}

		posts = append(posts, post)
	}

	// Sort by created_at descending (most recent first)
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].CreatedAt.After(posts[j].CreatedAt)
	})

	return posts, nil
}

func (s *FileStorage) DeletePost(ctx context.Context, id uuid.UUID) error {
	// Check if post exists
	metaPath := filepath.Join(s.postsDir, fmt.Sprintf("%s.meta.json", id))
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		return fmt.Errorf("blog post not found: %s", id)
	}

	// Delete metadata file
	if err := os.Remove(metaPath); err != nil {
		return fmt.Errorf("failed to delete post metadata: %w", err)
	}

	// Delete markdown file (if it exists)
	mdPath := filepath.Join(s.postsDir, fmt.Sprintf("%s.md", id))
	if err := os.Remove(mdPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete post content: %w", err)
	}

	// Delete versions directory (if it exists)
	versDir := filepath.Join(s.versDir, id.String())
	if err := os.RemoveAll(versDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete versions: %w", err)
	}

	return nil
}

// Version operations

func (s *FileStorage) CreateVersion(ctx context.Context, v *PostVersion) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}

	// Create versions directory for this post
	postVersDir := filepath.Join(s.versDir, v.PostID.String())
	if err := os.MkdirAll(postVersDir, 0755); err != nil {
		return fmt.Errorf("failed to create versions directory: %w", err)
	}

	// Write version file
	versionPath := filepath.Join(postVersDir, fmt.Sprintf("v%d.json", v.VersionNumber))
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal version: %w", err)
	}

	if err := os.WriteFile(versionPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write version file: %w", err)
	}

	return nil
}

func (s *FileStorage) GetVersion(ctx context.Context, postID uuid.UUID, versionNumber int) (*PostVersion, error) {
	versionPath := filepath.Join(s.versDir, postID.String(), fmt.Sprintf("v%d.json", versionNumber))

	data, err := os.ReadFile(versionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("version %d not found for post %s", versionNumber, postID)
		}
		return nil, fmt.Errorf("failed to read version file: %w", err)
	}

	var version PostVersion
	if err := json.Unmarshal(data, &version); err != nil {
		return nil, fmt.Errorf("failed to parse version: %w", err)
	}

	return &version, nil
}

func (s *FileStorage) GetNextVersionNumber(ctx context.Context, postID uuid.UUID) (int, error) {
	postVersDir := filepath.Join(s.versDir, postID.String())

	// If directory doesn't exist, this is version 1
	if _, err := os.Stat(postVersDir); os.IsNotExist(err) {
		return 1, nil
	}

	entries, err := os.ReadDir(postVersDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read versions directory: %w", err)
	}

	maxVersion := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "v") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Extract version number from filename (v1.json -> 1)
		versionStr := strings.TrimPrefix(entry.Name(), "v")
		versionStr = strings.TrimSuffix(versionStr, ".json")

		var versionNum int
		if _, err := fmt.Sscanf(versionStr, "%d", &versionNum); err == nil {
			if versionNum > maxVersion {
				maxVersion = versionNum
			}
		}
	}

	return maxVersion + 1, nil
}

func (s *FileStorage) ListVersions(ctx context.Context, postID uuid.UUID) ([]*PostVersion, error) {
	postVersDir := filepath.Join(s.versDir, postID.String())

	// If directory doesn't exist, return empty list
	if _, err := os.Stat(postVersDir); os.IsNotExist(err) {
		return []*PostVersion{}, nil
	}

	entries, err := os.ReadDir(postVersDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read versions directory: %w", err)
	}

	var versions []*PostVersion
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "v") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Extract version number
		versionStr := strings.TrimPrefix(entry.Name(), "v")
		versionStr = strings.TrimSuffix(versionStr, ".json")

		var versionNum int
		if _, err := fmt.Sscanf(versionStr, "%d", &versionNum); err != nil {
			continue // Skip invalid filenames
		}

		version, err := s.GetVersion(ctx, postID, versionNum)
		if err != nil {
			continue // Skip versions with read errors
		}

		versions = append(versions, version)
	}

	// Sort by version number descending (most recent first)
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].VersionNumber > versions[j].VersionNumber
	})

	return versions, nil
}

// Lifecycle

func (s *FileStorage) Close() error {
	// No resources to clean up for file storage
	return nil
}

// Helper methods

func (s *FileStorage) writePost(post *BlogPost) error {
	// Write metadata file
	metaPath := filepath.Join(s.postsDir, fmt.Sprintf("%s.meta.json", post.ID))
	metaData, err := json.MarshalIndent(post, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal post metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return fmt.Errorf("failed to write post metadata: %w", err)
	}

	// Write markdown file (final content)
	if post.FinalContent != "" {
		mdPath := filepath.Join(s.postsDir, fmt.Sprintf("%s.md", post.ID))
		if err := os.WriteFile(mdPath, []byte(post.FinalContent), 0644); err != nil {
			return fmt.Errorf("failed to write post content: %w", err)
		}
	}

	return nil
}
