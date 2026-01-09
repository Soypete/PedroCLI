package blog

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PostStatus represents the status of a blog post
type PostStatus string

const (
	StatusDictated  PostStatus = "dictated"
	StatusDrafted   PostStatus = "drafted"
	StatusEdited    PostStatus = "edited"
	StatusPublished PostStatus = "published"
	StatusPublic    PostStatus = "public"
)

// BlogPost represents a blog post in the pipeline
type BlogPost struct {
	ID                        uuid.UUID              `json:"id"`
	Title                     string                 `json:"title"`
	Status                    PostStatus             `json:"status"`
	RawTranscription          string                 `json:"raw_transcription,omitempty"`
	TranscriptionDurationSecs int                    `json:"transcription_duration_seconds,omitempty"`
	WriterOutput              string                 `json:"writer_output,omitempty"`
	EditorOutput              string                 `json:"editor_output,omitempty"`
	FinalContent              string                 `json:"final_content,omitempty"`
	NewsletterAddendum        map[string]interface{} `json:"newsletter_addendum,omitempty"`
	SocialPosts               map[string]string      `json:"social_posts,omitempty"`
	NotionPageID              string                 `json:"notion_page_id,omitempty"`
	SubstackURL               string                 `json:"substack_url,omitempty"`
	PaywallUntil              *time.Time             `json:"paywall_until,omitempty"`
	CurrentVersion            int                    `json:"current_version,omitempty"`
	CreatedAt                 time.Time              `json:"created_at"`
	UpdatedAt                 time.Time              `json:"updated_at"`
}

// PostStore handles blog post database operations
type PostStore struct {
	db *sql.DB
}

// NewPostStore creates a new post store
func NewPostStore(db *sql.DB) *PostStore {
	return &PostStore{db: db}
}

// Create creates a new blog post
func (s *PostStore) Create(post *BlogPost) error {
	if post.ID == uuid.Nil {
		post.ID = uuid.New()
	}
	if post.Status == "" {
		post.Status = StatusDictated
	}

	var addendumJSON []byte
	if post.NewsletterAddendum != nil {
		var err error
		addendumJSON, err = json.Marshal(post.NewsletterAddendum)
		if err != nil {
			return fmt.Errorf("failed to marshal newsletter addendum: %w", err)
		}
	}

	var socialPostsJSON []byte
	if post.SocialPosts != nil {
		var err error
		socialPostsJSON, err = json.Marshal(post.SocialPosts)
		if err != nil {
			return fmt.Errorf("failed to marshal social posts: %w", err)
		}
	}

	query := `
		INSERT INTO blog_posts (
			id, title, status, raw_transcription, transcription_duration_seconds,
			writer_output, editor_output, final_content, newsletter_addendum,
			social_posts, notion_page_id, substack_url, paywall_until
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING created_at, updated_at
	`

	err := s.db.QueryRow(
		query,
		post.ID, post.Title, post.Status, post.RawTranscription,
		post.TranscriptionDurationSecs, post.WriterOutput, post.EditorOutput,
		post.FinalContent, addendumJSON, socialPostsJSON, post.NotionPageID, post.SubstackURL,
		post.PaywallUntil,
	).Scan(&post.CreatedAt, &post.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create blog post: %w", err)
	}

	return nil
}

// Get retrieves a blog post by ID
func (s *PostStore) Get(id uuid.UUID) (*BlogPost, error) {
	query := `
		SELECT id, title, status, raw_transcription, transcription_duration_seconds,
		       writer_output, editor_output, final_content, newsletter_addendum,
		       social_posts, notion_page_id, substack_url, paywall_until,
		       current_version, created_at, updated_at
		FROM blog_posts
		WHERE id = $1
	`

	post := &BlogPost{}
	var addendumJSON []byte
	var socialPostsJSON []byte

	err := s.db.QueryRow(query, id).Scan(
		&post.ID, &post.Title, &post.Status, &post.RawTranscription,
		&post.TranscriptionDurationSecs, &post.WriterOutput, &post.EditorOutput,
		&post.FinalContent, &addendumJSON, &socialPostsJSON, &post.NotionPageID, &post.SubstackURL,
		&post.PaywallUntil, &post.CurrentVersion, &post.CreatedAt, &post.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("blog post not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get blog post: %w", err)
	}

	if addendumJSON != nil && len(addendumJSON) > 0 {
		if err := json.Unmarshal(addendumJSON, &post.NewsletterAddendum); err != nil {
			return nil, fmt.Errorf("failed to unmarshal newsletter addendum: %w", err)
		}
	}

	if socialPostsJSON != nil && len(socialPostsJSON) > 0 {
		if err := json.Unmarshal(socialPostsJSON, &post.SocialPosts); err != nil {
			return nil, fmt.Errorf("failed to unmarshal social posts: %w", err)
		}
	}

	return post, nil
}

