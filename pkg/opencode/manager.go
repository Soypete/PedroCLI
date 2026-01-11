// Package opencode provides a unified manager for OpenCode-inspired features.
// This integrates agents, commands, skills, keybinds, permissions, and MCP servers.
package opencode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/soypete/pedrocli/pkg/agentregistry"
	"github.com/soypete/pedrocli/pkg/commands"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/keybinds"
	"github.com/soypete/pedrocli/pkg/mcp"
	"github.com/soypete/pedrocli/pkg/permission"
	"github.com/soypete/pedrocli/pkg/skills"
	"github.com/soypete/pedrocli/pkg/tools"
)

// Manager coordinates all OpenCode-inspired features
type Manager struct {
	// Core registries
	Agents      *agentregistry.AgentRegistry
	Commands    *commands.CommandRegistry
	Skills      *skills.SkillRegistry
	Keybinds    *keybinds.KeybindManager
	Permissions *permission.PermissionManager
	MCP         *mcp.Client

	// Configuration
	config  *config.Config
	workDir string
}

// NewManager creates a new OpenCode manager
func NewManager(cfg *config.Config, workDir string) *Manager {
	return &Manager{
		Agents:      agentregistry.NewAgentRegistry(),
		Commands:    commands.NewCommandRegistry(workDir),
		Skills:      skills.NewSkillRegistry(),
		Keybinds:    keybinds.NewKeybindManager(),
		Permissions: permission.NewPermissionManager(),
		MCP:         mcp.NewClient(),
		config:      cfg,
		workDir:     workDir,
	}
}

// Initialize loads all configurations and starts services
func (m *Manager) Initialize(ctx context.Context) error {
	// Load agents from config and markdown files
	if err := m.loadAgents(); err != nil {
		return fmt.Errorf("failed to load agents: %w", err)
	}

	// Load commands from config and markdown files
	if err := m.loadCommands(); err != nil {
		return fmt.Errorf("failed to load commands: %w", err)
	}

	// Discover skills
	if err := m.loadSkills(); err != nil {
		return fmt.Errorf("failed to load skills: %w", err)
	}

	// Load keybinds from config
	m.loadKeybinds()

	// Load permissions from config
	m.loadPermissions()

	// Initialize MCP servers
	if err := m.initializeMCP(ctx); err != nil {
		// Log but don't fail - MCP is optional
		fmt.Printf("Warning: MCP initialization failed: %v\n", err)
	}

	return nil
}

// loadAgents loads agents from config and markdown files
func (m *Manager) loadAgents() error {
	// Load from .pedro/agent/ and ~/.config/pedrocli/agent/
	agentPaths := []string{
		filepath.Join(m.workDir, ".pedro", "agent"),
	}

	if home, err := os.UserHomeDir(); err == nil {
		agentPaths = append(agentPaths, filepath.Join(home, ".config", "pedrocli", "agent"))
	}

	return m.Agents.LoadMarkdownAgents(agentPaths...)
}

// loadCommands loads commands from config and markdown files
func (m *Manager) loadCommands() error {
	// Load from .pedro/command/ and ~/.config/pedrocli/command/
	cmdPaths := []string{
		filepath.Join(m.workDir, ".pedro", "command"),
	}

	if home, err := os.UserHomeDir(); err == nil {
		cmdPaths = append(cmdPaths, filepath.Join(home, ".config", "pedrocli", "command"))
	}

	return m.Commands.LoadMarkdownCommands(cmdPaths...)
}

// loadSkills discovers skills from standard paths
func (m *Manager) loadSkills() error {
	return m.Skills.DiscoverWithDefaults(m.workDir)
}

// loadKeybinds loads keybinds from config (placeholder for future config integration)
func (m *Manager) loadKeybinds() {
	// Future: Load from config.Keybinds
	// For now, use defaults which are already set in NewKeybindManager()
}

// loadPermissions loads permissions from config (placeholder for future config integration)
func (m *Manager) loadPermissions() {
	// Future: Load from config.Permission
	// For now, set some sensible defaults
	m.Permissions.SetToolPermission("edit", permission.PermissionAllow)
	m.Permissions.SetToolPermission("write", permission.PermissionAllow)
	m.Permissions.SetToolPermission("bash", permission.PermissionAsk)
	m.Permissions.SetToolPermission("git", permission.PermissionAllow)

	// Dangerous bash commands
	m.Permissions.SetBashCommand("rm -rf*", permission.PermissionDeny)
	m.Permissions.SetBashCommand("sudo *", permission.PermissionDeny)
}

