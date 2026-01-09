package httpbridge

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/config"
)

// Server represents the HTTP server
type Server struct {
	config       *config.Config
	appCtx       *AppContext
	ctx          context.Context
	mux          *http.ServeMux
	templates    *template.Template
	sseBroadcast *SSEBroadcaster
}

// NewServer creates a new HTTP server with embedded dependencies
func NewServer(cfg *config.Config, ctx context.Context) (*Server, error) {
	// Create application context with all dependencies
	appCtx, err := NewAppContext(cfg)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	// Load HTML templates (must load all files individually, ** glob doesn't work)
	templates := template.Must(template.ParseFiles(
		"pkg/web/templates/base.html",
		"pkg/web/templates/index.html",
		"pkg/web/templates/components/job_card.html",
		"pkg/web/templates/blog_review.html",
	))

	// Create SSE broadcaster
	sseBroadcast := NewSSEBroadcaster(appCtx.JobManager, ctx)

	server := &Server{
		config:       cfg,
		appCtx:       appCtx,
		ctx:          ctx,
		mux:          mux,
		templates:    templates,
		sseBroadcast: sseBroadcast,
	}

	// Setup routes
	server.setupRoutes()

	// Start background polling for real-time updates (every 2 seconds)
	go sseBroadcast.StartPolling(2 * time.Second)

	return server, nil
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Serve static files
	fs := http.FileServer(http.Dir("pkg/web/static"))
	s.mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Web UI routes
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/blog/review/", s.handleBlogReviewPage)

	// API routes
	s.mux.HandleFunc("/api/jobs", s.handleJobs)
	s.mux.HandleFunc("/api/jobs/", s.handleJobsWithID)
	s.mux.HandleFunc("/api/stream/jobs/", s.handleJobStream)
	s.mux.HandleFunc("/api/blog", s.handleBlog)
	s.mux.HandleFunc("/api/blog/orchestrate", s.handleBlogOrchestrate)
	s.mux.HandleFunc("/api/podcast", s.handlePodcast)
	s.mux.HandleFunc("/api/health", s.handleHealth)

	// Blog review API routes
	s.mux.HandleFunc("/api/blog/posts", s.handleBlogPosts)
	s.mux.HandleFunc("/api/blog/posts/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/blog/posts/")
		parts := strings.Split(path, "/")

		if len(parts) == 1 {
			// /api/blog/posts/:id
			s.handleBlogPostByID(w, r)
		} else if len(parts) == 2 && parts[1] == "edit" {
			// /api/blog/posts/:id/edit
			s.handleBlogPostEdit(w, r)
		} else if len(parts) == 2 && parts[1] == "revise" {
			// /api/blog/posts/:id/revise
			s.handleBlogPostRevise(w, r)
		} else {
			http.Error(w, "Not found", http.StatusNotFound)
		}
	})

	// Voice transcription routes
	s.mux.HandleFunc("/api/voice/transcribe", s.handleVoiceTranscribe)
	s.mux.HandleFunc("/api/voice/status", s.handleVoiceStatus)
}

// Run starts the HTTP server
func (s *Server) Run(addr string) error {
	log.Printf("Starting HTTP server on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

// Close closes all server resources (database, etc.)
func (s *Server) Close() error {
	if s.appCtx != nil {
		return s.appCtx.Close()
	}
	return nil
}

// handleIndex serves the main page
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data := map[string]interface{}{
		"title": "PedroCLI - Autonomous Coding Agent",
	}

	if err := s.templates.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleJobStream handles SSE connections for job updates
func (s *Server) handleJobStream(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path: /api/stream/jobs/:id
	jobID := r.URL.Path[len("/api/stream/jobs/"):]
	if jobID == "" {
		http.Error(w, "Job ID required", http.StatusBadRequest)
		return
	}

	// Serve SSE stream
	s.sseBroadcast.ServeHTTP(w, r, jobID)
}
