// Package queue implements a Postgres-native job queue using LISTEN/NOTIFY
// with SKIP LOCKED claim semantics.
package queue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/soypete/pedrocli/pkg/db"
)

// HandlerFunc processes a claimed job. Return nil on success.
type HandlerFunc func(ctx context.Context, job db.StudyJob) error

// Worker listens for new jobs and dispatches them to registered handlers.
type Worker struct {
	db       *db.DB
	pool     *pgxpool.Pool
	handlers map[db.StudyJobType]HandlerFunc
	pollIvl  time.Duration
}

// NewWorker creates a Worker. The pool is used for the dedicated LISTEN connection.
func NewWorker(database *db.DB) *Worker {
	return &Worker{
		db:       database,
		pool:     database.Pool,
		handlers: make(map[db.StudyJobType]HandlerFunc),
		pollIvl:  30 * time.Second,
	}
}

// Register adds a handler for a job type.
func (w *Worker) Register(jobType db.StudyJobType, fn HandlerFunc) {
	w.handlers[jobType] = fn
}

// Start runs the claim-and-process loop. It blocks until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) error {
	// Acquire a dedicated connection for LISTEN.
	conn, err := w.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire listen conn: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN job_ready"); err != nil {
		return fmt.Errorf("LISTEN: %w", err)
	}

	log.Println("queue: worker started, listening for jobs")

	for {
		// Wait for notification or poll timeout.
		waitCtx, cancel := context.WithTimeout(ctx, w.pollIvl)
		_, err := conn.Conn().WaitForNotification(waitCtx)
		cancel()

		if ctx.Err() != nil {
			log.Println("queue: shutting down")
			return ctx.Err()
		}
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			log.Printf("queue: notification error: %v", err)
		}

		// Drain all available jobs.
		w.drainJobs(ctx)
	}
}

func (w *Worker) drainJobs(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		job, err := w.db.ClaimJob(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return // no more pending jobs
		}
		if err != nil {
			log.Printf("queue: claim error: %v", err)
			return
		}
		w.processJob(ctx, job)
	}
}

func (w *Worker) processJob(ctx context.Context, job db.StudyJob) {
	handler, ok := w.handlers[job.JobType]
	if !ok {
		log.Printf("queue: no handler for job type %s (job %s)", job.JobType, job.ID)
		errMsg := fmt.Sprintf("no handler registered for job type %s", job.JobType)
		_ = w.db.MarkJobError(ctx, job.ID, errMsg)
		return
	}

	if err := handler(ctx, job); err != nil {
		log.Printf("queue: job %s failed: %v", job.ID, err)
		_ = w.db.MarkJobError(ctx, job.ID, err.Error())
		return
	}

	if err := w.db.MarkJobDone(ctx, job.ID); err != nil {
		log.Printf("queue: failed to mark job %s done: %v", job.ID, err)
	}
}
