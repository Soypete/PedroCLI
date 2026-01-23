package blog

import (
	"context"

	"github.com/google/uuid"
)

// BlogStorage is the interface for blog post and version storage
// It abstracts the underlying storage mechanism (database, files, memory)
// allowing the BlogContentAgent to work independently of storage implementation
type BlogStorage interface {
	// Post operations
	CreatePost(ctx context.Context, post *BlogPost) error
	GetPost(ctx context.Context, id uuid.UUID) (*BlogPost, error)
	UpdatePost(ctx context.Context, post *BlogPost) error
	ListPosts(ctx context.Context, status PostStatus) ([]*BlogPost, error)
	DeletePost(ctx context.Context, id uuid.UUID) error

	// Version operations
	CreateVersion(ctx context.Context, v *PostVersion) error
	GetVersion(ctx context.Context, postID uuid.UUID, versionNumber int) (*PostVersion, error)
	GetNextVersionNumber(ctx context.Context, postID uuid.UUID) (int, error)
	ListVersions(ctx context.Context, postID uuid.UUID) ([]*PostVersion, error)

	// Lifecycle
	Close() error
}
