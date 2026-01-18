package content

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewFileContentStore(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileContentStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileContentStore failed: %v", err)
	}

	// Verify subdirectories were created
	subdirs := []string{"blog", "podcast", "code"}
	for _, subdir := range subdirs {
		path := filepath.Join(tmpDir, subdir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected subdirectory %s to exist", subdir)
		}
	}

	if store == nil {
		t.Error("Expected non-nil store")
	}
}

func TestFileContentStore_Create(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileContentStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileContentStore failed: %v", err)
	}

	ctx := context.Background()
	now := time.Now()

	testCases := []struct {
		name        string
		content     *Content
		expectError bool
	}{
		{
			name: "blog content",
			content: &Content{
				ID:        uuid.New(),
				Type:      ContentTypeBlog,
				Status:    StatusDraft,
				Title:     "Test Blog Post",
				Data:      map[string]interface{}{"author": "Test Author"},
				CreatedAt: now,
				UpdatedAt: now,
			},
			expectError: false,
		},
		{
			name: "podcast content",
			content: &Content{
				ID:        uuid.New(),
				Type:      ContentTypePodcast,
				Status:    StatusInProgress,
				Title:     "Test Podcast Episode",
				Data:      map[string]interface{}{"duration": 3600},
				CreatedAt: now,
				UpdatedAt: now,
			},
			expectError: false,
		},
		{
			name: "code content",
			content: &Content{
				ID:        uuid.New(),
				Type:      ContentTypeCode,
				Status:    StatusReview,
				Title:     "Test Code Changes",
				Data:      map[string]interface{}{"files_changed": 5},
				CreatedAt: now,
				UpdatedAt: now,
			},
			expectError: false,
		},
		{
			name: "auto-generate ID",
			content: &Content{
				Type:   ContentTypeBlog,
				Status: StatusDraft,
				Title:  "Auto ID Test",
				Data:   map[string]interface{}{},
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := store.Create(ctx, tc.content)

			if tc.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tc.expectError {
				// Verify file was created
				subdir := string(tc.content.Type)
				if subdir == "" {
					subdir = "blog"
				}
				filename := filepath.Join(tmpDir, subdir, tc.content.ID.String()+".json")
				if _, err := os.Stat(filename); os.IsNotExist(err) {
					t.Errorf("Expected file %s to exist", filename)
				}

				// Verify content can be read back
				data, err := os.ReadFile(filename)
				if err != nil {
					t.Fatalf("Failed to read created file: %v", err)
				}

				var retrieved Content
				if err := json.Unmarshal(data, &retrieved); err != nil {
					t.Fatalf("Failed to unmarshal content: %v", err)
				}

				if retrieved.Title != tc.content.Title {
					t.Errorf("Expected title %s, got %s", tc.content.Title, retrieved.Title)
				}
			}
		})
	}
}

