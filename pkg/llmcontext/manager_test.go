package llmcontext

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name      string
		jobID     string
		debugMode bool
	}{
		{
			name:      "create manager with debug off",
			jobID:     "test-job-1",
			debugMode: false,
		},
		{
			name:      "create manager with debug on",
			jobID:     "test-job-2",
			debugMode: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, err := NewManager(tt.jobID, tt.debugMode)
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}
			defer func() {
		if err := mgr.Cleanup(); err != nil {
			t.Errorf("Cleanup() error = %v", err)
		}
	}()

			if mgr.jobID != tt.jobID {
				t.Errorf("jobID = %v, want %v", mgr.jobID, tt.jobID)
			}

			if mgr.debugMode != tt.debugMode {
				t.Errorf("debugMode = %v, want %v", mgr.debugMode, tt.debugMode)
			}

			// Verify directory was created
			if _, err := os.Stat(mgr.jobDir); os.IsNotExist(err) {
				t.Errorf("Job directory was not created: %s", mgr.jobDir)
			}

			// Verify directory name contains job ID
			if !strings.Contains(mgr.jobDir, tt.jobID) {
				t.Errorf("Job directory %s should contain job ID %s", mgr.jobDir, tt.jobID)
			}
		})
	}
}

func TestGetJobDir(t *testing.T) {
	mgr, err := NewManager("test-job", false)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer func() {
		if err := mgr.Cleanup(); err != nil {
			t.Errorf("Cleanup() error = %v", err)
		}
	}()

	jobDir := mgr.GetJobDir()
	if jobDir == "" {
		t.Error("GetJobDir() should not return empty string")
	}

	if jobDir != mgr.jobDir {
		t.Errorf("GetJobDir() = %v, want %v", jobDir, mgr.jobDir)
	}
}

func TestSavePrompt(t *testing.T) {
	mgr, err := NewManager("test-job", false)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer func() {
		if err := mgr.Cleanup(); err != nil {
			t.Errorf("Cleanup() error = %v", err)
		}
	}()

	tests := []struct {
		name   string
		prompt string
	}{
		{
			name:   "save simple prompt",
			prompt: "This is a test prompt",
		},
		{
			name:   "save multiline prompt",
			prompt: "Line 1\nLine 2\nLine 3",
		},
		{
			name:   "save empty prompt",
			prompt: "",
		},
	}

	// Save all prompts first
	for _, tt := range tests {
		err := mgr.SavePrompt(tt.prompt)
		if err != nil {
			t.Errorf("SavePrompt() error = %v for %s", err, tt.name)
		}
	}

	// Verify all files were created correctly
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Files are numbered starting from 001
			expectedFile := filepath.Join(mgr.jobDir, fmt.Sprintf("%03d-prompt.txt", i+1))
			content, err := os.ReadFile(expectedFile)
			if err != nil {
				t.Errorf("Failed to read saved prompt: %v", err)
				return
			}

			if string(content) != tt.prompt {
				t.Errorf("Saved content = %q, want %q", string(content), tt.prompt)
			}
		})
	}
}

func TestSaveResponse(t *testing.T) {
	mgr, err := NewManager("test-job", false)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer func() {
		if err := mgr.Cleanup(); err != nil {
			t.Errorf("Cleanup() error = %v", err)
		}
	}()

	response := "This is a test response"
	err = mgr.SaveResponse(response)
	if err != nil {
		t.Errorf("SaveResponse() error = %v", err)
	}

	// Verify file was created
	expectedFile := filepath.Join(mgr.jobDir, "001-response.txt")
	content, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Errorf("Failed to read saved response: %v", err)
	}

	if string(content) != response {
		t.Errorf("Saved content = %q, want %q", string(content), response)
	}
}

func TestSaveToolCalls(t *testing.T) {
	mgr, err := NewManager("test-job", false)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer func() {
		if err := mgr.Cleanup(); err != nil {
			t.Errorf("Cleanup() error = %v", err)
		}
	}()

	calls := []ToolCall{
		{
			Name: "file",
			Args: map[string]interface{}{
				"action": "read",
				"path":   "/tmp/test.txt",
			},
		},
		{
			Name: "git",
			Args: map[string]interface{}{
				"action": "status",
			},
		},
	}

	err = mgr.SaveToolCalls(calls)
	if err != nil {
		t.Errorf("SaveToolCalls() error = %v", err)
	}

	// Verify file was created
	expectedFile := filepath.Join(mgr.jobDir, "001-tool-calls.json")
	content, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Errorf("Failed to read saved tool calls: %v", err)
	}

	// Verify JSON is valid and matches
	var savedCalls []ToolCall
	if err := json.Unmarshal(content, &savedCalls); err != nil {
		t.Errorf("Failed to unmarshal saved tool calls: %v", err)
	}

	if len(savedCalls) != len(calls) {
		t.Errorf("Saved %d calls, want %d", len(savedCalls), len(calls))
	}

	if savedCalls[0].Name != calls[0].Name {
		t.Errorf("First call name = %v, want %v", savedCalls[0].Name, calls[0].Name)
	}
}

