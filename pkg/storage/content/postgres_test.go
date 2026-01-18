package content

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// setupTestDB creates a test database connection
// Returns nil if DATABASE_URL is not set (skip integration tests)
func setupTestDB(t *testing.T) *sql.DB {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set - skipping integration test")
		return nil
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("Failed to ping test database: %v", err)
	}

	// Ensure tables exist (migrations should be run separately)
	// This is just a connectivity test
	return db
}

// cleanupTestData removes test data from database
func cleanupTestData(t *testing.T, db *sql.DB, ids []uuid.UUID) {
	for _, id := range ids {
		db.Exec("DELETE FROM content_versions WHERE content_id = $1", id)
		db.Exec("DELETE FROM content WHERE id = $1", id)
	}
}

func TestPostgresContentStore_Create(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	store := NewPostgresContentStore(db)
	ctx := context.Background()

	testIDs := []uuid.UUID{}
	defer cleanupTestData(t, db, testIDs)

	testCases := []struct {
		name        string
		content     *Content
		expectError bool
	}{
		{
			name: "blog content",
			content: &Content{
				ID:     uuid.New(),
				Type:   ContentTypeBlog,
				Status: StatusDraft,
				Title:  "Test Blog Post",
				Data:   map[string]interface{}{"author": "Test Author"},
			},
			expectError: false,
		},
		{
			name: "podcast content",
			content: &Content{
				ID:     uuid.New(),
				Type:   ContentTypePodcast,
				Status: StatusInProgress,
				Title:  "Test Podcast",
				Data:   map[string]interface{}{"duration": 3600},
			},
			expectError: false,
		},
		{
			name: "auto-generate ID",
			content: &Content{
				Type:   ContentTypeBlog,
				Status: StatusDraft,
				Title:  "Auto ID",
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
				testIDs = append(testIDs, tc.content.ID)

				// Verify timestamps were set
				if tc.content.CreatedAt.IsZero() {
					t.Error("Expected CreatedAt to be set")
				}
				if tc.content.UpdatedAt.IsZero() {
					t.Error("Expected UpdatedAt to be set")
				}
			}
		})
	}
}

func TestPostgresContentStore_Get(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	store := NewPostgresContentStore(db)
	ctx := context.Background()

	testContent := &Content{
		ID:     uuid.New(),
		Type:   ContentTypeBlog,
		Status: StatusDraft,
		Title:  "Test Get",
		Data:   map[string]interface{}{"key": "value"},
	}
	defer cleanupTestData(t, db, []uuid.UUID{testContent.ID})

	// Create first
	if err := store.Create(ctx, testContent); err != nil {
		t.Fatalf("Failed to create test content: %v", err)
	}

	// Get it back
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

	// Test non-existent
	nonExistent := uuid.New()
	_, err = store.Get(ctx, nonExistent)
	if err == nil {
		t.Error("Expected error for non-existent content")
	}
}

func TestPostgresContentStore_Update(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	store := NewPostgresContentStore(db)
	ctx := context.Background()

	testContent := &Content{
		ID:     uuid.New(),
		Type:   ContentTypeBlog,
		Status: StatusDraft,
		Title:  "Original Title",
		Data:   map[string]interface{}{"version": 1},
	}
	defer cleanupTestData(t, db, []uuid.UUID{testContent.ID})

	// Create first
	if err := store.Create(ctx, testContent); err != nil {
		t.Fatalf("Failed to create test content: %v", err)
	}

	originalUpdatedAt := testContent.UpdatedAt
	time.Sleep(10 * time.Millisecond) // Ensure timestamp difference

	// Update
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
	if retrieved.UpdatedAt.Before(originalUpdatedAt) || retrieved.UpdatedAt.Equal(originalUpdatedAt) {
		t.Error("Expected UpdatedAt to be newer")
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

func TestPostgresContentStore_List(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	store := NewPostgresContentStore(db)
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

	testIDs := []uuid.UUID{}
	for _, content := range contents {
		if err := store.Create(ctx, content); err != nil {
			t.Fatalf("Failed to create content: %v", err)
		}
		testIDs = append(testIDs, content.ID)
	}
	defer cleanupTestData(t, db, testIDs)

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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := store.List(ctx, tc.filter)
			if err != nil {
				t.Fatalf("List failed: %v", err)
			}

			// We may have more results from other tests, so use >=
			if len(results) < tc.expectedCount {
				t.Errorf("Expected at least %d results, got %d", tc.expectedCount, len(results))
			}

			// Verify all results match filter
			for _, result := range results {
				if tc.filter.Type != nil && result.Type != *tc.filter.Type {
					t.Errorf("Result type %s doesn't match filter %s", result.Type, *tc.filter.Type)
				}
				if tc.filter.Status != nil && result.Status != *tc.filter.Status {
					t.Errorf("Result status %s doesn't match filter %s", result.Status, *tc.filter.Status)
				}
			}
		})
	}
}

