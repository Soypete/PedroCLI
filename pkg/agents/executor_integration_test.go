package agents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/soypete/pedrocli/pkg/agents/testutil"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
)

// TestInferenceExecutor_SuccessfulCompletion tests that the executor completes
// when the LLM returns a TASK_COMPLETE signal.
func TestInferenceExecutor_SuccessfulCompletion(t *testing.T) {
	// Setup
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	// Create job first
	job, err := mockJobMgr.Create(ctx, "test", "test job", nil)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Create base agent
	agent := NewBaseAgent("test-agent", "test description", cfg, mockLLM, mockJobMgr)

	// Register a mock tool
	mockTool := testutil.NewMockTool("search", "Search for code").WithSuccess("Found 3 files")
	agent.RegisterTool(mockTool)

	// Setup LLM responses:
	// 1. First call: LLM requests a tool call
	// 2. Second call: LLM returns TASK_COMPLETE
	mockLLM.AddToolCallResponse("search", map[string]interface{}{"query": "test"})
	mockLLM.AddCompletionResponse("I found what I was looking for.")

	// Create context manager
	contextMgr, err := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	if err != nil {
		t.Fatalf("Failed to create context manager: %v", err)
	}
	defer contextMgr.Cleanup()

	// Create executor
	executor := NewInferenceExecutor(agent, contextMgr)

	// Execute
	err = executor.Execute(ctx, "Find all test files")

	// Verify
	if err != nil {
		t.Errorf("Expected successful completion, got error: %v", err)
	}

	// Verify LLM was called twice
	if mockLLM.GetCallCount() != 2 {
		t.Errorf("Expected 2 LLM calls, got %d", mockLLM.GetCallCount())
	}

	// Verify tool was called once
	if mockTool.GetCallCount() != 1 {
		t.Errorf("Expected 1 tool call, got %d", mockTool.GetCallCount())
	}

	// Verify the tool was called with correct args
	lastCall := mockTool.GetLastCall()
	if lastCall == nil {
		t.Fatal("Expected at least one tool call")
	}
	if lastCall.Args["query"] != "test" {
		t.Errorf("Expected query='test', got %v", lastCall.Args["query"])
	}
}

// TestInferenceExecutor_LLMError tests that the executor handles LLM errors correctly.
func TestInferenceExecutor_LLMError(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "test job", nil)

	agent := NewBaseAgent("test-agent", "test description", cfg, mockLLM, mockJobMgr)

	// LLM returns an error on first call
	mockLLM.AddError(errors.New("connection refused"))

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewInferenceExecutor(agent, contextMgr)
	err := executor.Execute(ctx, "Find all test files")

	// Should return the error
	if err == nil {
		t.Error("Expected error from LLM failure")
	}
	if !errors.Is(err, errors.New("inference failed")) {
		// Check error message contains expected text
		if err.Error() != "inference failed: connection refused" {
			t.Logf("Got error: %v (acceptable)", err)
		}
	}

	// LLM should have been called exactly once before failing
	if mockLLM.GetCallCount() != 1 {
		t.Errorf("Expected 1 LLM call before error, got %d", mockLLM.GetCallCount())
	}
}

// TestInferenceExecutor_MaxRoundsReached tests that the executor stops after max rounds.
func TestInferenceExecutor_MaxRoundsReached(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(3) // Low limit for testing
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "test job", nil)

	agent := NewBaseAgent("test-agent", "test description", cfg, mockLLM, mockJobMgr)
	mockTool := testutil.NewMockTool("search", "Search for code").WithSuccess("Found files")
	agent.RegisterTool(mockTool)

	// LLM keeps requesting tools without completing
	for i := 0; i < 5; i++ { // More responses than max rounds
		mockLLM.AddToolCallResponse("search", map[string]interface{}{"query": "more"})
	}

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewInferenceExecutor(agent, contextMgr)
	err := executor.Execute(ctx, "Keep searching forever")

	// Should error due to max rounds
	if err == nil {
		t.Error("Expected max rounds error")
	}
	if err.Error() != "max inference rounds (3) reached without completion" {
		t.Errorf("Unexpected error message: %v", err)
	}

	// Should have hit max rounds
	if mockLLM.GetCallCount() != 3 {
		t.Errorf("Expected 3 LLM calls (max rounds), got %d", mockLLM.GetCallCount())
	}
}

