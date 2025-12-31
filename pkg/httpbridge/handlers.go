package httpbridge

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
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

// handleCreateJob creates a new job via MCP
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

	// Build arguments for MCP tool call
	args := make(map[string]interface{})

	switch req.Type {
	case "builder":
		args["description"] = req.Description
		if req.Issue != "" {
			args["issue"] = req.Issue
		}
	case "debugger":
		args["symptoms"] = req.Symptoms
		if req.Logs != "" {
			args["logs"] = req.Logs
		}
	case "reviewer":
		args["branch"] = req.Branch
	case "triager":
		args["description"] = req.Description

	// Podcast job types
	case "create_podcast_script":
		args["topic"] = req.Topic
		if req.Notes != "" {
			args["notes"] = req.Notes
		}
	case "add_notion_link":
		args["url"] = req.URL
		if req.Title != "" {
			args["title"] = req.Title
		}
		if req.Notes != "" {
			args["notes"] = req.Notes
		}
	case "add_guest":
		args["name"] = req.Name
		if req.Bio != "" {
			args["bio"] = req.Bio
		}
		if req.Email != "" {
			args["email"] = req.Email
		}
	case "review_news_summary":
		if req.FocusTopic != "" {
			args["focus_topic"] = req.FocusTopic
		}

	default:
		respondJSON(w, http.StatusBadRequest, JobResponse{
			Success: false,
			Error:   fmt.Sprintf("Unknown job type: %s", req.Type),
		})
		return
	}

	// Call MCP tool (SAME AS CLI)
	result, err := s.mcpClient.CallTool(s.ctx, req.Type, args)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, JobResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create job: %v", err),
		})
		return
	}

	// Extract job ID from response
	jobID, err := extractJobID(result.Content[0].Text)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, JobResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to extract job ID: %v", err),
		})
		return
	}

	// Return response
	response := JobResponse{
		JobID:   jobID,
		Message: result.Content[0].Text,
		Success: true,
	}

	// Return HTML fragment for HTMX or JSON for API
	if r.Header.Get("HX-Request") == "true" {
		// Return HTMX-compatible HTML fragment
		data := map[string]interface{}{
			"job_id": jobID,
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

// handleListJobs lists all jobs via MCP
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	// Call MCP list_jobs tool (SAME AS CLI)
	result, err := s.mcpClient.CallTool(s.ctx, "list_jobs", map[string]interface{}{})
	if err != nil {
		// Log the error for debugging
		fmt.Printf("Error listing jobs: %v\n", err)

		if r.Header.Get("HX-Request") == "true" {
			// Return error as HTML for HTMX
			html := fmt.Sprintf(`<div class="bg-red-50 p-4 rounded border border-red-200">
				<p class="text-sm text-red-700">Error loading jobs: %s</p>
			</div>`, err.Error())
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK) // Return 200 so HTMX doesn't show error popup
			w.Write([]byte(html))
		} else {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("Failed to list jobs: %v", err),
			})
		}
		return
	}

	// Check if we have content
	if len(result.Content) == 0 {
		fmt.Println("Warning: list_jobs returned no content")
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
			respondJSON(w, http.StatusOK, map[string]string{"jobs": "No jobs found"})
		}
		return
	}

	// For now, return the raw text response wrapped in simple HTML
	if r.Header.Get("HX-Request") == "true" {
		// Jobs exist - enable auto-refresh by adding hx-trigger to parent
		html := fmt.Sprintf(`<div hx-get="/api/jobs" hx-trigger="every 5s" hx-swap="innerHTML" hx-target="#job-list">
			<div class="bg-gray-50 p-4 rounded border border-gray-200">
				<pre class="text-sm text-gray-700 whitespace-pre-wrap">%s</pre>
			</div>
		</div>`, result.Content[0].Text)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	} else {
		respondJSON(w, http.StatusOK, map[string]string{
			"jobs": result.Content[0].Text,
		})
	}
}

// handleGetJob gets a single job status via MCP
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request, jobID string) {
	// Call MCP get_job_status tool (SAME AS CLI)
	result, err := s.mcpClient.CallTool(s.ctx, "get_job_status", map[string]interface{}{
		"job_id": jobID,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Failed to get job: %v", err),
		})
		return
	}

	// Return response
	if r.Header.Get("HX-Request") == "true" {
		// Return HTMX-compatible HTML
		data := map[string]interface{}{
			"status_text": result.Content[0].Text,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.templates.ExecuteTemplate(w, "job_card.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		respondJSON(w, http.StatusOK, map[string]string{
			"status": result.Content[0].Text,
		})
	}
}