func TestPostgresContentStore_Delete(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	store := NewPostgresContentStore(db)
	ctx := context.Background()

	testContent := &Content{
		ID:     uuid.New(),
		Type:   ContentTypeBlog,
		Status: StatusDraft,
		Title:  "To Delete",
		Data:   map[string]interface{}{},
	}

	// Create first
	if err := store.Create(ctx, testContent); err != nil {
		t.Fatalf("Failed to create test content: %v", err)
	}

	// Delete it
	if err := store.Delete(ctx, testContent.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	_, err := store.Get(ctx, testContent.ID)
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

func TestPostgresVersionStore_SaveVersion(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create a content first (versions reference content)
	contentStore := NewPostgresContentStore(db)
	ctx := context.Background()

	testContent := &Content{
		ID:     uuid.New(),
		Type:   ContentTypeBlog,
		Status: StatusDraft,
		Title:  "Test Content",
		Data:   map[string]interface{}{},
	}
	defer cleanupTestData(t, db, []uuid.UUID{testContent.ID})

	if err := contentStore.Create(ctx, testContent); err != nil {
		t.Fatalf("Failed to create test content: %v", err)
	}

	// Now test version store
	versionStore := NewPostgresVersionStore(db)

	version := &Version{
		ID:         uuid.New(),
		ContentID:  testContent.ID,
		Phase:      "Outline",
		VersionNum: 1,
		Snapshot:   map[string]interface{}{"sections": 5},
	}

	if err := versionStore.SaveVersion(ctx, version); err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	// Verify timestamps were set
	if version.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
}

func TestPostgresVersionStore_GetVersion(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create content first
	contentStore := NewPostgresContentStore(db)
	versionStore := NewPostgresVersionStore(db)
	ctx := context.Background()

	testContent := &Content{
		ID:     uuid.New(),
		Type:   ContentTypeBlog,
		Status: StatusDraft,
		Title:  "Test Content",
		Data:   map[string]interface{}{},
	}
	defer cleanupTestData(t, db, []uuid.UUID{testContent.ID})

	if err := contentStore.Create(ctx, testContent); err != nil {
		t.Fatalf("Failed to create test content: %v", err)
	}

	// Save a version
	version := &Version{
		ID:         uuid.New(),
		ContentID:  testContent.ID,
		Phase:      "Outline",
		VersionNum: 1,
		Snapshot:   map[string]interface{}{"sections": 5},
	}

	if err := versionStore.SaveVersion(ctx, version); err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	// Get it back
	retrieved, err := versionStore.GetVersion(ctx, testContent.ID, 1)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}

	if retrieved.Phase != "Outline" {
		t.Errorf("Expected phase Outline, got %s", retrieved.Phase)
	}
	if retrieved.VersionNum != 1 {
		t.Errorf("Expected version 1, got %d", retrieved.VersionNum)
	}
	if sections, ok := retrieved.Snapshot["sections"].(float64); !ok || sections != 5 {
		t.Errorf("Expected sections=5, got %v", retrieved.Snapshot["sections"])
	}

	// Test non-existent version
	_, err = versionStore.GetVersion(ctx, testContent.ID, 99)
	if err == nil {
		t.Error("Expected error for non-existent version")
	}
}

func TestPostgresVersionStore_ListVersions(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create content first
	contentStore := NewPostgresContentStore(db)
	versionStore := NewPostgresVersionStore(db)
	ctx := context.Background()

	testContent := &Content{
		ID:     uuid.New(),
		Type:   ContentTypeBlog,
		Status: StatusDraft,
		Title:  "Test Content",
		Data:   map[string]interface{}{},
	}
	defer cleanupTestData(t, db, []uuid.UUID{testContent.ID})

	if err := contentStore.Create(ctx, testContent); err != nil {
		t.Fatalf("Failed to create test content: %v", err)
	}

	// Save multiple versions
	versions := []*Version{
		{
			ID:         uuid.New(),
			ContentID:  testContent.ID,
			Phase:      "Outline",
			VersionNum: 1,
			Snapshot:   map[string]interface{}{},
		},
		{
			ID:         uuid.New(),
			ContentID:  testContent.ID,
			Phase:      "Sections",
			VersionNum: 2,
			Snapshot:   map[string]interface{}{},
		},
		{
			ID:         uuid.New(),
			ContentID:  testContent.ID,
			Phase:      "Assemble",
			VersionNum: 3,
			Snapshot:   map[string]interface{}{},
		},
	}

	for _, v := range versions {
		if err := versionStore.SaveVersion(ctx, v); err != nil {
			t.Fatalf("SaveVersion failed: %v", err)
		}
	}

	// List all versions
	retrieved, err := versionStore.ListVersions(ctx, testContent.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}

	if len(retrieved) != 3 {
		t.Errorf("Expected 3 versions, got %d", len(retrieved))
	}

	// Verify ordering (should be ASC by version_num)
	for i, v := range retrieved {
		expectedNum := i + 1
		if v.VersionNum != expectedNum {
			t.Errorf("Expected version %d at index %d, got %d", expectedNum, i, v.VersionNum)
		}
	}
}

func TestPostgresVersionStore_DeleteVersions(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create content first
	contentStore := NewPostgresContentStore(db)
	versionStore := NewPostgresVersionStore(db)
	ctx := context.Background()

	testContent := &Content{
		ID:     uuid.New(),
		Type:   ContentTypeBlog,
		Status: StatusDraft,
		Title:  "Test Content",
		Data:   map[string]interface{}{},
	}
	defer cleanupTestData(t, db, []uuid.UUID{testContent.ID})

	if err := contentStore.Create(ctx, testContent); err != nil {
		t.Fatalf("Failed to create test content: %v", err)
	}

	// Save a version
	version := &Version{
		ID:         uuid.New(),
		ContentID:  testContent.ID,
		Phase:      "Outline",
		VersionNum: 1,
		Snapshot:   map[string]interface{}{},
	}

	if err := versionStore.SaveVersion(ctx, version); err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	// Delete all versions
	if err := versionStore.DeleteVersions(ctx, testContent.ID); err != nil {
		t.Fatalf("DeleteVersions failed: %v", err)
	}

	// Verify they're gone
	versions, err := versionStore.ListVersions(ctx, testContent.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("Expected 0 versions after delete, got %d", len(versions))
	}

	// Test delete non-existent (should not error)
	nonExistent := uuid.New()
	if err := versionStore.DeleteVersions(ctx, nonExistent); err != nil {
		t.Errorf("DeleteVersions should not error for non-existent: %v", err)
	}
}

func TestPostgresVersionStore_CascadeDelete(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Test that deleting content cascades to versions
	contentStore := NewPostgresContentStore(db)
	versionStore := NewPostgresVersionStore(db)
	ctx := context.Background()

	testContent := &Content{
		ID:     uuid.New(),
		Type:   ContentTypeBlog,
		Status: StatusDraft,
		Title:  "Cascade Test",
		Data:   map[string]interface{}{},
	}

	if err := contentStore.Create(ctx, testContent); err != nil {
		t.Fatalf("Failed to create test content: %v", err)
	}

	// Save a version
	version := &Version{
		ID:         uuid.New(),
		ContentID:  testContent.ID,
		Phase:      "Outline",
		VersionNum: 1,
		Snapshot:   map[string]interface{}{},
	}

	if err := versionStore.SaveVersion(ctx, version); err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	// Delete the content
	if err := contentStore.Delete(ctx, testContent.ID); err != nil {
		t.Fatalf("Delete content failed: %v", err)
	}

	// Verify versions were cascade deleted
	versions, err := versionStore.ListVersions(ctx, testContent.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("Expected 0 versions after cascade delete, got %d", len(versions))
	}
}
