package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/tools"
)

func TestBuilderAgent_E2E(t *testing.T) {
	SkipUnlessE2E(t)

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	// Create builder agent
	builder := agents.NewBuilderAgent(env.Config, env.Backend, env.JobManager)

	// Register tools
	registerTools(builder, env)

	// Mock backend response
	mockBackend := env.Backend.(*MockBackend)
	mockBackend.SetResponse("default", `I'll create a simple greeting function.

{
  "tool": "file",
  "arguments": {
    "action": "write",
    "path": "greet.go",
    "content": "package main\n\nfunc Greet(name string) string {\n\treturn \"Hello, \" + name + \"!\"\n}\n"
  }
}

**TASK_COMPLETE**
`)

	// Execute builder agent
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := map[string]interface{}{
		"description": "Add a simple greeting function",
	}

	job, err := builder.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Builder agent failed: %v", err)
	}

	// Wait for job to complete
	waitForJob(t, env.JobManager, job.ID, 10*time.Second)

	// Verify job completed successfully
	finalJob, err := env.JobManager.Get(job.ID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if finalJob.Status != jobs.StatusCompleted {
		t.Errorf("Expected job status %s, got %s", jobs.StatusCompleted, finalJob.Status)
	}

	// Verify file was created
	env.AssertFileExists("greet.go")
	env.AssertFileContains("greet.go", "func Greet")
}

func TestDebuggerAgent_E2E(t *testing.T) {
	SkipUnlessE2E(t)

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	// Create a buggy file
	env.CreateFile("buggy.go", `package main

func Add(a, b int) int {
	return a - b  // Bug: should be +
}
`)

	// Create debugger agent
	debugger := agents.NewDebuggerAgent(env.Config, env.Backend, env.JobManager)
	registerTools(debugger, env)

	// Mock backend response
	mockBackend := env.Backend.(*MockBackend)
	mockBackend.SetResponse("default", `I found the issue. The Add function is using subtraction instead of addition.

{
  "tool": "code_edit",
  "arguments": {
    "action": "edit_lines",
    "path": "buggy.go",
    "start_line": 3,
    "end_line": 5,
    "content": "func Add(a, b int) int {\n\treturn a + b\n}\n"
  }
}

**TASK_COMPLETE**
`)

	// Execute debugger agent
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := map[string]interface{}{
		"symptoms": "Add function returns wrong result",
	}

	job, err := debugger.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Debugger agent failed: %v", err)
	}

	// Wait for job to complete
	waitForJob(t, env.JobManager, job.ID, 10*time.Second)

	// Verify job completed
	finalJob, err := env.JobManager.Get(job.ID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if finalJob.Status != jobs.StatusCompleted {
		t.Errorf("Expected job status %s, got %s", jobs.StatusCompleted, finalJob.Status)
	}

	// Verify fix was applied
	content, _ := env.ReadFile("buggy.go")
	if !contains(content, "a + b") {
		t.Errorf("Expected buggy.go to contain 'a + b', got: %s", content)
	}
}

func TestReviewerAgent_E2E(t *testing.T) {
	SkipUnlessE2E(t)

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	// Create some code to review
	env.CreateFile("feature.go", `package main

func Process(data string) string {
	// TODO: Add validation
	return data
}
`)

	// Create reviewer agent
	reviewer := agents.NewReviewerAgent(env.Config, env.Backend, env.JobManager)
	registerTools(reviewer, env)

	// Mock backend response
	mockBackend := env.Backend.(*MockBackend)
	mockBackend.SetResponse("default", `I've reviewed the code. Here are my findings:

1. Missing input validation
2. No error handling
3. TODO comment should be addressed

**TASK_COMPLETE**
`)

	// Execute reviewer agent
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := map[string]interface{}{
		"branch": "test-branch",
	}

	job, err := reviewer.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Reviewer agent failed: %v", err)
	}

	// Wait for job to complete
	waitForJob(t, env.JobManager, job.ID, 10*time.Second)

	// Verify job completed
	finalJob, err := env.JobManager.Get(job.ID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if finalJob.Status != jobs.StatusCompleted {
		t.Errorf("Expected job status %s, got %s", jobs.StatusCompleted, finalJob.Status)
	}
}