// Update updates a blog post
func (s *PostStore) Update(post *BlogPost) error {
	var addendumJSON []byte
	if post.NewsletterAddendum != nil {
		var err error
		addendumJSON, err = json.Marshal(post.NewsletterAddendum)
		if err != nil {
			return fmt.Errorf("failed to marshal newsletter addendum: %w", err)
		}
	}

	var socialPostsJSON []byte
	if post.SocialPosts != nil {
		var err error
		socialPostsJSON, err = json.Marshal(post.SocialPosts)
		if err != nil {
			return fmt.Errorf("failed to marshal social posts: %w", err)
		}
	}

	query := `
		UPDATE blog_posts SET
			title = $2,
			status = $3,
			raw_transcription = $4,
			transcription_duration_seconds = $5,
			writer_output = $6,
			editor_output = $7,
			final_content = $8,
			newsletter_addendum = $9,
			social_posts = $10,
			notion_page_id = $11,
			substack_url = $12,
			paywall_until = $13,
			current_version = $14
		WHERE id = $1
		RETURNING updated_at
	`

	err := s.db.QueryRow(
		query,
		post.ID, post.Title, post.Status, post.RawTranscription,
		post.TranscriptionDurationSecs, post.WriterOutput, post.EditorOutput,
		post.FinalContent, addendumJSON, socialPostsJSON, post.NotionPageID, post.SubstackURL,
		post.PaywallUntil, post.CurrentVersion,
	).Scan(&post.UpdatedAt)

	if err == sql.ErrNoRows {
		return fmt.Errorf("blog post not found: %s", post.ID)
	}
	if err != nil {
		return fmt.Errorf("failed to update blog post: %w", err)
	}

	return nil
}

// UpdateStatus updates just the status of a blog post
func (s *PostStore) UpdateStatus(id uuid.UUID, status PostStatus) error {
	query := `UPDATE blog_posts SET status = $2 WHERE id = $1`
	result, err := s.db.Exec(query, id, status)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("blog post not found: %s", id)
	}

	return nil
}

// List returns all blog posts, optionally filtered by status
func (s *PostStore) List(status PostStatus) ([]*BlogPost, error) {
	var query string
	var rows *sql.Rows
	var err error

	if status != "" {
		query = `
			SELECT id, title, status, raw_transcription, transcription_duration_seconds,
			       writer_output, editor_output, final_content, newsletter_addendum,
			       social_posts, notion_page_id, substack_url, paywall_until,
			       current_version, created_at, updated_at
			FROM blog_posts
			WHERE status = $1
			ORDER BY created_at DESC
		`
		rows, err = s.db.Query(query, status)
	} else {
		query = `
			SELECT id, title, status, raw_transcription, transcription_duration_seconds,
			       writer_output, editor_output, final_content, newsletter_addendum,
			       social_posts, notion_page_id, substack_url, paywall_until,
			       current_version, created_at, updated_at
			FROM blog_posts
			ORDER BY created_at DESC
		`
		rows, err = s.db.Query(query)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list blog posts: %w", err)
	}
	defer rows.Close()

	var posts []*BlogPost
	for rows.Next() {
		post := &BlogPost{}
		var addendumJSON []byte
		var socialPostsJSON []byte

		err := rows.Scan(
			&post.ID, &post.Title, &post.Status, &post.RawTranscription,
			&post.TranscriptionDurationSecs, &post.WriterOutput, &post.EditorOutput,
			&post.FinalContent, &addendumJSON, &socialPostsJSON, &post.NotionPageID, &post.SubstackURL,
			&post.PaywallUntil, &post.CurrentVersion, &post.CreatedAt, &post.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan blog post: %w", err)
		}

		if addendumJSON != nil && len(addendumJSON) > 0 {
			if err := json.Unmarshal(addendumJSON, &post.NewsletterAddendum); err != nil {
				return nil, fmt.Errorf("failed to unmarshal newsletter addendum: %w", err)
			}
		}

		if socialPostsJSON != nil && len(socialPostsJSON) > 0 {
			if err := json.Unmarshal(socialPostsJSON, &post.SocialPosts); err != nil {
				return nil, fmt.Errorf("failed to unmarshal social posts: %w", err)
			}
		}

		posts = append(posts, post)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating blog posts: %w", err)
	}

	return posts, nil
}

// Delete deletes a blog post
func (s *PostStore) Delete(id uuid.UUID) error {
	query := `DELETE FROM blog_posts WHERE id = $1`
	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete blog post: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("blog post not found: %s", id)
	}

	return nil
}

// GetPostsWithExpiredPaywall returns posts whose paywall has expired
func (s *PostStore) GetPostsWithExpiredPaywall() ([]*BlogPost, error) {
	query := `
		SELECT id, title, status, raw_transcription, transcription_duration_seconds,
		       writer_output, editor_output, final_content, newsletter_addendum,
		       notion_page_id, substack_url, paywall_until, created_at, updated_at
		FROM blog_posts
		WHERE paywall_until IS NOT NULL
		  AND paywall_until < NOW()
		  AND status = $1
		ORDER BY paywall_until
	`

	rows, err := s.db.Query(query, StatusPublished)
	if err != nil {
		return nil, fmt.Errorf("failed to get posts with expired paywall: %w", err)
	}
	defer rows.Close()

	var posts []*BlogPost
	for rows.Next() {
		post := &BlogPost{}
		var addendumJSON []byte

		err := rows.Scan(
			&post.ID, &post.Title, &post.Status, &post.RawTranscription,
			&post.TranscriptionDurationSecs, &post.WriterOutput, &post.EditorOutput,
			&post.FinalContent, &addendumJSON, &post.NotionPageID, &post.SubstackURL,
			&post.PaywallUntil, &post.CreatedAt, &post.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan blog post: %w", err)
		}

		if addendumJSON != nil {
			if err := json.Unmarshal(addendumJSON, &post.NewsletterAddendum); err != nil {
				return nil, fmt.Errorf("failed to unmarshal newsletter addendum: %w", err)
			}
		}

		posts = append(posts, post)
	}

	return posts, rows.Err()
}
