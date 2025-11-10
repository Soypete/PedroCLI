package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/stt"
	"github.com/soypete/pedrocli/pkg/tools"
	"github.com/soypete/pedrocli/pkg/tts"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins in development
		},
	}
)

type Server struct {
	config      *config.Config
	backend     llm.Backend
	jobManager  *jobs.Manager
	agents      map[string]agents.Agent
	sttClient   *stt.WhisperClient
	ttsClient   *tts.PiperClient
	connections map[*websocket.Conn]bool
	connMutex   sync.Mutex
	configsDir  string
	configPath  string
}

func main() {
	port := flag.String("port", "8080", "HTTP server port")
	configPath := flag.String("config", ".pedroceli.json", "Config file path")
	configsDir := flag.String("configs-dir", "", "Directory containing multiple config files (optional)")
	jobsDir := flag.String("jobs-dir", "/tmp/pedroceli-jobs", "Directory for job storage")
	whisperBin := flag.String("whisper-bin", "", "Path to whisper.cpp binary (optional)")
	whisperModel := flag.String("whisper-model", "", "Path to whisper model file (optional)")
	piperBin := flag.String("piper-bin", "", "Path to piper binary (optional)")
	piperModel := flag.String("piper-model", "", "Path to piper model file (optional)")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		cfg, err = config.LoadDefault()
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	}

	// Create LLM backend
	backend, err := llm.NewBackend(cfg)
	if err != nil {
		log.Fatalf("Failed to create backend: %v", err)
	}

	// Create job manager
	jobManager, err := jobs.NewManager(*jobsDir)
	if err != nil {
		log.Fatalf("Failed to create job manager: %v", err)
	}

	// Create STT client (optional)
	var sttClient *stt.WhisperClient
	if *whisperBin != "" && *whisperModel != "" {
		sttClient, err = stt.NewWhisperClient(*whisperBin, *whisperModel)
		if err != nil {
			log.Printf("⚠️  Failed to create STT client: %v", err)
			log.Printf("   Voice input will be disabled")
		} else {
			log.Printf("✅ Speech-to-text enabled (whisper.cpp)")
		}
	}

	// Create TTS client (optional)
	var ttsClient *tts.PiperClient
	if *piperBin != "" && *piperModel != "" {
		ttsClient, err = tts.NewPiperClient(*piperBin, *piperModel)
		if err != nil {
			log.Printf("⚠️  Failed to create TTS client: %v", err)
			log.Printf("   Voice output will be disabled")
		} else {
			log.Printf("✅ Text-to-speech enabled (Piper)")
		}
	}

	// Create server
	server := &Server{
		config:      cfg,
		backend:     backend,
		jobManager:  jobManager,
		agents:      make(map[string]agents.Agent),
		sttClient:   sttClient,
		ttsClient:   ttsClient,
		connections: make(map[*websocket.Conn]bool),
		configsDir:  *configsDir,
		configPath:  *configPath,
	}

	// Register agents and tools
	server.setupAgents()

	// Setup HTTP routes
	http.HandleFunc("/", server.handleIndex)
	http.HandleFunc("/ws", server.handleWebSocket)
	http.HandleFunc("/api/agents", server.handleGetAgents)
	http.HandleFunc("/api/jobs", server.handleJobs)
	http.HandleFunc("/api/jobs/", server.handleJobDetail)
	http.HandleFunc("/api/transcribe", server.handleTranscribe)
	http.HandleFunc("/api/speak", server.handleSpeak)
	http.HandleFunc("/api/configs", server.handleGetConfigs)
	http.HandleFunc("/api/config/current", server.handleGetCurrentConfig)

	// Serve static files from disk
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Start server
	addr := fmt.Sprintf(":%s", *port)
	log.Printf("🚀 PedroCLI Web UI starting on http://localhost%s", addr)
	log.Printf("📝 Using config: %s", *configPath)
	log.Printf("🤖 Backend: %s", cfg.Model.Type)
	if *configsDir != "" {
		log.Printf("📂 Configs directory: %s", *configsDir)
	}

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func (s *Server) setupAgents() {
	// Create tools
	fileTool := tools.NewFileTool()
	codeEditTool := tools.NewCodeEditTool()
	searchTool := tools.NewSearchTool(s.config.Project.Workdir)
	navigateTool := tools.NewNavigateTool(s.config.Project.Workdir)
	gitTool := tools.NewGitTool(s.config.Project.Workdir)
	bashTool := tools.NewBashTool(s.config, s.config.Project.Workdir)
	testTool := tools.NewTestTool(s.config.Project.Workdir)

	// Create builder agent
	builder := agents.NewBuilderAgent(s.config, s.backend, s.jobManager)
	builder.RegisterTool(fileTool)
	builder.RegisterTool(codeEditTool)
	builder.RegisterTool(searchTool)
	builder.RegisterTool(navigateTool)
	builder.RegisterTool(gitTool)
	builder.RegisterTool(bashTool)
	builder.RegisterTool(testTool)
	s.agents["builder"] = builder

	// Create debugger agent
	debugger := agents.NewDebuggerAgent(s.config, s.backend, s.jobManager)
	debugger.RegisterTool(fileTool)
	debugger.RegisterTool(codeEditTool)
	debugger.RegisterTool(searchTool)
	debugger.RegisterTool(navigateTool)
	debugger.RegisterTool(gitTool)
	debugger.RegisterTool(bashTool)
	debugger.RegisterTool(testTool)
	s.agents["debugger"] = debugger

	// Create reviewer agent
	reviewer := agents.NewReviewerAgent(s.config, s.backend, s.jobManager)
	reviewer.RegisterTool(fileTool)
	reviewer.RegisterTool(codeEditTool)
	reviewer.RegisterTool(searchTool)
	reviewer.RegisterTool(navigateTool)
	reviewer.RegisterTool(gitTool)
	reviewer.RegisterTool(bashTool)
	reviewer.RegisterTool(testTool)
	s.agents["reviewer"] = reviewer

	// Create triager agent
	triager := agents.NewTriagerAgent(s.config, s.backend, s.jobManager)
	triager.RegisterTool(fileTool)
	triager.RegisterTool(codeEditTool)
	triager.RegisterTool(searchTool)
	triager.RegisterTool(navigateTool)
	triager.RegisterTool(gitTool)
	triager.RegisterTool(bashTool)
	triager.RegisterTool(testTool)
	s.agents["triager"] = triager

	log.Printf("✅ Registered %d agents", len(s.agents))
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/templates/index.html")
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Register connection
	s.connMutex.Lock()
	s.connections[conn] = true
	s.connMutex.Unlock()

	log.Printf("WebSocket client connected")

	// Handle messages
	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		// Handle message
		go s.handleWSMessage(conn, msg)
	}

	// Unregister connection
	s.connMutex.Lock()
	delete(s.connections, conn)
	s.connMutex.Unlock()

	log.Printf("WebSocket client disconnected")
}

