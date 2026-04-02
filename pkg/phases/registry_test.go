package phases

import (
	"testing"
)

func TestDefaultRegistry_GetPhase(t *testing.T) {
	registry := DefaultRegistry()

	tests := []struct {
		name      string
		phaseName string
		wantFound bool
		wantTools int
	}{
		{
			name:      "analyze phase exists",
			phaseName: "analyze",
			wantFound: true,
			wantTools: 5,
		},
		{
			name:      "plan phase exists",
			phaseName: "plan",
			wantFound: true,
			wantTools: 4,
		},
		{
			name:      "implement phase exists",
			phaseName: "implement",
			wantFound: true,
			wantTools: 7,
		},
		{
			name:      "validate phase exists",
			phaseName: "validate",
			wantFound: true,
			wantTools: 7,
		},
		{
			name:      "deliver phase exists",
			phaseName: "deliver",
			wantFound: true,
			wantTools: 2,
		},
		{
			name:      "non-existent phase",
			phaseName: "nonexistent",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase, err := registry.GetPhase(tt.phaseName)
			if err != nil {
				t.Fatalf("GetPhase() error = %v", err)
			}

			if tt.wantFound && phase == nil {
				t.Errorf("expected to find phase %q, got nil", tt.phaseName)
			}
			if !tt.wantFound && phase != nil {
				t.Errorf("expected not to find phase %q, got %v", tt.phaseName, phase)
			}

			if tt.wantFound && tt.wantTools > 0 && len(phase.Tools) != tt.wantTools {
				t.Errorf("expected %d tools, got %d", tt.wantTools, len(phase.Tools))
			}
		})
	}
}

func TestDefaultRegistry_GetPhases(t *testing.T) {
	registry := DefaultRegistry()

	phases, err := registry.GetPhases([]string{"analyze", "plan", "implement"})
	if err != nil {
		t.Fatalf("GetPhases() error = %v", err)
	}

	if len(phases) != 3 {
		t.Errorf("expected 3 phases, got %d", len(phases))
	}

	if phases[0].Name != "analyze" {
		t.Errorf("expected first phase to be 'analyze', got %q", phases[0].Name)
	}
	if phases[1].Name != "plan" {
		t.Errorf("expected second phase to be 'plan', got %q", phases[1].Name)
	}
	if phases[2].Name != "implement" {
		t.Errorf("expected third phase to be 'implement', got %q", phases[2].Name)
	}
}

func TestDefaultRegistry_GetPhases_NotFound(t *testing.T) {
	registry := DefaultRegistry()

	phases, err := registry.GetPhases([]string{"analyze", "nonexistent", "plan"})
	if err != nil {
		t.Fatalf("GetPhases() error = %v", err)
	}

	if phases != nil {
		t.Errorf("expected nil when phase not found, got %v", phases)
	}
}

func TestDefaultRegistry_ListPhases(t *testing.T) {
	registry := DefaultRegistry()

	phases := registry.ListPhases()

	if len(phases) == 0 {
		t.Error("expected non-empty phase list")
	}

	expectedPhases := map[string]bool{
		"analyze":     true,
		"plan":        true,
		"implement":   true,
		"validate":    true,
		"deliver":     true,
		"review":      true,
		"reproduce":   true,
		"investigate": true,
		"isolate":     true,
		"fix":         true,
		"verify":      true,
		"commit":      true,
		"gather":      true,
		"security":    true,
		"quality":     true,
		"compile":     true,
		"publish":     true,
	}

	for _, phase := range phases {
		if !expectedPhases[phase] {
			t.Errorf("unexpected phase: %q", phase)
		}
		delete(expectedPhases, phase)
	}

	for phase := range expectedPhases {
		t.Errorf("missing expected phase: %q", phase)
	}
}

func TestDefaultRegistry_RegisterPhase(t *testing.T) {
	registry := NewRegistry()

	newPhase := PhaseDefinition{
		Name:        "custom",
		Description: "A custom test phase",
		Tools:       []string{"file", "search"},
		MaxRounds:   5,
		ExpectsJSON: true,
	}

	registry.RegisterPhase(newPhase)

	phase, err := registry.GetPhase("custom")
	if err != nil {
		t.Fatalf("GetPhase() error = %v", err)
	}
	if phase == nil {
		t.Fatal("expected to find custom phase")
		return
	}

	if phase.Name != "custom" {
		t.Errorf("expected name 'custom', got %q", phase.Name)
	}
	if len(phase.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(phase.Tools))
	}
}

func TestDefaultRegistry_RegisterPhase_Overwrite(t *testing.T) {
	registry := NewRegistry()

	original, _ := registry.GetPhase("analyze")
	if original == nil {
		t.Fatal("expected to find 'analyze' phase in new registry")
		return
	}
	_ = len(original.Tools) // used in test

	registry.RegisterPhase(PhaseDefinition{
		Name:        "analyze",
		Description: "Overwritten",
		Tools:       []string{"custom_tool"},
		MaxRounds:   99,
	})

	updated, _ := registry.GetPhase("analyze")
	if updated == nil {
		t.Fatal("expected to find 'analyze' phase after overwrite")
		return
	}
	if updated.Tools[0] != "custom_tool" {
		t.Errorf("expected tools to be overwritten, got %v", updated.Tools)
	}
	if updated.MaxRounds != 99 {
		t.Errorf("expected MaxRounds 99, got %d", updated.MaxRounds)
	}
}

func TestPhaseDefinition_JSONTags(t *testing.T) {
	phase := PhaseDefinition{
		Name:        "test",
		Description: "Test phase",
		Tools:       []string{"tool1", "tool2"},
		MaxRounds:   10,
		ExpectsJSON: true,
		DependsOn:   []string{"previous"},
	}

	if phase.Name != "test" {
		t.Errorf("expected Name 'test', got %q", phase.Name)
	}
	if !phase.ExpectsJSON {
		t.Error("expected ExpectsJSON to be true")
	}
	if len(phase.DependsOn) != 1 || phase.DependsOn[0] != "previous" {
		t.Errorf("expected DependsOn ['previous'], got %v", phase.DependsOn)
	}
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	phases := registry.ListPhases()
	if len(phases) == 0 {
		t.Error("expected NewRegistry to have standard phases")
	}

	phase, err := registry.GetPhase("analyze")
	if err != nil {
		t.Fatalf("GetPhase() error = %v", err)
	}
	if phase == nil {
		t.Error("expected analyze phase to exist")
	}
}

func TestSetDefaultRegistry(t *testing.T) {
	original := DefaultRegistry()

	custom := NewRegistry()
	custom.RegisterPhase(PhaseDefinition{
		Name:        "custom_phase",
		Description: "Custom phase",
		Tools:       []string{"tool"},
		MaxRounds:   5,
	})

	SetDefaultRegistry(custom)

	customResult, _ := DefaultRegistry().GetPhase("custom_phase")
	if customResult == nil {
		t.Error("expected custom phase to be in default registry after SetDefaultRegistry")
	}

	SetDefaultRegistry(original)
}
