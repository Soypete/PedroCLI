package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// updateModel is a test helper that calls Update and type-asserts the result.
func updateModel(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	result, cmd := m.Update(msg)
	model, ok := result.(Model)
	if !ok {
		t.Fatalf("Update returned %T, expected Model", result)
	}
	return model, cmd
}

func TestNew(t *testing.T) {
	m := New("code", "build")

	if m.mode != "code" {
		t.Errorf("expected mode 'code', got %q", m.mode)
	}
	if m.agent != "build" {
		t.Errorf("expected agent 'build', got %q", m.agent)
	}
	if m.agentBusy {
		t.Error("expected agentBusy to be false initially")
	}
	if m.quitting {
		t.Error("expected quitting to be false initially")
	}
}

func TestModel_WindowSizeMsg(t *testing.T) {
	m := New("code", "build")

	model, _ := updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	if model.width != 120 {
		t.Errorf("expected width 120, got %d", model.width)
	}
	if model.height != 40 {
		t.Errorf("expected height 40, got %d", model.height)
	}
	if !model.ready {
		t.Error("expected ready to be true after WindowSizeMsg")
	}
}

func TestModel_AgentStartMsg(t *testing.T) {
	m := New("code", "build")

	model, _ := updateModel(t, m, AgentStartMsg{
		Agent:  "build",
		Prompt: "Add a login page",
		Phases: []string{"Analyze", "Plan", "Implement"},
	})

	if !model.agentBusy {
		t.Error("expected agentBusy to be true after AgentStartMsg")
	}
}

func TestModel_AgentDoneMsg_Success(t *testing.T) {
	m := New("code", "build")

	// Start first
	model, _ := updateModel(t, m, AgentStartMsg{Agent: "build", Prompt: "test", Phases: []string{"A"}})
	if !model.agentBusy {
		t.Fatal("expected agentBusy after start")
	}

	// Complete
	model, _ = updateModel(t, model, AgentDoneMsg{Success: true, Output: "done"})
	if model.agentBusy {
		t.Error("expected agentBusy to be false after AgentDoneMsg")
	}
}

func TestModel_AgentDoneMsg_Failure(t *testing.T) {
	m := New("code", "build")

	model, _ := updateModel(t, m, AgentStartMsg{Agent: "build", Prompt: "test", Phases: []string{"A"}})

	model, _ = updateModel(t, model, AgentDoneMsg{Success: false, Error: "something broke"})
	if model.agentBusy {
		t.Error("expected agentBusy to be false after failed AgentDoneMsg")
	}
}

func TestModel_ModeChangeMsg(t *testing.T) {
	m := New("code", "build")

	model, _ := updateModel(t, m, ModeChangeMsg{Agent: "debug"})

	if model.agent != "debug" {
		t.Errorf("expected agent 'debug', got %q", model.agent)
	}
}

func TestModel_ProgressMsg(t *testing.T) {
	m := New("code", "build")

	model, _ := updateModel(t, m, AgentStartMsg{
		Agent:  "build",
		Prompt: "test",
		Phases: []string{"Analyze", "Plan"},
	})

	model, _ = updateModel(t, model, ProgressMsg{
		Phase:    "Analyze",
		Status:   "done",
		ToolUses: 3,
	})

	if model.progress.phases[0].Status != "done" {
		t.Errorf("expected phase status 'done', got %q", model.progress.phases[0].Status)
	}
}

func TestModel_TickMsg(t *testing.T) {
	m := New("code", "build")
	initialFrame := m.progress.frame

	model, cmd := updateModel(t, m, TickMsg{})

	if model.progress.frame != initialFrame+1 {
		t.Errorf("expected frame to increment on tick")
	}
	if cmd == nil {
		t.Error("expected tick to schedule another tick")
	}
}

func TestModel_View_NotReady(t *testing.T) {
	m := New("code", "build")
	view := m.View()

	if view != "Initializing..." {
		t.Errorf("expected 'Initializing...' when not ready, got: %q", view)
	}
}

func TestModel_View_Quitting(t *testing.T) {
	m := New("code", "build")
	m.quitting = true

	view := m.View()
	if view != "Goodbye!\n" {
		t.Errorf("expected 'Goodbye!' when quitting, got: %q", view)
	}
}

func TestModel_QuitMsg(t *testing.T) {
	m := New("code", "build")

	model, _ := updateModel(t, m, QuitMsg{})
	if !model.quitting {
		t.Error("expected quitting after QuitMsg")
	}
}

func TestDefaultPhases(t *testing.T) {
	tests := []struct {
		agent    string
		expected int
	}{
		{"build", 5},
		{"debug", 4},
		{"review", 3},
		{"triage", 3},
		{"blog", 5},
		{"podcast", 3},
		{"unknown", 1},
	}

	for _, tc := range tests {
		phases := defaultPhases(tc.agent)
		if len(phases) != tc.expected {
			t.Errorf("defaultPhases(%q) returned %d phases, want %d", tc.agent, len(phases), tc.expected)
		}
	}
}

func TestValidAgents(t *testing.T) {
	codeAgents := validAgents("code")
	if len(codeAgents) != 4 {
		t.Errorf("expected 4 code agents, got %d", len(codeAgents))
	}

	blogAgents := validAgents("blog")
	if len(blogAgents) != 3 {
		t.Errorf("expected 3 blog agents, got %d", len(blogAgents))
	}

	podcastAgents := validAgents("podcast")
	if len(podcastAgents) != 1 {
		t.Errorf("expected 1 podcast agent, got %d", len(podcastAgents))
	}
}

func TestIsValidAgent(t *testing.T) {
	if !isValidAgent("build", "code") {
		t.Error("expected 'build' to be valid for code mode")
	}
	if isValidAgent("blog", "code") {
		t.Error("expected 'blog' to be invalid for code mode")
	}
	if !isValidAgent("writer", "blog") {
		t.Error("expected 'writer' to be valid for blog mode")
	}
}

func TestHelpText(t *testing.T) {
	text := helpText("code")
	if text == "" {
		t.Error("expected non-empty help text")
	}
	if len(text) < 50 {
		t.Error("help text seems too short")
	}
}
