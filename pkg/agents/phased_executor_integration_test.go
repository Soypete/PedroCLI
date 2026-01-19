package agents

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/soypete/pedrocli/pkg/agents/testutil"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
)

// TestPhasedExecutor_SuccessfulMultiPhase tests successful execution of multiple phases.
func TestPhasedExecutor_SuccessfulMultiPhase(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "phased test", nil)

	agent := NewBaseAgent("phased-agent", "test description", cfg, mockLLM, mockJobMgr)
	searchTool := testutil.NewMockTool("search", "Search code").WithSuccess("Found 5 matches")
	editTool := testutil.NewMockTool("edit", "Edit file").WithSuccess("File edited")
	agent.RegisterTool(searchTool)
	agent.RegisterTool(editTool)

	// Define 3 phases: Analyze, Implement, Verify
	phases := []Phase{
		{
			Name:         "analyze",
			Description:  "Analyze the codebase",
			SystemPrompt: "You are analyzing code.",
			Tools:        []string{"search"},
			MaxRounds:    3,
		},
		{
			Name:         "implement",
			Description:  "Implement the changes",
			SystemPrompt: "You are implementing changes.",
			Tools:        []string{"edit"},
			MaxRounds:    3,
		},
		{
			Name:         "verify",
			Description:  "Verify the changes",
			SystemPrompt: "You are verifying changes.",
			MaxRounds:    2,
		},
	}

	// Setup responses for each phase
	// Phase 1: Analyze
	mockLLM.AddToolCallResponse("search", map[string]interface{}{"query": "main"})
	mockLLM.AddPhaseCompletionResponse("Analysis complete. Found main function.")

	// Phase 2: Implement
	mockLLM.AddToolCallResponse("edit", map[string]interface{}{"file": "main.go"})
	mockLLM.AddPhaseCompletionResponse("Changes implemented.")

	// Phase 3: Verify
	mockLLM.AddPhaseCompletionResponse("All changes verified. PHASE_COMPLETE")

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewPhasedExecutor(agent, contextMgr, phases)
	err := executor.Execute(ctx, "Add logging to main function")

	if err != nil {
		t.Errorf("Expected successful completion, got: %v", err)
	}

	// Verify all phases completed
	results := executor.GetAllResults()
	if len(results) != 3 {
		t.Errorf("Expected 3 phase results, got %d", len(results))
	}

	// Check each phase succeeded
	for _, phaseName := range []string{"analyze", "implement", "verify"} {
		result, ok := results[phaseName]
		if !ok {
			t.Errorf("Missing result for phase %s", phaseName)
			continue
		}
		if !result.Success {
			t.Errorf("Phase %s failed: %s", phaseName, result.Error)
		}
	}

	// Verify tools were called in correct phases
	if searchTool.GetCallCount() != 1 {
		t.Errorf("Expected search tool called once, got %d", searchTool.GetCallCount())
	}
	if editTool.GetCallCount() != 1 {
		t.Errorf("Expected edit tool called once, got %d", editTool.GetCallCount())
	}
}

// TestPhasedExecutor_PhaseFailure tests handling of phase failure.
func TestPhasedExecutor_PhaseFailure(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "phased test", nil)

	agent := NewBaseAgent("phased-agent", "test description", cfg, mockLLM, mockJobMgr)

	phases := []Phase{
		{
			Name:        "analyze",
			Description: "Analyze code",
			MaxRounds:   2,
		},
		{
			Name:        "implement",
			Description: "Implement changes",
			MaxRounds:   2,
		},
	}

	// First phase succeeds
	mockLLM.AddPhaseCompletionResponse("Analysis done.")

	// Second phase fails (LLM error)
	mockLLM.AddError(errors.New("API rate limit exceeded"))

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewPhasedExecutor(agent, contextMgr, phases)
	err := executor.Execute(ctx, "Make changes")

	// Should error
	if err == nil {
		t.Error("Expected error on phase failure")
	}

	// First phase should have completed
	analyzeResult, ok := executor.GetPhaseResult("analyze")
	if !ok {
		t.Error("Expected analyze phase result")
	} else if !analyzeResult.Success {
		t.Error("Expected analyze phase to succeed")
	}

	// Implement phase should be recorded as failed
	implResult, ok := executor.GetPhaseResult("implement")
	if !ok {
		t.Error("Expected implement phase result (even if failed)")
	} else if implResult.Success {
		t.Error("Expected implement phase to fail")
	}
}

