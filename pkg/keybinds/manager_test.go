package keybinds

import (
	"testing"
)

func TestNewKeybindManager(t *testing.T) {
	manager := NewKeybindManager()

	if manager == nil {
		t.Fatal("expected manager to be created")
	}

	// Check default leader
	if manager.GetLeader() != "ctrl+x" {
		t.Errorf("expected default leader 'ctrl+x', got %q", manager.GetLeader())
	}

	// Check some default bindings
	bindings := manager.GetAllBindings()
	if len(bindings) == 0 {
		t.Error("expected default bindings to be set")
	}
}

func TestKeybindManager_SetLeader(t *testing.T) {
	manager := NewKeybindManager()

	manager.SetLeader("ctrl+a")
	if manager.GetLeader() != "ctrl+a" {
		t.Errorf("expected leader 'ctrl+a', got %q", manager.GetLeader())
	}
}

func TestKeybindManager_SetBinding(t *testing.T) {
	manager := NewKeybindManager()

	manager.SetBinding(ActionAgentCycle, "ctrl+tab")
	bindings := manager.GetBinding(ActionAgentCycle)

	if len(bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(bindings))
	}
	if bindings[0] != "ctrl+tab" {
		t.Errorf("expected 'ctrl+tab', got %q", bindings[0])
	}
}

func TestKeybindManager_HandleKey(t *testing.T) {
	manager := NewKeybindManager()

	// Track if handler was called
	handlerCalled := false
	manager.RegisterHandler(ActionAgentCycle, func() {
		handlerCalled = true
	})

	// Handle the key
	handled := manager.HandleKey("tab")
	if !handled {
		t.Error("expected key to be handled")
	}
	if !handlerCalled {
		t.Error("expected handler to be called")
	}
}

func TestKeybindManager_LeaderSequence(t *testing.T) {
	manager := NewKeybindManager()

	// Track if handler was called
	handlerCalled := false
	manager.RegisterHandler(ActionSessionNew, func() {
		handlerCalled = true
	})

	// Press leader key
	handled := manager.HandleKey("ctrl+x")
	if !handled {
		t.Error("expected leader key to be handled")
	}
	if !manager.IsLeaderActive() {
		t.Error("expected leader to be active")
	}

	// Press 'n' for session_new (<leader>n)
	_ = manager.HandleKey("n")
	if !handlerCalled {
		t.Error("expected handler to be called for leader sequence")
	}
	if manager.IsLeaderActive() {
		t.Error("expected leader to be deactivated after action")
	}
}

func TestKeybindManager_CancelLeader(t *testing.T) {
	manager := NewKeybindManager()

	// Press leader key
	manager.HandleKey("ctrl+x")
	if !manager.IsLeaderActive() {
		t.Error("expected leader to be active")
	}

	// Cancel leader
	manager.CancelLeader()
	if manager.IsLeaderActive() {
		t.Error("expected leader to be cancelled")
	}
}

func TestKeybindManager_LoadFromConfig(t *testing.T) {
	manager := NewKeybindManager()

	cfg := KeybindConfig{
		Leader:     "ctrl+b",
		AgentCycle: "ctrl+tab",
		SessionNew: "<leader>s",
	}

	manager.LoadFromConfig(cfg)

	if manager.GetLeader() != "ctrl+b" {
		t.Errorf("expected leader 'ctrl+b', got %q", manager.GetLeader())
	}

	bindings := manager.GetBinding(ActionAgentCycle)
	if len(bindings) != 1 || bindings[0] != "ctrl+tab" {
		t.Error("expected agent_cycle binding to be updated")
	}
}

func TestKeybindManager_GetBindingString(t *testing.T) {
	manager := NewKeybindManager()

	// Default leader is ctrl+x
	bindingStr := manager.GetBindingString(ActionSessionNew)
	if bindingStr != "ctrl+x n" {
		t.Errorf("expected 'ctrl+x n', got %q", bindingStr)
	}
}

func TestKeybindManager_GetHelp(t *testing.T) {
	manager := NewKeybindManager()

	help := manager.GetHelp()
	if help == "" {
		t.Error("expected non-empty help text")
	}
	if !containsString(help, "Leader key") {
		t.Error("expected help to mention leader key")
	}
	if !containsString(help, "ctrl+x") {
		t.Error("expected help to show default leader")
	}
}

func TestKeybindManager_CustomBindings(t *testing.T) {
	manager := NewKeybindManager()

	cfg := KeybindConfig{
		Custom: map[string]string{
			"custom_action": "ctrl+shift+c",
		},
	}

	manager.LoadFromConfig(cfg)

	bindings := manager.GetBinding(Action("custom_action"))
	if len(bindings) != 1 {
		t.Fatalf("expected 1 custom binding, got %d", len(bindings))
	}
	if bindings[0] != "ctrl+shift+c" {
		t.Errorf("expected 'ctrl+shift+c', got %q", bindings[0])
	}
}

func TestParseKeys(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"ctrl+c", []string{"ctrl+c"}},
		{"ctrl+c,ctrl+d", []string{"ctrl+c", "ctrl+d"}},
		{"ctrl+c, ctrl+d", []string{"ctrl+c", "ctrl+d"}},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseKeys(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseKeys(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseKeys(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestNormalizeKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Ctrl+C", "ctrl+c"},
		{"ENTER", "return"},
		{"Enter", "return"},
		{"Esc", "escape"},
		{"Del", "delete"},
		{"ctrl+c", "ctrl+c"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeKey(tt.input)
			if got != tt.want {
				t.Errorf("normalizeKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
