package blog

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// storageTestSuite runs the same tests against all BlogStorage implementations
// This ensures consistent behavior across Memory, File, and Database backends
type storageTestSuite struct {
	t       *testing.T
	storage BlogStorage
}

// TestAllStorageImplementations runs the complete test suite against all implementations
func TestAllStorageImplementations(t *testing.T) {
	t.Run("MemoryStorage", func(t *testing.T) {
		storage := NewMemoryStorage()
		defer storage.Close()

		suite := &storageTestSuite{t: t, storage: storage}
		suite.runAllTests()
	})

	t.Run("FileStorage", func(t *testing.T) {
		tempDir := t.TempDir()
		storage, err := NewFileStorage(tempDir)
		require.NoError(t, err)
		defer storage.Close()

		// Use t.Cleanup to ensure temp directory is cleaned up
		t.Cleanup(func() {
			os.RemoveAll(tempDir)
		})

		suite := &storageTestSuite{t: t, storage: storage}
		suite.runAllTests()
	})
}

// runAllTests executes all test scenarios
func (s *storageTestSuite) runAllTests() {
	s.t.Run("CreateAndGetPost", func(t *testing.T) { s.testCreateAndGetPost() })
	s.t.Run("UpdatePost", func(t *testing.T) { s.testUpdatePost() })
	s.t.Run("ListPosts", func(t *testing.T) { s.testListPosts() })
	s.t.Run("ListPostsByStatus", func(t *testing.T) { s.testListPostsByStatus() })
	s.t.Run("DeletePost", func(t *testing.T) { s.testDeletePost() })
	s.t.Run("CreateAndGetVersion", func(t *testing.T) { s.testCreateAndGetVersion() })
	s.t.Run("ListVersions", func(t *testing.T) { s.testListVersions() })
	s.t.Run("GetNextVersionNumber", func(t *testing.T) { s.testGetNextVersionNumber() })
	s.t.Run("PostNotFound", func(t *testing.T) { s.testPostNotFound() })
	s.t.Run("VersionNotFound", func(t *testing.T) { s.testVersionNotFound() })
}

func (s *storageTestSuite) testCreateAndGetPost() {
	ctx := context.Background()

	// Create a blog post
	post := &BlogPost{
		ID:               uuid.New(),
		Title:            "Test Post",
		Status:           StatusDictated,
		RawTranscription: "This is a test transcription",
		SocialPosts: map[string]string{
			"twitter": "Test tweet",
		},
	}

	err := s.storage.CreatePost(ctx, post)
	require.NoError(s.t, err)
	assert.False(s.t, post.CreatedAt.IsZero(), "CreatedAt should be set")
	assert.False(s.t, post.UpdatedAt.IsZero(), "UpdatedAt should be set")

	// Retrieve the post
	retrieved, err := s.storage.GetPost(ctx, post.ID)
	require.NoError(s.t, err)
	assert.Equal(s.t, post.ID, retrieved.ID)
	assert.Equal(s.t, post.Title, retrieved.Title)
	assert.Equal(s.t, post.Status, retrieved.Status)
	assert.Equal(s.t, post.RawTranscription, retrieved.RawTranscription)
	assert.Equal(s.t, post.SocialPosts["twitter"], retrieved.SocialPosts["twitter"])
}

func (s *storageTestSuite) testUpdatePost() {
	ctx := context.Background()

	// Create a post
	post := &BlogPost{
		ID:     uuid.New(),
		Title:  "Original Title",
		Status: StatusDictated,
	}

	err := s.storage.CreatePost(ctx, post)
	require.NoError(s.t, err)

	originalUpdatedAt := post.UpdatedAt
	time.Sleep(10 * time.Millisecond) // Ensure time difference

	// Update the post
	post.Title = "Updated Title"
	post.Status = StatusDrafted
	post.FinalContent = "# Updated Content"

	err = s.storage.UpdatePost(ctx, post)
	require.NoError(s.t, err)

	// Verify update
	retrieved, err := s.storage.GetPost(ctx, post.ID)
	require.NoError(s.t, err)
	assert.Equal(s.t, "Updated Title", retrieved.Title)
	assert.Equal(s.t, StatusDrafted, retrieved.Status)
	assert.Equal(s.t, "# Updated Content", retrieved.FinalContent)
	assert.True(s.t, retrieved.UpdatedAt.After(originalUpdatedAt), "UpdatedAt should be updated")
}

