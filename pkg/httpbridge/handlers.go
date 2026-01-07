package httpbridge

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/jobs"
)

// CreateJobRequest represents the job creation request
type CreateJobRequest struct {
	// Coding job fields
	Type        string `json:"type"`        // builder, debugger, reviewer, triager
	Description string `json:"description"` // Job description
	Issue       string `json:"issue"`       // Optional issue number
	Symptoms    string `json:"symptoms"`    // For debugger
	Logs        string `json:"logs"`        // For debugger
	Branch      string `json:"branch"`      // For reviewer

	// Blog job fields
	Topic      string `json:"topic"`       // For blog orchestrator
	Notes      string `json:"notes"`       // For blog orchestrator
	FocusTopic string `json:"focus_topic"` // For blog orchestrator
}

// JobResponse represents the job response
type JobResponse struct {
	JobID   string `json:"job_id"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// handleJobs handles /api/jobs (GET for list, POST for create)
func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListJobs(w, r)
	case http.MethodPost:
		s.handleCreateJob(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleJobsWithID handles /api/jobs/:id (GET for status, DELETE for cancel)
func (s *Server) handleJobsWithID(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path
	jobID := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	if jobID == "" {
		http.Error(w, "Job ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleGetJob(w, r, jobID)
	case http.MethodDelete:
		s.handleCancelJob(w, r, jobID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCreateJob creates a new job using agent factories
func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req CreateJobRequest

	// Parse based on content type
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSON(w, http.StatusBadRequest, JobResponse{
				Success: false,
				Error:   fmt.Sprintf("Invalid request: %v", err),
			})
			return
		}
	} else {
		// Form data (from HTMX)
		if err := r.ParseForm(); err != nil {
			respondJSON(w, http.StatusBadRequest, JobResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to parse form: %v", err),
			})
			return
		}

		req.Type = r.FormValue("type")
		req.Description = r.FormValue("description")
		req.Issue = r.FormValue("issue")
		req.Symptoms = r.FormValue("symptoms")
		req.Logs = r.FormValue("logs")
		req.Branch = r.FormValue("branch")
		req.Topic = r.FormValue("topic")
		req.Notes = r.FormValue("notes")
		req.FocusTopic = r.FormValue("focus_topic")
	}

	// Validate required fields
	if req.Type == "" {
		respondJSON(w, http.StatusBadRequest, JobResponse{
			Success: false,
			Error:   "Job type is required",
		})
		return
	}

	// Create job using appropriate agent
	var job *jobs.Job
	var err error

	switch req.Type {
	case "builder":
		if req.Description == "" {
			respondJSON(w, http.StatusBadRequest, JobResponse{
				Success: false,
				Error:   "Description is required for builder jobs",
			})
			return
		}
		agent := s.appCtx.NewBuilderAgent()
		input := map[string]interface{}{
			"description": req.Description,
		}
		if req.Issue != "" {
			input["issue"] = req.Issue
		}
		job, err = agent.Execute(s.ctx, input)

	case "debugger":
		if req.Symptoms == "" {
			respondJSON(w, http.StatusBadRequest, JobResponse{
				Success: false,
				Error:   "Symptoms are required for debugger jobs",
			})
			return
		}
		agent := s.appCtx.NewDebuggerAgent()
		input := map[string]interface{}{
			"symptoms": req.Symptoms,
		}
		if req.Logs != "" {
			input["logs"] = req.Logs
		}
		job, err = agent.Execute(s.ctx, input)

	case "reviewer":
		if req.Branch == "" {
			respondJSON(w, http.StatusBadRequest, JobResponse{
				Success: false,
				Error:   "Branch is required for reviewer jobs",
			})
			return
		}
		agent := s.appCtx.NewReviewerAgent()
		job, err = agent.Execute(s.ctx, map[string]interface{}{
			"branch": req.Branch,
		})

	case "triager":
		if req.Description == "" {
			respondJSON(w, http.StatusBadRequest, JobResponse{
				Success: false,
				Error:   "Description is required for triager jobs",
			})
			return
		}
		agent := s.appCtx.NewTriagerAgent()
		job, err = agent.Execute(s.ctx, map[string]interface{}{
			"description": req.Description,
		})

	default:
		respondJSON(w, http.StatusBadRequest, JobResponse{
			Success: false,
			Error:   fmt.Sprintf("Unknown job type: %s", req.Type),
		})
		return
	}

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, JobResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create job: %v", err),
		})
		return
	}

	// Return response
	respondJSON(w, http.StatusOK, JobResponse{
		JobID:   job.ID,
		Message: fmt.Sprintf("Job %s created successfully", job.ID),
		Success: true,
	})
}

// handleListJobs lists all jobs via JobManager
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobList, err := s.appCtx.JobManager.List(s.ctx)
	if err != nil {
		fmt.Printf("Error listing jobs: %v\n", err)

		if r.Header.Get("HX-Request") == "true" {
			// Return error as HTML for HTMX
			html := fmt.Sprintf(`<div class="bg-red-50 p-4 rounded border border-red-200">
				<p class="text-sm text-red-700">Error loading jobs: %s</p>
			</div>`, err.Error())
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(html))
		} else {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("Failed to list jobs: %v", err),
			})
		}
		return
	}

	// For HTMX requests, render job cards
	if r.Header.Get("HX-Request") == "true" {
		if len(jobList) == 0 {
			html := `<div class="text-center py-12 text-gray-500">
				<p>No jobs yet. Create one above to get started!</p>
			</div>`
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(html))
			return
		}

		// Render each job card
		var html strings.Builder
		for _, job := range jobList {
			html.WriteString(renderJobCard(job))
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html.String()))
	} else {
		// JSON response for API clients
		respondJSON(w, http.StatusOK, jobList)
	}
}

// handleGetJob gets a single job status via JobManager
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request, jobID string) {
	job, err := s.appCtx.JobManager.Get(s.ctx, jobID)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("Job not found: %v", err),
		})
		return
	}

	respondJSON(w, http.StatusOK, job)
}

// handleCancelJob cancels a job via JobManager
func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request, jobID string) {
	err := s.appCtx.JobManager.Cancel(s.ctx, jobID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Failed to cancel job: %v", err),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Job %s cancelled successfully", jobID),
	})
}

// BlogResponse represents a blog operation response
type BlogResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	JobID   string `json:"job_id,omitempty"`
	Error   string `json:"error,omitempty"`
}

// BlogRequest represents a blog creation request
type BlogRequest struct {
	Title     string `json:"title"`
	Dictation string `json:"dictation"` // Voice transcription or raw text
}

// OrchestratedBlogRequest represents a blog orchestrator request
type OrchestratedBlogRequest struct {
	Prompt  string `json:"prompt"`
	Publish bool   `json:"publish"`
}

// OrchestratedBlogResponse represents a blog orchestrator response
type OrchestratedBlogResponse struct {
	Success bool   `json:"success"`
	JobID   string `json:"job_id,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// handleBlog creates a blog post using DynamicBlogAgent
func (s *Server) handleBlog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BlogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, BlogResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	if req.Dictation == "" {
		respondJSON(w, http.StatusBadRequest, BlogResponse{
			Success: false,
			Error:   "Dictation content is required",
		})
		return
	}

	// Create dynamic blog agent
	agent := s.appCtx.NewDynamicBlogAgent()

	// Execute blog creation asynchronously
	job, err := agent.Execute(s.ctx, map[string]interface{}{
		"title":   req.Title,
		"content": req.Dictation,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, BlogResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create blog post: %v", err),
		})
		return
	}

	respondJSON(w, http.StatusOK, BlogResponse{
		Success: true,
		JobID:   job.ID,
		Message: fmt.Sprintf("Blog post job %s created successfully", job.ID),
	})
}