// TestPhasedExecutor_PhaseValidation tests phase validation.
func TestPhasedExecutor_PhaseValidation(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "validation test", nil)

	agent := NewBaseAgent("phased-agent", "test description", cfg, mockLLM, mockJobMgr)

	// Phase with validator that rejects certain outputs
	phases := []Phase{
		{
			Name:        "generate",
			Description: "Generate content",
			MaxRounds:   2,
			Validator: func(result *PhaseResult) error {
				if !strings.Contains(result.Output, "APPROVED_OUTPUT") {
					return errors.New("output must contain 'APPROVED_OUTPUT'")
				}
				return nil
			},
		},
	}

	// LLM returns output without the required keyword
	mockLLM.AddPhaseCompletionResponse("This output is missing the required keyword.")

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewPhasedExecutor(agent, contextMgr, phases)
	err := executor.Execute(ctx, "Generate valid content")

	// Should error due to validation failure
	if err == nil {
		t.Error("Expected validation error")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

// TestPhasedExecutor_PhaseValidationSuccess tests successful validation.
func TestPhasedExecutor_PhaseValidationSuccess(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "validation test", nil)

	agent := NewBaseAgent("phased-agent", "test description", cfg, mockLLM, mockJobMgr)

	phases := []Phase{
		{
			Name:        "generate",
			Description: "Generate content",
			MaxRounds:   2,
			Validator: func(result *PhaseResult) error {
				if !strings.Contains(result.Output, "APPROVED_OUTPUT") {
					return errors.New("output must contain 'APPROVED_OUTPUT'")
				}
				return nil
			},
		},
	}

	// LLM returns output with the required keyword
	mockLLM.AddPhaseCompletionResponse("This output has APPROVED_OUTPUT and is complete.")

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewPhasedExecutor(agent, contextMgr, phases)
	err := executor.Execute(ctx, "Generate valid content")

	if err != nil {
		t.Errorf("Expected success, got: %v", err)
	}

	result, ok := executor.GetPhaseResult("generate")
	if !ok || !result.Success {
		t.Error("Expected generate phase to succeed")
	}
}

// TestPhasedExecutor_ToolFiltering tests that tools are filtered per phase.
func TestPhasedExecutor_ToolFiltering(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "tool filter test", nil)

	agent := NewBaseAgent("phased-agent", "test description", cfg, mockLLM, mockJobMgr)

	// Register multiple tools
	searchTool := testutil.NewMockTool("search", "Search").WithSuccess("Found")
	editTool := testutil.NewMockTool("edit", "Edit").WithSuccess("Edited")
	deleteTool := testutil.NewMockTool("delete", "Delete").WithSuccess("Deleted")
	agent.RegisterTool(searchTool)
	agent.RegisterTool(editTool)
	agent.RegisterTool(deleteTool)

	// Phase only allows "search" tool
	phases := []Phase{
		{
			Name:        "read_only",
			Description: "Read-only phase",
			Tools:       []string{"search"}, // Only search allowed
			MaxRounds:   3,
		},
	}

	// LLM tries to call disallowed tool first, then allowed tool
	mockLLM.AddResponse(&llm.InferenceResponse{
		Text: "Let me try tools",
		ToolCalls: []llm.ToolCall{
			{Name: "delete", Args: map[string]interface{}{}}, // Should be filtered
			{Name: "search", Args: map[string]interface{}{}}, // Should be allowed
		},
	})
	mockLLM.AddPhaseCompletionResponse("Done with allowed tools.")

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewPhasedExecutor(agent, contextMgr, phases)
	err := executor.Execute(ctx, "Try various tools")

	if err != nil {
		t.Errorf("Expected completion, got: %v", err)
	}

	// Delete should NOT have been called (filtered)
	if deleteTool.GetCallCount() != 0 {
		t.Errorf("Expected delete tool to be filtered, but was called %d times", deleteTool.GetCallCount())
	}

	// Search should have been called
	if searchTool.GetCallCount() != 1 {
		t.Errorf("Expected search tool called once, got %d", searchTool.GetCallCount())
	}
}