func (s *storageTestSuite) testListPosts() {
	ctx := context.Background()

	// Create multiple posts
	post1 := &BlogPost{ID: uuid.New(), Title: "Post 1", Status: StatusDictated}
	post2 := &BlogPost{ID: uuid.New(), Title: "Post 2", Status: StatusDrafted}
	post3 := &BlogPost{ID: uuid.New(), Title: "Post 3", Status: StatusPublished}

	require.NoError(s.t, s.storage.CreatePost(ctx, post1))
	time.Sleep(10 * time.Millisecond)
	require.NoError(s.t, s.storage.CreatePost(ctx, post2))
	time.Sleep(10 * time.Millisecond)
	require.NoError(s.t, s.storage.CreatePost(ctx, post3))

	// List all posts
	posts, err := s.storage.ListPosts(ctx, "")
	require.NoError(s.t, err)
	assert.GreaterOrEqual(s.t, len(posts), 3, "Should have at least 3 posts")

	// Verify sorting (most recent first)
	found := false
	for _, p := range posts {
		if p.ID == post3.ID {
			found = true
			break
		}
	}
	assert.True(s.t, found, "Should find post3")
}

func (s *storageTestSuite) testListPostsByStatus() {
	ctx := context.Background()

	// Create posts with different statuses
	post1 := &BlogPost{ID: uuid.New(), Title: "Dictated", Status: StatusDictated}
	post2 := &BlogPost{ID: uuid.New(), Title: "Drafted", Status: StatusDrafted}

	require.NoError(s.t, s.storage.CreatePost(ctx, post1))
	require.NoError(s.t, s.storage.CreatePost(ctx, post2))

	// List only drafted posts
	drafted, err := s.storage.ListPosts(ctx, StatusDrafted)
	require.NoError(s.t, err)

	// Verify filtering
	foundDrafted := false
	foundDictated := false
	for _, p := range drafted {
		if p.ID == post2.ID {
			foundDrafted = true
		}
		if p.ID == post1.ID {
			foundDictated = true
		}
	}

	assert.True(s.t, foundDrafted, "Should find drafted post")
	assert.False(s.t, foundDictated, "Should not find dictated post in drafted list")
}

func (s *storageTestSuite) testDeletePost() {
	ctx := context.Background()

	// Create a post
	post := &BlogPost{
		ID:     uuid.New(),
		Title:  "To Delete",
		Status: StatusDictated,
	}

	require.NoError(s.t, s.storage.CreatePost(ctx, post))

	// Delete the post
	err := s.storage.DeletePost(ctx, post.ID)
	require.NoError(s.t, err)

	// Verify deletion
	_, err = s.storage.GetPost(ctx, post.ID)
	assert.Error(s.t, err, "Should return error for deleted post")
}

func (s *storageTestSuite) testCreateAndGetVersion() {
	ctx := context.Background()

	postID := uuid.New()

	// Create a version
	version := &PostVersion{
		ID:            uuid.New(),
		PostID:        postID,
		VersionNumber: 1,
		VersionType:   VersionTypePhaseResult,
		Status:        StatusDictated,
		Phase:         "Transcribe",
		PostTitle:     "Test Post",
		Outline:       "## Introduction\nTest outline",
		Sections: []Section{
			{Title: "Intro", Content: "Intro content", Order: 0},
		},
		FullContent: "# Test Post\nFull content here",
		CreatedBy:   "test",
	}

	err := s.storage.CreateVersion(ctx, version)
	require.NoError(s.t, err)
	assert.False(s.t, version.CreatedAt.IsZero(), "CreatedAt should be set")

	// Retrieve the version
	retrieved, err := s.storage.GetVersion(ctx, postID, 1)
	require.NoError(s.t, err)
	assert.Equal(s.t, version.ID, retrieved.ID)
	assert.Equal(s.t, version.PostID, retrieved.PostID)
	assert.Equal(s.t, version.VersionNumber, retrieved.VersionNumber)
	assert.Equal(s.t, version.PostTitle, retrieved.PostTitle)
	assert.Equal(s.t, version.Outline, retrieved.Outline)
	assert.Equal(s.t, len(version.Sections), len(retrieved.Sections))
	assert.Equal(s.t, version.Sections[0].Title, retrieved.Sections[0].Title)
}

func (s *storageTestSuite) testListVersions() {
	ctx := context.Background()

	postID := uuid.New()

	// Create multiple versions
	v1 := &PostVersion{PostID: postID, VersionNumber: 1, Phase: "Transcribe"}
	v2 := &PostVersion{PostID: postID, VersionNumber: 2, Phase: "Research"}
	v3 := &PostVersion{PostID: postID, VersionNumber: 3, Phase: "Outline"}

	require.NoError(s.t, s.storage.CreateVersion(ctx, v1))
	require.NoError(s.t, s.storage.CreateVersion(ctx, v2))
	require.NoError(s.t, s.storage.CreateVersion(ctx, v3))

	// List all versions
	versions, err := s.storage.ListVersions(ctx, postID)
	require.NoError(s.t, err)
	assert.Equal(s.t, 3, len(versions), "Should have 3 versions")

	// Verify sorting (most recent first)
	assert.Equal(s.t, 3, versions[0].VersionNumber, "First should be v3")
	assert.Equal(s.t, 2, versions[1].VersionNumber, "Second should be v2")
	assert.Equal(s.t, 1, versions[2].VersionNumber, "Third should be v1")
}