func TestSaveToolResults(t *testing.T) {
	mgr, err := NewManager("test-job", false)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer func() {
		if err := mgr.Cleanup(); err != nil {
			t.Errorf("Cleanup() error = %v", err)
		}
	}()

	results := []ToolResult{
		{
			Name:    "file",
			Success: true,
			Output:  "File content here",
		},
		{
			Name:    "git",
			Success: false,
			Error:   "Not a git repository",
		},
	}

	err = mgr.SaveToolResults(results)
	if err != nil {
		t.Errorf("SaveToolResults() error = %v", err)
	}

	// Verify file was created
	expectedFile := filepath.Join(mgr.jobDir, "001-tool-results.json")
	content, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Errorf("Failed to read saved tool results: %v", err)
	}

	// Verify JSON is valid
	var savedResults []ToolResult
	if err := json.Unmarshal(content, &savedResults); err != nil {
		t.Errorf("Failed to unmarshal saved tool results: %v", err)
	}

	if len(savedResults) != len(results) {
		t.Errorf("Saved %d results, want %d", len(savedResults), len(results))
	}
}

func TestGetHistory(t *testing.T) {
	mgr, err := NewManager("test-job", false)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer func() {
		if err := mgr.Cleanup(); err != nil {
			t.Errorf("Cleanup() error = %v", err)
		}
	}()

	// Save some prompts and responses
	if err := mgr.SavePrompt("First prompt"); err != nil {
		t.Fatalf("SavePrompt() error = %v", err)
	}
	if err := mgr.SaveResponse("First response"); err != nil {
		t.Fatalf("SaveResponse() error = %v", err)
	}
	if err := mgr.SavePrompt("Second prompt"); err != nil {
		t.Fatalf("SavePrompt() error = %v", err)
	}
	if err := mgr.SaveResponse("Second response"); err != nil {
		t.Fatalf("SaveResponse() error = %v", err)
	}

	history, err := mgr.GetHistory()
	if err != nil {
		t.Errorf("GetHistory() error = %v", err)
	}

	// Verify all content is present
	if !strings.Contains(history, "First prompt") {
		t.Error("History should contain 'First prompt'")
	}
	if !strings.Contains(history, "First response") {
		t.Error("History should contain 'First response'")
	}
	if !strings.Contains(history, "Second prompt") {
		t.Error("History should contain 'Second prompt'")
	}
	if !strings.Contains(history, "Second response") {
		t.Error("History should contain 'Second response'")
	}

	// Verify file markers are present
	if !strings.Contains(history, "===") {
		t.Error("History should contain file markers (===)")
	}
}

func TestGetHistoryWithinBudget(t *testing.T) {
	mgr, err := NewManager("test-job", false)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer func() {
		if err := mgr.Cleanup(); err != nil {
			t.Errorf("Cleanup() error = %v", err)
		}
	}()

	// Save multiple prompts and responses
	for i := 1; i <= 5; i++ {
		prompt := strings.Repeat("a", 100) // 100 chars ≈ 25 tokens
		response := strings.Repeat("b", 100)
		if err := mgr.SavePrompt(prompt); err != nil {
			t.Fatalf("SavePrompt() error = %v", err)
		}
		if err := mgr.SaveResponse(response); err != nil {
			t.Fatalf("SaveResponse() error = %v", err)
		}
	}

	tests := []struct {
		name         string
		budget       int
		expectRecent bool // Should contain recent prompts
	}{
		{
			name:         "small budget",
			budget:       50, // Should fit 1-2 files
			expectRecent: true,
		},
		{
			name:         "medium budget",
			budget:       200, // Should fit several files
			expectRecent: true,
		},
		{
			name:         "large budget",
			budget:       1000, // Should fit all files
			expectRecent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			history, err := mgr.GetHistoryWithinBudget(tt.budget)
			if err != nil {
				t.Errorf("GetHistoryWithinBudget() error = %v", err)
			}

			if history == "" {
				t.Error("History should not be empty")
			}

			// History should respect budget
			// We can't check exact token count, but verify something was returned
			if tt.expectRecent && !strings.Contains(history, "aaaaa") {
				t.Error("History should contain recent content")
			}
		})
	}
}