func TestFileContentStore_Get(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileContentStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileContentStore failed: %v", err)
	}

	ctx := context.Background()

	// Create a content first
	testContent := &Content{
		ID:        uuid.New(),
		Type:      ContentTypeBlog,
		Status:    StatusDraft,
		Title:     "Test Get",
		Data:      map[string]interface{}{"key": "value"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := store.Create(ctx, testContent); err != nil {
		t.Fatalf("Failed to create test content: %v", err)
	}

	// Test Get
	retrieved, err := store.Get(ctx, testContent.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.ID != testContent.ID {
		t.Errorf("Expected ID %s, got %s", testContent.ID, retrieved.ID)
	}
	if retrieved.Title != testContent.Title {
		t.Errorf("Expected title %s, got %s", testContent.Title, retrieved.Title)
	}
	if retrieved.Data["key"] != "value" {
		t.Errorf("Expected data key=value, got %v", retrieved.Data["key"])
	}

	// Test Get non-existent
	nonExistent := uuid.New()
	_, err = store.Get(ctx, nonExistent)
	if err == nil {
		t.Error("Expected error for non-existent content")
	}
}

func TestFileContentStore_Update(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileContentStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileContentStore failed: %v", err)
	}

	ctx := context.Background()

	// Create a content first
	testContent := &Content{
		ID:        uuid.New(),
		Type:      ContentTypeBlog,
		Status:    StatusDraft,
		Title:     "Original Title",
		Data:      map[string]interface{}{"version": 1},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := store.Create(ctx, testContent); err != nil {
		t.Fatalf("Failed to create test content: %v", err)
	}

	// Update the content
	testContent.Title = "Updated Title"
	testContent.Status = StatusReview
	testContent.Data["version"] = 2

	if err := store.Update(ctx, testContent); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify update
	retrieved, err := store.Get(ctx, testContent.ID)
	if err != nil {
		t.Fatalf("Get failed after update: %v", err)
	}

	if retrieved.Title != "Updated Title" {
		t.Errorf("Expected updated title, got %s", retrieved.Title)
	}
	if retrieved.Status != StatusReview {
		t.Errorf("Expected status %s, got %s", StatusReview, retrieved.Status)
	}
	if version, ok := retrieved.Data["version"].(float64); !ok || version != 2 {
		t.Errorf("Expected version 2, got %v", retrieved.Data["version"])
	}

	// Test update non-existent
	nonExistent := &Content{
		ID:     uuid.New(),
		Type:   ContentTypeBlog,
		Status: StatusDraft,
		Title:  "Non-existent",
		Data:   map[string]interface{}{},
	}
	err = store.Update(ctx, nonExistent)
	if err == nil {
		t.Error("Expected error updating non-existent content")
	}
}

func TestFileContentStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileContentStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileContentStore failed: %v", err)
	}

	ctx := context.Background()

	// Create multiple contents
	contents := []*Content{
		{
			ID:     uuid.New(),
			Type:   ContentTypeBlog,
			Status: StatusDraft,
			Title:  "Blog 1",
			Data:   map[string]interface{}{},
		},
		{
			ID:     uuid.New(),
			Type:   ContentTypeBlog,
			Status: StatusPublished,
			Title:  "Blog 2",
			Data:   map[string]interface{}{},
		},
		{
			ID:     uuid.New(),
			Type:   ContentTypePodcast,
			Status: StatusDraft,
			Title:  "Podcast 1",
			Data:   map[string]interface{}{},
		},
	}

	for _, content := range contents {
		if err := store.Create(ctx, content); err != nil {
			t.Fatalf("Failed to create content: %v", err)
		}
	}

	testCases := []struct {
		name          string
		filter        Filter
		expectedCount int
	}{
		{
			name:          "all blog content",
			filter:        Filter{Type: ptrContentType(ContentTypeBlog)},
			expectedCount: 2,
		},
		{
			name:          "all podcast content",
			filter:        Filter{Type: ptrContentType(ContentTypePodcast)},
			expectedCount: 1,
		},
		{
			name:          "draft status",
			filter:        Filter{Status: ptrStatus(StatusDraft)},
			expectedCount: 2,
		},
		{
			name:          "published status",
			filter:        Filter{Status: ptrStatus(StatusPublished)},
			expectedCount: 1,
		},
		{
			name:          "blog and draft",
			filter:        Filter{Type: ptrContentType(ContentTypeBlog), Status: ptrStatus(StatusDraft)},
			expectedCount: 1,
		},
		{
			name:          "no filter",
			filter:        Filter{},
			expectedCount: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := store.List(ctx, tc.filter)
			if err != nil {
				t.Fatalf("List failed: %v", err)
			}

			if len(results) != tc.expectedCount {
				t.Errorf("Expected %d results, got %d", tc.expectedCount, len(results))
			}
		})
	}
}

func TestFileContentStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileContentStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileContentStore failed: %v", err)
	}

	ctx := context.Background()

	// Create a content first
	testContent := &Content{
		ID:     uuid.New(),
		Type:   ContentTypeBlog,
		Status: StatusDraft,
		Title:  "To Delete",
		Data:   map[string]interface{}{},
	}

	if err := store.Create(ctx, testContent); err != nil {
		t.Fatalf("Failed to create test content: %v", err)
	}

	// Delete it
	if err := store.Delete(ctx, testContent.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	_, err = store.Get(ctx, testContent.ID)
	if err == nil {
		t.Error("Expected error getting deleted content")
	}

	// Test delete non-existent
	nonExistent := uuid.New()
	err = store.Delete(ctx, nonExistent)
	if err == nil {
		t.Error("Expected error deleting non-existent content")
	}
}