func TestTriagerAgent_E2E(t *testing.T) {
	SkipUnlessE2E(t)

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	// Create triager agent
	triager := agents.NewTriagerAgent(env.Config, env.Backend, env.JobManager)
	registerTools(triager, env)

	// Mock backend response
	mockBackend := env.Backend.(*MockBackend)
	mockBackend.SetResponse("default", `I've analyzed the issue. Here's my diagnosis:

**Root Cause**: Missing dependency in go.mod
**Severity**: High
**Impact**: Build failures
**Recommendation**: Add the missing module dependency

**TASK_COMPLETE**
`)

	// Execute triager agent
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := map[string]interface{}{
		"description": "Build fails with missing module error",
	}

	job, err := triager.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Triager agent failed: %v", err)
	}

	// Wait for job to complete
	waitForJob(t, env.JobManager, job.ID, 10*time.Second)

	// Verify job completed
	finalJob, err := env.JobManager.Get(job.ID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if finalJob.Status != jobs.StatusCompleted {
		t.Errorf("Expected job status %s, got %s", jobs.StatusCompleted, finalJob.Status)
	}
}

// Helper functions

func registerTools(agent agents.Agent, env *TestEnvironment) {
	// Register all tools with the agent
	fileTool := tools.NewFileTool()
	codeEditTool := tools.NewCodeEditTool()
	searchTool := tools.NewSearchTool(env.WorkDir)
	navigateTool := tools.NewNavigateTool(env.WorkDir)
	gitTool := tools.NewGitTool(env.WorkDir)
	bashTool := tools.NewBashTool(env.Config, env.WorkDir)
	testTool := tools.NewTestTool(env.WorkDir)

	// Type assertion to access RegisterTool
	if ba, ok := agent.(*agents.BuilderAgent); ok {
		ba.RegisterTool(fileTool)
		ba.RegisterTool(codeEditTool)
		ba.RegisterTool(searchTool)
		ba.RegisterTool(navigateTool)
		ba.RegisterTool(gitTool)
		ba.RegisterTool(bashTool)
		ba.RegisterTool(testTool)
	} else if da, ok := agent.(*agents.DebuggerAgent); ok {
		da.RegisterTool(fileTool)
		da.RegisterTool(codeEditTool)
		da.RegisterTool(searchTool)
		da.RegisterTool(navigateTool)
		da.RegisterTool(gitTool)
		da.RegisterTool(bashTool)
		da.RegisterTool(testTool)
	} else if ra, ok := agent.(*agents.ReviewerAgent); ok {
		ra.RegisterTool(fileTool)
		ra.RegisterTool(codeEditTool)
		ra.RegisterTool(searchTool)
		ra.RegisterTool(navigateTool)
		ra.RegisterTool(gitTool)
		ra.RegisterTool(bashTool)
		ra.RegisterTool(testTool)
	} else if ta, ok := agent.(*agents.TriagerAgent); ok {
		ta.RegisterTool(fileTool)
		ta.RegisterTool(codeEditTool)
		ta.RegisterTool(searchTool)
		ta.RegisterTool(navigateTool)
		ta.RegisterTool(gitTool)
		ta.RegisterTool(bashTool)
		ta.RegisterTool(testTool)
	}
}

func waitForJob(t *testing.T, jobManager *jobs.Manager, jobID string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		job, err := jobManager.Get(jobID)
		if err != nil {
			t.Fatalf("Failed to get job status: %v", err)
		}

		if job.Status == jobs.StatusCompleted || job.Status == jobs.StatusFailed {
			return
		}

		<-ticker.C
	}

	t.Fatalf("Job %s did not complete within timeout", jobID)
}