func TestCompactHistory(t *testing.T) {
	mgr, err := NewManager("test-job", false)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer func() {
		if err := mgr.Cleanup(); err != nil {
			t.Errorf("Cleanup() error = %v", err)
		}
	}()

	// Save multiple prompts with tool calls
	for i := 1; i <= 5; i++ {
		if err := mgr.SavePrompt(strings.Repeat("prompt", 10)); err != nil {
			t.Fatalf("SavePrompt() error = %v", err)
		}
		if err := mgr.SaveResponse(strings.Repeat("response", 10)); err != nil {
			t.Fatalf("SaveResponse() error = %v", err)
		}

		// Save tool calls and results for summarization
		calls := []ToolCall{
			{Name: "file", Args: map[string]interface{}{"action": "read"}},
		}
		if err := mgr.SaveToolCalls(calls); err != nil {
			t.Fatalf("SaveToolCalls() error = %v", err)
		}

		results := []ToolResult{
			{
				Name:          "file",
				Success:       true,
				ModifiedFiles: []string{"/tmp/test.txt"},
			},
		}
		if err := mgr.SaveToolResults(results); err != nil {
			t.Fatalf("SaveToolResults() error = %v", err)
		}
	}

	compact, err := mgr.CompactHistory(2)
	if err != nil {
		t.Errorf("CompactHistory() error = %v", err)
	}

	// Should contain summary section
	if !strings.Contains(compact, "Previous Work Summary") {
		t.Error("Compacted history should contain summary section")
	}

	// Should contain recent section
	if !strings.Contains(compact, "Recent Context") {
		t.Error("Compacted history should contain recent context section")
	}

	// Verify recent prompts are in the compacted output
	if !strings.Contains(compact, "prompt") {
		t.Error("Compacted history should contain recent prompt content")
	}
}

func TestCompactHistoryFewFiles(t *testing.T) {
	mgr, err := NewManager("test-job", false)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer func() {
		if err := mgr.Cleanup(); err != nil {
			t.Errorf("Cleanup() error = %v", err)
		}
	}()

	// Save only 2 prompts
	if err := mgr.SavePrompt("First prompt"); err != nil {
		t.Fatalf("SavePrompt() error = %v", err)
	}
	if err := mgr.SaveResponse("First response"); err != nil {
		t.Fatalf("SaveResponse() error = %v", err)
	}

	// Request keeping 2 recent files - should just return full history
	compact, err := mgr.CompactHistory(2)
	if err != nil {
		t.Errorf("CompactHistory() error = %v", err)
	}

	if !strings.Contains(compact, "First prompt") {
		t.Error("Should contain first prompt")
	}
}

func TestCleanup(t *testing.T) {
	// Test normal cleanup (debug off)
	mgr, err := NewManager("test-cleanup-1", false)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	jobDir := mgr.jobDir

	// Verify directory exists
	if _, err := os.Stat(jobDir); os.IsNotExist(err) {
		t.Fatal("Job directory should exist before cleanup")
	}

	// Cleanup
	err = mgr.Cleanup()
	if err != nil {
		t.Errorf("Cleanup() error = %v", err)
	}

	// Verify directory was removed
	if _, err := os.Stat(jobDir); !os.IsNotExist(err) {
		t.Error("Job directory should be removed after cleanup")
	}
}

func TestCleanupDebugMode(t *testing.T) {
	// Test cleanup with debug mode (should keep files)
	mgr, err := NewManager("test-cleanup-2", true)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	jobDir := mgr.jobDir

	// Save something
	mgr.SavePrompt("test")

	// Cleanup (should keep files in debug mode)
	err = mgr.Cleanup()
	if err != nil {
		t.Errorf("Cleanup() error = %v", err)
	}

	// Verify directory still exists
	if _, err := os.Stat(jobDir); os.IsNotExist(err) {
		t.Error("Job directory should be kept in debug mode")
	}

	// Manual cleanup
	_ = os.RemoveAll(jobDir)
}

func TestCounterIncrement(t *testing.T) {
	mgr, err := NewManager("test-counter", false)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer func() {
		if err := mgr.Cleanup(); err != nil {
			t.Errorf("Cleanup() error = %v", err)
		}
	}()

	if mgr.counter != 0 {
		t.Errorf("Initial counter = %d, want 0", mgr.counter)
	}

	mgr.SavePrompt("test")
	if mgr.counter != 1 {
		t.Errorf("Counter after SavePrompt = %d, want 1", mgr.counter)
	}

	if err := mgr.SaveResponse("test"); err != nil {
		t.Fatalf("SaveResponse() error = %v", err)
	}
	if mgr.counter != 2 {
		t.Errorf("Counter after SaveResponse = %d, want 2", mgr.counter)
	}

	if err := mgr.SaveToolCalls([]ToolCall{}); err != nil {
		t.Fatalf("SaveToolCalls() error = %v", err)
	}
	if mgr.counter != 3 {
		t.Errorf("Counter after SaveToolCalls = %d, want 3", mgr.counter)
	}

	if err := mgr.SaveToolResults([]ToolResult{}); err != nil {
		t.Fatalf("SaveToolResults() error = %v", err)
	}
	if mgr.counter != 4 {
		t.Errorf("Counter after SaveToolResults = %d, want 4", mgr.counter)
	}
}
