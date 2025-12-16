package agents

import (
	"fmt"
	"os"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/tools"
)

// AgentFactory creates and configures agents with all tools registered
type AgentFactory struct {
	config     *config.Config
	backend    llm.Backend
	jobManager *jobs.Manager
	workDir    string
	tools      map[string]tools.Tool
}

// NewAgentFactory creates a new agent factory
func NewAgentFactory(cfg *config.Config, backend llm.Backend, jobMgr *jobs.Manager, workDir string) *AgentFactory {
	// Determine work directory
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// Create all tools
	toolsMap := make(map[string]tools.Tool)
	fileTool := tools.NewFileTool()
	gitTool := tools.NewGitTool(workDir)
	bashTool := tools.NewBashTool(cfg, workDir)
	testTool := tools.NewTestTool(workDir)
	codeEditTool := tools.NewCodeEditTool()
	searchTool := tools.NewSearchTool(workDir)
	navigateTool := tools.NewNavigateTool(workDir)

	toolsMap[fileTool.Name()] = fileTool
	toolsMap[gitTool.Name()] = gitTool
	toolsMap[bashTool.Name()] = bashTool
	toolsMap[testTool.Name()] = testTool
	toolsMap[codeEditTool.Name()] = codeEditTool
	toolsMap[searchTool.Name()] = searchTool
	toolsMap[navigateTool.Name()] = navigateTool

	return &AgentFactory{
		config:     cfg,
		backend:    backend,
		jobManager: jobMgr,
		workDir:    workDir,
		tools:      toolsMap,
	}
}

// CreateAllAgents creates all agents with tools registered
func (f *AgentFactory) CreateAllAgents() map[string]Agent {
	agents := make(map[string]Agent)

	builderAgent := NewBuilderAgent(f.config, f.backend, f.jobManager)
	f.registerToolsForAgent(builderAgent)
	agents["builder"] = builderAgent

	reviewerAgent := NewReviewerAgent(f.config, f.backend, f.jobManager)
	f.registerToolsForAgent(reviewerAgent)
	agents["reviewer"] = reviewerAgent

	debuggerAgent := NewDebuggerAgent(f.config, f.backend, f.jobManager)
	f.registerToolsForAgent(debuggerAgent)
	agents["debugger"] = debuggerAgent

	triagerAgent := NewTriagerAgent(f.config, f.backend, f.jobManager)
	f.registerToolsForAgent(triagerAgent)
	agents["triager"] = triagerAgent

	return agents
}

// CreateAgent creates a single agent by name with tools
func (f *AgentFactory) CreateAgent(name string) (Agent, error) {
	var agent Agent

	switch name {
	case "builder":
		builderAgent := NewBuilderAgent(f.config, f.backend, f.jobManager)
		f.registerToolsForAgent(builderAgent)
		agent = builderAgent
	case "reviewer":
		reviewerAgent := NewReviewerAgent(f.config, f.backend, f.jobManager)
		f.registerToolsForAgent(reviewerAgent)
		agent = reviewerAgent
	case "debugger":
		debuggerAgent := NewDebuggerAgent(f.config, f.backend, f.jobManager)
		f.registerToolsForAgent(debuggerAgent)
		agent = debuggerAgent
	case "triager":
		triagerAgent := NewTriagerAgent(f.config, f.backend, f.jobManager)
		f.registerToolsForAgent(triagerAgent)
		agent = triagerAgent
	default:
		return nil, fmt.Errorf("unknown agent type: %s", name)
	}

	return agent, nil
}

// GetAllTools returns all tools created by this factory
func (f *AgentFactory) GetAllTools() map[string]tools.Tool {
	return f.tools
}

// registerToolsForAgent registers all tools for an agent
func (f *AgentFactory) registerToolsForAgent(agent Agent) {
	for _, tool := range f.tools {
		agent.RegisterTool(tool)
	}
}
