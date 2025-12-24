package httpbridge

import (
	"context"
	"html/template"
	"log"
	"net/http"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/mcp"
)

// Server represents the HTTP server
type Server struct {
	config    *config.Config
	mcpClient *mcp.Client
	ctx       context.Context
	mux       *http.ServeMux
	templates *template.Template
}

// NewServer creates a new HTTP server
func NewServer(cfg *config.Config, mcpClient *mcp.Client, ctx context.Context) *Server {
	mux := http.NewServeMux()

	// Load HTML templates (must load all files individually, ** glob doesn't work)
	templates := template.Must(template.ParseFiles(
		"pkg/web/templates/base.html",
		"pkg/web/templates/index.html",
		"pkg/web/templates/components/job_card.html",
	))

	server := &Server{
		config:    cfg,
		mcpClient: mcpClient,
		ctx:       ctx,
		mux:       mux,
		templates: templates,
	}

	// Setup routes
	server.setupRoutes()

	return server
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Serve static files
	fs := http.FileServer(http.Dir("pkg/web/static"))
	s.mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Web UI routes
	s.mux.HandleFunc("/", s.handleIndex)

	// API routes
	s.mux.HandleFunc("/api/jobs", s.handleJobs)
	s.mux.HandleFunc("/api/jobs/", s.handleJobsWithID)
}

// Run starts the HTTP server
func (s *Server) Run(addr string) error {
	log.Printf("Starting HTTP server on %s", addr)
	return http.ListenAndServe(addr, s.mux)
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
