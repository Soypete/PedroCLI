package tools

import (
	"context"
	"testing"
)

func TestDefaultCapabilityChecker_Check(t *testing.T) {
	checker := NewCapabilityChecker()

	// Git and bash are usually available on development machines
	// These tests verify the check runs without error
	tests := []struct {
		cap      Capability
		testable bool // some capabilities may not be available in CI
	}{
		{CapabilityGit, true},
		{CapabilityBash, true},
		{CapabilityNetwork, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.cap), func(t *testing.T) {
			// Just verify check doesn't panic
			result := checker.Check(tt.cap)
			t.Logf("Capability %s: %v", tt.cap, result)
		})
	}
}

func TestDefaultCapabilityChecker_Override(t *testing.T) {
	checker := NewCapabilityChecker()

	// Override should take precedence
	checker.Override[CapabilityGit] = false
	if checker.Check(CapabilityGit) {
		t.Error("Expected override to return false for git")
	}

	checker.Override[CapabilityNotionAPI] = true
	if !checker.Check(CapabilityNotionAPI) {
		t.Error("Expected override to return true for notion_api")
	}
}

func TestDefaultCapabilityChecker_CheckAll(t *testing.T) {
	checker := NewCapabilityChecker()

	// Override to control the test
	checker.Override[CapabilityGit] = true
	checker.Override[CapabilityBash] = true
	checker.Override[CapabilityNotionAPI] = false
	checker.Override[CapabilityGitHubAPI] = false

	missing := checker.CheckAll([]Capability{
		CapabilityGit,
		CapabilityBash,
		CapabilityNotionAPI,
		CapabilityGitHubAPI,
	})

	if len(missing) != 2 {
		t.Errorf("Expected 2 missing capabilities, got %d: %v", len(missing), missing)
	}

	// Check specific missing capabilities
	missingMap := make(map[Capability]bool)
	for _, m := range missing {
		missingMap[m] = true
	}
	if !missingMap[CapabilityNotionAPI] {
		t.Error("Expected CapabilityNotionAPI to be missing")
	}
	if !missingMap[CapabilityGitHubAPI] {
		t.Error("Expected CapabilityGitHubAPI to be missing")
	}
}

func TestDefaultCapabilityChecker_Available(t *testing.T) {
	checker := NewCapabilityChecker()

	// Override all to control test
	checker.Override[CapabilityGit] = true
	checker.Override[CapabilityBash] = true
	checker.Override[CapabilityNetwork] = true
	checker.Override[CapabilityNotionAPI] = false
	checker.Override[CapabilityGitHubAPI] = false
	checker.Override[CapabilityGitLabAPI] = false
	checker.Override[CapabilityWhisper] = false
	checker.Override[CapabilityOllama] = false

	available := checker.Available()

	if len(available) != 3 {
		t.Errorf("Expected 3 available capabilities, got %d: %v", len(available), available)
	}
}

// mockCapTool creates a tool with specific capability requirements
type mockCapTool struct {
	name         string
	capabilities []string
}

func (m *mockCapTool) Name() string        { return m.name }
func (m *mockCapTool) Description() string { return "Mock tool with capabilities" }
func (m *mockCapTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	return &Result{Success: true}, nil
}
func (m *mockCapTool) Metadata() *ToolMetadata {
	return &ToolMetadata{
		Category:             CategoryUtility,
		Optionality:          ToolOptional,
		RequiresCapabilities: m.capabilities,
	}
}

func TestToolRegistry_ListAvailable(t *testing.T) {
	registry := NewToolRegistry()

	// Register tools with different capability requirements
	registry.RegisterExtended(&mockCapTool{name: "git_tool", capabilities: []string{"git"}})
	registry.RegisterExtended(&mockCapTool{name: "notion_tool", capabilities: []string{"notion_api"}})
	registry.RegisterExtended(&mockCapTool{name: "multi_tool", capabilities: []string{"git", "network"}})
	registry.RegisterExtended(&mockCapTool{name: "no_req_tool", capabilities: []string{}})

	// Create checker with controlled capabilities
	checker := NewCapabilityChecker()
	checker.Override[CapabilityGit] = true
	checker.Override[CapabilityNetwork] = true
	checker.Override[CapabilityNotionAPI] = false

	available := registry.ListAvailable(checker)

	if len(available) != 3 {
		t.Errorf("Expected 3 available tools, got %d", len(available))
	}

	// Check specific tools
	names := make(map[string]bool)
	for _, tool := range available {
		names[tool.Name()] = true
	}

	if !names["git_tool"] {
		t.Error("Expected git_tool to be available")
	}
	if !names["multi_tool"] {
		t.Error("Expected multi_tool to be available")
	}
	if !names["no_req_tool"] {
		t.Error("Expected no_req_tool to be available")
	}
	if names["notion_tool"] {
		t.Error("Expected notion_tool to NOT be available")
	}
}

func TestToolRegistry_ListAvailable_WithLegacyTools(t *testing.T) {
	registry := NewToolRegistry()

	// Register a legacy tool (no Metadata)
	registry.Register(&mockBundleTool{name: "legacy_tool"})

	// Register an extended tool
	registry.RegisterExtended(&mockCapTool{name: "cap_tool", capabilities: []string{"git"}})

	checker := NewCapabilityChecker()
	checker.Override[CapabilityGit] = true

	available := registry.ListAvailable(checker)

	// Both should be available (legacy tools have nil metadata, so always available)
	if len(available) != 2 {
		t.Errorf("Expected 2 available tools, got %d", len(available))
	}
}

func TestToolRegistry_ListUnavailable(t *testing.T) {
	registry := NewToolRegistry()

	registry.RegisterExtended(&mockCapTool{name: "git_tool", capabilities: []string{"git"}})
	registry.RegisterExtended(&mockCapTool{name: "notion_tool", capabilities: []string{"notion_api"}})
	registry.RegisterExtended(&mockCapTool{name: "multi_tool", capabilities: []string{"notion_api", "github_api"}})

	checker := NewCapabilityChecker()
	checker.Override[CapabilityGit] = true
	checker.Override[CapabilityNotionAPI] = false
	checker.Override[CapabilityGitHubAPI] = false

	unavailable := registry.ListUnavailable(checker)

	if len(unavailable) != 2 {
		t.Errorf("Expected 2 unavailable tools, got %d: %v", len(unavailable), unavailable)
	}

	// notion_tool should be missing notion_api
	if missing, ok := unavailable["notion_tool"]; !ok {
		t.Error("Expected notion_tool to be unavailable")
	} else if len(missing) != 1 || missing[0] != "notion_api" {
		t.Errorf("Expected notion_tool to be missing [notion_api], got %v", missing)
	}

	// multi_tool should be missing both notion_api and github_api
	if missing, ok := unavailable["multi_tool"]; !ok {
		t.Error("Expected multi_tool to be unavailable")
	} else if len(missing) != 2 {
		t.Errorf("Expected multi_tool to be missing 2 capabilities, got %v", missing)
	}

	// git_tool should NOT be in unavailable
	if _, ok := unavailable["git_tool"]; ok {
		t.Error("Expected git_tool to be available")
	}
}

func TestUnknownCapability(t *testing.T) {
	checker := NewCapabilityChecker()

	// Unknown capability should return false
	if checker.Check(Capability("unknown_capability")) {
		t.Error("Expected unknown capability to return false")
	}
}
