package tools

import (
	"context"
	"testing"
)

// mockTool creates a simple mock tool for testing
type mockBundleTool struct {
	name string
}

func (m *mockBundleTool) Name() string        { return m.name }
func (m *mockBundleTool) Description() string { return "Mock tool: " + m.name }
func (m *mockBundleTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	return &Result{Success: true}, nil
}

func TestToolBundle_ApplyBundle(t *testing.T) {
	// Create source registry with all tools
	source := NewToolRegistry()
	source.Register(&mockBundleTool{name: "file"})
	source.Register(&mockBundleTool{name: "code_edit"})
	source.Register(&mockBundleTool{name: "search"})
	source.Register(&mockBundleTool{name: "navigate"})
	source.Register(&mockBundleTool{name: "git"})
	source.Register(&mockBundleTool{name: "bash"})
	source.Register(&mockBundleTool{name: "test"})

	// Apply CodeAgentBundle to a new target registry
	target := NewToolRegistry()
	missing := CodeAgentBundle.ApplyBundle(source, target)

	// Should have no missing tools
	if len(missing) != 0 {
		t.Errorf("Expected no missing tools, got %v", missing)
	}

	// Should have all required + optional tools
	expectedTools := []string{"file", "code_edit", "search", "navigate", "git", "bash", "test"}
	for _, name := range expectedTools {
		if _, ok := target.Get(name); !ok {
			t.Errorf("Expected tool %s to be registered", name)
		}
	}
}

func TestToolBundle_ApplyBundle_MissingRequired(t *testing.T) {
	// Create source registry with only some tools
	source := NewToolRegistry()
	source.Register(&mockBundleTool{name: "file"})
	source.Register(&mockBundleTool{name: "code_edit"})
	// Missing: search, navigate, git

	target := NewToolRegistry()
	missing := CodeAgentBundle.ApplyBundle(source, target)

	// Should report missing required tools
	expectedMissing := map[string]bool{"search": true, "navigate": true, "git": true}
	if len(missing) != len(expectedMissing) {
		t.Errorf("Expected %d missing tools, got %d: %v", len(expectedMissing), len(missing), missing)
	}
	for _, name := range missing {
		if !expectedMissing[name] {
			t.Errorf("Unexpected missing tool: %s", name)
		}
	}

	// Should still have available tools registered
	if _, ok := target.Get("file"); !ok {
		t.Error("Expected 'file' tool to be registered")
	}
	if _, ok := target.Get("code_edit"); !ok {
		t.Error("Expected 'code_edit' tool to be registered")
	}
}

func TestToolBundle_ApplyBundle_MissingOptional(t *testing.T) {
	// Create source registry with required but not optional tools
	source := NewToolRegistry()
	source.Register(&mockBundleTool{name: "file"})
	source.Register(&mockBundleTool{name: "code_edit"})
	source.Register(&mockBundleTool{name: "search"})
	source.Register(&mockBundleTool{name: "navigate"})
	source.Register(&mockBundleTool{name: "git"})
	// Missing optional: bash, test

	target := NewToolRegistry()
	missing := CodeAgentBundle.ApplyBundle(source, target)

	// Should report NO missing tools (optional are not tracked)
	if len(missing) != 0 {
		t.Errorf("Expected no missing tools (optional not tracked), got %v", missing)
	}

	// Should have all required tools
	if len(target.List()) != 5 {
		t.Errorf("Expected 5 tools, got %d", len(target.List()))
	}
}

func TestToolBundle_AllToolNames(t *testing.T) {
	names := CodeAgentBundle.AllToolNames()
	expected := []string{"file", "code_edit", "search", "navigate", "git", "bash", "test"}

	if len(names) != len(expected) {
		t.Errorf("Expected %d names, got %d", len(expected), len(names))
	}

	nameMap := make(map[string]bool)
	for _, n := range names {
		nameMap[n] = true
	}
	for _, e := range expected {
		if !nameMap[e] {
			t.Errorf("Expected tool %s in AllToolNames", e)
		}
	}
}

func TestToolBundle_HasTool(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		expected bool
	}{
		{"required tool", "file", true},
		{"optional tool", "bash", true},
		{"non-existent tool", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CodeAgentBundle.HasTool(tt.tool)
			if result != tt.expected {
				t.Errorf("HasTool(%s) = %v, expected %v", tt.tool, result, tt.expected)
			}
		})
	}
}

func TestAllBundles(t *testing.T) {
	bundles := AllBundles()

	// Should have at least the predefined bundles
	if len(bundles) < 5 {
		t.Errorf("Expected at least 5 bundles, got %d", len(bundles))
	}

	// Check each bundle has a name
	for _, b := range bundles {
		if b.Name == "" {
			t.Error("Found bundle with empty name")
		}
	}
}

func TestGetBundle(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"code_agent", true},
		{"blog_agent", true},
		{"blog_orchestrator", true},
		{"research", true},
		{"utility", true},
		{"nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := GetBundle(tt.name)
			if (bundle != nil) != tt.expected {
				t.Errorf("GetBundle(%s) found = %v, expected %v", tt.name, bundle != nil, tt.expected)
			}
		})
	}
}

func TestBlogAgentBundle(t *testing.T) {
	// BlogAgentBundle should have no required tools (graceful degradation)
	if len(BlogAgentBundle.Required) != 0 {
		t.Errorf("BlogAgentBundle should have no required tools, got %v", BlogAgentBundle.Required)
	}

	// Should have optional research/publishing tools
	expectedOptional := []string{"rss_feed", "static_links", "blog_publish", "calendar"}
	for _, opt := range expectedOptional {
		if !BlogAgentBundle.HasTool(opt) {
			t.Errorf("BlogAgentBundle should have optional tool %s", opt)
		}
	}
}