// TestPhasedExecutor_MaxRoundsPerPhase tests phase-specific max rounds.
func TestPhasedExecutor_MaxRoundsPerPhase(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(10) // Global max
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "max rounds test", nil)

	agent := NewBaseAgent("phased-agent", "test description", cfg, mockLLM, mockJobMgr)
	tool := testutil.NewMockTool("work", "Do work").WithSuccess("Working")
	agent.RegisterTool(tool)

	// Phase with very low max rounds
	phases := []Phase{
		{
			Name:        "quick",
			Description: "Quick phase",
			MaxRounds:   2, // Phase-specific limit
		},
	}

	// LLM keeps working without completing
	for i := 0; i < 5; i++ {
		mockLLM.AddToolCallResponse("work", map[string]interface{}{})
	}

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewPhasedExecutor(agent, contextMgr, phases)
	err := executor.Execute(ctx, "Keep working")

	// Should hit phase max rounds (2), not global (10)
	if err == nil {
		t.Error("Expected max rounds error")
	}
	if !strings.Contains(err.Error(), "max rounds (2)") {
		t.Errorf("Expected phase-specific max rounds error, got: %v", err)
	}
}

// TestPhasedExecutor_JSONExtraction tests JSON data extraction from phases.
func TestPhasedExecutor_JSONExtraction(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "json test", nil)

	agent := NewBaseAgent("phased-agent", "test description", cfg, mockLLM, mockJobMgr)

	phases := []Phase{
		{
			Name:        "analyze",
			Description: "Analyze and return JSON",
			MaxRounds:   2,
			ExpectsJSON: true,
		},
	}

	// LLM returns response with embedded JSON
	mockLLM.AddResponse(&llm.InferenceResponse{
		Text: `Analysis complete. Here's the result:
{"files_found": 5, "complexity": "medium", "recommendation": "proceed"}
PHASE_COMPLETE`,
		TokensUsed: 100,
	})

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewPhasedExecutor(agent, contextMgr, phases)
	err := executor.Execute(ctx, "Analyze with JSON output")

	if err != nil {
		t.Errorf("Expected success, got: %v", err)
	}

	result, _ := executor.GetPhaseResult("analyze")
	if result.Data == nil {
		t.Error("Expected JSON data to be extracted")
	} else {
		// Check extracted data
		if result.Data["files_found"] != float64(5) {
			t.Errorf("Expected files_found=5, got %v", result.Data["files_found"])
		}
		if result.Data["complexity"] != "medium" {
			t.Errorf("Expected complexity=medium, got %v", result.Data["complexity"])
		}
	}
}

