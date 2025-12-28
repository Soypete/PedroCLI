package mcp

import (
	"context"
	"fmt"
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

func TestAgentToolExecuteSuccess(t *testing.T) {
	job := &jobs.Job{
		ID:     "job-123",
		Status: jobs.StatusCompleted,
		Output: map[string]interface{}{
			"response": "Feature built successfully",
		},
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
		t.Error("Expected success to be true")
	}
	// Agent tool now returns async job ID, not final output
	expectedOutput := "Job job-123 started and running in background. Use get_job_status to check progress."
	if result.Output != expectedOutput {
		t.Errorf("Expected output '%s', got '%s'", expectedOutput, result.Output)
	}
	if result.Error != "" {
		t.Errorf("Expected no error, got '%s'", result.Error)
	}
}

func TestAgentToolExecuteFailure(t *testing.T) {
	job := &jobs.Job{
		ID:     "job-456",
		Status: jobs.StatusFailed,
		Error:  "Build failed: compilation error",
	}

	agent := &mockAgent{
		name:       "builder",
		executeJob: job,
	}

	agentTool := NewAgentTool(agent)

	result, err := agentTool.Execute(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	// Agent tool returns success=true when job is started, even if it later fails
	if !result.Success {
		t.Error("Expected success to be true (job started successfully)")
	}
	// Agent tool returns job ID message, not the final error
	expectedOutput := "Job job-456 started and running in background. Use get_job_status to check progress."
	if result.Output != expectedOutput {
		t.Errorf("Expected output '%s', got '%s'", expectedOutput, result.Output)
	}
}

func TestAgentToolExecuteWithReviewText(t *testing.T) {
	job := &jobs.Job{
		ID:     "job-789",
		Status: jobs.StatusCompleted,
		Output: map[string]interface{}{
			"review_text": "Code looks good. No issues found.",
		},
	}

	agent := &mockAgent{
		name:       "reviewer",
		executeJob: job,
	}

	agentTool := NewAgentTool(agent)

	result, err := agentTool.Execute(context.Background(), map[string]interface{}{
		"branch": "feature/new-api",
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.Success {
		t.Error("Expected success to be true")
	}
	expectedOutput := "Job job-789 started and running in background. Use get_job_status to check progress."
	if result.Output != expectedOutput {
		t.Errorf("Expected output '%s', got '%s'", expectedOutput, result.Output)
	}
}

func TestAgentToolExecuteWithDiagnosis(t *testing.T) {
	job := &jobs.Job{
		ID:     "job-101",
		Status: jobs.StatusCompleted,
		Output: map[string]interface{}{
			"diagnosis": "Memory leak detected in handler.go:42",
		},
	}

	agent := &mockAgent{
		name:       "triager",
		executeJob: job,
	}

	agentTool := NewAgentTool(agent)

	result, err := agentTool.Execute(context.Background(), map[string]interface{}{
		"description": "App crashes after 1 hour",
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.Success {
		t.Error("Expected success to be true")
	}
	expectedOutput := "Job job-101 started and running in background. Use get_job_status to check progress."
	if result.Output != expectedOutput {
		t.Errorf("Expected output '%s', got '%s'", expectedOutput, result.Output)
	}
}

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
}

func TestAgentToolExecuteWithEmptyOutput(t *testing.T) {
	job := &jobs.Job{
		ID:     "job-202",
		Status: jobs.StatusCompleted,
		Output: map[string]interface{}{}, // Empty output
	}

	agent := &mockAgent{
		name:       "builder",
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
	expectedOutput := "Job job-202 started and running in background. Use get_job_status to check progress."
	if result.Output != expectedOutput {
		t.Errorf("Expected output '%s', got '%s'", expectedOutput, result.Output)
	}
}

func TestAgentToolExecuteStatusMapping(t *testing.T) {
	testCases := []struct {
		name      string
		jobStatus jobs.Status
	}{
		{
			name:      "Completed job",
			jobStatus: jobs.StatusCompleted,
		},
		{
			name:      "Failed job",
			jobStatus: jobs.StatusFailed,
		},
		{
			name:      "Running job",
			jobStatus: jobs.StatusRunning,
		},
		{
			name:      "Pending job",
			jobStatus: jobs.StatusPending,
		},
		{
			name:      "Cancelled job",
			jobStatus: jobs.StatusCancelled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			job := &jobs.Job{
				ID:     "test-job",
				Status: tc.jobStatus,
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

			// Agent tool always returns success=true when job is started,
			// regardless of the job's status. The client checks status later via get_job_status.
			if !result.Success {
				t.Errorf("Expected success=true for async job start, got %v", result.Success)
			}

			// Verify job ID is in the output message
			expectedOutput := "Job test-job started and running in background. Use get_job_status to check progress."
			if result.Output != expectedOutput {
				t.Errorf("Expected output '%s', got '%s'", expectedOutput, result.Output)
			}
		})
	}
}
