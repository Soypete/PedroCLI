package e2e

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestMCP_Initialize(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	// Start MCP server
	server := startMCPServer(t, env)
	defer server.Stop()

	// Send initialize request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]interface{}{},
	}

	response := server.SendRequest(t, request)

	// Verify response
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result object, got: %v", response)
	}

	if result["protocolVersion"] != "1.0" {
		t.Errorf("Expected protocolVersion 1.0, got: %v", result["protocolVersion"])
	}

	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected serverInfo object")
	}

	if serverInfo["name"] != "pedroceli" {
		t.Errorf("Expected server name 'pedroceli', got: %v", serverInfo["name"])
	}
}

func TestMCP_ToolsList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	server := startMCPServer(t, env)
	defer server.Stop()

	// Send tools/list request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	response := server.SendRequest(t, request)

	// Verify response
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result object, got: %v", response)
	}

	toolsList, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("Expected tools array, got: %v", result["tools"])
	}

	if len(toolsList) == 0 {
		t.Error("Expected tools list to be non-empty")
	}

	// Verify each tool has required fields
	for i, toolIface := range toolsList {
		tool, ok := toolIface.(map[string]interface{})
		if !ok {
			t.Errorf("Tool %d is not an object", i)
			continue
		}

		if tool["name"] == nil {
			t.Errorf("Tool %d missing name", i)
		}
		if tool["description"] == nil {
			t.Errorf("Tool %d missing description", i)
		}
		if tool["inputSchema"] == nil {
			t.Errorf("Tool %d missing inputSchema", i)
		}
	}

	// Verify expected tools are present
	expectedTools := []string{"file", "code_edit", "search", "navigate", "git", "bash", "test"}
	for _, expectedTool := range expectedTools {
		found := false
		for _, toolIface := range toolsList {
			tool := toolIface.(map[string]interface{})
			if tool["name"] == expectedTool {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool %q not found in tools list", expectedTool)
		}
	}
}

func TestMCP_ToolCall_FileRead(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	// Create a test file
	testContent := "Hello, World!"
	env.CreateFile("test.txt", testContent)

	server := startMCPServer(t, env)
	defer server.Stop()

	// Send tools/call request to read the file
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "file",
			"arguments": map[string]interface{}{
				"action": "read",
				"path":   filepath.Join(env.WorkDir, "test.txt"),
			},
		},
	}

	response := server.SendRequest(t, request)

	// Verify response
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result object, got: %v", response)
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("Expected content array, got: %v", result["content"])
	}

	textBlock, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected text block, got: %v", content[0])
	}

	text, ok := textBlock["text"].(string)
	if !ok {
		t.Fatalf("Expected text string, got: %v", textBlock["text"])
	}

	if !contains(text, testContent) {
		t.Errorf("Expected file content %q to contain %q", text, testContent)
	}
}

func TestMCP_ToolCall_FileWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	server := startMCPServer(t, env)
	defer server.Stop()

	// Send tools/call request to write a file
	testContent := "Test content"
	testPath := filepath.Join(env.WorkDir, "newfile.txt")

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "file",
			"arguments": map[string]interface{}{
				"action":  "write",
				"path":    testPath,
				"content": testContent,
			},
		},
	}

	response := server.SendRequest(t, request)

	// Verify response
	if response["error"] != nil {
		t.Fatalf("Expected no error, got: %v", response["error"])
	}

	// Verify file was created
	env.AssertFileExists("newfile.txt")
	env.AssertFileContains("newfile.txt", testContent)
}

func TestMCP_ToolCall_SearchGrep(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	// Create files with searchable content
	env.CreateFile("file1.go", "package main\n\nfunc Hello() {\n\tfmt.Println(\"Hello\")\n}\n")
	env.CreateFile("file2.go", "package main\n\nfunc Goodbye() {\n\tfmt.Println(\"Goodbye\")\n}\n")

	server := startMCPServer(t, env)
	defer server.Stop()

	// Search for "Hello"
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "search",
			"arguments": map[string]interface{}{
				"action":  "grep",
				"pattern": "Hello",
				"path":    env.WorkDir,
			},
		},
	}

	response := server.SendRequest(t, request)

	// Verify response
	if response["error"] != nil {
		t.Fatalf("Search failed: %v", response["error"])
	}

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result object, got: %v", response)
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("Expected content array, got: %v", result["content"])
	}
}

// MCPServer represents a running MCP server for testing
type MCPServer struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	t      *testing.T
}

func startMCPServer(t *testing.T, env *TestEnvironment) *MCPServer {
	// Build MCP server binary
	tmpDir := t.TempDir()
	serverPath := filepath.Join(tmpDir, "mcp-server")

	buildCmd := exec.Command("go", "build", "-o", serverPath, "../../cmd/mcp-server")
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build MCP server: %v\nOutput: %s", err, string(output))
	}

	// Create config file
	configPath := filepath.Join(env.WorkDir, ".pedroceli.json")
	createTestConfigFile(t, configPath, env.WorkDir)

	// Start MCP server
	cmd := exec.Command(serverPath)
	cmd.Dir = env.WorkDir
	cmd.Env = append(os.Environ(), "PEDROCLI_CONFIG="+configPath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("Failed to create stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start MCP server: %v", err)
	}

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	return &MCPServer{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		t:      t,
	}
}

func (s *MCPServer) SendRequest(t *testing.T, request map[string]interface{}) map[string]interface{} {
	// Send JSON-RPC request
	requestBytes, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	if _, err := s.stdin.Write(append(requestBytes, '\n')); err != nil {
		t.Fatalf("Failed to write request: %v", err)
	}

	// Read JSON-RPC response
	scanner := bufio.NewScanner(s.stdout)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}
		t.Fatal("No response received")
	}

	var response map[string]interface{}
	if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v\nResponse: %s", err, scanner.Text())
	}

	return response
}

func (s *MCPServer) Stop() {
	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd.Wait()
	}
}
