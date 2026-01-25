// Package agentregistry provides OpenCode-inspired agent management with primary/subagent modes.
package agentregistry

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// AgentMode represents the type of agent
type AgentMode string

const (
	AgentModePrimary  AgentMode = "primary"
	AgentModeSubagent AgentMode = "subagent"
)

// Agent represents a registered agent with its configuration
type Agent struct {
	Name        string
	Description string
	Mode        AgentMode
	Model       string
	Temperature float64
	MaxSteps    int
	Prompt      string            // System prompt content
	PromptFile  string            // Path to prompt file (for lazy loading)
	Tools       map[string]bool   // Tool enable/disable
	Permission  map[string]string // Per-tool permissions
	Hidden      bool              // Hidden from agent list
	Disabled    bool              // Agent is disabled
	Source      string            // "builtin", "config", "markdown"
}

// AgentRegistry manages agent registration and cycling
type AgentRegistry struct {
	mu       sync.RWMutex
	agents   map[string]*Agent
	primary  []*Agent // Ordered list for cycling
	current  int      // Current primary agent index
	builtins map[string]*Agent
}

// NewAgentRegistry creates a new agent registry
func NewAgentRegistry() *AgentRegistry {
	r := &AgentRegistry{
		agents:   make(map[string]*Agent),
		primary:  make([]*Agent, 0),
		builtins: make(map[string]*Agent),
	}
	r.registerBuiltins()
	return r
}

// registerBuiltins registers the default built-in agents
func (r *AgentRegistry) registerBuiltins() {
	builtins := []*Agent{
		{
			Name:        "build",
			Description: "Full access development agent for building features",
			Mode:        AgentModePrimary,
			MaxSteps:    50,
			Tools:       map[string]bool{"*": true},
			Permission:  map[string]string{"*": "allow"},
			Source:      "builtin",
		},
		{
			Name:        "plan",
			Description: "Read-only analysis and planning agent",
			Mode:        AgentModePrimary,
			MaxSteps:    30,
			Tools:       map[string]bool{"read": true, "grep": true, "glob": true, "navigate": true},
			Permission:  map[string]string{"edit": "deny", "write": "deny", "bash": "ask"},
			Source:      "builtin",
		},
		{
			Name:        "debug",
			Description: "Debugging agent for fixing issues",
			Mode:        AgentModePrimary,
			MaxSteps:    50,
			Tools:       map[string]bool{"*": true},
			Permission:  map[string]string{"*": "allow"},
			Source:      "builtin",
		},
		{
			Name:        "review",
			Description: "Code review agent for analyzing changes",
			Mode:        AgentModePrimary,
			MaxSteps:    30,
			Tools:       map[string]bool{"read": true, "grep": true, "git": true, "navigate": true},
			Permission:  map[string]string{"edit": "deny", "write": "deny"},
			Source:      "builtin",
		},
		{
			Name:        "research",
			Description: "Deep research subagent for complex questions",
			Mode:        AgentModeSubagent,
			MaxSteps:    20,
			Tools:       map[string]bool{"read": true, "grep": true, "webscrape": true, "websearch": true},
			Permission:  map[string]string{"*": "allow"},
			Source:      "builtin",
		},
		{
			Name:        "blog",
			Description: "Blog post writing and editing agent",
			Mode:        AgentModePrimary,
			MaxSteps:    40,
			Tools:       map[string]bool{"*": true},
			Permission:  map[string]string{"*": "allow"},
			Source:      "builtin",
		},
		{
			Name:        "podcast",
			Description: "Podcast episode planning and scripting agent",
			Mode:        AgentModePrimary,
			MaxSteps:    40,
			Tools:       map[string]bool{"*": true},
			Permission:  map[string]string{"*": "allow"},
			Source:      "builtin",
		},
	}

	for _, agent := range builtins {
		r.builtins[agent.Name] = agent
		r.agents[agent.Name] = agent
		if agent.Mode == AgentModePrimary && !agent.Disabled {
			r.primary = append(r.primary, agent)
		}
	}
}

// Register adds or updates an agent in the registry
func (r *AgentRegistry) Register(name string, agent *Agent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent.Name = name
	r.agents[name] = agent

	// Update primary agent list if needed
	if agent.Mode == AgentModePrimary && !agent.Disabled && !agent.Hidden {
		// Check if already in primary list
		found := false
		for i, a := range r.primary {
			if a.Name == name {
				r.primary[i] = agent
				found = true
				break
			}
		}
		if !found {
			r.primary = append(r.primary, agent)
		}
	} else {
		// Remove from primary list if no longer primary
		for i, a := range r.primary {
			if a.Name == name {
				r.primary = append(r.primary[:i], r.primary[i+1:]...)
				break
			}
		}
	}
}

// Get returns an agent by name
func (r *AgentRegistry) Get(name string) (*Agent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.agents[name]
	return agent, ok
}

// List returns all registered agents
func (r *AgentRegistry) List() []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Agent, 0, len(r.agents))
	for _, agent := range r.agents {
		if !agent.Disabled && !agent.Hidden {
			result = append(result, agent)
		}
	}
	return result
}

// ListPrimary returns all primary agents
func (r *AgentRegistry) ListPrimary() []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Agent, len(r.primary))
	copy(result, r.primary)
	return result
}

// ListSubagents returns all subagents
func (r *AgentRegistry) ListSubagents() []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Agent, 0)
	for _, agent := range r.agents {
		if agent.Mode == AgentModeSubagent && !agent.Disabled && !agent.Hidden {
			result = append(result, agent)
		}
	}
	return result
}