// handleBlogOrchestrate handles complex blog creation using BlogOrchestrator
func (s *Server) handleBlogOrchestrate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req OrchestratedBlogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, OrchestratedBlogResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	if req.Prompt == "" {
		respondJSON(w, http.StatusBadRequest, OrchestratedBlogResponse{
			Success: false,
			Error:   "Prompt is required",
		})
		return
	}

	// Create blog orchestrator agent
	agent := s.appCtx.NewBlogOrchestratorAgent()

	// Execute orchestration asynchronously
	job, err := agent.Execute(s.ctx, map[string]interface{}{
		"prompt":  req.Prompt,
		"publish": req.Publish,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, OrchestratedBlogResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to start blog orchestrator: %v", err),
		})
		return
	}

	respondJSON(w, http.StatusOK, OrchestratedBlogResponse{
		Success: true,
		JobID:   job.ID,
		Message: fmt.Sprintf("Blog orchestration job %s created successfully", job.ID),
	})
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status      string `json:"status"`       // "healthy" or "degraded"
	MCPRunning  bool   `json:"mcp_running"`  // Kept for API compatibility
	BackendType string `json:"backend_type"` // "ollama" or "llamacpp"
	Timestamp   string `json:"timestamp"`
}

// handleHealth checks if the system is healthy
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Check database health
	dbHealthy := true
	if s.appCtx.Database != nil {
		if err := s.appCtx.Database.Ping(); err != nil {
			dbHealthy = false
		}
	}

	// System is healthy if database is up
	status := "healthy"
	if !dbHealthy {
		status = "degraded"
	}

	respondJSON(w, http.StatusOK, HealthResponse{
		Status:      status,
		MCPRunning:  dbHealthy, // Keep field name for API compatibility
		BackendType: s.config.Model.Type,
		Timestamp:   time.Now().Format(time.RFC3339),
	})
}