// TestInferenceExecutor_ToolExecutionFailure tests handling of tool failures.
func TestInferenceExecutor_ToolExecutionFailure(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "test job", nil)

	agent := NewBaseAgent("test-agent", "test description", cfg, mockLLM, mockJobMgr)

	// Tool that fails
	failingTool := testutil.NewMockTool("write_file", "Write to file").
		WithFailure("permission denied")
	agent.RegisterTool(failingTool)

	// LLM tries tool, gets failure, then completes
	mockLLM.AddToolCallResponse("write_file", map[string]interface{}{"path": "/etc/passwd"})
	mockLLM.AddCompletionResponse("The file write failed. TASK_COMPLETE")

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewInferenceExecutor(agent, contextMgr)
	err := executor.Execute(ctx, "Write to system file")

	// Should complete (LLM acknowledged the failure)
	if err != nil {
		t.Errorf("Expected completion despite tool failure, got: %v", err)
	}

	// Tool should have been called
	if failingTool.GetCallCount() != 1 {
		t.Errorf("Expected tool to be called once, got %d", failingTool.GetCallCount())
	}
}

// TestInferenceExecutor_ToolError tests handling of tool execution errors.
func TestInferenceExecutor_ToolError(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "test job", nil)

	agent := NewBaseAgent("test-agent", "test description", cfg, mockLLM, mockJobMgr)

	// Tool that returns an error (not a failure result)
	errorTool := testutil.NewMockTool("dangerous", "Dangerous operation").
		WithError(errors.New("panic: null pointer"))
	agent.RegisterTool(errorTool)

	// LLM tries tool, executor handles error gracefully, LLM then completes
	mockLLM.AddToolCallResponse("dangerous", map[string]interface{}{})
	mockLLM.AddCompletionResponse("Operation failed. Moving on. TASK_COMPLETE")

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewInferenceExecutor(agent, contextMgr)
	err := executor.Execute(ctx, "Do something dangerous")

	// Should complete - executor wraps tool errors
	if err != nil {
		t.Errorf("Expected completion despite tool error, got: %v", err)
	}
}

// TestInferenceExecutor_UnknownTool tests handling of calls to unknown tools.
func TestInferenceExecutor_UnknownTool(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "test job", nil)

	agent := NewBaseAgent("test-agent", "test description", cfg, mockLLM, mockJobMgr)
	// No tools registered!

	// LLM tries non-existent tool, then completes
	mockLLM.AddToolCallResponse("nonexistent_tool", map[string]interface{}{})
	mockLLM.AddCompletionResponse("Tool not found. TASK_COMPLETE")

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewInferenceExecutor(agent, contextMgr)
	err := executor.Execute(ctx, "Use a tool that doesn't exist")

	// Should complete - unknown tool returns a failure result
	if err != nil {
		t.Errorf("Expected completion, got: %v", err)
	}
}

// TestInferenceExecutor_CompletionSignalVariants tests various completion signals.
func TestInferenceExecutor_CompletionSignalVariants(t *testing.T) {
	signals := []string{
		"TASK_COMPLETE",
		"task_complete",
		"Task Complete",
		"I'm done with this task.",
		"All done here!",
		"The work is complete.",
		"Finished!",
	}

	for _, signal := range signals {
		t.Run(signal, func(t *testing.T) {
			ctx := context.Background()
			cfg := testutil.NewTestConfigWithMaxRounds(3)
			mockLLM := testutil.NewMockLLMBackend()
			mockJobMgr := testutil.NewMockJobManager()

			job, _ := mockJobMgr.Create(ctx, "test", "test job", nil)

			agent := NewBaseAgent("test-agent", "test description", cfg, mockLLM, mockJobMgr)

			// Response with completion signal
			mockLLM.AddResponse(&llm.InferenceResponse{
				Text:       signal,
				ToolCalls:  nil,
				TokensUsed: 50,
			})

			contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
			defer contextMgr.Cleanup()

			executor := NewInferenceExecutor(agent, contextMgr)
			err := executor.Execute(ctx, "Do something")

			if err != nil {
				t.Errorf("Signal '%s' should trigger completion, got error: %v", signal, err)
			}
		})
	}
}

