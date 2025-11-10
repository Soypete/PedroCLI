package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestWebServer_APIEndpoints tests all REST API endpoints
func TestWebServer_APIEndpoints(t *testing.T) {
	SkipUnlessE2E(t)

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	// Start web server in background
	server := startWebServer(t, env)
	defer server.Stop()

	// Wait for server to be ready
	waitForServer(t, server.URL, 10*time.Second)

	t.Run("GET /api/agents returns agent list", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/agents")
		if err != nil {
			t.Fatalf("Failed to get agents: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		agents, ok := result["agents"].([]interface{})
		if !ok {
			t.Fatalf("Expected 'agents' field in response")
		}

		if len(agents) != 4 {
			t.Errorf("Expected 4 agents, got %d", len(agents))
		}

		// Verify agent names
		expectedAgents := []string{"builder", "debugger", "reviewer", "triager"}
		foundAgents := make(map[string]bool)
		for _, agentIface := range agents {
			agent := agentIface.(map[string]interface{})
			name := agent["name"].(string)
			foundAgents[name] = true
		}

		for _, expected := range expectedAgents {
			if !foundAgents[expected] {
				t.Errorf("Expected agent %q not found", expected)
			}
		}
	})

	t.Run("GET /api/jobs returns empty job list initially", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/jobs")
		if err != nil {
			t.Fatalf("Failed to get jobs: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		jobs, ok := result["jobs"].([]interface{})
		if !ok {
			t.Fatalf("Expected 'jobs' field in response")
		}

		if len(jobs) != 0 {
			t.Errorf("Expected 0 jobs initially, got %d", len(jobs))
		}
	})

	t.Run("GET /api/jobs/invalid returns 404", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/jobs/nonexistent-job-id")
		if err != nil {
			t.Fatalf("Failed to get job: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("POST /api/transcribe returns 503 without STT client", func(t *testing.T) {
		// Server doesn't have STT client configured
		resp, err := http.Post(server.URL+"/api/transcribe", "multipart/form-data", nil)
		if err != nil {
			t.Fatalf("Failed to post: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("Expected status 503, got %d", resp.StatusCode)
		}
	})

	t.Run("POST /api/speak returns 503 without TTS client", func(t *testing.T) {
		// Server doesn't have TTS client configured
		body := bytes.NewBufferString(`{"text":"Hello"}`)
		resp, err := http.Post(server.URL+"/api/speak", "application/json", body)
		if err != nil {
			t.Fatalf("Failed to post: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("Expected status 503, got %d", resp.StatusCode)
		}
	})
}

// TestWebServer_WebSocket tests WebSocket functionality
func TestWebServer_WebSocket(t *testing.T) {
	SkipUnlessE2E(t)

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	server := startWebServer(t, env)
	defer server.Stop()

	waitForServer(t, server.URL, 10*time.Second)

	// Connect to WebSocket
	wsURL := strings.Replace(server.URL, "http://", "ws://", 1) + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	t.Run("WebSocket accepts connection", func(t *testing.T) {
		// If we got here, connection was successful
		t.Log("WebSocket connection established")
	})

	t.Run("WebSocket handles invalid message type", func(t *testing.T) {
		// Send invalid message
		msg := map[string]interface{}{
			"type": "invalid_type",
		}

		if err := conn.WriteJSON(msg); err != nil {
			t.Fatalf("Failed to write message: %v", err)
		}

		// Read error response
		var response map[string]interface{}
		if err := conn.ReadJSON(&response); err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		if response["type"] != "error" {
			t.Errorf("Expected error response, got %v", response["type"])
		}
	})

	t.Run("WebSocket handles missing agent name", func(t *testing.T) {
		msg := map[string]interface{}{
			"type": "run_agent",
			// Missing "agent" field
			"input": map[string]interface{}{
				"description": "test",
			},
		}

		if err := conn.WriteJSON(msg); err != nil {
			t.Fatalf("Failed to write message: %v", err)
		}

		var response map[string]interface{}
		if err := conn.ReadJSON(&response); err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		if response["type"] != "error" {
			t.Errorf("Expected error response, got %v", response["type"])
		}
	})

	t.Run("WebSocket handles unknown agent", func(t *testing.T) {
		msg := map[string]interface{}{
			"type":  "run_agent",
			"agent": "unknown_agent",
			"input": map[string]interface{}{
				"description": "test",
			},
		}

		if err := conn.WriteJSON(msg); err != nil {
			t.Fatalf("Failed to write message: %v", err)
		}

		var response map[string]interface{}
		if err := conn.ReadJSON(&response); err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		if response["type"] != "error" {
			t.Errorf("Expected error response, got %v", response["type"])
		}

		errorMsg, ok := response["error"].(string)
		if !ok || !strings.Contains(errorMsg, "unknown agent") {
			t.Errorf("Expected 'unknown agent' error, got %v", response["error"])
		}
	})
}

// TestWebServer_JobExecution tests creating and monitoring jobs via WebSocket
// Note: This test requires Ollama to be running, so it gracefully handles backend failures
func TestWebServer_JobExecution(t *testing.T) {
	SkipUnlessE2E(t)

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	server := startWebServer(t, env)
	defer server.Stop()

	waitForServer(t, server.URL, 10*time.Second)

	// Connect to WebSocket
	wsURL := strings.Replace(server.URL, "http://", "ws://", 1) + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	t.Run("WebSocket job communication protocol works", func(t *testing.T) {
		// Send run_agent message
		msg := map[string]interface{}{
			"type":  "run_agent",
			"agent": "builder",
			"input": map[string]interface{}{
				"description": "Create a test function",
			},
		}

		if err := conn.WriteJSON(msg); err != nil {
			t.Fatalf("Failed to write message: %v", err)
		}

		// Read response - could be job_started or error (if Ollama isn't running)
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var response map[string]interface{}
		if err := conn.ReadJSON(&response); err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		responseType, ok := response["type"].(string)
		if !ok {
			t.Fatalf("Response missing 'type' field")
		}

		// Accept either job_started (if Ollama is available) or error (if not)
		if responseType == "job_started" {
			t.Log("✓ Job started successfully (Ollama is available)")

			job, ok := response["job"].(map[string]interface{})
			if !ok {
				t.Fatalf("Expected job object in response, got: %v", response)
			}

			jobID, ok := job["id"].(string)
			if !ok || jobID == "" {
				t.Errorf("Expected job ID, got %v", job["id"])
			}

			t.Logf("Job started with ID: %s", jobID)

			// Try to read job updates
			for i := 0; i < 5; i++ {
				conn.SetReadDeadline(time.Now().Add(2 * time.Second))
				var update map[string]interface{}
				if err := conn.ReadJSON(&update); err != nil {
					break
				}

				if update["type"] == "job_update" {
					t.Logf("✓ Received job update")
				}
			}
		} else if responseType == "error" {
			errorMsg := response["error"].(string)
			if strings.Contains(errorMsg, "connection refused") || strings.Contains(errorMsg, "ollama") {
				t.Logf("⚠ Ollama not available (expected in test environment): %s", errorMsg)
				t.Log("✓ WebSocket error handling works correctly")
			} else {
				t.Errorf("Unexpected error: %s", errorMsg)
			}
		} else {
			t.Errorf("Expected job_started or error, got %s", responseType)
		}
	})
}

// TestWebServer_ConcurrentConnections tests multiple WebSocket connections
func TestWebServer_ConcurrentConnections(t *testing.T) {
	SkipUnlessE2E(t)

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	server := startWebServer(t, env)
	defer server.Stop()

	waitForServer(t, server.URL, 10*time.Second)

	// Connect multiple WebSocket clients
	numClients := 5
	connections := make([]*websocket.Conn, numClients)

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1) + "/ws"

	for i := 0; i < numClients; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect client %d: %v", i, err)
		}
		connections[i] = conn
		defer conn.Close()
	}

	t.Logf("Successfully connected %d WebSocket clients", numClients)

	// Verify all connections work
	for i, conn := range connections {
		msg := map[string]interface{}{
			"type": "get_job_status",
			"job_id": "nonexistent",
		}

		if err := conn.WriteJSON(msg); err != nil {
			t.Errorf("Failed to write to client %d: %v", i, err)
		}

		var response map[string]interface{}
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err := conn.ReadJSON(&response); err != nil {
			t.Errorf("Failed to read from client %d: %v", i, err)
		}
	}
}

// TestWebServer_StaticFiles tests static file serving
func TestWebServer_StaticFiles(t *testing.T) {
	SkipUnlessE2E(t)

	// Create temporary static files
	tmpDir := t.TempDir()
	staticDir := filepath.Join(tmpDir, "web", "static")
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		t.Fatalf("Failed to create static directory: %v", err)
	}

	// Create a test CSS file
	cssContent := "body { color: red; }"
	if err := os.WriteFile(filepath.Join(staticDir, "test.css"), []byte(cssContent), 0644); err != nil {
		t.Fatalf("Failed to create test CSS: %v", err)
	}

	// Create a test HTML file (index)
	htmlContent := "<html><body>Test Page</body></html>"
	templatesDir := filepath.Join(tmpDir, "web", "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "index.html"), []byte(htmlContent), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	// Start server with custom working directory
	server := startWebServerInDir(t, env, tmpDir)
	defer server.Stop()

	waitForServer(t, server.URL, 10*time.Second)

	t.Run("GET / serves index.html", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/")
		if err != nil {
			t.Fatalf("Failed to get index: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Test Page") {
			t.Errorf("Expected index.html content, got: %s", string(body))
		}
	})

	t.Run("GET /static/test.css serves CSS file", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/static/test.css")
		if err != nil {
			t.Fatalf("Failed to get CSS: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if string(body) != cssContent {
			t.Errorf("Expected CSS content, got: %s", string(body))
		}
	})
}

// WebServerProcess represents a running web server
type WebServerProcess struct {
	cmd       *exec.Cmd
	URL       string
	configPath string
	t         *testing.T
}

func (w *WebServerProcess) Stop() {
	if w.cmd != nil && w.cmd.Process != nil {
		w.cmd.Process.Kill()
		w.cmd.Wait()
	}
	if w.configPath != "" {
		os.Remove(w.configPath)
	}
}

// startWebServer starts the web server and returns the process
func startWebServer(t *testing.T, env *TestEnvironment) *WebServerProcess {
	return startWebServerInDir(t, env, env.WorkDir)
}

// startWebServerInDir starts the web server in a specific directory
func startWebServerInDir(t *testing.T, env *TestEnvironment, workDir string) *WebServerProcess {
	// Build web server binary
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "web-server")

	// Get the actual project root (where go.mod is)
	projectRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/web-server")
	buildCmd.Dir = projectRoot
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build web server: %v\nOutput: %s", err, string(output))
	}

	// Create config file
	configPath := filepath.Join(tmpDir, ".pedroceli.json")
	createWebServerTestConfigFile(t, configPath, env.WorkDir)

	// Create unique jobs directory for this test
	jobsDir := filepath.Join(tmpDir, "jobs")
	if err := os.MkdirAll(jobsDir, 0755); err != nil {
		t.Fatalf("Failed to create jobs directory: %v", err)
	}

	// Find available port
	port := findAvailablePort(t)

	// Start web server - retry a few times if port binding fails
	var cmd *exec.Cmd
	var serverURL string
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		cmd = exec.Command(binaryPath,
			"-port", fmt.Sprintf("%d", port),
			"-config", configPath,
			"-jobs-dir", jobsDir,
		)
		cmd.Dir = workDir

		// Capture output for debugging
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start web server: %v", err)
		}

		serverURL = fmt.Sprintf("http://localhost:%d", port)

		// Give server a moment to start and check if it bound successfully
		time.Sleep(200 * time.Millisecond)

		// Test if server is actually running on this port
		resp, err := http.Get(serverURL + "/api/agents")
		if err == nil {
			resp.Body.Close()
			// Server is running successfully
			break
		}

		// Server failed to bind, kill it and try a new port
		cmd.Process.Kill()
		cmd.Wait()

		if i < maxRetries-1 {
			t.Logf("Port %d failed, retrying with new port...", port)
			port = findAvailablePort(t)
		} else {
			t.Fatalf("Failed to start server after %d retries", maxRetries)
		}
	}

	return &WebServerProcess{
		cmd:       cmd,
		URL:       serverURL,
		configPath: configPath,
		t:         t,
	}
}