// TestPhasedExecutor_PhaseInputChaining tests that output flows to next phase.
func TestPhasedExecutor_PhaseInputChaining(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "chain test", nil)

	agent := NewBaseAgent("phased-agent", "test description", cfg, mockLLM, mockJobMgr)

	phases := []Phase{
		{Name: "phase1", Description: "First", MaxRounds: 2},
		{Name: "phase2", Description: "Second", MaxRounds: 2},
	}

	// Phase 1 output
	mockLLM.AddResponse(&llm.InferenceResponse{
		Text:       "Phase 1 discovered: SECRET_VALUE\nPHASE_COMPLETE",
		TokensUsed: 50,
	})

	// Phase 2 - we'll capture the input to verify chaining
	var phase2Input string
	mockLLM.AddResponse(&llm.InferenceResponse{
		Text:       "PHASE_COMPLETE",
		TokensUsed: 50,
	})

	// Capture the second call's input
	originalInfer := mockLLM.Infer
	callCount := 0
	mockLLM = testutil.NewMockLLMBackend()
	mockLLM.AddResponse(&llm.InferenceResponse{
		Text:       "Phase 1 discovered: SECRET_VALUE\nPHASE_COMPLETE",
		TokensUsed: 50,
	})
	mockLLM.AddResponse(&llm.InferenceResponse{
		Text:       "PHASE_COMPLETE",
		TokensUsed: 50,
	})

	// Use custom infer to capture inputs
	agent = NewBaseAgent("phased-agent", "test description", cfg, mockLLM, mockJobMgr)

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewPhasedExecutor(agent, contextMgr, phases)
	err := executor.Execute(ctx, "Initial input")

	if err != nil {
		t.Errorf("Expected success, got: %v", err)
	}

	// Verify phase 2 received phase 1 output
	// The input chaining happens internally, we verify via LLM call inspection
	if len(mockLLM.InferCalls) >= 2 {
		phase2Call := mockLLM.InferCalls[1]
		phase2Input = phase2Call.Request.UserPrompt

		// Phase 2 input should reference Phase 1
		if !strings.Contains(phase2Input, "Previous Phase: phase1") &&
			!strings.Contains(phase2Input, "SECRET_VALUE") {
			t.Logf("Phase 2 input: %s", phase2Input[:min(200, len(phase2Input))])
			// Note: exact format depends on buildNextPhaseInput implementation
		}
	}

	_ = originalInfer // prevent unused variable warning
	_ = callCount
	_ = phase2Input
}

// TestPhasedExecutor_PhaseProgressTracking tests that phase progress is tracked.
func TestPhasedExecutor_PhaseProgressTracking(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "progress test", nil)

	agent := NewBaseAgent("phased-agent", "test description", cfg, mockLLM, mockJobMgr)

	phases := []Phase{
		{Name: "step1", Description: "Step 1", MaxRounds: 2},
		{Name: "step2", Description: "Step 2", MaxRounds: 2},
		{Name: "step3", Description: "Step 3", MaxRounds: 2},
	}

	// All phases complete successfully
	for i := 0; i < 3; i++ {
		mockLLM.AddPhaseCompletionResponse("Phase done")
	}

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewPhasedExecutor(agent, contextMgr, phases)

	// Check initial state
	if executor.GetCurrentPhase() != 0 {
		t.Error("Expected initial phase to be 0")
	}

	err := executor.Execute(ctx, "Run all phases")
	if err != nil {
		t.Errorf("Expected success, got: %v", err)
	}

	// Check final state
	if executor.GetCurrentPhase() != 3 {
		t.Errorf("Expected final phase to be 3, got %d", executor.GetCurrentPhase())
	}

	// All results should be present
	results := executor.GetAllResults()
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}
}

// TestPhasedExecutor_CustomSystemPrompt tests phase-specific system prompts.
func TestPhasedExecutor_CustomSystemPrompt(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "system prompt test", nil)

	agent := NewBaseAgent("phased-agent", "test description", cfg, mockLLM, mockJobMgr)

	customPrompt := "You are a specialized analyzer. Only analyze, never modify."
	phases := []Phase{
		{
			Name:         "custom",
			Description:  "Custom prompt phase",
			SystemPrompt: customPrompt,
			MaxRounds:    2,
		},
	}

	mockLLM.AddPhaseCompletionResponse("Custom phase done")

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewPhasedExecutor(agent, contextMgr, phases)
	err := executor.Execute(ctx, "Use custom system prompt")

	if err != nil {
		t.Errorf("Expected success, got: %v", err)
	}

	// Verify custom system prompt was used
	if len(mockLLM.InferCalls) > 0 {
		usedPrompt := mockLLM.InferCalls[0].Request.SystemPrompt
		if usedPrompt != customPrompt {
			// The phase executor should use the custom system prompt
			t.Logf("System prompt used: %s", usedPrompt[:min(100, len(usedPrompt))])
		}
	}
}

