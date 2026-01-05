package prompts

import (
	"context"
	"strings"
	"testing"

	"github.com/soypete/pedrocli/pkg/logits"
	"github.com/soypete/pedrocli/pkg/tools"
)

// mockExtendedTool implements ExtendedTool for testing
type mockExtendedTool struct {
	name        string
	description string
	metadata    *tools.ToolMetadata
}

func (m *mockExtendedTool) Name() string        { return m.name }
func (m *mockExtendedTool) Description() string { return m.description }
func (m *mockExtendedTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	return &tools.Result{Success: true}, nil
}
func (m *mockExtendedTool) Metadata() *tools.ToolMetadata { return m.metadata }

// TODO(issue): Update for native API tool calling - prompt format has changed
func TestToolPromptGenerator_GenerateToolSection(t *testing.T) {
	t.Skip("TODO: Tool prompt generator is for old GBNF system, needs update for native API")
	registry := tools.NewToolRegistry()

	// Register a tool with full metadata
	_ = registry.RegisterExtended(&mockExtendedTool{
		name:        "file",
		description: "Read, write, and modify files",
		metadata: &tools.ToolMetadata{
			Schema: &logits.JSONSchema{
				Type: "object",
				Properties: map[string]*logits.JSONSchema{
					"action": {
						Type:        "string",
						Description: "The operation to perform",
						Enum:        []interface{}{"read", "write", "list", "delete"},
					},
					"path": {
						Type:        "string",
						Description: "File path (absolute or relative)",
					},
					"content": {
						Type:        "string",
						Description: "Content for write action",
					},
				},
				Required: []string{"action", "path"},
			},
			Category:    tools.CategoryCode,
			Optionality: tools.ToolRequired,
			UsageHint:   "Use for reading files before modification or writing new files",
			Examples: []tools.ToolExample{
				{
					Description: "Read a file",
					Input:       map[string]interface{}{"action": "read", "path": "main.go"},
				},
			},
		},
	})

	generator := NewToolPromptGenerator(registry)
	output := generator.GenerateToolSection()

	// Verify key elements are present
	assertions := []struct {
		name     string
		expected string
	}{
		{"tool name header", "## file (required)"},
		{"tool description", "Read, write, and modify files"},
		{"parameters section", "**Parameters:**"},
		{"action parameter", "`action` (string, required)"},
		{"path parameter", "`path` (string, required)"},
		{"content parameter", "`content` (string, optional)"},
		{"enum values", "Valid values:"},
		{"usage hint", "**When to use:**"},
		{"examples section", "**Examples:**"},
	}

	for _, a := range assertions {
		if !strings.Contains(output, a.expected) {
			t.Errorf("%s: expected output to contain %q", a.name, a.expected)
		}
	}
}

func TestToolPromptGenerator_GenerateForBundle(t *testing.T) {
	registry := tools.NewToolRegistry()

	// Register tools that match the code_agent bundle
	bundleTools := []string{"file", "code_edit", "search", "navigate", "git"}
	for _, name := range bundleTools {
		_ = registry.RegisterExtended(&mockExtendedTool{
			name:        name,
			description: "Mock " + name + " tool",
			metadata: &tools.ToolMetadata{
				Category:    tools.CategoryCode,
				Optionality: tools.ToolRequired,
			},
		})
	}

	generator := NewToolPromptGenerator(registry)
	output := generator.GenerateForBundle("code_agent")

	// Verify all bundle tools are included
	for _, name := range bundleTools {
		if !strings.Contains(output, "## "+name) {
			t.Errorf("Expected bundle output to contain tool %q", name)
		}
	}
}

func TestToolPromptGenerator_GenerateForBundle_NotFound(t *testing.T) {
	registry := tools.NewToolRegistry()
	generator := NewToolPromptGenerator(registry)

	output := generator.GenerateForBundle("nonexistent_bundle")

	if output != "" {
		t.Errorf("Expected empty output for nonexistent bundle, got %q", output)
	}
}

