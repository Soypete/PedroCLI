// Package jobs provides job management for PedroCLI agents.
package jobs

import (
	"context"
	"time"

	"github.com/soypete/pedrocli/pkg/storage"
)

// JobManager defines the interface for job management operations.
// This interface allows for multiple implementations:
// - File-based manager (legacy, deprecated)
// - Database-backed manager (recommended)
type JobManager interface {
	// Create creates a new job and returns it.
	Create(ctx context.Context, jobType, description string, input map[string]interface{}) (*Job, error)

	// Get retrieves a job by ID.
	Get(ctx context.Context, id string) (*Job, error)

	// List returns all jobs.
	List(ctx context.Context) ([]*Job, error)

	// Update updates a job's status and output.
	Update(ctx context.Context, id string, status Status, output map[string]interface{}, err error) error

	// Cancel cancels a job.
	Cancel(ctx context.Context, id string) error

	// CleanupOldJobs removes completed/failed/cancelled jobs older than the specified duration.
	CleanupOldJobs(ctx context.Context, olderThan time.Duration) error

	// SetWorkDir sets the working directory for a job.
	SetWorkDir(ctx context.Context, id string, workDir string) error

	// SetContextDir sets the context directory for a job.
	SetContextDir(ctx context.Context, id string, contextDir string) error

	// Conversation history for debugging
	AppendConversation(ctx context.Context, id string, entry storage.ConversationEntry) error
	GetConversation(ctx context.Context, id string) ([]storage.ConversationEntry, error)
}

// Verify that Manager implements JobManager (compile-time check).
var _ JobManager = (*Manager)(nil)