// VoiceTranscribeRequest represents a voice transcription request
type VoiceTranscribeRequest struct {
	Audio string `json:"audio"` // Base64 encoded audio data
}

// VoiceTranscribeResponse represents a voice transcription response
type VoiceTranscribeResponse struct {
	Success bool   `json:"success"`
	Text    string `json:"text,omitempty"`
	Error   string `json:"error,omitempty"`
}

// handleVoiceTranscribe handles voice transcription requests
func (s *Server) handleVoiceTranscribe(w http.ResponseWriter, r *http.Request) {
	// Voice transcription implementation would go here
	// For now, return not implemented
	respondJSON(w, http.StatusNotImplemented, VoiceTranscribeResponse{
		Success: false,
		Error:   "Voice transcription not yet implemented",
	})
}

// handleVoiceStatus checks if voice transcription service is available
func (s *Server) handleVoiceStatus(w http.ResponseWriter, r *http.Request) {
	// Voice status check implementation would go here
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"available": false,
		"reason":    "Voice transcription not yet configured",
	})
}

// Helper functions

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// renderJobCard renders an HTML job card for HTMX
func renderJobCard(job *jobs.Job) string {
	statusClass := "bg-gray-100 text-gray-700"
	statusIcon := "⏳"

	switch job.Status {
	case jobs.StatusCompleted:
		statusClass = "bg-green-100 text-green-700"
		statusIcon = "✅"
	case jobs.StatusFailed:
		statusClass = "bg-red-100 text-red-700"
		statusIcon = "❌"
	case jobs.StatusCancelled:
		statusClass = "bg-yellow-100 text-yellow-700"
		statusIcon = "⚠️"
	}

	return fmt.Sprintf(`
	<div class="border rounded-lg p-4 mb-4 hover:shadow-md transition-shadow">
		<div class="flex justify-between items-start mb-2">
			<div>
				<h3 class="font-semibold text-lg">%s</h3>
				<p class="text-sm text-gray-600">%s</p>
			</div>
			<span class="px-3 py-1 rounded-full text-sm font-medium %s">
				%s %s
			</span>
		</div>
		<div class="text-xs text-gray-500 mt-2">
			<span>Created: %s</span>
		</div>
	</div>
	`, job.ID, job.Description, statusClass, statusIcon, job.Status, job.CreatedAt.Format("2006-01-02 15:04:05"))
}
