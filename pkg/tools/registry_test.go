package tools

import (
	"context"
	"sync"
	"testing"

	"github.com/soypete/pedrocli/pkg/logits"
)

// mockTool is a simple Tool implementation for testing
type mockTool struct {
	name        string
	description string
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string { return m.description }
func (m *mockTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	return &Result{Success: true, Output: "mock output"}, nil
}

// mockExtendedTool is an ExtendedTool implementation for testing
type mockExtendedTool struct {
	mockTool
	metadata *ToolMetadata
}

func (m *mockExtendedTool) Metadata() *ToolMetadata {
	return m.metadata
}

func newMockExtendedTool(name string, category ToolCategory, optionality ToolOptionality) *mockExtendedTool {
	return &mockExtendedTool{
		mockTool: mockTool{name: name, description: "Mock " + name},
		metadata: &ToolMetadata{
			Category:    category,
			Optionality: optionality,
			Schema: &logits.JSONSchema{
				Type: "object",
				Properties: map[string]*logits.JSONSchema{
					"input": {Type: "string"},
				},
			},
		},
	}
}

func TestNewToolRegistry(t *testing.T) {
	r := NewToolRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if r.Count() != 0 {
		t.Errorf("expected empty registry, got %d tools", r.Count())
	}
}

func TestToolRegistry_Register(t *testing.T) {
	r := NewToolRegistry()
	tool := &mockTool{name: "test_tool", description: "A test tool"}

	err := r.Register(tool)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.Count() != 1 {
		t.Errorf("expected 1 tool, got %d", r.Count())
	}

	// Verify tool is wrapped as ExtendedTool
	got, exists := r.Get("test_tool")
	if !exists {
		t.Fatal("tool not found after registration")
	}
	if got.Name() != "test_tool" {
		t.Errorf("expected name 'test_tool', got %q", got.Name())
	}

	// Wrapped tools should have nil metadata
	if got.Metadata() != nil {
		t.Error("expected nil metadata for wrapped tool")
	}
}

func TestToolRegistry_Register_Duplicate(t *testing.T) {
	r := NewToolRegistry()
	tool := &mockTool{name: "test_tool", description: "A test tool"}

	_ = r.Register(tool)
	err := r.Register(tool)

	if err == nil {
		t.Error("expected error when registering duplicate tool")
	}
}

func TestToolRegistry_RegisterExtended(t *testing.T) {
	r := NewToolRegistry()
	tool := newMockExtendedTool("extended_tool", CategoryCode, ToolRequired)

	err := r.RegisterExtended(tool)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, exists := r.Get("extended_tool")
	if !exists {
		t.Fatal("tool not found after registration")
	}

	meta := got.Metadata()
	if meta == nil {
		t.Fatal("expected non-nil metadata")
	}
	if meta.Category != CategoryCode {
		t.Errorf("expected category %q, got %q", CategoryCode, meta.Category)
	}
}

func TestToolRegistry_Unregister(t *testing.T) {
	r := NewToolRegistry()
	tool := &mockTool{name: "test_tool", description: "A test tool"}
	_ = r.Register(tool)

	err := r.Unregister("test_tool")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.Count() != 0 {
		t.Errorf("expected 0 tools after unregister, got %d", r.Count())
	}

	_, exists := r.Get("test_tool")
	if exists {
		t.Error("tool should not exist after unregistration")
	}
}

func TestToolRegistry_Unregister_NotFound(t *testing.T) {
	r := NewToolRegistry()

	err := r.Unregister("nonexistent")
	if err == nil {
		t.Error("expected error when unregistering nonexistent tool")
	}
}

func TestToolRegistry_List(t *testing.T) {
	r := NewToolRegistry()
	_ = r.Register(&mockTool{name: "tool1", description: "Tool 1"})
	_ = r.Register(&mockTool{name: "tool2", description: "Tool 2"})
	_ = r.Register(&mockTool{name: "tool3", description: "Tool 3"})

	tools := r.List()
	if len(tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(tools))
	}
}

func TestToolRegistry_ListNames(t *testing.T) {
	r := NewToolRegistry()
	_ = r.Register(&mockTool{name: "tool1", description: "Tool 1"})
	_ = r.Register(&mockTool{name: "tool2", description: "Tool 2"})

	names := r.ListNames()
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}

	nameMap := make(map[string]bool)
	for _, n := range names {
		nameMap[n] = true
	}
	if !nameMap["tool1"] || !nameMap["tool2"] {
		t.Error("expected both tool1 and tool2 in names")
	}
}

func TestToolRegistry_FilterByCategory(t *testing.T) {
	r := NewToolRegistry()
	_ = r.RegisterExtended(newMockExtendedTool("file", CategoryCode, ToolRequired))
	_ = r.RegisterExtended(newMockExtendedTool("search", CategoryCode, ToolRequired))
	_ = r.RegisterExtended(newMockExtendedTool("git", CategoryVCS, ToolRequired))
	_ = r.RegisterExtended(newMockExtendedTool("rss", CategoryResearch, ToolOptional))

	codeTools := r.FilterByCategory(CategoryCode)
	if len(codeTools) != 2 {
		t.Errorf("expected 2 code tools, got %d", len(codeTools))
	}

	vcsTools := r.FilterByCategory(CategoryVCS)
	if len(vcsTools) != 1 {
		t.Errorf("expected 1 vcs tool, got %d", len(vcsTools))
	}

	researchTools := r.FilterByCategory(CategoryResearch)
	if len(researchTools) != 1 {
		t.Errorf("expected 1 research tool, got %d", len(researchTools))
	}
}