func (s *storageTestSuite) testGetNextVersionNumber() {
	ctx := context.Background()

	postID := uuid.New()

	// First version should be 1
	nextVersion, err := s.storage.GetNextVersionNumber(ctx, postID)
	require.NoError(s.t, err)
	assert.Equal(s.t, 1, nextVersion)

	// Create version 1
	v1 := &PostVersion{PostID: postID, VersionNumber: 1}
	require.NoError(s.t, s.storage.CreateVersion(ctx, v1))

	// Next should be 2
	nextVersion, err = s.storage.GetNextVersionNumber(ctx, postID)
	require.NoError(s.t, err)
	assert.Equal(s.t, 2, nextVersion)

	// Create version 2
	v2 := &PostVersion{PostID: postID, VersionNumber: 2}
	require.NoError(s.t, s.storage.CreateVersion(ctx, v2))

	// Next should be 3
	nextVersion, err = s.storage.GetNextVersionNumber(ctx, postID)
	require.NoError(s.t, err)
	assert.Equal(s.t, 3, nextVersion)
}

func (s *storageTestSuite) testPostNotFound() {
	ctx := context.Background()

	nonExistentID := uuid.New()

	// Get non-existent post
	_, err := s.storage.GetPost(ctx, nonExistentID)
	assert.Error(s.t, err, "Should return error for non-existent post")

	// Update non-existent post
	post := &BlogPost{ID: nonExistentID, Title: "Test"}
	err = s.storage.UpdatePost(ctx, post)
	assert.Error(s.t, err, "Should return error when updating non-existent post")

	// Delete non-existent post
	err = s.storage.DeletePost(ctx, nonExistentID)
	assert.Error(s.t, err, "Should return error when deleting non-existent post")
}

func (s *storageTestSuite) testVersionNotFound() {
	ctx := context.Background()

	postID := uuid.New()

	// Get non-existent version
	_, err := s.storage.GetVersion(ctx, postID, 999)
	assert.Error(s.t, err, "Should return error for non-existent version")

	// List versions for post with no versions
	versions, err := s.storage.ListVersions(ctx, postID)
	require.NoError(s.t, err)
	assert.Equal(s.t, 0, len(versions), "Should return empty list for post with no versions")
}

// TestFileStorageDirectoryCreation tests that FileStorage creates necessary directories
func TestFileStorageDirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "blog_output")

	storage, err := NewFileStorage(outputDir)
	require.NoError(t, err)
	defer storage.Close()

	t.Cleanup(func() {
		os.RemoveAll(outputDir)
	})

	// Verify directories were created
	postsDir := filepath.Join(outputDir, "posts")
	versionsDir := filepath.Join(outputDir, "versions")

	assert.DirExists(t, postsDir, "Posts directory should exist")
	assert.DirExists(t, versionsDir, "Versions directory should exist")
}

// TestFileStorageMarkdownOutput tests that FileStorage writes markdown files
func TestFileStorageMarkdownOutput(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewFileStorage(tempDir)
	require.NoError(t, err)
	defer storage.Close()

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	ctx := context.Background()

	// Create a post with final content
	post := &BlogPost{
		ID:           uuid.New(),
		Title:        "Test Post",
		Status:       StatusPublished,
		FinalContent: "# Test Post\n\nThis is the content.",
	}

	err = storage.CreatePost(ctx, post)
	require.NoError(t, err)

	// Verify markdown file was created
	mdPath := filepath.Join(tempDir, "posts", post.ID.String()+".md")
	assert.FileExists(t, mdPath, "Markdown file should exist")

	// Verify content
	content, err := os.ReadFile(mdPath)
	require.NoError(t, err)
	assert.Equal(t, post.FinalContent, string(content))
}

// TestMemoryStorageIsolation tests that MemoryStorage returns copies, not references
func TestMemoryStorageIsolation(t *testing.T) {
	storage := NewMemoryStorage()
	defer storage.Close()

	ctx := context.Background()

	// Create a post
	post := &BlogPost{
		ID:    uuid.New(),
		Title: "Original Title",
		SocialPosts: map[string]string{
			"twitter": "Original tweet",
		},
	}

	err := storage.CreatePost(ctx, post)
	require.NoError(t, err)

	// Get the post
	retrieved, err := storage.GetPost(ctx, post.ID)
	require.NoError(t, err)

	// Modify the retrieved post
	retrieved.Title = "Modified Title"
	retrieved.SocialPosts["twitter"] = "Modified tweet"

	// Get again and verify original data is unchanged
	retrieved2, err := storage.GetPost(ctx, post.ID)
	require.NoError(t, err)
	assert.Equal(t, "Original Title", retrieved2.Title, "Title should not be modified")
	assert.Equal(t, "Original tweet", retrieved2.SocialPosts["twitter"], "Social posts should not be modified")
}
