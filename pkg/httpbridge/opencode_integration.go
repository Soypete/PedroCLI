package httpbridge

import (
	"context"
	"log"

	"github.com/soypete/pedrocli/pkg/opencode"
	"github.com/soypete/pedrocli/pkg/tools"
)

// OpenCodeAppContext extends AppContext with OpenCode features
type OpenCodeAppContext struct {
	*AppContext
	OpenCode *opencode.Manager
}

// NewOpenCodeAppContext creates an AppContext with OpenCode features enabled
func NewOpenCodeAppContext(ctx *AppContext) (*OpenCodeAppContext, error) {
	// Create OpenCode manager
	ocManager := opencode.NewManager(ctx.Config, ctx.WorkDir)

	// Initialize OpenCode features
	if err := ocManager.Initialize(context.Background()); err != nil {
		log.Printf("Warning: OpenCode initialization failed: %v", err)
		// Continue without OpenCode features
	}

	return &OpenCodeAppContext{
		AppContext: ctx,
		OpenCode:   ocManager,
	}, nil
}

// RegisterOpenCodeTools registers OpenCode tools with a tool registry
func (ctx *OpenCodeAppContext) RegisterOpenCodeTools(registry *tools.ToolRegistry) error {
	if ctx.OpenCode == nil {
		return nil
	}

	// Register skill tools
	if err := ctx.OpenCode.RegisterSkillTools(registry); err != nil {
		log.Printf("Warning: failed to register skill tools: %v", err)
	}

	// Register MCP tools
	if err := ctx.OpenCode.RegisterMCPTools(registry); err != nil {
		log.Printf("Warning: failed to register MCP tools: %v", err)
	}

	return nil
}

// Close closes all resources including OpenCode manager
func (ctx *OpenCodeAppContext) Close() error {
	if ctx.OpenCode != nil {
		ctx.OpenCode.Close()
	}
	return ctx.AppContext.Close()
}

// GetCurrentAgentName returns the current OpenCode agent name
func (ctx *OpenCodeAppContext) GetCurrentAgentName() string {
	if ctx.OpenCode == nil {
		return "build" // Default
	}
	if agent := ctx.OpenCode.GetCurrentAgent(); agent != nil {
		return agent.Name
	}
	return "build"
}

// CycleAgent cycles to the next primary agent and returns its name
func (ctx *OpenCodeAppContext) CycleAgent() string {
	if ctx.OpenCode == nil {
		return "build"
	}
	if agent := ctx.OpenCode.CycleAgent(); agent != nil {
		return agent.Name
	}
	return "build"
}

// ExecuteCommand executes an OpenCode slash command
func (ctx *OpenCodeAppContext) ExecuteCommand(name string, args []string) (string, error) {
	if ctx.OpenCode == nil {
		return "", nil
	}
	return ctx.OpenCode.ExecuteCommand(name, args)
}

// LoadSkill loads a skill by name
func (ctx *OpenCodeAppContext) LoadSkill(name string) (string, error) {
	if ctx.OpenCode == nil {
		return "", nil
	}
	return ctx.OpenCode.LoadSkill(name)
}

// CheckPermission checks if a tool is allowed to execute
func (ctx *OpenCodeAppContext) CheckPermission(tool, description string, args ...string) (bool, error) {
	if ctx.OpenCode == nil {
		return true, nil // Allow by default if OpenCode not enabled
	}
	return ctx.OpenCode.CheckPermission(tool, description, args...)
}