// TestInferenceExecutor_MultipleToolCalls tests handling of multiple tool calls in one response.
func TestInferenceExecutor_MultipleToolCalls(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "test job", nil)

	agent := NewBaseAgent("test-agent", "test description", cfg, mockLLM, mockJobMgr)

	searchTool := testutil.NewMockTool("search", "Search").WithSuccess("Found results")
	readTool := testutil.NewMockTool("read", "Read file").WithSuccess("File contents")
	agent.RegisterTool(searchTool)
	agent.RegisterTool(readTool)

	// Response with multiple tool calls
	mockLLM.AddResponse(&llm.InferenceResponse{
		Text: "I'll search and read files.",
		ToolCalls: []llm.ToolCall{
			{Name: "search", Args: map[string]interface{}{"query": "main"}},
			{Name: "read", Args: map[string]interface{}{"path": "main.go"}},
		},
		TokensUsed: 100,
	})
	mockLLM.AddCompletionResponse("Found and read the files.")

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewInferenceExecutor(agent, contextMgr)
	err := executor.Execute(ctx, "Find main function")

	if err != nil {
		t.Errorf("Expected completion, got: %v", err)
	}

	// Both tools should have been called
	if searchTool.GetCallCount() != 1 {
		t.Errorf("Expected search tool called once, got %d", searchTool.GetCallCount())
	}
	if readTool.GetCallCount() != 1 {
		t.Errorf("Expected read tool called once, got %d", readTool.GetCallCount())
	}
}

// TestInferenceExecutor_ConversationLogging tests that conversation is logged correctly.
func TestInferenceExecutor_ConversationLogging(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "test job", nil)

	agent := NewBaseAgent("test-agent", "test description", cfg, mockLLM, mockJobMgr)
	mockTool := testutil.NewMockTool("echo", "Echo").WithSuccess("echoed")
	agent.RegisterTool(mockTool)

	mockLLM.AddToolCallResponse("echo", map[string]interface{}{"msg": "hello"})
	mockLLM.AddCompletionResponse("Done!")

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewInferenceExecutor(agent, contextMgr)
	_ = executor.Execute(ctx, "Echo hello")

	// Check conversation history
	conversation := mockJobMgr.GetConversationEntries(job.ID)
	if len(conversation) == 0 {
		t.Fatal("Expected conversation entries to be logged")
	}

	// Should have: user prompt, assistant response, tool_call, tool_result, user feedback, assistant completion
	roles := make([]string, len(conversation))
	for i, entry := range conversation {
		roles[i] = entry.Role
	}

	// Verify we have the expected sequence
	hasUser := false
	hasAssistant := false
	hasToolCall := false
	hasToolResult := false

	for _, role := range roles {
		switch role {
		case "user":
			hasUser = true
		case "assistant":
			hasAssistant = true
		case "tool_call":
			hasToolCall = true
		case "tool_result":
			hasToolResult = true
		}
	}

	if !hasUser || !hasAssistant || !hasToolCall || !hasToolResult {
		t.Errorf("Missing expected conversation roles. Got: %v", roles)
	}
}

// TestInferenceExecutor_PRCreatedSignal tests the special "pr created" completion signal.
func TestInferenceExecutor_PRCreatedSignal(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "test job", nil)

	agent := NewBaseAgent("test-agent", "test description", cfg, mockLLM, mockJobMgr)

	// Tool that returns "PR CREATED" in output
	gitTool := testutil.NewMockTool("git", "Git operations").WithSuccess("PR created: https://github.com/org/repo/pull/123")
	agent.RegisterTool(gitTool)

	mockLLM.AddToolCallResponse("git", map[string]interface{}{"action": "create_pr"})

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewInferenceExecutor(agent, contextMgr)
	err := executor.Execute(ctx, "Create a PR")

	// Should complete due to PR created signal
	if err != nil {
		t.Errorf("Expected completion on PR created, got: %v", err)
	}

	// Should only have one LLM call (completed on tool result)
	if mockLLM.GetCallCount() != 1 {
		t.Errorf("Expected 1 LLM call, got %d", mockLLM.GetCallCount())
	}
}