// waitForServer waits for the server to be ready
func waitForServer(t *testing.T, url string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url + "/api/agents")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("Server did not become ready within %v", timeout)
}

// findAvailablePort finds an available port for testing
func findAvailablePort(t *testing.T) int {
	// Create a listener on port 0 to get a random free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		// Fallback to random port in test range
		return 8000 + (os.Getpid() % 1000)
	}
	defer listener.Close()

	// Extract port from listener address
	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port
}

// createWebServerTestConfigFile creates a test configuration file for web server
func createWebServerTestConfigFile(t *testing.T, path string, workDir string) {
	config := map[string]interface{}{
		"model": map[string]interface{}{
			"type":        "ollama",
			"model_name":  "qwen2.5-coder:7b",
			"ollama_url":  "http://localhost:11434",
			"temperature": 0.2,
		},
		"project": map[string]interface{}{
			"name":       "TestProject",
			"workdir":    workDir,
			"tech_stack": []string{"Go"},
		},
		"tools": map[string]interface{}{
			"allowed_bash_commands": []string{"echo", "ls", "cat", "pwd"},
			"forbidden_commands":    []string{"rm", "mv", "dd", "sudo"},
		},
		"limits": map[string]interface{}{
			"max_task_duration_minutes": 5,
			"max_inference_runs":        5,
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
}
