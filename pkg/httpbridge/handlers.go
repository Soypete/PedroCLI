package httpbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/jobs"
)

// CreateJobRequest represents the job creation request
type CreateJobRequest struct {
	// Coding job fields
	Type        string `json:"type"`        // builder, debugger, reviewer, triager, or podcast types
	Description string `json:"description"` // Job description
	Issue       string `json:"issue"`       // Optional issue number
	Symptoms    string `json:"symptoms"`    // For debugger
	Logs        string `json:"logs"`        // For debugger
	Branch      string `json:"branch"`      // For reviewer

	// Podcast job fields
	Topic      string `json:"topic"`       // For create_podcast_script, create_episode_outline
	Notes      string `json:"notes"`       // For create_podcast_script, add_notion_link, create_episode_outline
	URL        string `json:"url"`         // For add_notion_link
	Title      string `json:"title"`       // For add_notion_link
	Name       string `json:"name"`        // For add_guest
	Bio        string `json:"bio"`         // For add_guest
	Email      string `json:"email"`       // For add_guest
	FocusTopic string `json:"focus_topic"` // For review_news_summary
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

// handleCreateJob creates a new job by executing an agent directly
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

		// Coding job fields
		req.Type = r.FormValue("type")
		req.Description = r.FormValue("description")
		req.Issue = r.FormValue("issue")
		req.Symptoms = r.FormValue("symptoms")
		req.Logs = r.FormValue("logs")
		req.Branch = r.FormValue("branch")

		// Podcast job fields
		req.Topic = r.FormValue("topic")
		req.Notes = r.FormValue("notes")
		req.URL = r.FormValue("url")
		req.Title = r.FormValue("title")
		req.Name = r.FormValue("name")
		req.Bio = r.FormValue("bio")
		req.Email = r.FormValue("email")
		req.FocusTopic = r.FormValue("focus_topic")

		if req.Type == "" {
			respondJSON(w, http.StatusBadRequest, JobResponse{
				Success: false,
				Error:   "type is required",
			})
			return
		}

		// Validate required fields per job type
		var validationErr string
		switch req.Type {
		case "builder", "triager":
			if req.Description == "" {
				validationErr = "description is required"
			}
		case "debugger":
			if req.Symptoms == "" {
				validationErr = "symptoms is required"
			}
		case "reviewer":
			if req.Branch == "" {
				validationErr = "branch is required"
			}
		case "create_podcast_script":
			if req.Topic == "" {
				validationErr = "topic is required"
			}
		case "add_notion_link":
			if req.URL == "" {
				validationErr = "url is required"
			}
		case "add_guest":
			if req.Name == "" {
				validationErr = "name is required"
			}
		case "review_news_summary":
			if req.FocusTopic == "" {
				validationErr = "focus_topic is required"
			}
		}
		if validationErr != "" {
			respondJSON(w, http.StatusBadRequest, JobResponse{
				Success: false,
				Error:   validationErr,
			})
			return
		}
	}

	// Build arguments for the agent
	args := make(map[string]interface{})

	// Get the appropriate agent
	var agent agents.Agent

	switch req.Type {
	case "builder":
		args["description"] = req.Description
		if req.Issue != "" {
			args["issue"] = req.Issue
		}
		agent = s.appCtx.NewBuilderAgent()

	case "debugger":
		args["description"] = req.Symptoms
		if req.Logs != "" {
			args["error_log"] = req.Logs
		}
		agent = s.appCtx.NewDebuggerAgent()

	case "reviewer":
		args["branch"] = req.Branch
		agent = s.appCtx.NewReviewerAgent()

	case "triager":
		args["description"] = req.Description
		agent = s.appCtx.NewTriagerAgent()

	case "blog_orchestrator":
		args["prompt"] = req.Description
		agent = s.appCtx.NewBlogOrchestratorAgent()

	default:
		respondJSON(w, http.StatusBadRequest, JobResponse{
			Success: false,
			Error:   fmt.Sprintf("Unknown job type: %s", req.Type),
		})
		return
	}

	// Execute the agent
	job, err := agent.Execute(s.ctx, args)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, JobResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create job: %v", err),
		})
		return
	}

	// Return response
	response := JobResponse{
		JobID:   job.ID,
		Message: fmt.Sprintf("Job %s started", job.ID),
		Success: true,
	}

	// Return HTML fragment for HTMX or JSON for API
	if r.Header.Get("HX-Request") == "true" {
		// Return HTMX-compatible HTML fragment
		data := map[string]interface{}{
			"job_id": job.ID,
			"type":   req.Type,
			"status": "running",
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.templates.ExecuteTemplate(w, "job_card.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		respondJSON(w, http.StatusOK, response)
	}
}

// handleListJobs lists all jobs from the job manager
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobList, err := s.appCtx.JobManager.List(r.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Failed to list jobs: %v", err),
		})
		return
	}

	if len(jobList) == 0 {
		if r.Header.Get("HX-Request") == "true" {
			// No jobs - don't auto-refresh
			html := `<div class="text-center py-12 text-gray-500">
				<p>No jobs yet</p>
				<p class="text-sm mt-2">Create a job to get started</p>
			</div>`
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(html))
		} else {
			respondJSON(w, http.StatusOK, map[string]interface{}{"jobs": []interface{}{}})
		}
		return
	}

	// Build job list text
	var jobsText strings.Builder
	for _, job := range jobList {
		statusEmoji := "⏳"
		switch job.Status {
		case jobs.StatusCompleted:
			statusEmoji = "✅"
		case jobs.StatusFailed:
			statusEmoji = "❌"
		case jobs.StatusRunning:
			statusEmoji = "🔄"
		}
		jobsText.WriteString(fmt.Sprintf("%s %s [%s] %s\n", job.ID, statusEmoji, job.Status, truncateString(job.Description, 50)))
	}

	if r.Header.Get("HX-Request") == "true" {
		// Jobs exist - enable auto-refresh by adding hx-trigger to parent
		html := fmt.Sprintf(`<div hx-get="/api/jobs" hx-trigger="every 5s" hx-swap="innerHTML" hx-target="#job-list">
			<div class="bg-gray-50 p-4 rounded border border-gray-200">
				<pre class="text-sm text-gray-700 whitespace-pre-wrap">%s</pre>
			</div>
		</div>`, jobsText.String())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	} else {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"jobs": jobsText.String(),
		})
	}
}

