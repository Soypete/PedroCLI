package tools

import (
	"testing"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCodeToolsSetup(t *testing.T) {
	cfg := &config.Config{
		Project: config.ProjectConfig{Workdir: "/tmp/test"},
		Tools: config.ToolsConfig{
			AllowedBashCommands: []string{"echo", "ls"},
			ForbiddenCommands:   []string{"rm"},
		},
	}

	setup := NewCodeToolsSetup(cfg, "/tmp/test")

	// Verify all tools created
	require.NotNil(t, setup.FileTool, "FileTool should be created")
	require.NotNil(t, setup.CodeEditTool, "CodeEditTool should be created")
	require.NotNil(t, setup.SearchTool, "SearchTool should be created")
	require.NotNil(t, setup.NavigateTool, "NavigateTool should be created")
	require.NotNil(t, setup.GitTool, "GitTool should be created")
	require.NotNil(t, setup.BashTool, "BashTool should be created")
	require.NotNil(t, setup.TestTool, "TestTool should be created")
	require.NotNil(t, setup.GitHubTool, "GitHubTool should be created")

	// Verify registry created and tools registered
	require.NotNil(t, setup.Registry, "Registry should be created")
	toolsList := setup.Registry.List()
	assert.Equal(t, 8, len(toolsList), "Should have 8 tools in registry")

	// Verify tool names
	toolNames := make(map[string]bool)
	for _, tool := range toolsList {
		toolNames[tool.Name()] = true
	}

	expectedTools := []string{"file", "code_edit", "search", "navigate", "git", "bash", "test", "github"}
	for _, expected := range expectedTools {
		assert.True(t, toolNames[expected], "Tool %s should be in registry", expected)
	}
}

func TestCodeToolsSetupConsistency(t *testing.T) {
	// This test verifies that all modes use the same tool setup
	// by checking that the NewCodeToolsSetup helper creates a consistent tool set

	cfg := &config.Config{
		Project: config.ProjectConfig{Workdir: "/tmp/test"},
		Model: config.ModelConfig{
			EnableTools: true,
		},
		Tools: config.ToolsConfig{
			AllowedBashCommands: []string{"echo"},
		},
	}

	// Create setup
	setup := NewCodeToolsSetup(cfg, "/tmp/test")

	// Verify consistent tool set
	expectedToolCount := 8
	actualToolCount := len(setup.Registry.List())

	assert.Equal(t, expectedToolCount, actualToolCount,
		"All modes should register %d tools (file, code_edit, search, navigate, git, bash, test, github)",
		expectedToolCount)

	// Verify all tools have proper metadata
	for _, tool := range setup.Registry.List() {
		assert.NotEmpty(t, tool.Name(), "Tool name should not be empty")
		assert.NotEmpty(t, tool.Description(), "Tool description should not be empty")
	}
}

// mockAgent implements the agent interface for testing
type mockAgent struct {
	tools    []Tool
	registry *ToolRegistry
}

func (m *mockAgent) RegisterTool(tool Tool) {
	m.tools = append(m.tools, tool)
}

func (m *mockAgent) SetRegistry(reg *ToolRegistry) {
	m.registry = reg
}

func TestRegisterWithAgent(t *testing.T) {
	cfg := &config.Config{
		Project: config.ProjectConfig{Workdir: "/tmp/test"},
		Tools: config.ToolsConfig{
			AllowedBashCommands: []string{"echo"},
		},
	}

	setup := NewCodeToolsSetup(cfg, "/tmp/test")

	// Create mock agent
	mock := &mockAgent{
		tools: make([]Tool, 0),
	}

	// Register tools and set registry
	setup.RegisterWithAgent(mock)

	// Verify tools were registered
	assert.Equal(t, 8, len(mock.tools), "Should have registered 8 tools with agent")
	assert.NotNil(t, mock.registry, "Registry should be set on agent")
	assert.Equal(t, setup.Registry, mock.registry, "Registry should match setup registry")
}

func TestCodeToolsSetupToolNames(t *testing.T) {
	cfg := &config.Config{
		Project: config.ProjectConfig{Workdir: "/tmp/test"},
		Tools: config.ToolsConfig{
			AllowedBashCommands: []string{"echo"},
		},
	}

	setup := NewCodeToolsSetup(cfg, "/tmp/test")

	// Test individual tool names
	assert.Equal(t, "file", setup.FileTool.Name())
	assert.Equal(t, "code_edit", setup.CodeEditTool.Name())
	assert.Equal(t, "search", setup.SearchTool.Name())
	assert.Equal(t, "navigate", setup.NavigateTool.Name())
	assert.Equal(t, "git", setup.GitTool.Name())
	assert.Equal(t, "bash", setup.BashTool.Name())
	assert.Equal(t, "test", setup.TestTool.Name())
	assert.Equal(t, "github", setup.GitHubTool.Name())
}
