package agentregistry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewAgentRegistry(t *testing.T) {
	registry := NewAgentRegistry()

	// Check that builtins are registered
	if len(registry.agents) == 0 {
		t.Error("expected builtin agents to be registered")
	}

	// Check specific builtins
	builtins := []string{"build", "plan", "debug", "review", "research", "blog", "podcast"}
	for _, name := range builtins {
		if _, ok := registry.Get(name); !ok {
			t.Errorf("expected builtin agent %s to be registered", name)
		}
	}
}

func TestAgentRegistry_PrimaryAgentCycling(t *testing.T) {
	registry := NewAgentRegistry()

	// Get initial primary agents
	primaryAgents := registry.ListPrimary()
	if len(primaryAgents) == 0 {
		t.Fatal("expected primary agents to be available")
	}

	// Get current agent
	current := registry.Current()
	if current == nil {
		t.Fatal("expected current agent to be set")
	}

	// Cycle through agents
	first := current.Name
	registry.CycleNext()
	second := registry.Current().Name

	if first == second && len(primaryAgents) > 1 {
		t.Error("expected CycleNext to change current agent")
	}

	// Cycle back (go back by 1 to return to the first agent)
	registry.CyclePrev()
	if registry.Current().Name != first {
		t.Errorf("expected to return to first agent %q, got %q", first, registry.Current().Name)
	}
}

func TestAgentRegistry_SetCurrent(t *testing.T) {
	registry := NewAgentRegistry()

	// Set current to a specific agent
	err := registry.SetCurrent("plan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if registry.Current().Name != "plan" {
		t.Error("expected current agent to be 'plan'")
	}

	// Try to set to non-existent agent
	err = registry.SetCurrent("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent agent")
	}
}

func TestAgentRegistry_Register(t *testing.T) {
	registry := NewAgentRegistry()

	// Register a new agent
	agent := &Agent{
		Name:        "custom",
		Description: "Custom agent",
		Mode:        AgentModePrimary,
		MaxSteps:    10,
	}
	registry.Register("custom", agent)

	// Check it's registered
	retrieved, ok := registry.Get("custom")
	if !ok {
		t.Fatal("expected custom agent to be registered")
	}
	if retrieved.Description != "Custom agent" {
		t.Error("expected agent description to match")
	}

	// Check it's in primary list
	found := false
	for _, a := range registry.ListPrimary() {
		if a.Name == "custom" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected custom agent to be in primary list")
	}
}

func TestAgentRegistry_Subagents(t *testing.T) {
	registry := NewAgentRegistry()

	subagents := registry.ListSubagents()
	if len(subagents) == 0 {
		t.Error("expected at least one subagent (research)")
	}

	// Check research is a subagent
	found := false
	for _, a := range subagents {
		if a.Name == "research" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'research' to be in subagents list")
	}
}

func TestAgentRegistry_LoadMarkdownAgents(t *testing.T) {
	// Create a temporary directory with a test agent
	tmpDir := t.TempDir()
	agentContent := `---
name: test-agent
description: A test agent
mode: primary
temperature: 0.5
max_steps: 25
---

You are a test agent.
`
	agentPath := filepath.Join(tmpDir, "test-agent.md")
	if err := os.WriteFile(agentPath, []byte(agentContent), 0644); err != nil {
		t.Fatalf("failed to create test agent file: %v", err)
	}

	registry := NewAgentRegistry()
	err := registry.LoadMarkdownAgents(tmpDir)
	if err != nil {
		t.Fatalf("failed to load markdown agents: %v", err)
	}

	// Check the agent was loaded
	agent, ok := registry.Get("test-agent")
	if !ok {
		t.Fatal("expected test-agent to be loaded")
	}
	if agent.Description != "A test agent" {
		t.Errorf("expected description 'A test agent', got '%s'", agent.Description)
	}
	if agent.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %f", agent.Temperature)
	}
	if agent.MaxSteps != 25 {
		t.Errorf("expected max_steps 25, got %d", agent.MaxSteps)
	}
	if agent.Source != "markdown" {
		t.Errorf("expected source 'markdown', got '%s'", agent.Source)
	}
}

func TestAgentRegistry_LoadFromConfig(t *testing.T) {
	registry := NewAgentRegistry()

	configs := map[string]AgentConfig{
		"config-agent": {
			Mode:        "primary",
			Description: "Agent from config",
			MaxSteps:    30,
			Tools:       map[string]bool{"edit": true, "bash": false},
		},
	}

	err := registry.LoadFromConfig(configs)
	if err != nil {
		t.Fatalf("failed to load from config: %v", err)
	}

	agent, ok := registry.Get("config-agent")
	if !ok {
		t.Fatal("expected config-agent to be loaded")
	}
	if agent.Description != "Agent from config" {
		t.Error("expected description to match")
	}
	if agent.Source != "config" {
		t.Errorf("expected source 'config', got '%s'", agent.Source)
	}
}

func TestAgentRegistry_Reset(t *testing.T) {
	registry := NewAgentRegistry()

	// Register a custom agent
	registry.Register("custom", &Agent{
		Name: "custom",
		Mode: AgentModePrimary,
	})

	// Verify it's there
	if _, ok := registry.Get("custom"); !ok {
		t.Fatal("expected custom agent to be registered")
	}

	// Reset
	registry.Reset()

	// Custom agent should be gone
	if _, ok := registry.Get("custom"); ok {
		t.Error("expected custom agent to be removed after reset")
	}

	// Builtins should still be there
	if _, ok := registry.Get("build"); !ok {
		t.Error("expected builtin 'build' to still be registered")
	}
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantFM   string
		wantBody string
	}{
		{
			name: "with frontmatter",
			content: `---
name: test
---

Body content`,
			wantFM:   "name: test",
			wantBody: "\nBody content",
		},
		{
			name:     "without frontmatter",
			content:  "Just body content",
			wantFM:   "",
			wantBody: "Just body content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body := parseFrontmatter(tt.content)
			if fm != tt.wantFM {
				t.Errorf("frontmatter: got %q, want %q", fm, tt.wantFM)
			}
			if body != tt.wantBody {
				t.Errorf("body: got %q, want %q", body, tt.wantBody)
			}
		})
	}
}
