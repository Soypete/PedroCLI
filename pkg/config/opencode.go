package config

// OpenCodeConfig contains OpenCode-inspired configuration sections
// These extend the base Config to support agents, commands, skills, etc.

// AgentDefinition defines an agent configuration (from JSON or markdown)
type AgentDefinition struct {
	Name        string            `json:"name,omitempty"`        // Agent name (from key or frontmatter)
	Mode        string            `json:"mode"`                  // "primary" or "subagent"
	Description string            `json:"description,omitempty"` // Human-readable description
	Model       string            `json:"model,omitempty"`       // Override model (e.g., "ollama/qwen2.5-coder:32b")
	Temperature float64           `json:"temperature,omitempty"` // Override temperature
	MaxSteps    int               `json:"max_steps,omitempty"`   // Max inference iterations
	Prompt      string            `json:"prompt,omitempty"`      // System prompt or file reference
	Tools       map[string]bool   `json:"tools,omitempty"`       // Tool enable/disable overrides
	Permission  map[string]string `json:"permission,omitempty"`  // Per-tool permission overrides
	Hidden      bool              `json:"hidden,omitempty"`      // Hide from agent list
	Disabled    bool              `json:"disabled,omitempty"`    // Disable this agent
}

// CommandDefinition defines a slash command configuration
type CommandDefinition struct {
	Name        string `json:"name,omitempty"`        // Command name (from key or frontmatter)
	Template    string `json:"template"`              // Prompt template with substitutions
	Description string `json:"description,omitempty"` // Human-readable description
	Agent       string `json:"agent,omitempty"`       // Override agent for this command
	Model       string `json:"model,omitempty"`       // Override model for this command
	Subtask     bool   `json:"subtask,omitempty"`     // Run as subagent/subtask
}

// SkillDefinition defines a loadable skill configuration
type SkillDefinition struct {
	Name        string `json:"name,omitempty"`        // Skill name
	Description string `json:"description,omitempty"` // Human-readable description
	Content     string `json:"content,omitempty"`     // Full skill content (loaded from file)
	Path        string `json:"path,omitempty"`        // Path to SKILL.md file
}

// KeybindConfig contains keybind configuration
type KeybindConfig struct {
	Leader             string            `json:"leader,omitempty"`              // Leader key (default: ctrl+x)
	AppExit            string            `json:"app_exit,omitempty"`            // Exit application
	SessionNew         string            `json:"session_new,omitempty"`         // New session
	SessionList        string            `json:"session_list,omitempty"`        // List sessions
	AgentCycle         string            `json:"agent_cycle,omitempty"`         // Next primary agent
	AgentCycleReverse  string            `json:"agent_cycle_reverse,omitempty"` // Previous primary agent
	ModelList          string            `json:"model_list,omitempty"`          // Select model
	CommandList        string            `json:"command_list,omitempty"`        // Command palette
	InputSubmit        string            `json:"input_submit,omitempty"`        // Send message
	InputNewline       string            `json:"input_newline,omitempty"`       // Insert newline
	MessagesUndo       string            `json:"messages_undo,omitempty"`       // Undo last change
	MessagesRedo       string            `json:"messages_redo,omitempty"`       // Redo undone change
	InputClear         string            `json:"input_clear,omitempty"`         // Clear input
	HistoryPrev        string            `json:"history_prev,omitempty"`        // Previous history item
	HistoryNext        string            `json:"history_next,omitempty"`        // Next history item
	AutocompleteToggle string            `json:"autocomplete_toggle,omitempty"` // Toggle autocomplete
	HelpToggle         string            `json:"help_toggle,omitempty"`         // Toggle help panel
	Custom             map[string]string `json:"custom,omitempty"`              // Custom keybinds
}

// PermissionConfig contains permission configuration
type PermissionConfig struct {
	// Global tool permissions
	Edit     string `json:"edit,omitempty"`     // "allow", "deny", "ask"
	Write    string `json:"write,omitempty"`    // "allow", "deny", "ask"
	Bash     string `json:"bash,omitempty"`     // "allow", "deny", "ask"
	WebFetch string `json:"webfetch,omitempty"` // "allow", "deny", "ask"
	Git      string `json:"git,omitempty"`      // "allow", "deny", "ask"
	// Pattern-based permissions (e.g., "mcp_github_*": "ask")
	Patterns map[string]string `json:"patterns,omitempty"`
	// Command-specific bash permissions
	BashCommands map[string]string `json:"bash_commands,omitempty"` // e.g., "rm -rf*": "deny"
	// Skill permissions
	Skills map[string]string `json:"skills,omitempty"` // e.g., "internal-*": "deny"
}

// MCPServerConfig defines an MCP server configuration (extended from existing)
type ExtendedMCPConfig struct {
	Type        string            `json:"type,omitempty"`        // "local" or "remote"
	Command     []string          `json:"command,omitempty"`     // Command to start server
	URL         string            `json:"url,omitempty"`         // Remote URL
	Headers     map[string]string `json:"headers,omitempty"`     // HTTP headers
	Environment map[string]string `json:"environment,omitempty"` // Environment variables
	Enabled     bool              `json:"enabled"`               // Enable/disable
}

// DefaultKeybinds returns the default keybind configuration
func DefaultKeybinds() KeybindConfig {
	return KeybindConfig{
		Leader:             "ctrl+x",
		AppExit:            "ctrl+c,ctrl+d,<leader>q",
		SessionNew:         "<leader>n",
		SessionList:        "<leader>l",
		AgentCycle:         "tab",
		AgentCycleReverse:  "shift+tab",
		ModelList:          "<leader>m",
		CommandList:        "ctrl+p",
		InputSubmit:        "return",
		InputNewline:       "shift+return,ctrl+return",
		MessagesUndo:       "<leader>u",
		MessagesRedo:       "<leader>r",
		InputClear:         "ctrl+c",
		HistoryPrev:        "up",
		HistoryNext:        "down",
		AutocompleteToggle: "ctrl+space",
		HelpToggle:         "<leader>?",
	}
}

// DefaultPermissions returns the default permission configuration
func DefaultPermissions() PermissionConfig {
	return PermissionConfig{
		Edit:     "allow",
		Write:    "allow",
		Bash:     "ask",
		WebFetch: "allow",
		Git:      "allow",
		BashCommands: map[string]string{
			"go *":    "allow",
			"git *":   "allow",
			"make *":  "allow",
			"rm -rf*": "deny",
			"sudo *":  "deny",
		},
	}
}
