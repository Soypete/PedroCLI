package orchestration

import (
	"testing"
	"time"
)

func TestSubagentStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   SubagentStatus
		expected SubagentStatus
	}{
		{"running", SubagentRunning, "running"},
		{"completed", SubagentCompleted, "completed"},
		{"failed", SubagentFailed, "failed"},
		{"cancelled", SubagentCancelled, "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.status != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.status)
			}
		})
	}
}

func TestNewSubagentHandle(t *testing.T) {
	handle := NewSubagentHandle("sa-123", "task-456", "explorer", "parent-789")

	if handle.ID != "sa-123" {
		t.Errorf("expected ID sa-123, got %s", handle.ID)
	}
	if handle.TaskID != "task-456" {
		t.Errorf("expected TaskID task-456, got %s", handle.TaskID)
	}
	if handle.Agent != "explorer" {
		t.Errorf("expected Agent explorer, got %s", handle.Agent)
	}
	if handle.ParentID != "parent-789" {
		t.Errorf("expected ParentID parent-789, got %s", handle.ParentID)
	}
	if handle.Status != SubagentRunning {
		t.Errorf("expected Status running, got %s", handle.Status)
	}
}

func TestSubagentHandleIsRunning(t *testing.T) {
	runningHandle := SubagentHandle{Status: SubagentRunning}
	if !runningHandle.IsRunning() {
		t.Error("expected IsRunning() to return true for running status")
	}

	completedHandle := SubagentHandle{Status: SubagentCompleted}
	if completedHandle.IsRunning() {
		t.Error("expected IsRunning() to return false for completed status")
	}
}

func TestSubagentHandleMarkCompleted(t *testing.T) {
	handle := SubagentHandle{Status: SubagentRunning}
	handle.MarkCompleted()

	if handle.Status != SubagentCompleted {
		t.Errorf("expected status completed, got %s", handle.Status)
	}
}

func TestSubagentHandleMarkFailed(t *testing.T) {
	handle := SubagentHandle{Status: SubagentRunning}
	handle.MarkFailed()

	if handle.Status != SubagentFailed {
		t.Errorf("expected status failed, got %s", handle.Status)
	}
}

func TestSubagentHandleMarkCancelled(t *testing.T) {
	handle := SubagentHandle{Status: SubagentRunning}
	handle.MarkCancelled()

	if handle.Status != SubagentCancelled {
		t.Errorf("expected status cancelled, got %s", handle.Status)
	}
}

func TestGetSubagentConfig(t *testing.T) {
	tests := []struct {
		agentType   string
		expectedMax int
	}{
		{"explorer", 10},
		{"implementer", 20},
		{"tester", 15},
		{"reviewer", 10},
		{"doc-writer", 10},
		{"unknown", 20},
	}

	for _, tt := range tests {
		t.Run(tt.agentType, func(t *testing.T) {
			cfg := GetSubagentConfig(tt.agentType)
			if cfg.MaxRounds != tt.expectedMax {
				t.Errorf("expected MaxRounds %d, got %d", tt.expectedMax, cfg.MaxRounds)
			}
		})
	}
}

func TestGetSubagentTypes(t *testing.T) {
	types := GetSubagentTypes()

	expected := []string{"explorer", "implementer", "tester", "reviewer", "doc-writer"}
	if len(types) != len(expected) {
		t.Errorf("expected %d types, got %d", len(expected), len(types))
	}

	for _, exp := range expected {
		found := false
		for _, t := range types {
			if t == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected type %s not found in list", exp)
		}
	}
}

func TestDefaultSubagentConfigs(t *testing.T) {
	if len(DefaultSubagentConfigs) == 0 {
		t.Error("expected DefaultSubagentConfigs to not be empty")
	}

	if explorer, ok := DefaultSubagentConfigs["explorer"]; !ok {
		t.Error("expected explorer config to exist")
	} else {
		if len(explorer.Tools) == 0 {
			t.Error("expected explorer tools to not be empty")
		}
		if explorer.MaxRounds <= 0 {
			t.Error("expected explorer MaxRounds to be positive")
		}
	}
}

func TestSubagentResult(t *testing.T) {
	handle := NewSubagentHandle("sa-1", "task-1", "explorer", "parent-1")
	result := &TaskResult{
		ID:      "task-1",
		Output:  "completed",
		Success: true,
	}

	subagentResult := SubagentResult{
		Handle:     handle,
		TaskResult: result,
		Duration:   time.Second,
		Error:      nil,
	}

	if subagentResult.Handle.ID != "sa-1" {
		t.Errorf("expected Handle.ID sa-1, got %s", subagentResult.Handle.ID)
	}
	if subagentResult.TaskResult == nil {
		t.Error("expected TaskResult to not be nil")
	}
	if subagentResult.Duration != time.Second {
		t.Errorf("expected Duration 1s, got %v", subagentResult.Duration)
	}
}
