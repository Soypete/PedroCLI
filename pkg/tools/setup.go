package tools

import (
	"github.com/soypete/pedrocli/pkg/config"
)

// CodeToolsSetup holds a registry and tools for coding agents.
// This provides a consistent set of tools across all execution modes (interactive, CLI, web server).
type CodeToolsSetup struct {
	Registry     *ToolRegistry
	FileTool     *FileTool
	CodeEditTool *CodeEditTool
	SearchTool   *SearchTool
	NavigateTool *NavigateTool
	GitTool      *GitTool
	BashTool     *BashTool
	TestTool     *TestTool
	GitHubTool   *GitHubTool
}

// NewCodeToolsSetup creates a consistent set of coding tools with registry.
// All 8 standard coding tools are created and registered with the registry
// for proper JSON schema generation.
func NewCodeToolsSetup(cfg *config.Config, workDir string) *CodeToolsSetup {
	registry := NewToolRegistry()

	setup := &CodeToolsSetup{
		Registry:     registry,
		FileTool:     NewFileTool(),
		CodeEditTool: NewCodeEditTool(),
		SearchTool:   NewSearchTool(workDir),
		NavigateTool: NewNavigateTool(workDir),
		GitTool:      NewGitTool(workDir),
		BashTool:     NewBashTool(cfg, workDir),
		TestTool:     NewTestTool(workDir),
		GitHubTool:   NewGitHubTool(workDir),
	}

	// Register all tools with the registry for proper schemas
	// Note: Register errors are safe to ignore here - tools are guaranteed unique
	_ = registry.Register(setup.FileTool)
	_ = registry.Register(setup.CodeEditTool)
	_ = registry.Register(setup.SearchTool)
	_ = registry.Register(setup.NavigateTool)
	_ = registry.Register(setup.GitTool)
	_ = registry.Register(setup.BashTool)
	_ = registry.Register(setup.TestTool)
	_ = registry.Register(setup.GitHubTool)

	return setup
}

// RegisterWithAgent registers all code tools with an agent and sets the registry.
// This ensures both tool execution (via RegisterTool) and proper schemas (via SetRegistry).
func (s *CodeToolsSetup) RegisterWithAgent(agent interface {
	RegisterTool(Tool)
	SetRegistry(*ToolRegistry)
}) {
	// Register tools with agent (for tool execution)
	agent.RegisterTool(s.FileTool)
	agent.RegisterTool(s.CodeEditTool)
	agent.RegisterTool(s.SearchTool)
	agent.RegisterTool(s.NavigateTool)
	agent.RegisterTool(s.GitTool)
	agent.RegisterTool(s.BashTool)
	agent.RegisterTool(s.TestTool)
	agent.RegisterTool(s.GitHubTool)

	// Set registry for dynamic prompts and schemas
	agent.SetRegistry(s.Registry)
}