func TestToolRegistry_FilterByOptionality(t *testing.T) {
	r := NewToolRegistry()
	_ = r.RegisterExtended(newMockExtendedTool("file", CategoryCode, ToolRequired))
	_ = r.RegisterExtended(newMockExtendedTool("git", CategoryVCS, ToolRequired))
	_ = r.RegisterExtended(newMockExtendedTool("rss", CategoryResearch, ToolOptional))
	_ = r.RegisterExtended(newMockExtendedTool("calendar", CategoryResearch, ToolOptional))
	_ = r.RegisterExtended(newMockExtendedTool("bash", CategoryBuild, ToolConditional))

	required := r.FilterRequired()
	if len(required) != 2 {
		t.Errorf("expected 2 required tools, got %d", len(required))
	}

	optional := r.FilterOptional()
	if len(optional) != 2 {
		t.Errorf("expected 2 optional tools, got %d", len(optional))
	}

	conditional := r.FilterByOptionality(ToolConditional)
	if len(conditional) != 1 {
		t.Errorf("expected 1 conditional tool, got %d", len(conditional))
	}
}

func TestToolRegistry_EventListener(t *testing.T) {
	r := NewToolRegistry()

	var events []RegistryEvent
	r.AddListener(func(event RegistryEvent) {
		events = append(events, event)
	})

	tool := &mockTool{name: "test_tool", description: "Test"}
	_ = r.Register(tool)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventToolRegistered {
		t.Errorf("expected EventToolRegistered, got %s", events[0].Type)
	}
	if events[0].ToolName != "test_tool" {
		t.Errorf("expected tool name 'test_tool', got %q", events[0].ToolName)
	}

	_ = r.Unregister("test_tool")

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[1].Type != EventToolUnregistered {
		t.Errorf("expected EventToolUnregistered, got %s", events[1].Type)
	}
}

func TestToolRegistry_Clone(t *testing.T) {
	r := NewToolRegistry()
	_ = r.Register(&mockTool{name: "tool1", description: "Tool 1"})
	_ = r.Register(&mockTool{name: "tool2", description: "Tool 2"})

	clone := r.Clone()
	if clone.Count() != 2 {
		t.Errorf("expected cloned registry to have 2 tools, got %d", clone.Count())
	}

	// Modifying clone shouldn't affect original
	_ = clone.Unregister("tool1")
	if r.Count() != 2 {
		t.Error("original registry should not be affected by clone modification")
	}
}

func TestToolRegistry_Merge(t *testing.T) {
	r1 := NewToolRegistry()
	_ = r1.Register(&mockTool{name: "tool1", description: "Tool 1"})

	r2 := NewToolRegistry()
	_ = r2.Register(&mockTool{name: "tool2", description: "Tool 2"})
	_ = r2.Register(&mockTool{name: "tool3", description: "Tool 3"})

	err := r1.Merge(r2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r1.Count() != 3 {
		t.Errorf("expected 3 tools after merge, got %d", r1.Count())
	}
}

func TestToolRegistry_Merge_Conflict(t *testing.T) {
	r1 := NewToolRegistry()
	_ = r1.Register(&mockTool{name: "tool1", description: "Tool 1"})

	r2 := NewToolRegistry()
	_ = r2.Register(&mockTool{name: "tool1", description: "Duplicate"})

	err := r1.Merge(r2)
	if err == nil {
		t.Error("expected error when merging registries with conflicting tool names")
	}
}

func TestToolRegistry_GetToolDefinitions(t *testing.T) {
	r := NewToolRegistry()
	_ = r.RegisterExtended(newMockExtendedTool("tool1", CategoryCode, ToolRequired))
	_ = r.RegisterExtended(newMockExtendedTool("tool2", CategoryVCS, ToolOptional))

	defs := r.GetToolDefinitions()
	if len(defs) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(defs))
	}

	defMap := make(map[string]*logits.ToolDefinition)
	for _, d := range defs {
		defMap[d.Name] = d
	}

	if defMap["tool1"] == nil || defMap["tool2"] == nil {
		t.Error("expected both tool1 and tool2 definitions")
	}
	if defMap["tool1"].Parameters == nil {
		t.Error("expected tool1 to have parameters schema")
	}
}

func TestToolRegistry_Concurrency(t *testing.T) {
	r := NewToolRegistry()
	var wg sync.WaitGroup

	// Concurrent registrations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tool := &mockTool{
				name:        string(rune('a' + (id % 26))),
				description: "Test",
			}
			_ = r.Register(tool) // Some will fail due to duplicates, that's OK
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.List()
			_ = r.ListNames()
			_ = r.Count()
		}()
	}

	wg.Wait()

	// Should have at most 26 tools (a-z)
	if r.Count() > 26 {
		t.Errorf("expected at most 26 tools, got %d", r.Count())
	}
}
