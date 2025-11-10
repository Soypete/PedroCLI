package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
)

// SkipUnlessE2E skips the test unless RUN_E2E_TESTS environment variable is set.
// E2E tests are expensive and should only run when explicitly enabled, such as:
// - When a PR is marked ready for review
// - When manually triggered in CI
// - During local development with: RUN_E2E_TESTS=1 go test ./test/e2e
func SkipUnlessE2E(t *testing.T) {
	if os.Getenv("RUN_E2E_TESTS") == "" {
		t.Skip("Skipping E2E test - set RUN_E2E_TESTS=1 to run")
	}
}

// TestEnvironment holds all the resources needed for E2E tests
type TestEnvironment struct {
	WorkDir    string
	Config     *config.Config
	Backend    llm.Backend
	JobManager *jobs.Manager
	T          *testing.T
}

// SetupTestEnvironment creates a test environment for E2E tests
func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	// Create temporary workspace
	workDir := t.TempDir()

	// Create test project structure
	setupTestProject(t, workDir)

	// Load test config
	cfg := createTestConfig(workDir)

	// Create mock backend
	backend := createMockBackend(cfg)

	// Create job manager
	jobManager, err := jobs.NewManager(filepath.Join(workDir, ".jobs"))
	if err != nil {
		t.Fatalf("Failed to create job manager: %v", err)
	}

	return &TestEnvironment{
		WorkDir:    workDir,
		Config:     cfg,
		Backend:    backend,
		JobManager: jobManager,
		T:          t,
	}
}

// Cleanup cleans up the test environment
func (e *TestEnvironment) Cleanup() {
	// Workspace is cleaned up by t.TempDir()
}

// setupTestProject creates a minimal test project structure
func setupTestProject(t *testing.T, workDir string) {
	// Create basic Go project structure
	files := map[string]string{
		"go.mod": `module example.com/testproject

go 1.21
`,
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`,
		"main_test.go": `package main

import "testing"

func TestMain(t *testing.T) {
	// Test passes
}
`,
		"README.md": `# Test Project

This is a test project for E2E testing.
`,
	}

	for path, content := range files {
		fullPath := filepath.Join(workDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", path, err)
		}
	}

	// Initialize git repo
	os.MkdirAll(filepath.Join(workDir, ".git"), 0755)
}

// createTestConfig creates a test configuration
func createTestConfig(workDir string) *config.Config {
	return &config.Config{
		Model: config.ModelConfig{
			Type:          "mock",
			Temperature:   0.2,
			ContextSize:   8192,
			UsableContext: 6144,
		},
		Project: config.ProjectConfig{
			Name:      "TestProject",
			Workdir:   workDir,
			TechStack: []string{"Go"},
		},
		Tools: config.ToolsConfig{
			AllowedBashCommands: []string{"echo", "ls", "cat", "pwd"},
			ForbiddenCommands:   []string{"rm", "mv", "dd", "sudo"},
		},
		Limits: config.LimitsConfig{
			MaxTaskDurationMinutes: 5,
			MaxInferenceRuns:       5,
		},
		Git: config.GitConfig{
			AlwaysDraftPR:  true,
			BranchPrefix:   "test/",
			Remote:         "origin",
		},
		Init: config.InitConfig{
			SkipChecks: true,
			Verbose:    false,
		},
	}
}

// createMockBackend creates a mock LLM backend for testing
func createMockBackend(cfg *config.Config) llm.Backend {
	return &MockBackend{
		responses: make(map[string]string),
	}
}

// MockBackend is a mock LLM backend for testing
type MockBackend struct {
	responses map[string]string
	callCount int
}

// SetResponse sets a canned response for the mock backend
func (m *MockBackend) SetResponse(key string, response string) {
	m.responses[key] = response
}

// Infer implements llm.Backend
func (m *MockBackend) Infer(ctx context.Context, req *llm.InferenceRequest) (*llm.InferenceResponse, error) {
	m.callCount++

	// Return canned response or default
	response := m.responses["default"]
	if response == "" {
		response = `I understand. Let me complete this task.

**TASK_COMPLETE**
`
	}

	return &llm.InferenceResponse{
		Text:       response,
		TokensUsed: 100,
		NextAction: "COMPLETE",
	}, nil
}

// GetContextWindow implements llm.Backend
func (m *MockBackend) GetContextWindow() int {
	return 8192
}

// GetUsableContext implements llm.Backend
func (m *MockBackend) GetUsableContext() int {
	return 6144
}

// CreateFile creates a file in the test environment
func (e *TestEnvironment) CreateFile(path, content string) error {
	fullPath := filepath.Join(e.WorkDir, path)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, []byte(content), 0644)
}

// ReadFile reads a file from the test environment
func (e *TestEnvironment) ReadFile(path string) (string, error) {
	fullPath := filepath.Join(e.WorkDir, path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// FileExists checks if a file exists in the test environment
func (e *TestEnvironment) FileExists(path string) bool {
	fullPath := filepath.Join(e.WorkDir, path)
	_, err := os.Stat(fullPath)
	return err == nil
}

// AssertFileContains asserts that a file contains the expected content
func (e *TestEnvironment) AssertFileContains(path, expected string) {
	content, err := e.ReadFile(path)
	if err != nil {
		e.T.Fatalf("Failed to read file %s: %v", path, err)
	}
	if !contains(content, expected) {
		e.T.Errorf("File %s does not contain expected content\nExpected substring: %s\nGot: %s", path, expected, content)
	}
}

// AssertFileExists asserts that a file exists
func (e *TestEnvironment) AssertFileExists(path string) {
	if !e.FileExists(path) {
		e.T.Errorf("Expected file %s to exist, but it doesn't", path)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || anyIndex(s, substr) >= 0)
}

func anyIndex(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