func TestFileVersionStore_SaveVersion(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileVersionStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileVersionStore failed: %v", err)
	}

	ctx := context.Background()
	contentID := uuid.New()

	version := &Version{
		ID:         uuid.New(),
		ContentID:  contentID,
		Phase:      "Outline",
		VersionNum: 1,
		Snapshot:   map[string]interface{}{"sections": 5},
		CreatedAt:  time.Now(),
	}

	if err := store.SaveVersion(ctx, version); err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	// Verify file was created
	filename := filepath.Join(tmpDir, "versions", contentID.String(), "1.json")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Errorf("Expected version file %s to exist", filename)
	}
}

func TestFileVersionStore_GetVersion(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileVersionStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileVersionStore failed: %v", err)
	}

	ctx := context.Background()
	contentID := uuid.New()

	// Save a version first
	version := &Version{
		ID:         uuid.New(),
		ContentID:  contentID,
		Phase:      "Outline",
		VersionNum: 1,
		Snapshot:   map[string]interface{}{"sections": 5},
		CreatedAt:  time.Now(),
	}

	if err := store.SaveVersion(ctx, version); err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	// Get it back
	retrieved, err := store.GetVersion(ctx, contentID, 1)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}

	if retrieved.Phase != "Outline" {
		t.Errorf("Expected phase Outline, got %s", retrieved.Phase)
	}
	if retrieved.VersionNum != 1 {
		t.Errorf("Expected version 1, got %d", retrieved.VersionNum)
	}

	// Test non-existent version
	_, err = store.GetVersion(ctx, contentID, 99)
	if err == nil {
		t.Error("Expected error for non-existent version")
	}
}

func TestFileVersionStore_ListVersions(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileVersionStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileVersionStore failed: %v", err)
	}

	ctx := context.Background()
	contentID := uuid.New()

	// Save multiple versions
	versions := []*Version{
		{
			ID:         uuid.New(),
			ContentID:  contentID,
			Phase:      "Outline",
			VersionNum: 1,
			Snapshot:   map[string]interface{}{},
			CreatedAt:  time.Now(),
		},
		{
			ID:         uuid.New(),
			ContentID:  contentID,
			Phase:      "Sections",
			VersionNum: 2,
			Snapshot:   map[string]interface{}{},
			CreatedAt:  time.Now(),
		},
		{
			ID:         uuid.New(),
			ContentID:  contentID,
			Phase:      "Assemble",
			VersionNum: 3,
			Snapshot:   map[string]interface{}{},
			CreatedAt:  time.Now(),
		},
	}

	for _, v := range versions {
		if err := store.SaveVersion(ctx, v); err != nil {
			t.Fatalf("SaveVersion failed: %v", err)
		}
	}

	// List all versions
	retrieved, err := store.ListVersions(ctx, contentID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}

	if len(retrieved) != 3 {
		t.Errorf("Expected 3 versions, got %d", len(retrieved))
	}

	// Test non-existent content
	emptyID := uuid.New()
	emptyList, err := store.ListVersions(ctx, emptyID)
	if err != nil {
		t.Fatalf("ListVersions failed for non-existent: %v", err)
	}
	if len(emptyList) != 0 {
		t.Errorf("Expected 0 versions for non-existent content, got %d", len(emptyList))
	}
}

func TestFileVersionStore_DeleteVersions(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileVersionStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileVersionStore failed: %v", err)
	}

	ctx := context.Background()
	contentID := uuid.New()

	// Save a version
	version := &Version{
		ID:         uuid.New(),
		ContentID:  contentID,
		Phase:      "Outline",
		VersionNum: 1,
		Snapshot:   map[string]interface{}{},
		CreatedAt:  time.Now(),
	}

	if err := store.SaveVersion(ctx, version); err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	// Delete all versions
	if err := store.DeleteVersions(ctx, contentID); err != nil {
		t.Fatalf("DeleteVersions failed: %v", err)
	}

	// Verify they're gone
	versions, err := store.ListVersions(ctx, contentID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("Expected 0 versions after delete, got %d", len(versions))
	}

	// Test delete non-existent (should not error)
	nonExistent := uuid.New()
	if err := store.DeleteVersions(ctx, nonExistent); err != nil {
		t.Errorf("DeleteVersions should not error for non-existent: %v", err)
	}
}

// Helper functions
func ptrContentType(ct ContentType) *ContentType {
	return &ct
}

func ptrStatus(s Status) *Status {
	return &s
}