// initializeMCP initializes MCP servers from config
func (m *Manager) initializeMCP(ctx context.Context) error {
	// Load MCP servers from config.Execution.MCPServers
	for _, serverCfg := range m.config.Execution.MCPServers {
		cfg := mcp.ServerConfig{
			Type:    mcp.ServerTypeLocal,
			Command: append([]string{serverCfg.Command}, serverCfg.Args...),
			Enabled: true,
		}

		// Parse environment variables
		if len(serverCfg.Env) > 0 {
			cfg.Environment = make(map[string]string)
			for _, env := range serverCfg.Env {
				parts := splitEnvVar(env)
				if len(parts) == 2 {
					cfg.Environment[parts[0]] = parts[1]
				}
			}
		}

		if err := m.MCP.AddServer(serverCfg.Name, cfg); err != nil {
			return err
		}
	}

	return m.MCP.Initialize(ctx)
}

// splitEnvVar splits "KEY=VALUE" into [KEY, VALUE]
func splitEnvVar(env string) []string {
	for i := 0; i < len(env); i++ {
		if env[i] == '=' {
			return []string{env[:i], env[i+1:]}
		}
	}
	return []string{env}
}

// RegisterSkillTools registers skill tools with a tool registry
func (m *Manager) RegisterSkillTools(registry *tools.ToolRegistry) error {
	skillTool := skills.NewSkillTool(m.Skills)
	skillListTool := skills.NewSkillListTool(m.Skills)

	if err := registry.Register(skillTool); err != nil {
		return err
	}
	return registry.Register(skillListTool)
}

// RegisterMCPTools registers MCP tools with a tool registry
func (m *Manager) RegisterMCPTools(registry *tools.ToolRegistry) error {
	// Register the meta tool
	metaTool := mcp.NewMCPMetaTool(m.MCP)
	if err := registry.Register(metaTool); err != nil {
		return err
	}

	// Register all discovered MCP tools
	return mcp.RegisterMCPTools(m.MCP, registry)
}

// GetCurrentAgent returns the current primary agent
func (m *Manager) GetCurrentAgent() *agentregistry.Agent {
	return m.Agents.Current()
}

// CycleAgent cycles to the next primary agent
func (m *Manager) CycleAgent() *agentregistry.Agent {
	return m.Agents.CycleNext()
}

// ExecuteCommand expands and returns a command template
func (m *Manager) ExecuteCommand(name string, args []string) (string, error) {
	cmd, ok := m.Commands.Get(name)
	if !ok {
		return "", fmt.Errorf("command not found: %s", name)
	}

	// Handle builtins
	if m.Commands.IsBuiltin(name) {
		ctx := &commands.ExecutionContext{
			WorkDir: m.workDir,
			Agent:   m.GetCurrentAgent().Name,
			Model:   m.config.Model.ModelName,
		}
		return m.Commands.ExecuteBuiltin(name, args, ctx)
	}

	// Expand template
	ctx := &commands.ExecutionContext{
		WorkDir: m.workDir,
		Agent:   m.GetCurrentAgent().Name,
		Model:   m.config.Model.ModelName,
	}
	return m.Commands.ExpandTemplate(cmd.Template, args, ctx)
}

// CheckPermission checks if a tool execution is allowed
func (m *Manager) CheckPermission(tool string, description string, args ...string) (bool, error) {
	return m.Permissions.CheckAndRequest(tool, description, args...)
}

// LoadSkill loads a skill by name
func (m *Manager) LoadSkill(name string) (string, error) {
	return m.Skills.Load(name)
}

// HandleKeypress handles a keypress and returns true if it was handled
func (m *Manager) HandleKeypress(key string) bool {
	return m.Keybinds.HandleKey(key)
}

// Close cleans up resources
func (m *Manager) Close() error {
	return m.MCP.Close()
}

// GetAgentForCommand returns the agent to use for a command
func (m *Manager) GetAgentForCommand(cmdName string) (*agentregistry.Agent, error) {
	cmd, ok := m.Commands.Get(cmdName)
	if !ok {
		return nil, fmt.Errorf("command not found: %s", cmdName)
	}

	// If command specifies an agent, use that
	if cmd.Agent != "" {
		agent, ok := m.Agents.Get(cmd.Agent)
		if !ok {
			return nil, fmt.Errorf("agent not found: %s", cmd.Agent)
		}
		return agent, nil
	}

	// Otherwise use current agent
	return m.GetCurrentAgent(), nil
}

// SetupKeybindHandlers sets up keybind handlers with callbacks
func (m *Manager) SetupKeybindHandlers(handlers map[keybinds.Action]func()) {
	for action, handler := range handlers {
		m.Keybinds.RegisterHandler(action, handler)
	}
}
