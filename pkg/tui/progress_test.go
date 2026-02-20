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

func TestProgressPanel_PedroAnimation_Dancing(t *testing.T) {
	p := NewProgressPanel()
	p.SetWidth(80)
	p.StartAgent("build", "test", []string{"A"})
	p.UpdatePhase(ProgressMsg{Phase: "A", Status: "in_progress"})

	// Pedro should use dancing frames when active
	pedro := p.renderPedro()
	if pedro == "" {
		t.Error("expected non-empty Pedro frame when active")
	}
	// Should contain box-drawing chars from the Pedro art
	if !strings.Contains(pedro, "┌───┐") {
		t.Errorf("expected Pedro box-drawing art, got:\n%s", pedro)
	}
}

func TestProgressPanel_PedroAnimation_Done(t *testing.T) {
	p := NewProgressPanel()
	p.SetWidth(80)
	p.StartAgent("build", "test", []string{"A"})
	p.UpdatePhase(ProgressMsg{Phase: "A", Status: "done"})

	if !p.allDone() {
		t.Error("expected allDone() to be true when all phases are done")
	}

	pedro := p.renderPedro()
	// Done frame has arms up: backslash and forward-slash
	if !strings.Contains(pedro, "\\") || !strings.Contains(pedro, "/") {
		t.Errorf("expected celebration pose with arms up, got:\n%s", pedro)
	}
}

func TestProgressPanel_PedroAnimation_Idle(t *testing.T) {
	p := NewProgressPanel()
	p.SetWidth(80)
	p.StartAgent("build", "test", []string{"A"})
	// Status is "pending" - not active, not done

	pedro := p.renderPedro()
	if pedro == "" {
		t.Error("expected non-empty Pedro frame when idle")
	}
}

func TestProgressPanel_PedroAnimation_FrameAdvances(t *testing.T) {
	p := NewProgressPanel()
	p.SetWidth(80)
	p.StartAgent("build", "test", []string{"A"})
	p.UpdatePhase(ProgressMsg{Phase: "A", Status: "in_progress"})

	// Collect frames across ticks - dance advances every 2 ticks
	frames := make(map[string]bool)
	for i := 0; i < 16; i++ {
		pedro := p.renderPedro()
		frames[pedro] = true
		p.Tick()
	}

	// Should have seen multiple distinct frames
	if len(frames) < 2 {
		t.Errorf("expected multiple Pedro frames across ticks, got %d unique frames", len(frames))
	}
}

func TestProgressPanel_AllDone(t *testing.T) {
	p := NewProgressPanel()

	// No phases = not done
	if p.allDone() {
		t.Error("expected allDone() false with no phases")
	}

	p.StartAgent("build", "test", []string{"A", "B"})

	// All pending = not done
	if p.allDone() {
		t.Error("expected allDone() false with pending phases")
	}

	p.UpdatePhase(ProgressMsg{Phase: "A", Status: "done"})
	// One done, one pending = not done
	if p.allDone() {
		t.Error("expected allDone() false with mixed phases")
	}

	p.UpdatePhase(ProgressMsg{Phase: "B", Status: "done"})
	// All done
	if !p.allDone() {
		t.Error("expected allDone() true when all phases done")
	}
}

func TestProgressPanel_View_ContainsPedro(t *testing.T) {
	p := NewProgressPanel()
	p.SetWidth(80)
	p.StartAgent("build", "test task", []string{"Plan", "Build"})

	view := p.View()
	// View should contain both Pedro art and phase names
	if !strings.Contains(view, "┌───┐") {
		t.Errorf("expected Pedro art in combined view, got:\n%s", view)
	}
	if !strings.Contains(view, "Plan") {
		t.Errorf("expected phase name 'Plan' in combined view, got:\n%s", view)
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