func TestToolPromptGenerator_GenerateAvailable(t *testing.T) {
	registry := tools.NewToolRegistry()

	// Register tool requiring git capability
	_ = registry.RegisterExtended(&mockExtendedTool{
		name:        "git_tool",
		description: "Git operations",
		metadata: &tools.ToolMetadata{
			RequiresCapabilities: []string{"git"},
		},
	})

	// Register tool requiring notion_api capability
	_ = registry.RegisterExtended(&mockExtendedTool{
		name:        "notion_tool",
		description: "Notion operations",
		metadata: &tools.ToolMetadata{
			RequiresCapabilities: []string{"notion_api"},
		},
	})

	// Register tool with no requirements
	_ = registry.RegisterExtended(&mockExtendedTool{
		name:        "basic_tool",
		description: "Basic operations",
		metadata:    &tools.ToolMetadata{},
	})

	// Create checker with git available but not notion
	checker := tools.NewCapabilityChecker()
	checker.Override[tools.CapabilityGit] = true
	checker.Override[tools.CapabilityNotionAPI] = false

	generator := NewToolPromptGenerator(registry)
	output := generator.GenerateAvailable(checker)

	// Should include git_tool and basic_tool
	if !strings.Contains(output, "## git_tool") {
		t.Error("Expected available tools to include git_tool")
	}
	if !strings.Contains(output, "## basic_tool") {
		t.Error("Expected available tools to include basic_tool")
	}

	// Should NOT include notion_tool
	if strings.Contains(output, "## notion_tool") {
		t.Error("Expected available tools to NOT include notion_tool")
	}
}

// TODO(issue): Update for native API tool calling - prompt format has changed
func TestToolPromptGenerator_FormatTool(t *testing.T) {
	t.Skip("TODO: Tool prompt generator is for old GBNF system, needs update for native API")
	registry := tools.NewToolRegistry()
	generator := NewToolPromptGenerator(registry)

	tool := &mockExtendedTool{
		name:        "test_tool",
		description: "A test tool for testing",
		metadata: &tools.ToolMetadata{
			Optionality: tools.ToolOptional,
			UsageHint:   "Use this for testing purposes",
			Schema: &logits.JSONSchema{
				Type: "object",
				Properties: map[string]*logits.JSONSchema{
					"input": {
						Type:        "string",
						Description: "Input value",
					},
				},
				Required: []string{"input"},
			},
			Examples: []tools.ToolExample{
				{
					Description: "Basic usage",
					Input:       map[string]interface{}{"input": "hello"},
				},
			},
		},
	}

	output := generator.FormatTool(tool)

	// Verify structure
	if !strings.Contains(output, "## test_tool (optional)") {
		t.Error("Expected optional indicator in header")
	}
	if !strings.Contains(output, "A test tool for testing") {
		t.Error("Expected tool description")
	}
	if !strings.Contains(output, "`input` (string, required)") {
		t.Error("Expected parameter definition")
	}
	if !strings.Contains(output, "Use this for testing purposes") {
		t.Error("Expected usage hint")
	}
	if !strings.Contains(output, "Basic usage") {
		t.Error("Expected example description")
	}
}

func TestToolPromptGenerator_FormatTool_NoMetadata(t *testing.T) {
	registry := tools.NewToolRegistry()
	generator := NewToolPromptGenerator(registry)

	tool := &mockExtendedTool{
		name:        "simple_tool",
		description: "A simple tool without metadata",
		metadata:    nil,
	}

	output := generator.FormatTool(tool)

	// Should still produce basic output
	if !strings.Contains(output, "## simple_tool") {
		t.Error("Expected tool name header")
	}
	if !strings.Contains(output, "A simple tool without metadata") {
		t.Error("Expected tool description")
	}
}

