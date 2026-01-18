package content

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ContentStore abstracts storage for agent-generated content.
// This allows the same agent code to work with both PostgreSQL (Web UI)
// and file-based storage (CLI) without code duplication.
type ContentStore interface {
	// Create stores new content
	Create(ctx context.Context, content *Content) error

	// Get retrieves content by ID
	Get(ctx context.Context, id uuid.UUID) (*Content, error)

	// Update modifies existing content
	Update(ctx context.Context, content *Content) error

	// List retrieves content matching filter
	List(ctx context.Context, filter Filter) ([]*Content, error)

	// Delete removes content
	Delete(ctx context.Context, id uuid.UUID) error
}

// VersionStore abstracts version history storage.
// Versions are snapshots of content at specific workflow phases.
type VersionStore interface {
	// SaveVersion stores a snapshot at a specific phase
	SaveVersion(ctx context.Context, version *Version) error

	// GetVersion retrieves a specific version
	GetVersion(ctx context.Context, contentID uuid.UUID, versionNum int) (*Version, error)

	// ListVersions retrieves all versions for content
	ListVersions(ctx context.Context, contentID uuid.UUID) ([]*Version, error)

	// DeleteVersions removes all versions for content
	DeleteVersions(ctx context.Context, contentID uuid.UUID) error
}

// Content represents any agent-generated content (blog, podcast, code changes)
type Content struct {
	ID        uuid.UUID              `json:"id"`
	Type      ContentType            `json:"type"`
	Status    Status                 `json:"status"`
	Title     string                 `json:"title"`
	Data      map[string]interface{} `json:"data"` // Flexible schema for type-specific fields
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// Version represents a snapshot at a specific workflow phase
type Version struct {
	ID         uuid.UUID              `json:"id"`
	ContentID  uuid.UUID              `json:"content_id"`
	Phase      string                 `json:"phase"`       // e.g., "outline", "draft", "final"
	VersionNum int                    `json:"version_num"` // Incremental version number
	Snapshot   map[string]interface{} `json:"snapshot"`    // Phase-specific data
	CreatedAt  time.Time              `json:"created_at"`
}

// ContentType identifies the type of content
type ContentType string

const (
	ContentTypeBlog    ContentType = "blog"
	ContentTypePodcast ContentType = "podcast"
	ContentTypeCode    ContentType = "code"
)

// Status represents the lifecycle state of content
type Status string

const (
	StatusDraft      Status = "draft"       // Initial creation
	StatusInProgress Status = "in_progress" // Agent is working on it
	StatusReview     Status = "review"      // Ready for human review
	StatusPublished  Status = "published"   // Final/published state
	StatusFailed     Status = "failed"      // Workflow failed
)

// Filter provides query options for listing content
type Filter struct {
	Type       *ContentType `json:"type,omitempty"`
	Status     *Status      `json:"status,omitempty"`
	Limit      int          `json:"limit,omitempty"`
	Offset     int          `json:"offset,omitempty"`
	SearchText string       `json:"search_text,omitempty"` // Search in title or data
	Since      *time.Time   `json:"since,omitempty"`       // Created after this time
	Until      *time.Time   `json:"until,omitempty"`       // Created before this time
}

// DefaultFilter returns a filter with sensible defaults
func DefaultFilter() Filter {
	return Filter{
		Limit:  50,
		Offset: 0,
	}
}

// WithType sets the content type filter
func (f Filter) WithType(t ContentType) Filter {
	f.Type = &t
	return f
}

// WithStatus sets the status filter
func (f Filter) WithStatus(s Status) Filter {
	f.Status = &s
	return f
}

// WithLimit sets the limit
func (f Filter) WithLimit(limit int) Filter {
	f.Limit = limit
	return f
}

// WithOffset sets the offset for pagination
func (f Filter) WithOffset(offset int) Filter {
	f.Offset = offset
	return f
}

// WithSearchText sets the search text
func (f Filter) WithSearchText(text string) Filter {
	f.SearchText = text
	return f
}

// WithSince filters content created after the specified time
func (f Filter) WithSince(t time.Time) Filter {
	f.Since = &t
	return f
}

// WithUntil filters content created before the specified time
func (f Filter) WithUntil(t time.Time) Filter {
	f.Until = &t
	return f
}