// Current returns the current primary agent
func (r *AgentRegistry) Current() *Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.primary) == 0 {
		return nil
	}
	return r.primary[r.current]
}

// CycleNext cycles to the next primary agent
func (r *AgentRegistry) CycleNext() *Agent {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.primary) == 0 {
		return nil
	}
	r.current = (r.current + 1) % len(r.primary)
	return r.primary[r.current]
}

// CyclePrev cycles to the previous primary agent
func (r *AgentRegistry) CyclePrev() *Agent {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.primary) == 0 {
		return nil
	}
	r.current = (r.current - 1 + len(r.primary)) % len(r.primary)
	return r.primary[r.current]
}

// SetCurrent sets the current primary agent by name
func (r *AgentRegistry) SetCurrent(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, agent := range r.primary {
		if agent.Name == name {
			r.current = i
			return nil
		}
	}
	return fmt.Errorf("agent not found or not a primary agent: %s", name)
}

// LoadFromConfig loads agents from JSON configuration
func (r *AgentRegistry) LoadFromConfig(agents map[string]AgentConfig) error {
	for name, cfg := range agents {
		agent := cfg.ToAgent()
		agent.Name = name
		agent.Source = "config"
		r.Register(name, agent)
	}
	return nil
}

// LoadMarkdownAgents loads agents from markdown files
func (r *AgentRegistry) LoadMarkdownAgents(basePaths ...string) error {
	for _, basePath := range basePaths {
		entries, err := os.ReadDir(basePath)
		if err != nil {
			continue // Skip non-existent directories
		}

		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				path := filepath.Join(basePath, entry.Name())
				agent, err := r.loadMarkdownAgent(path)
				if err != nil {
					continue // Skip invalid files
				}
				agent.Source = "markdown"
				r.Register(agent.Name, agent)
			}
		}
	}
	return nil
}

// AgentFrontmatter represents the YAML frontmatter in agent markdown files
type AgentFrontmatter struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Mode        string            `yaml:"mode"`
	Model       string            `yaml:"model"`
	Temperature float64           `yaml:"temperature"`
	MaxSteps    int               `yaml:"max_steps"`
	Tools       map[string]bool   `yaml:"tools"`
	Permission  map[string]string `yaml:"permission"`
	Hidden      bool              `yaml:"hidden"`
	Disabled    bool              `yaml:"disabled"`
}

// loadMarkdownAgent loads a single agent from a markdown file
func (r *AgentRegistry) loadMarkdownAgent(path string) (*Agent, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse frontmatter
	frontmatter, body := parseFrontmatter(string(content))

	var fm AgentFrontmatter
	if frontmatter != "" {
		if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
			return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
		}
	}

	// Default name from filename
	if fm.Name == "" {
		fm.Name = strings.TrimSuffix(filepath.Base(path), ".md")
	}

	// Default mode to primary
	mode := AgentModePrimary
	if fm.Mode == "subagent" {
		mode = AgentModeSubagent
	}

	return &Agent{
		Name:        fm.Name,
		Description: fm.Description,
		Mode:        mode,
		Model:       fm.Model,
		Temperature: fm.Temperature,
		MaxSteps:    fm.MaxSteps,
		Prompt:      body,
		Tools:       fm.Tools,
		Permission:  fm.Permission,
		Hidden:      fm.Hidden,
		Disabled:    fm.Disabled,
	}, nil
}

// parseFrontmatter extracts YAML frontmatter and body from markdown content
func parseFrontmatter(content string) (frontmatter, body string) {
	re := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\n(.*)$`)
	matches := re.FindStringSubmatch(content)
	if len(matches) == 3 {
		return matches[1], matches[2]
	}
	return "", content
}

// AgentConfig represents the JSON configuration for an agent
type AgentConfig struct {
	Mode        string            `json:"mode"`
	Description string            `json:"description,omitempty"`
	Model       string            `json:"model,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
	MaxSteps    int               `json:"max_steps,omitempty"`
	Prompt      string            `json:"prompt,omitempty"`
	Tools       map[string]bool   `json:"tools,omitempty"`
	Permission  map[string]string `json:"permission,omitempty"`
	Hidden      bool              `json:"hidden,omitempty"`
	Disabled    bool              `json:"disabled,omitempty"`
}

// ToAgent converts an AgentConfig to an Agent
func (c *AgentConfig) ToAgent() *Agent {
	mode := AgentModePrimary
	if c.Mode == "subagent" {
		mode = AgentModeSubagent
	}

	return &Agent{
		Description: c.Description,
		Mode:        mode,
		Model:       c.Model,
		Temperature: c.Temperature,
		MaxSteps:    c.MaxSteps,
		Prompt:      c.Prompt,
		Tools:       c.Tools,
		Permission:  c.Permission,
		Hidden:      c.Hidden,
		Disabled:    c.Disabled,
	}
}

// GetBuiltin returns a builtin agent by name
func (r *AgentRegistry) GetBuiltin(name string) (*Agent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.builtins[name]
	return agent, ok
}

// Reset resets the registry to only builtin agents
func (r *AgentRegistry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.agents = make(map[string]*Agent)
	r.primary = make([]*Agent, 0)
	r.current = 0

	for name, agent := range r.builtins {
		r.agents[name] = agent
		if agent.Mode == AgentModePrimary && !agent.Disabled {
			r.primary = append(r.primary, agent)
		}
	}
}
