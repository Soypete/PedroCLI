package agents

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/tools"
)

func TestCheckpoint_SaveAndLoad(t *testing.T) {
	// Create test config with checkpoints enabled
	cfg := &config.Config{
		Limits: config.LimitsConfig{
			EnablePhaseCheckpoints: true,
			MaxInferenceRuns:       10,
		},
		Debug: config.DebugConfig{
			Enabled: false,
		},
	}

	// Create mock LLM
	mockLLM := &mockLLMBackend{
		responses: []string{
			"Phase 1 complete",
			"Phase 2 complete",
		},
	}

	// Create context manager with test job dir
	contextMgr, err := llmcontext.NewManager("test-checkpoint", false, 32768)
	if err != nil {
		t.Fatalf("Failed to create context manager: %v", err)
	}
	t.Cleanup(func() { contextMgr.Cleanup() })

	// Create base agent
	agent := &BaseAgent{
		config: cfg,
		llm:    mockLLM,
		tools:  make(map[string]tools.Tool),
	}

	// Define test phases
	phases := []Phase{
		{
			Name:        "phase1",
			Description: "First phase",
			MaxRounds:   2,
		},
		{
			Name:        "phase2",
			Description: "Second phase",
			MaxRounds:   2,
		},
	}

	// Create phased executor
	executor := NewPhasedExecutor(agent, contextMgr, phases)

	// Execute first phase only
	executor.currentPhase = 1 // Simulate completing phase 0
	executor.phaseResults["phase1"] = &PhaseResult{
		PhaseName:   "phase1",
		Success:     true,
		Output:      "Phase 1 output",
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		RoundsUsed:  1,
	}

	// Save checkpoint
	err = executor.SaveCheckpoint("Next phase input")
	if err != nil {
		t.Fatalf("SaveCheckpoint() failed: %v", err)
	}

	// Verify checkpoint file was created
	checkpointPath := filepath.Join(contextMgr.GetJobDir(), "checkpoint.json")
	if _, err := os.Stat(checkpointPath); os.IsNotExist(err) {
		t.Error("Checkpoint file was not created")
	}

	// Load checkpoint
	loaded, err := LoadCheckpoint(contextMgr.GetJobDir())
	if err != nil {
		t.Fatalf("LoadCheckpoint() failed: %v", err)
	}

	// Verify checkpoint contents
	if loaded.JobID != "test-checkpoint" {
		t.Errorf("JobID = %s, want test-checkpoint", loaded.JobID)
	}
	if loaded.CurrentPhase != 1 {
		t.Errorf("CurrentPhase = %d, want 1", loaded.CurrentPhase)
	}
	if len(loaded.CompletedPhases) != 1 {
		t.Errorf("CompletedPhases count = %d, want 1", len(loaded.CompletedPhases))
	}
	if loaded.CompletedPhases[0] != "phase1" {
		t.Errorf("CompletedPhases[0] = %s, want phase1", loaded.CompletedPhases[0])
	}
	if loaded.LastInput != "Next phase input" {
		t.Errorf("LastInput = %s, want 'Next phase input'", loaded.LastInput)
	}
	if loaded.PhaseResults["phase1"] == nil {
		t.Error("PhaseResults[phase1] should not be nil")
	}
}

func TestCheckpoint_CanResume(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// No checkpoint exists
	if CanResume(tempDir) {
		t.Error("CanResume() should return false when no checkpoint exists")
	}

	// Create checkpoint file
	checkpoint := Checkpoint{
		Version:      1,
		JobID:        "test",
		CurrentPhase: 1,
	}
	data, _ := json.Marshal(checkpoint)
	checkpointPath := filepath.Join(tempDir, "checkpoint.json")
	if err := os.WriteFile(checkpointPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test checkpoint: %v", err)
	}

	// Checkpoint exists
	if !CanResume(tempDir) {
		t.Error("CanResume() should return true when checkpoint exists")
	}
}

