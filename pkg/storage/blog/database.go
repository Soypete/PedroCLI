package blog

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

// DatabaseStorage implements BlogStorage using PostgreSQL
// It wraps the existing PostStore and VersionStore implementations
type DatabaseStorage struct {
	db           *sql.DB
	postStore    *PostStore
	versionStore *VersionStore
}

// NewDatabaseStorage creates a new database-backed storage
func NewDatabaseStorage(db *sql.DB) BlogStorage {
	if db == nil {
		panic("database connection cannot be nil")
	}

	return &DatabaseStorage{
		db:           db,
		postStore:    NewPostStore(db),
		versionStore: NewVersionStore(db),
	}
}

// Post operations

func (s *DatabaseStorage) CreatePost(ctx context.Context, post *BlogPost) error {
	return s.postStore.Create(post)
}

func (s *DatabaseStorage) GetPost(ctx context.Context, id uuid.UUID) (*BlogPost, error) {
	return s.postStore.Get(id)
}

func (s *DatabaseStorage) UpdatePost(ctx context.Context, post *BlogPost) error {
	return s.postStore.Update(post)
}

func (s *DatabaseStorage) ListPosts(ctx context.Context, status PostStatus) ([]*BlogPost, error) {
	return s.postStore.List(status)
}

func (s *DatabaseStorage) DeletePost(ctx context.Context, id uuid.UUID) error {
	return s.postStore.Delete(id)
}

// Version operations

func (s *DatabaseStorage) CreateVersion(ctx context.Context, v *PostVersion) error {
	return s.versionStore.CreateVersion(ctx, v)
}

func (s *DatabaseStorage) GetVersion(ctx context.Context, postID uuid.UUID, versionNumber int) (*PostVersion, error) {
	return s.versionStore.GetVersion(ctx, postID, versionNumber)
}

func (s *DatabaseStorage) GetNextVersionNumber(ctx context.Context, postID uuid.UUID) (int, error) {
	return s.versionStore.GetNextVersionNumber(ctx, postID)
}

func (s *DatabaseStorage) ListVersions(ctx context.Context, postID uuid.UUID) ([]*PostVersion, error) {
	return s.versionStore.ListVersions(ctx, postID)
}

// Lifecycle

func (s *DatabaseStorage) Close() error {
	// Database connection is managed externally, don't close here
	return nil
}