func TestToolPromptGenerator_GenerateSummary(t *testing.T) {
	registry := tools.NewToolRegistry()

	_ = registry.RegisterExtended(&mockExtendedTool{
		name:        "alpha_tool",
		description: "First tool",
		metadata:    &tools.ToolMetadata{},
	})
	_ = registry.RegisterExtended(&mockExtendedTool{
		name:        "beta_tool",
		description: "Second tool",
		metadata:    &tools.ToolMetadata{},
	})

	generator := NewToolPromptGenerator(registry)
	output := generator.GenerateSummary()

	// Verify format and sorting
	lines := strings.Split(output, "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}

	if !strings.HasPrefix(lines[0], "- alpha_tool:") {
		t.Errorf("Expected alpha_tool first (sorted), got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "- beta_tool:") {
		t.Errorf("Expected beta_tool second (sorted), got %q", lines[1])
	}
}

func TestToolPromptGenerator_GenerateFullSection(t *testing.T) {
	registry := tools.NewToolRegistry()

	_ = registry.RegisterExtended(&mockExtendedTool{
		name:        "test_tool",
		description: "Test tool",
		metadata:    &tools.ToolMetadata{},
	})

	generator := NewToolPromptGenerator(registry)
	output := generator.GenerateFullSection()

	// Verify all sections are present
	sections := []string{
		"# Available Tools",
		"## Tool Call Format",
		"## Task Completion",
		"TASK_COMPLETE",
	}

	for _, section := range sections {
		if !strings.Contains(output, section) {
			t.Errorf("Expected full section to contain %q", section)
		}
	}
}

func TestToolPromptGenerator_EmptyRegistry(t *testing.T) {
	registry := tools.NewToolRegistry()
	generator := NewToolPromptGenerator(registry)

	output := generator.GenerateToolSection()

	if output != "No tools available." {
		t.Errorf("Expected 'No tools available.' for empty registry, got %q", output)
	}

	summary := generator.GenerateSummary()
	if summary != "No tools available." {
		t.Errorf("Expected 'No tools available.' for empty registry summary, got %q", summary)
	}
}

// TODO(issue): Update for native API tool calling - prompt format has changed
func TestToolPromptGenerator_ParameterWithDefaults(t *testing.T) {
	t.Skip("TODO: Tool prompt generator is for old GBNF system, needs update for native API")
	registry := tools.NewToolRegistry()

	defaultVal := "default_value"
	_ = registry.RegisterExtended(&mockExtendedTool{
		name:        "tool_with_defaults",
		description: "Tool with default parameter values",
		metadata: &tools.ToolMetadata{
			Schema: &logits.JSONSchema{
				Type: "object",
				Properties: map[string]*logits.JSONSchema{
					"mode": {
						Type:        "string",
						Description: "Operation mode",
						Default:     defaultVal,
					},
				},
			},
		},
	})

	generator := NewToolPromptGenerator(registry)
	output := generator.GenerateToolSection()

	if !strings.Contains(output, "(default: "+defaultVal+")") {
		t.Error("Expected default value to be shown")
	}
}

func TestToolPromptGenerator_GenerateForCategory(t *testing.T) {
	registry := tools.NewToolRegistry()

	// Register tools in different categories
	_ = registry.RegisterExtended(&mockExtendedTool{
		name:        "code_tool",
		description: "Code tool",
		metadata: &tools.ToolMetadata{
			Category: tools.CategoryCode,
		},
	})
	_ = registry.RegisterExtended(&mockExtendedTool{
		name:        "vcs_tool",
		description: "VCS tool",
		metadata: &tools.ToolMetadata{
			Category: tools.CategoryVCS,
		},
	})
	_ = registry.RegisterExtended(&mockExtendedTool{
		name:        "research_tool",
		description: "Research tool",
		metadata: &tools.ToolMetadata{
			Category: tools.CategoryResearch,
		},
	})

	generator := NewToolPromptGenerator(registry)
	output := generator.GenerateForCategory(tools.CategoryCode)

	if !strings.Contains(output, "## code_tool") {
		t.Error("Expected code_tool in category output")
	}
	if strings.Contains(output, "## vcs_tool") {
		t.Error("Did not expect vcs_tool in code category output")
	}
	if strings.Contains(output, "## research_tool") {
		t.Error("Did not expect research_tool in code category output")
	}
}