func TestCheckpoint_DisabledConfig(t *testing.T) {
	// Create test config with checkpoints DISABLED
	cfg := &config.Config{
		Limits: config.LimitsConfig{
			EnablePhaseCheckpoints: false, // Disabled!
			MaxInferenceRuns:       10,
		},
		Debug: config.DebugConfig{
			Enabled: false,
		},
	}

	contextMgr, err := llmcontext.NewManager("test-no-checkpoint", false, 32768)
	if err != nil {
		t.Fatalf("Failed to create context manager: %v", err)
	}
	t.Cleanup(func() { contextMgr.Cleanup() })

	agent := &BaseAgent{
		config: cfg,
		llm:    &mockLLMBackend{},
		tools:  make(map[string]tools.Tool),
	}

	phases := []Phase{{Name: "test", Description: "Test"}}
	executor := NewPhasedExecutor(agent, contextMgr, phases)

	// Save checkpoint should return nil (no error) but not create file
	err = executor.SaveCheckpoint("test input")
	if err != nil {
		t.Errorf("SaveCheckpoint() should not error when disabled, got: %v", err)
	}

	// Verify NO checkpoint file was created
	checkpointPath := filepath.Join(contextMgr.GetJobDir(), "checkpoint.json")
	if _, err := os.Stat(checkpointPath); !os.IsNotExist(err) {
		t.Error("Checkpoint file should NOT be created when disabled")
	}
}

func TestCheckpoint_InvalidLoad(t *testing.T) {
	// Try to load from non-existent directory
	_, err := LoadCheckpoint("/nonexistent/path")
	if err == nil {
		t.Error("LoadCheckpoint() should error on non-existent path")
	}

	// Try to load from directory with invalid JSON
	tempDir := t.TempDir()
	checkpointPath := filepath.Join(tempDir, "checkpoint.json")
	if err := os.WriteFile(checkpointPath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write invalid checkpoint: %v", err)
	}

	_, err = LoadCheckpoint(tempDir)
	if err == nil {
		t.Error("LoadCheckpoint() should error on invalid JSON")
	}
}

func TestCheckpoint_GetCompletedPhaseNames(t *testing.T) {
	cfg := &config.Config{
		Limits: config.LimitsConfig{
			EnablePhaseCheckpoints: true,
		},
	}

	contextMgr, err := llmcontext.NewManager("test-names", false, 32768)
	if err != nil {
		t.Fatalf("Failed to create context manager: %v", err)
	}
	t.Cleanup(func() { contextMgr.Cleanup() })

	phases := []Phase{
		{Name: "analyze"},
		{Name: "plan"},
		{Name: "implement"},
		{Name: "verify"},
	}

	agent := &BaseAgent{config: cfg}
	executor := NewPhasedExecutor(agent, contextMgr, phases)

	// No phases completed yet
	names := executor.getCompletedPhaseNames()
	if len(names) != 0 {
		t.Errorf("Expected 0 completed phases, got %d", len(names))
	}

	// Complete 2 phases
	executor.currentPhase = 2
	names = executor.getCompletedPhaseNames()
	if len(names) != 2 {
		t.Errorf("Expected 2 completed phases, got %d", len(names))
	}
	if names[0] != "analyze" || names[1] != "plan" {
		t.Errorf("Unexpected phase names: %v", names)
	}

	// Complete all phases
	executor.currentPhase = 4
	names = executor.getCompletedPhaseNames()
	if len(names) != 4 {
		t.Errorf("Expected 4 completed phases, got %d", len(names))
	}
}

// mockLLMBackend for testing
type mockLLMBackend struct {
	responses []string
	callCount int
}

func (m *mockLLMBackend) Infer(ctx context.Context, req *llm.InferenceRequest) (*llm.InferenceResponse, error) {
	response := "TASK_COMPLETE"
	if m.callCount < len(m.responses) {
		response = m.responses[m.callCount]
	}
	m.callCount++

	return &llm.InferenceResponse{
		Text:       response,
		NextAction: "COMPLETE",
		TokensUsed: 100,
	}, nil
}

func (m *mockLLMBackend) GetContextWindow() int {
	return 32768
}

func (m *mockLLMBackend) GetUsableContext() int {
	return 24576
}