// handleGetJob gets a single job status from the job manager
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request, jobID string) {
	job, err := s.appCtx.JobManager.Get(r.Context(), jobID)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("Job not found: %v", err),
		})
		return
	}

	// Build status text
	statusText := fmt.Sprintf("Job: %s\nType: %s\nStatus: %s\nCreated: %s",
		job.ID, job.Type, job.Status, job.CreatedAt.Format(time.RFC3339))
	if job.StartedAt != nil {
		statusText += fmt.Sprintf("\nStarted: %s", job.StartedAt.Format(time.RFC3339))
	}
	if job.CompletedAt != nil {
		statusText += fmt.Sprintf("\nCompleted: %s", job.CompletedAt.Format(time.RFC3339))
	}
	if job.Error != "" {
		statusText += fmt.Sprintf("\nError: %s", job.Error)
	}

	if r.Header.Get("HX-Request") == "true" {
		// Return HTMX-compatible HTML
		data := map[string]interface{}{
			"status_text": statusText,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.templates.ExecuteTemplate(w, "job_card.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"job_id":       job.ID,
			"type":         job.Type,
			"status":       job.Status,
			"description":  job.Description,
			"created_at":   job.CreatedAt,
			"started_at":   job.StartedAt,
			"completed_at": job.CompletedAt,
			"error":        job.Error,
		})
	}
}

// handleCancelJob cancels a job using the job manager
func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request, jobID string) {
	if err := s.appCtx.JobManager.Cancel(r.Context(), jobID); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Failed to cancel job: %v", err),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("Job %s cancelled", jobID),
		"success": true,
	})
}

// respondJSON is a helper to write JSON responses
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// BlogRequest represents a blog post creation request
type BlogRequest struct {
	Title     string `json:"title"`
	Dictation string `json:"dictation"` // Raw voice dictation / prompt for orchestrator
	Content   string `json:"content"`   // Legacy field for direct content
	SkipAI    bool   `json:"skip_ai"`   // Skip AI expansion, post directly
	Publish   bool   `json:"publish"`   // Publish to Notion (default true)
}

// BlogResponse represents the blog post creation response
type BlogResponse struct {
	Success         bool     `json:"success"`
	Message         string   `json:"message"`
	Error           string   `json:"error,omitempty"`
	JobID           string   `json:"job_id,omitempty"`
	NotionURL       string   `json:"notion_url,omitempty"`
	SuggestedTitles []string `json:"suggested_titles,omitempty"`
	Tags            []string `json:"tags,omitempty"`
}