// handleCancelJob cancels a job via MCP
func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request, jobID string) {
	// Call MCP cancel_job tool (SAME AS CLI)
	result, err := s.mcpClient.CallTool(s.ctx, "cancel_job", map[string]interface{}{
		"job_id": jobID,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Failed to cancel job: %v", err),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": result.Content[0].Text,
		"success": true,
	})
}

// extractJobID extracts job ID from agent response text (SAME AS CLI)
func extractJobID(text string) (string, error) {
	// Look for "Job job-XXXXX started"
	re := regexp.MustCompile(`Job (job-\d+)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not extract job ID from response: %s", text)
	}
	return matches[1], nil
}

// respondJSON is a helper to write JSON responses
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// BlogRequest represents a blog post creation request
type BlogRequest struct {
	Title     string `json:"title"`
	Dictation string `json:"dictation"` // Raw voice dictation
	Content   string `json:"content"`   // Legacy field for direct content
	SkipAI    bool   `json:"skip_ai"`   // Skip AI expansion, post directly
}

// BlogResponse represents the blog post creation response
type BlogResponse struct {
	Success         bool     `json:"success"`
	Message         string   `json:"message"`
	Error           string   `json:"error,omitempty"`
	JobID           string   `json:"job_id,omitempty"`
	SuggestedTitles []string `json:"suggested_titles,omitempty"`
	Tags            []string `json:"tags,omitempty"`
}

// handleBlog handles POST /api/blog for blog post creation
// This is the full AI-powered workflow: dictation -> AI expansion -> Notion
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

	// Step 1: Call the writer agent to expand dictation
	writerResult, err := s.mcpClient.CallTool(s.ctx, "writer", map[string]interface{}{
		"transcription": req.Dictation,
		"title":         req.Title,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, BlogResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to start AI writer: %v", err),
		})
		return
	}

	// Extract job ID from writer response
	jobID, err := extractJobID(writerResult.Content[0].Text)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, BlogResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to extract job ID: %v", err),
		})
		return
	}

	// Step 2: Poll for job completion
	var jobOutput map[string]interface{}
	for i := 0; i < 60; i++ { // Poll for up to 60 seconds
		time.Sleep(1 * time.Second)

		statusResult, err := s.mcpClient.CallTool(s.ctx, "get_job_status", map[string]interface{}{
			"job_id": jobID,
		})
		if err != nil {
			continue // Keep polling
		}

		// Check status from structured data
		if statusResult.Data != nil {
			status, _ := statusResult.Data["status"].(string)

			if status == "completed" {
				// Get output from structured data
				if output, ok := statusResult.Data["output"].(map[string]interface{}); ok {
					jobOutput = output
				}
				break
			}

			if status == "failed" {
				errorMsg, _ := statusResult.Data["error"].(string)
				respondJSON(w, http.StatusInternalServerError, BlogResponse{
					Success: false,
					Error:   fmt.Sprintf("AI writer job failed: %s", errorMsg),
					JobID:   jobID,
				})
				return
			}
		} else {
			// Fallback to text parsing
			statusText := statusResult.Content[0].Text

			if strings.Contains(statusText, "completed") {
				if err := json.Unmarshal([]byte(extractJSON(statusText)), &jobOutput); err != nil {
					jobOutput = parseJobStatus(statusText)
				}
				break
			}

			if strings.Contains(statusText, "failed") {
				respondJSON(w, http.StatusInternalServerError, BlogResponse{
					Success: false,
					Error:   fmt.Sprintf("AI writer job failed: %s", statusText),
					JobID:   jobID,
				})
				return
			}
		}
	}

	if jobOutput == nil {
		respondJSON(w, http.StatusInternalServerError, BlogResponse{
			Success: false,
			Error:   "AI writer job timed out after 60 seconds",
			JobID:   jobID,
		})
		return
	}

	// Step 3: Get expanded content and publish to Notion
	expandedDraft, _ := jobOutput["expanded_draft"].(string)
	if expandedDraft == "" {
		expandedDraft = req.Dictation // Fallback to original dictation
	}

	// Get suggested titles
	suggestedTitles := []string{}
	if titles, ok := jobOutput["suggested_titles"].([]interface{}); ok {
		for _, t := range titles {
			if s, ok := t.(string); ok {
				suggestedTitles = append(suggestedTitles, s)
			}
		}
	}

	// Use first suggested title or original title
	title := req.Title
	if len(suggestedTitles) > 0 {
		title = suggestedTitles[0]
	}

	// Prepare blog_publish arguments
	publishArgs := map[string]interface{}{
		"title":              title,
		"expanded_draft":     expandedDraft,
		"original_dictation": req.Dictation,
		"suggested_titles":   jobOutput["suggested_titles"],
		"substack_tags":      jobOutput["substack_tags"],
		"twitter_post":       jobOutput["twitter_post"],
		"linkedin_post":      jobOutput["linkedin_post"],
		"bluesky_post":       jobOutput["bluesky_post"],
		"key_takeaways":      jobOutput["key_takeaways"],
		"target_audience":    jobOutput["target_audience"],
		"read_time":          jobOutput["read_time"],
	}

	// Call blog_publish tool
	publishResult, err := s.mcpClient.CallTool(s.ctx, "blog_publish", publishArgs)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, BlogResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to publish to Notion: %v", err),
			JobID:   jobID,
		})
		return
	}

	// Check for publish error
	if publishResult.IsError {
		errorMsg := "Unknown error"
		if len(publishResult.Content) > 0 {
			errorMsg = publishResult.Content[0].Text
		}
		respondJSON(w, http.StatusInternalServerError, BlogResponse{
			Success: false,
			Error:   errorMsg,
			JobID:   jobID,
		})
		return
	}

	// Get tags for response
	tags := []string{}
	if t, ok := jobOutput["substack_tags"].([]interface{}); ok {
		for _, tag := range t {
			if s, ok := tag.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	// Return success response
	message := "Blog post created"
	if len(publishResult.Content) > 0 {
		message = publishResult.Content[0].Text
	}

	respondJSON(w, http.StatusOK, BlogResponse{
		Success:         true,
		Message:         message,
		JobID:           jobID,
		SuggestedTitles: suggestedTitles,
		Tags:            tags,
	})
}

// handleBlogDirect posts content directly to Notion without AI expansion
func (s *Server) handleBlogDirect(w http.ResponseWriter, req BlogRequest) {
	title := req.Title
	if title == "" {
		title = "Untitled Draft"
	}

	// Call blog_publish tool directly with the raw dictation as content
	result, err := s.mcpClient.CallTool(s.ctx, "blog_publish", map[string]interface{}{
		"title":          title,
		"expanded_draft": req.Dictation,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, BlogResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create blog post: %v", err),
		})
		return
	}

	if result.IsError {
		errorMsg := "Unknown error"
		if len(result.Content) > 0 {
			errorMsg = result.Content[0].Text
		}
		respondJSON(w, http.StatusInternalServerError, BlogResponse{
			Success: false,
			Error:   errorMsg,
		})
		return
	}

	message := "Blog post created"
	if len(result.Content) > 0 {
		message = result.Content[0].Text
	}

	respondJSON(w, http.StatusOK, BlogResponse{
		Success: true,
		Message: message,
	})
}

// extractJSON tries to find a JSON object in text
func extractJSON(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start != -1 && end > start {
		return text[start : end+1]
	}
	return "{}"
}

// parseJobStatus parses job status text into a map
func parseJobStatus(text string) map[string]interface{} {
	result := make(map[string]interface{})

	// Look for key-value patterns in the status text
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(strings.ToLower(parts[0]))
				value := strings.TrimSpace(parts[1])
				result[key] = value
			}
		}
	}

	return result
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status     string `json:"status"`
	MCPRunning bool   `json:"mcp_running"`
	Timestamp  string `json:"timestamp"`
}

// handleHealth handles GET /api/health for health check
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if MCP client is running
	mcpRunning := s.mcpClient.IsRunning()

	status := "healthy"
	if !mcpRunning {
		status = "degraded"
	}

	respondJSON(w, http.StatusOK, HealthResponse{
		Status:     status,
		MCPRunning: mcpRunning,
		Timestamp:  time.Now().Format(time.RFC3339),
	})
}