// TestInferenceExecutor_ContextCancellation tests handling of context cancellation.
func TestInferenceExecutor_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := testutil.NewTestConfigWithMaxRounds(10)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "test job", nil)

	agent := NewBaseAgent("test-agent", "test description", cfg, mockLLM, mockJobMgr)

	// LLM that blocks and checks context
	mockLLM.AddResponse(&llm.InferenceResponse{
		Text:       "Still working...",
		ToolCalls:  nil, // No tool calls, no completion
		TokensUsed: 50,
	})

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewInferenceExecutor(agent, contextMgr)

	// Cancel context immediately
	cancel()

	// Execute with cancelled context
	err := executor.Execute(ctx, "Long running task")

	// Should get context error eventually
	// Note: The exact behavior depends on implementation
	// At minimum, we shouldn't hang forever
	t.Logf("Result with cancelled context: err=%v", err)
}

// TestInferenceExecutor_NoToolCallsButNotDone tests the feedback loop when LLM doesn't call tools.
func TestInferenceExecutor_NoToolCallsButNotDone(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(3)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "test job", nil)

	agent := NewBaseAgent("test-agent", "test description", cfg, mockLLM, mockJobMgr)
	mockTool := testutil.NewMockTool("search", "Search").WithSuccess("Found")
	agent.RegisterTool(mockTool)

	// First response: No tools, no completion signal
	mockLLM.AddResponse(&llm.InferenceResponse{
		Text:       "I'm thinking about what to do...",
		ToolCalls:  nil,
		TokensUsed: 50,
	})
	// Second response: Uses tool after prompt
	mockLLM.AddToolCallResponse("search", map[string]interface{}{"query": "test"})
	// Third response: Completes
	mockLLM.AddCompletionResponse("Found it!")

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewInferenceExecutor(agent, contextMgr)
	err := executor.Execute(ctx, "Find something")

	if err != nil {
		t.Errorf("Expected completion, got: %v", err)
	}

	// Should have 3 LLM calls
	if mockLLM.GetCallCount() != 3 {
		t.Errorf("Expected 3 LLM calls, got %d", mockLLM.GetCallCount())
	}
}

// TestInferenceExecutor_SystemPromptVerification verifies the system prompt is included.
func TestInferenceExecutor_SystemPromptVerification(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(3)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "test job", nil)

	agent := NewBaseAgent("test-agent", "test description", cfg, mockLLM, mockJobMgr)

	mockLLM.AddCompletionResponse("Done!")

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewInferenceExecutor(agent, contextMgr)
	_ = executor.Execute(ctx, "Test task")

	// Verify system prompt was included in the request
	if len(mockLLM.InferCalls) == 0 {
		t.Fatal("Expected at least one LLM call")
	}

	firstCall := mockLLM.InferCalls[0]
	if firstCall.Request.SystemPrompt == "" {
		t.Error("Expected system prompt to be set")
	}

	// Should contain key phrases from the dynamic system prompt
	prompt := firstCall.Request.SystemPrompt
	if !containsAny(prompt, "autonomous", "coding agent", "tool") {
		t.Errorf("System prompt missing expected content: %s", prompt[:min(200, len(prompt))])
	}
}

// TestInferenceExecutor_Timeout tests behavior with a deadline context.
func TestInferenceExecutor_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	cfg := testutil.NewTestConfigWithMaxRounds(100)
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "test job", nil)

	// Create a mock LLM that delays
	mockLLM := &slowMockBackend{delay: 200 * time.Millisecond}

	agent := NewBaseAgent("test-agent", "test description", cfg, mockLLM, mockJobMgr)

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewInferenceExecutor(agent, contextMgr)
	err := executor.Execute(ctx, "Slow task")

	// Should timeout
	if err == nil {
		t.Error("Expected timeout error")
	}

	// Should be a context deadline error
	if !errors.Is(err, context.DeadlineExceeded) {
		// Wrapped error is also acceptable
		t.Logf("Got error: %v (may be wrapped)", err)
	}
}

// Helper types and functions

type slowMockBackend struct {
	delay time.Duration
}

func (s *slowMockBackend) Infer(ctx context.Context, req *llm.InferenceRequest) (*llm.InferenceResponse, error) {
	select {
	case <-time.After(s.delay):
		return &llm.InferenceResponse{
			Text:       "TASK_COMPLETE",
			TokensUsed: 10,
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *slowMockBackend) GetContextWindow() int  { return 32768 }
func (s *slowMockBackend) GetUsableContext() int  { return 24576 }
func (s *slowMockBackend) Tokenize(ctx context.Context, text string) ([]int, error) {
	return []int{1, 2, 3}, nil
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstring(s, substr))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