func (s *Server) handleWSMessage(conn *websocket.Conn, msg map[string]interface{}) {
	msgType, ok := msg["type"].(string)
	if !ok {
		s.sendWSError(conn, "missing message type")
		return
	}

	switch msgType {
	case "run_agent":
		s.handleRunAgent(conn, msg)
	case "get_job_status":
		s.handleGetJobStatus(conn, msg)
	default:
		s.sendWSError(conn, fmt.Sprintf("unknown message type: %s", msgType))
	}
}

func (s *Server) handleRunAgent(conn *websocket.Conn, msg map[string]interface{}) {
	agentName, ok := msg["agent"].(string)
	if !ok {
		s.sendWSError(conn, "missing agent name")
		return
	}

	input, ok := msg["input"].(map[string]interface{})
	if !ok {
		s.sendWSError(conn, "missing input")
		return
	}

	agent, ok := s.agents[agentName]
	if !ok {
		s.sendWSError(conn, fmt.Sprintf("unknown agent: %s", agentName))
		return
	}

	// Run agent in background
	go func() {
		ctx := context.Background()
		job, err := agent.Execute(ctx, input)
		if err != nil {
			s.sendWSMessage(conn, map[string]interface{}{
				"type":  "error",
				"error": err.Error(),
			})
			return
		}

		s.sendWSMessage(conn, map[string]interface{}{
			"type": "job_started",
			"job":  job,
		})

		// Send periodic updates
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			currentJob, err := s.jobManager.Get(job.ID)
			if err != nil || currentJob == nil {
				break
			}

			s.sendWSMessage(conn, map[string]interface{}{
				"type": "job_update",
				"job":  currentJob,
			})

			if currentJob.Status == jobs.StatusCompleted || currentJob.Status == jobs.StatusFailed {
				break
			}
		}
	}()
}

