package blog

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryStorage implements BlogStorage using in-memory storage
// It's primarily used for unit testing
type MemoryStorage struct {
	mu       sync.RWMutex
	posts    map[uuid.UUID]*BlogPost
	versions map[uuid.UUID]map[int]*PostVersion // postID -> versionNumber -> version
}

// NewMemoryStorage creates a new in-memory storage
func NewMemoryStorage() BlogStorage {
	return &MemoryStorage{
		posts:    make(map[uuid.UUID]*BlogPost),
		versions: make(map[uuid.UUID]map[int]*PostVersion),
	}
}

// Post operations

func (s *MemoryStorage) CreatePost(ctx context.Context, post *BlogPost) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if post.ID == uuid.Nil {
		post.ID = uuid.New()
	}

	// Check if post already exists
	if _, exists := s.posts[post.ID]; exists {
		return fmt.Errorf("blog post already exists: %s", post.ID)
	}

	if post.CreatedAt.IsZero() {
		post.CreatedAt = time.Now()
	}
	if post.UpdatedAt.IsZero() {
		post.UpdatedAt = time.Now()
	}

	// Deep copy to avoid external modifications
	s.posts[post.ID] = s.copyPost(post)

	return nil
}

func (s *MemoryStorage) GetPost(ctx context.Context, id uuid.UUID) (*BlogPost, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	post, exists := s.posts[id]
	if !exists {
		return nil, fmt.Errorf("blog post not found: %s", id)
	}

	// Return a copy to avoid external modifications
	return s.copyPost(post), nil
}

func (s *MemoryStorage) UpdatePost(ctx context.Context, post *BlogPost) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.posts[post.ID]; !exists {
		return fmt.Errorf("blog post not found: %s", post.ID)
	}

	post.UpdatedAt = time.Now()

	// Deep copy to avoid external modifications
	s.posts[post.ID] = s.copyPost(post)

	return nil
}

func (s *MemoryStorage) ListPosts(ctx context.Context, status PostStatus) ([]*BlogPost, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var posts []*BlogPost
	for _, post := range s.posts {
		// Filter by status if specified
		if status != "" && post.Status != status {
			continue
		}

		posts = append(posts, s.copyPost(post))
	}

	// Sort by created_at descending (most recent first)
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].CreatedAt.After(posts[j].CreatedAt)
	})

	return posts, nil
}

func (s *MemoryStorage) DeletePost(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.posts[id]; !exists {
		return fmt.Errorf("blog post not found: %s", id)
	}

	delete(s.posts, id)
	delete(s.versions, id)

	return nil
}

// Version operations

func (s *MemoryStorage) CreateVersion(ctx context.Context, v *PostVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}

	// Initialize versions map for this post if needed
	if _, exists := s.versions[v.PostID]; !exists {
		s.versions[v.PostID] = make(map[int]*PostVersion)
	}

	// Check if version already exists
	if _, exists := s.versions[v.PostID][v.VersionNumber]; exists {
		return fmt.Errorf("version %d already exists for post %s", v.VersionNumber, v.PostID)
	}

	// Deep copy to avoid external modifications
	s.versions[v.PostID][v.VersionNumber] = s.copyVersion(v)

	return nil
}

func (s *MemoryStorage) GetVersion(ctx context.Context, postID uuid.UUID, versionNumber int) (*PostVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, exists := s.versions[postID]
	if !exists {
		return nil, fmt.Errorf("no versions found for post %s", postID)
	}

	version, exists := versions[versionNumber]
	if !exists {
		return nil, fmt.Errorf("version %d not found for post %s", versionNumber, postID)
	}

	// Return a copy to avoid external modifications
	return s.copyVersion(version), nil
}

func (s *MemoryStorage) GetNextVersionNumber(ctx context.Context, postID uuid.UUID) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, exists := s.versions[postID]
	if !exists || len(versions) == 0 {
		return 1, nil
	}

	maxVersion := 0
	for versionNum := range versions {
		if versionNum > maxVersion {
			maxVersion = versionNum
		}
	}

	return maxVersion + 1, nil
}

func (s *MemoryStorage) ListVersions(ctx context.Context, postID uuid.UUID) ([]*PostVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, exists := s.versions[postID]
	if !exists {
		return []*PostVersion{}, nil
	}

	var result []*PostVersion
	for _, version := range versions {
		result = append(result, s.copyVersion(version))
	}

	// Sort by version number descending (most recent first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].VersionNumber > result[j].VersionNumber
	})

	return result, nil
}

// Lifecycle

func (s *MemoryStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear all data
	s.posts = make(map[uuid.UUID]*BlogPost)
	s.versions = make(map[uuid.UUID]map[int]*PostVersion)

	return nil
}

// Helper methods for deep copying

func (s *MemoryStorage) copyPost(post *BlogPost) *BlogPost {
	if post == nil {
		return nil
	}

	copy := &BlogPost{
		ID:                        post.ID,
		Title:                     post.Title,
		Status:                    post.Status,
		RawTranscription:          post.RawTranscription,
		TranscriptionDurationSecs: post.TranscriptionDurationSecs,
		WriterOutput:              post.WriterOutput,
		EditorOutput:              post.EditorOutput,
		FinalContent:              post.FinalContent,
		NotionPageID:              post.NotionPageID,
		SubstackURL:               post.SubstackURL,
		CurrentVersion:            post.CurrentVersion,
		CreatedAt:                 post.CreatedAt,
		UpdatedAt:                 post.UpdatedAt,
	}

	// Copy PaywallUntil if present
	if post.PaywallUntil != nil {
		t := *post.PaywallUntil
		copy.PaywallUntil = &t
	}

	// Copy maps
	if post.NewsletterAddendum != nil {
		copy.NewsletterAddendum = make(map[string]interface{})
		for k, v := range post.NewsletterAddendum {
			copy.NewsletterAddendum[k] = v
		}
	}

	if post.SocialPosts != nil {
		copy.SocialPosts = make(map[string]string)
		for k, v := range post.SocialPosts {
			copy.SocialPosts[k] = v
		}
	}

	return copy
}

func (s *MemoryStorage) copyVersion(v *PostVersion) *PostVersion {
	if v == nil {
		return nil
	}

	copy := &PostVersion{
		ID:               v.ID,
		PostID:           v.PostID,
		VersionNumber:    v.VersionNumber,
		VersionType:      v.VersionType,
		Status:           v.Status,
		Phase:            v.Phase,
		PostTitle:        v.PostTitle,
		Title:            v.Title,
		RawTranscription: v.RawTranscription,
		Outline:          v.Outline,
		FullContent:      v.FullContent,
		CreatedBy:        v.CreatedBy,
		CreatedAt:        v.CreatedAt,
		ChangeNotes:      v.ChangeNotes,
	}

	// Copy sections slice
	if v.Sections != nil {
		copy.Sections = make([]Section, len(v.Sections))
		for i, sec := range v.Sections {
			copy.Sections[i] = Section{
				Title:   sec.Title,
				Content: sec.Content,
				Order:   sec.Order,
			}
		}
	}

	return copy
}
