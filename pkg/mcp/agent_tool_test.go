package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/soypete/pedrocli/pkg/jobs"
)

// mockAgent is a mock implementation of agents.Agent
type mockAgent struct {
	name        string
	description string
	executeJob  *jobs.Job
	executeErr  error
}

func (m *mockAgent) Name() string {
	return m.name
}

func (m *mockAgent) Description() string {
	return m.description
}

func (m *mockAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	if m.executeErr != nil {
		return nil, m.executeErr
	}
	return m.executeJob, nil
}

func TestNewAgentTool(t *testing.T) {
	agent := &mockAgent{
		name:        "test-agent",
		description: "A test agent",
	}

	agentTool := NewAgentTool(agent)

	if agentTool == nil {
		t.Fatal("NewAgentTool() returned nil")
	}
	if agentTool.agent != agent {
		t.Error("Agent not set correctly")
	}
}

func TestAgentToolName(t *testing.T) {
	agent := &mockAgent{
		name:        "builder",
		description: "Build features",
	}

	agentTool := NewAgentTool(agent)

	if agentTool.Name() != "builder" {
		t.Errorf("Expected name 'builder', got '%s'", agentTool.Name())
	}
}

func TestAgentToolDescription(t *testing.T) {
	agent := &mockAgent{
		name:        "debugger",
		description: "Debug and fix issues",
	}

	agentTool := NewAgentTool(agent)

	if agentTool.Description() != "Debug and fix issues" {
		t.Errorf("Expected description 'Debug and fix issues', got '%s'", agentTool.Description())
	}
}

// TestAgentToolExecuteReturnsJobID verifies that Execute returns a job ID immediately
// (agents run asynchronously, so we just get the job ID back)
func TestAgentToolExecuteReturnsJobID(t *testing.T) {
	job := &jobs.Job{
		ID:     "job-123",
		Status: jobs.StatusRunning,
	}

	agent := &mockAgent{
		name:       "builder",
		executeJob: job,
	}

	agentTool := NewAgentTool(agent)

	args := map[string]interface{}{
		"description": "Add new feature",
	}

	result, err := agentTool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	if !result.Success {
		t.Error("Expected success to be true when job starts")
	}
	if !strings.Contains(result.Output, "job-123") {
		t.Errorf("Expected output to contain job ID 'job-123', got '%s'", result.Output)
	}
	if !strings.Contains(result.Output, "started") {
		t.Errorf("Expected output to indicate job started, got '%s'", result.Output)
	}
}

// TestAgentToolExecuteWithError verifies error handling when agent.Execute fails
func TestAgentToolExecuteWithError(t *testing.T) {
	agent := &mockAgent{
		name:       "builder",
		executeErr: fmt.Errorf("agent execution failed"),
		executeJob: nil,
	}

	agentTool := NewAgentTool(agent)

	result, err := agentTool.Execute(context.Background(), map[string]interface{}{})

	// Should not return error from Execute, but wrap it in result
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	if result.Success {
		t.Error("Expected success to be false")
	}
	if result.Error == "" {
		t.Error("Expected error to be set")
	}
	if !strings.Contains(result.Error, "agent execution failed") {
		t.Errorf("Expected error to contain 'agent execution failed', got '%s'", result.Error)
	}
}

// TestAgentToolExecuteMultipleJobs verifies different job IDs are returned
func TestAgentToolExecuteMultipleJobs(t *testing.T) {
	testCases := []struct {
		name      string
		jobID     string
		agentName string
	}{
		{"builder", "job-001", "builder"},
		{"reviewer", "job-002", "reviewer"},
		{"debugger", "job-003", "debugger"},
		{"triager", "job-004", "triager"},
		{"writer", "job-005", "writer"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			job := &jobs.Job{
				ID:     tc.jobID,
				Status: jobs.StatusRunning,
			}

			agent := &mockAgent{
				name:       tc.agentName,
				executeJob: job,
			}

			agentTool := NewAgentTool(agent)

			result, err := agentTool.Execute(context.Background(), map[string]interface{}{})
			if err != nil {
				t.Fatalf("Execute() returned error: %v", err)
			}

			if !result.Success {
				t.Error("Expected success to be true")
			}
			if !strings.Contains(result.Output, tc.jobID) {
				t.Errorf("Expected output to contain job ID '%s', got '%s'", tc.jobID, result.Output)
			}
		})
	}
}

// TestAgentToolExecuteOutputFormat verifies the output message format
func TestAgentToolExecuteOutputFormat(t *testing.T) {
	job := &jobs.Job{
		ID:     "job-test-format",
		Status: jobs.StatusPending,
	}

	agent := &mockAgent{
		name:       "test-agent",
		executeJob: job,
	}

	agentTool := NewAgentTool(agent)

	result, err := agentTool.Execute(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	// Verify output contains expected components
	expectedParts := []string{
		"Job",
		"job-test-format",
		"started",
		"background",
		"get_job_status",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result.Output, part) {
			t.Errorf("Expected output to contain '%s', got '%s'", part, result.Output)
		}
	}
}