func (s *Server) handleGetJobStatus(conn *websocket.Conn, msg map[string]interface{}) {
	jobID, ok := msg["job_id"].(string)
	if !ok {
		s.sendWSError(conn, "missing job_id")
		return
	}

	job, err := s.jobManager.Get(jobID)
	if err != nil || job == nil {
		s.sendWSError(conn, "job not found")
		return
	}

	s.sendWSMessage(conn, map[string]interface{}{
		"type": "job_status",
		"job":  job,
	})
}

func (s *Server) sendWSMessage(conn *websocket.Conn, msg map[string]interface{}) {
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("WebSocket write error: %v", err)
	}
}

func (s *Server) sendWSError(conn *websocket.Conn, errMsg string) {
	s.sendWSMessage(conn, map[string]interface{}{
		"type":  "error",
		"error": errMsg,
	})
}

func (s *Server) handleGetAgents(w http.ResponseWriter, r *http.Request) {
	agentList := make([]map[string]string, 0, len(s.agents))
	for name, agent := range s.agents {
		agentList = append(agentList, map[string]string{
			"name":        name,
			"description": agent.Description(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agents": agentList,
	})
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	allJobs := s.jobManager.List()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jobs": allJobs,
	})
}

func (s *Server) handleJobDetail(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Path[len("/api/jobs/"):]
	job, err := s.jobManager.Get(jobID)

	if err != nil || job == nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func (s *Server) handleTranscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.sttClient == nil {
		http.Error(w, "Speech-to-text not available. Start server with -whisper-bin and -whisper-model flags.", http.StatusServiceUnavailable)
		return
	}

	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Get audio file
	file, _, err := r.FormFile("audio")
	if err != nil {
		http.Error(w, "Missing audio file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Transcribe audio
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	text, err := s.sttClient.TranscribeStream(ctx, file)
	if err != nil {
		log.Printf("Transcription error: %v", err)
		http.Error(w, "Transcription failed", http.StatusInternalServerError)
		return
	}

	// Return transcription
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"text": text,
	})
}

func (s *Server) handleSpeak(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.ttsClient == nil {
		http.Error(w, "Text-to-speech not available. Start server with -piper-bin and -piper-model flags.", http.StatusServiceUnavailable)
		return
	}

	// Parse JSON request
	var request struct {
		Text string `json:"text"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if request.Text == "" {
		http.Error(w, "Missing text field", http.StatusBadRequest)
		return
	}

	// Synthesize speech
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	audioData, err := s.ttsClient.Synthesize(ctx, request.Text)
	if err != nil {
		log.Printf("TTS error: %v", err)
		http.Error(w, "Speech synthesis failed", http.StatusInternalServerError)
		return
	}

	// Return audio
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(audioData)))
	w.Write(audioData)
}

func (s *Server) handleGetConfigs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.configsDir == "" {
		http.Error(w, "Configs directory not configured. Start server with -configs-dir flag.", http.StatusServiceUnavailable)
		return
	}

	// Read config directory
	entries, err := os.ReadDir(s.configsDir)
	if err != nil {
		log.Printf("Failed to read configs directory: %v", err)
		http.Error(w, "Failed to read configs directory", http.StatusInternalServerError)
		return
	}

	// Filter for .json files
	var configs []map[string]string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		// Try to parse config to get metadata
		configPath := filepath.Join(s.configsDir, name)
		cfg, err := config.Load(configPath)

		configInfo := map[string]string{
			"name": name,
			"path": configPath,
		}

		if err == nil {
			// Add metadata if config loaded successfully
			configInfo["backend"] = cfg.Model.Type
			configInfo["model"] = getModelName(cfg)
			configInfo["project"] = cfg.Project.Name
		}

		configs = append(configs, configInfo)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"configs": configs,
	})
}

func (s *Server) handleGetCurrentConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    s.configPath,
		"backend": s.config.Model.Type,
		"model":   getModelName(s.config),
		"project": s.config.Project.Name,
		"debug":   s.config.Debug.Enabled,
	})
}

// getModelName extracts the model name from config
func getModelName(cfg *config.Config) string {
	switch cfg.Model.Type {
	case "ollama":
		return cfg.Model.ModelName
	case "anthropic":
		return cfg.Model.ModelName
	case "llamacpp":
		return filepath.Base(cfg.Model.ModelPath)
	default:
		return cfg.Model.ModelName
	}
}
