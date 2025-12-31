package httpbridge

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
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
