package httpbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/soypete/pedrocli/pkg/jobs"
)

// SSEClient represents a connected SSE client
type SSEClient struct {
	ID      string
	JobID   string          // Specific job to watch, or "*" for all jobs
	Channel chan SSEMessage // Buffered channel for messages
	done    chan struct{}   // Signal when client disconnects
}

// SSEMessage represents a message to send via SSE
type SSEMessage struct {
	Event string      `json:"event"` // "update", "complete", "error"
	Data  interface{} `json:"data"`
}

// SSEBroadcaster manages SSE connections and broadcasts updates
type SSEBroadcaster struct {
	clients    map[string]*SSEClient
	mutex      sync.RWMutex
	jobManager jobs.JobManager
	ctx        context.Context
	lastStatus map[string]string // jobID -> last known status
}

// NewSSEBroadcaster creates a new SSE broadcaster
func NewSSEBroadcaster(jobManager jobs.JobManager, ctx context.Context) *SSEBroadcaster {
	return &SSEBroadcaster{
		clients:    make(map[string]*SSEClient),
		jobManager: jobManager,
		ctx:        ctx,
		lastStatus: make(map[string]string),
	}
}

// AddClient adds a new SSE client
func (b *SSEBroadcaster) AddClient(jobID string) *SSEClient {
	client := &SSEClient{
		ID:      uuid.New().String(),
		JobID:   jobID,
		Channel: make(chan SSEMessage, 10), // Buffer 10 messages
		done:    make(chan struct{}),
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.clients[client.ID] = client

	return client
}

// RemoveClient removes an SSE client
func (b *SSEBroadcaster) RemoveClient(clientID string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if client, exists := b.clients[clientID]; exists {
		close(client.done)
		close(client.Channel)
		delete(b.clients, clientID)
	}
}

// Broadcast sends a message to all clients watching a specific job or all jobs
func (b *SSEBroadcaster) Broadcast(jobID string, message SSEMessage) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	for _, client := range b.clients {
		// Send to clients watching this specific job or all jobs
		if client.JobID == jobID || client.JobID == "*" {
			select {
			case client.Channel <- message:
				// Message sent successfully
			default:
				// Channel full, skip (client will catch up on next poll)
			}
		}
	}
}

// StartPolling starts background polling for job updates
func (b *SSEBroadcaster) StartPolling(pollInterval time.Duration) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			b.pollJobs()
		}
	}
}

// pollJobs checks job statuses and broadcasts updates
func (b *SSEBroadcaster) pollJobs() {
	// Get list of all jobs
	jobList, err := b.jobManager.List(b.ctx)
	if err != nil {
		fmt.Printf("Error polling jobs: %v\n", err)
		return
	}

	// Broadcast full job list to clients watching all jobs
	b.Broadcast("*", SSEMessage{
		Event: "list",
		Data:  jobList,
	})

	// Check status of each tracked job
	b.mutex.RLock()
	trackedJobs := make(map[string]bool)
	for _, client := range b.clients {
		if client.JobID != "*" {
			trackedJobs[client.JobID] = true
		}
	}
	b.mutex.RUnlock()

	for jobID := range trackedJobs {
		b.checkJobStatus(jobID)
	}
}

// checkJobStatus checks a single job's status and broadcasts if changed
func (b *SSEBroadcaster) checkJobStatus(jobID string) {
	job, err := b.jobManager.Get(b.ctx, jobID)
	if err != nil {
		return
	}

	currentStatus := string(job.Status)

	// Check if status changed
	b.mutex.Lock()
	lastStatus, exists := b.lastStatus[jobID]
	if !exists || lastStatus != currentStatus {
		b.lastStatus[jobID] = currentStatus
		b.mutex.Unlock()

		// Broadcast update
		b.Broadcast(jobID, SSEMessage{
			Event: "update",
			Data:  job,
		})
	} else {
		b.mutex.Unlock()
	}
}

// ServeHTTP handles SSE connections for a specific job
func (b *SSEBroadcaster) ServeHTTP(w http.ResponseWriter, r *http.Request, jobID string) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create client
	client := b.AddClient(jobID)
	defer b.RemoveClient(client.ID)

	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send initial status
	b.sendInitialStatus(w, flusher, jobID)

	// Stream updates
	for {
		select {
		case <-r.Context().Done():
			// Client disconnected
			return
		case <-client.done:
			// Client removed
			return
		case msg := <-client.Channel:
			// Send SSE message
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}

			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", msg.Event, string(data))
			flusher.Flush()
		}
	}
}

// sendInitialStatus sends the current job status when client first connects
func (b *SSEBroadcaster) sendInitialStatus(w http.ResponseWriter, flusher http.Flusher, jobID string) {
	if jobID == "*" {
		// Send full job list
		jobList, err := b.jobManager.List(b.ctx)
		if err == nil {
			msg := SSEMessage{
				Event: "list",
				Data:  jobList,
			}
			data, _ := json.Marshal(msg)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", msg.Event, string(data))
			flusher.Flush()
		}
	} else {
		// Send specific job status
		job, err := b.jobManager.Get(b.ctx, jobID)
		if err == nil {
			msg := SSEMessage{
				Event: "update",
				Data:  job,
			}
			data, _ := json.Marshal(msg)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", msg.Event, string(data))
			flusher.Flush()
		}
	}
}