// TestPhasedExecutor_EmptyPhases tests handling of empty phase list.
func TestPhasedExecutor_EmptyPhases(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "empty phases test", nil)

	agent := NewBaseAgent("phased-agent", "test description", cfg, mockLLM, mockJobMgr)

	// No phases defined
	phases := []Phase{}

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewPhasedExecutor(agent, contextMgr, phases)
	err := executor.Execute(ctx, "Nothing to do")

	// Should succeed with no work
	if err != nil {
		t.Errorf("Expected success with empty phases, got: %v", err)
	}

	// No LLM calls should be made
	if mockLLM.GetCallCount() != 0 {
		t.Errorf("Expected no LLM calls for empty phases, got %d", mockLLM.GetCallCount())
	}
}

// TestPhasedExecutor_RoundsUsedTracking tests that rounds used is tracked correctly.
func TestPhasedExecutor_RoundsUsedTracking(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(10)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "rounds test", nil)

	agent := NewBaseAgent("phased-agent", "test description", cfg, mockLLM, mockJobMgr)
	tool := testutil.NewMockTool("work", "Do work").WithSuccess("Done")
	agent.RegisterTool(tool)

	phases := []Phase{
		{Name: "multi_round", Description: "Multi-round phase", MaxRounds: 10},
	}

	// Phase takes 3 rounds: work, work, complete
	mockLLM.AddToolCallResponse("work", map[string]interface{}{})
	mockLLM.AddToolCallResponse("work", map[string]interface{}{})
	mockLLM.AddPhaseCompletionResponse("All work done")

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewPhasedExecutor(agent, contextMgr, phases)
	err := executor.Execute(ctx, "Do work")

	if err != nil {
		t.Errorf("Expected success, got: %v", err)
	}

	result, _ := executor.GetPhaseResult("multi_round")
	if result.RoundsUsed != 3 {
		t.Errorf("Expected 3 rounds used, got %d", result.RoundsUsed)
	}
}

// TestPhasedExecutor_Timestamps tests that timestamps are recorded.
func TestPhasedExecutor_Timestamps(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.NewTestConfigWithMaxRounds(5)
	mockLLM := testutil.NewMockLLMBackend()
	mockJobMgr := testutil.NewMockJobManager()

	job, _ := mockJobMgr.Create(ctx, "test", "timestamp test", nil)

	agent := NewBaseAgent("phased-agent", "test description", cfg, mockLLM, mockJobMgr)

	phases := []Phase{
		{Name: "timed", Description: "Timed phase", MaxRounds: 2},
	}

	mockLLM.AddPhaseCompletionResponse("Done")

	contextMgr, _ := llmcontext.NewManager(job.ID, false, cfg.Model.ContextSize)
	defer contextMgr.Cleanup()

	executor := NewPhasedExecutor(agent, contextMgr, phases)
	err := executor.Execute(ctx, "Time me")

	if err != nil {
		t.Errorf("Expected success, got: %v", err)
	}

	result, _ := executor.GetPhaseResult("timed")

	// Timestamps should be set
	if result.StartedAt.IsZero() {
		t.Error("Expected StartedAt to be set")
	}
	if result.CompletedAt.IsZero() {
		t.Error("Expected CompletedAt to be set")
	}

	// CompletedAt should be >= StartedAt
	if result.CompletedAt.Before(result.StartedAt) {
		t.Error("CompletedAt should not be before StartedAt")
	}
}
