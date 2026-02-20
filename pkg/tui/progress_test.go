package tui

import (
	"strings"
	"testing"
)

func TestProgressPanel_Empty(t *testing.T) {
	p := NewProgressPanel()
	view := p.View()
	if view != "" {
		t.Errorf("expected empty view for no phases, got: %q", view)
	}
}

func TestProgressPanel_StartAgent(t *testing.T) {
	p := NewProgressPanel()
	p.SetWidth(80)
	p.StartAgent("build", "Add a login page", []string{"Analyze", "Plan", "Implement"})

	if len(p.phases) != 3 {
		t.Fatalf("expected 3 phases, got %d", len(p.phases))
	}

	for _, ph := range p.phases {
		if ph.Status != "pending" {
			t.Errorf("expected pending status, got %q for phase %q", ph.Status, ph.Name)
		}
	}

	if p.agent != "build" {
		t.Errorf("expected agent 'build', got %q", p.agent)
	}
}

func TestProgressPanel_UpdatePhase(t *testing.T) {
	p := NewProgressPanel()
	p.SetWidth(80)
	p.StartAgent("debug", "fix crash", []string{"Diagnose", "Locate", "Fix"})

	// Update existing phase
	p.UpdatePhase(ProgressMsg{
		Phase:    "Diagnose",
		Status:   "in_progress",
		ToolUses: 2,
		Tokens:   1500,
	})

	if p.phases[0].Status != "in_progress" {
		t.Errorf("expected in_progress, got %q", p.phases[0].Status)
	}
	if p.phases[0].ToolUses != 2 {
		t.Errorf("expected 2 tool uses, got %d", p.phases[0].ToolUses)
	}
	if p.phases[0].Tokens != 1500 {
		t.Errorf("expected 1500 tokens, got %d", p.phases[0].Tokens)
	}
}

func TestProgressPanel_UpdatePhase_Dynamic(t *testing.T) {
	p := NewProgressPanel()
	p.SetWidth(80)
	p.StartAgent("build", "test", []string{"Phase1"})

	// Add a phase that doesn't exist yet
	p.UpdatePhase(ProgressMsg{
		Phase:  "DynamicPhase",
		Status: "in_progress",
	})

	if len(p.phases) != 2 {
		t.Fatalf("expected 2 phases after dynamic add, got %d", len(p.phases))
	}
	if p.phases[1].Name != "DynamicPhase" {
		t.Errorf("expected DynamicPhase, got %q", p.phases[1].Name)
	}
}

func TestProgressPanel_View_ContainsPhaseNames(t *testing.T) {
	p := NewProgressPanel()
	p.SetWidth(80)
	p.StartAgent("review", "check code", []string{"Read", "Analyze", "Report"})

	view := p.View()

	for _, name := range []string{"Read", "Analyze", "Report"} {
		if !strings.Contains(view, name) {
			t.Errorf("expected view to contain %q, got:\n%s", name, view)
		}
	}
}

func TestProgressPanel_View_ShowsStats(t *testing.T) {
	p := NewProgressPanel()
	p.SetWidth(80)
	p.StartAgent("build", "test", []string{"Implement"})

	p.UpdatePhase(ProgressMsg{
		Phase:    "Implement",
		Status:   "done",
		ToolUses: 5,
		Tokens:   2500,
	})

	view := p.View()
	if !strings.Contains(view, "5 tools") {
		t.Errorf("expected '5 tools' in view, got:\n%s", view)
	}
	if !strings.Contains(view, "2.5k tok") {
		t.Errorf("expected '2.5k tok' in view, got:\n%s", view)
	}
}

func TestProgressPanel_View_ShowsError(t *testing.T) {
	p := NewProgressPanel()
	p.SetWidth(80)
	p.StartAgent("build", "test", []string{"Test"})

	p.UpdatePhase(ProgressMsg{
		Phase:  "Test",
		Status: "failed",
		Error:  "compilation failed",
	})

	view := p.View()
	if !strings.Contains(view, "compilation failed") {
		t.Errorf("expected error message in view, got:\n%s", view)
	}
}

func TestProgressPanel_Tick(t *testing.T) {
	p := NewProgressPanel()
	initialFrame := p.frame
	p.Tick()
	if p.frame != initialFrame+1 {
		t.Errorf("expected frame to increment, got %d", p.frame)
	}
}

func TestProgressPanel_IsActive(t *testing.T) {
	p := NewProgressPanel()
	p.StartAgent("build", "test", []string{"A", "B"})

	if p.IsActive() {
		t.Error("expected not active when all phases are pending")
	}

	p.UpdatePhase(ProgressMsg{Phase: "A", Status: "in_progress"})
	if !p.IsActive() {
		t.Error("expected active when a phase is in_progress")
	}

	p.UpdatePhase(ProgressMsg{Phase: "A", Status: "done"})
	if p.IsActive() {
		t.Error("expected not active when no phases are in_progress")
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0 tok"},
		{500, "500 tok"},
		{999, "999 tok"},
		{1000, "1.0k tok"},
		{1500, "1.5k tok"},
		{10000, "10.0k tok"},
	}

	for _, tc := range tests {
		result := formatTokens(tc.input)
		if result != tc.expected {
			t.Errorf("formatTokens(%d) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}