// handleBlog handles POST /api/blog for blog post creation
// Uses the BlogOrchestratorAgent for full pipeline: research -> outline -> expand -> publish
func (s *Server) handleBlog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BlogRequest

	// Parse based on content type
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSON(w, http.StatusBadRequest, BlogResponse{
				Success: false,
				Error:   fmt.Sprintf("Invalid request: %v", err),
			})
			return
		}
	} else {
		// Form data
		if err := r.ParseForm(); err != nil {
			respondJSON(w, http.StatusBadRequest, BlogResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to parse form: %v", err),
			})
			return
		}
		req.Title = r.FormValue("title")
		req.Dictation = r.FormValue("dictation")
		req.Content = r.FormValue("content")
		req.SkipAI = r.FormValue("skip_ai") == "true"
		req.Publish = r.FormValue("publish") != "false" // Default to true
	}

	// Support legacy "content" field as dictation
	if req.Dictation == "" && req.Content != "" {
		req.Dictation = req.Content
	}

	// Validate
	if req.Dictation == "" {
		respondJSON(w, http.StatusBadRequest, BlogResponse{
			Success: false,
			Error:   "dictation is required",
		})
		return
	}

	// If skip_ai is true, just post directly to Notion without AI
	if req.SkipAI {
		s.handleBlogDirect(w, req)
		return
	}

	// Build arguments for the blog_orchestrator agent
	args := map[string]interface{}{
		"prompt":  req.Dictation,
		"publish": req.Publish,
	}
	if req.Title != "" {
		args["title"] = req.Title
	}

	// Create and execute the blog orchestrator agent
	agent := s.appCtx.NewBlogOrchestratorAgent()
	job, err := agent.Execute(s.ctx, args)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, BlogResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to start blog orchestrator: %v", err),
		})
		return
	}

	// Return immediately with job ID - client can poll for status
	respondJSON(w, http.StatusAccepted, BlogResponse{
		Success: true,
		Message: fmt.Sprintf("Blog orchestration started. Poll /api/jobs/%s for status.", job.ID),
		JobID:   job.ID,
	})
}

// handleBlogDirect posts content directly to Notion without AI expansion
func (s *Server) handleBlogDirect(w http.ResponseWriter, req BlogRequest) {
	title := req.Title
	if title == "" {
		title = "Untitled Draft"
	}

	// Call blog_notion tool directly with the raw dictation as content
	ctx := context.Background()
	result, err := s.appCtx.BlogNotionTool.Execute(ctx, map[string]interface{}{
		"action":  "create",
		"title":   title,
		"content": req.Dictation,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, BlogResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create blog post: %v", err),
		})
		return
	}

	if !result.Success {
		respondJSON(w, http.StatusInternalServerError, BlogResponse{
			Success: false,
			Error:   result.Error,
		})
		return
	}

	respondJSON(w, http.StatusOK, BlogResponse{
		Success: true,
		Message: result.Output,
	})
}

// OrchestratedBlogRequest represents the orchestrated blog creation request
type OrchestratedBlogRequest struct {
	Title   string `json:"title"`   // Optional initial title
	Prompt  string `json:"prompt"`  // Complex prompt describing the blog post
	Publish bool   `json:"publish"` // Whether to auto-publish to Notion after generation
}

// OrchestratedBlogResponse represents the orchestrated blog response
type OrchestratedBlogResponse struct {
	Success        bool              `json:"success"`
	Message        string            `json:"message"`
	Error          string            `json:"error,omitempty"`
	JobID          string            `json:"job_id,omitempty"`
	SuggestedTitle string            `json:"suggested_title,omitempty"`
	FullContent    string            `json:"full_content,omitempty"`
	SocialPosts    map[string]string `json:"social_posts,omitempty"`
}

// handleBlogOrchestrate handles POST /api/blog/orchestrate for complex blog prompts
func (s *Server) handleBlogOrchestrate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req OrchestratedBlogRequest

	// Parse based on content type
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSON(w, http.StatusBadRequest, OrchestratedBlogResponse{
				Success: false,
				Error:   fmt.Sprintf("Invalid request: %v", err),
			})
			return
		}
	} else {
		// Form data
		if err := r.ParseForm(); err != nil {
			respondJSON(w, http.StatusBadRequest, OrchestratedBlogResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to parse form: %v", err),
			})
			return
		}
		req.Title = r.FormValue("title")
		req.Prompt = r.FormValue("prompt")
		req.Publish = r.FormValue("publish") == "true"
	}

	// Validate
	if req.Prompt == "" {
		respondJSON(w, http.StatusBadRequest, OrchestratedBlogResponse{
			Success: false,
			Error:   "prompt is required",
		})
		return
	}

	// Build arguments for the blog_orchestrator agent
	args := map[string]interface{}{
		"prompt":  req.Prompt,
		"publish": req.Publish,
	}
	if req.Title != "" {
		args["title"] = req.Title
	}

	// Create and execute the blog orchestrator agent
	agent := s.appCtx.NewBlogOrchestratorAgent()
	job, err := agent.Execute(s.ctx, args)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, OrchestratedBlogResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to start blog orchestrator: %v", err),
		})
		return
	}

	// Return immediately with job ID - client can poll for status
	respondJSON(w, http.StatusAccepted, OrchestratedBlogResponse{
		Success: true,
		Message: fmt.Sprintf("Blog orchestration job started. Poll /api/jobs/%s for status.", job.ID),
		JobID:   job.ID,
	})
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string `json:"status"`
	Ready     bool   `json:"ready"`
	Timestamp string `json:"timestamp"`
}

// handleHealth handles GET /api/health for health check
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if app context is initialized
	ready := s.appCtx != nil && s.appCtx.JobManager != nil && s.appCtx.Backend != nil

	status := "healthy"
	if !ready {
		status = "degraded"
	}

	respondJSON(w, http.StatusOK, HealthResponse{
		Status:    status,
		Ready:     ready,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}
