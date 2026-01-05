package toolformat

import (
	"context"
	"fmt"
	"sync"
)

// Registry is a centralized registry for all tools
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*ToolDefinition
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]*ToolDefinition),
	}
}

// Register adds a tool to the registry
func (r *Registry) Register(tool *ToolDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	if _, exists := r.tools[tool.Name]; exists {
		return fmt.Errorf("tool %q already registered", tool.Name)
	}

	r.tools[tool.Name] = tool
	return nil
}

// Get returns a tool by name
func (r *Registry) Get(name string) (*ToolDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools
func (r *Registry) List() []*ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

// ListByCategory returns tools in a specific category
func (r *Registry) ListByCategory(category ToolCategory) []*ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*ToolDefinition
	for _, tool := range r.tools {
		if tool.Category == category {
			result = append(result, tool)
		}
	}
	return result
}

// ListByCategories returns tools matching any of the specified categories
func (r *Registry) ListByCategories(categories ...ToolCategory) []*ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	categorySet := make(map[ToolCategory]bool)
	for _, c := range categories {
		categorySet[c] = true
	}

	var result []*ToolDefinition
	for _, tool := range r.tools {
		if categorySet[tool.Category] {
			result = append(result, tool)
		}
	}
	return result
}

// Execute executes a tool by name with the given arguments
func (r *Registry) Execute(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error) {
	tool, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}

	if tool.Handler == nil {
		return nil, fmt.Errorf("tool %q has no handler", name)
	}

	return tool.Handler(args)
}

// GetDefinitions returns tool definitions for the given tools (without handlers)
func (r *Registry) GetDefinitions(names ...string) []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []ToolDefinition
	if len(names) == 0 {
		// Return all tools
		for _, tool := range r.tools {
			result = append(result, *tool)
		}
	} else {
		// Return specific tools
		for _, name := range names {
			if tool, ok := r.tools[name]; ok {
				result = append(result, *tool)
			}
		}
	}
	return result
}

// GetDefinitionsByCategory returns tool definitions for a category
func (r *Registry) GetDefinitionsByCategory(category ToolCategory) []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []ToolDefinition
	for _, tool := range r.tools {
		if tool.Category == category {
			result = append(result, *tool)
		}
	}
	return result
}

// ToolMode represents a predefined set of tools for specific use cases
type ToolMode string

const (
	ModeCoding   ToolMode = "coding"   // Code manipulation tools
	ModeBlog     ToolMode = "blog"     // Blog writing tools
	ModePodcast  ToolMode = "podcast"  // Podcast tools (similar to blog)
	ModeResearch ToolMode = "research" // Research tools only
	ModeDB       ToolMode = "db"       // Database migration tools
	ModeAll      ToolMode = "all"      // All tools
)

// GetToolsForMode returns tools appropriate for a specific mode
func (r *Registry) GetToolsForMode(mode ToolMode) []ToolDefinition {
	switch mode {
	case ModeCoding:
		return r.GetDefinitionsByCategory(CategoryCode)
	case ModeBlog, ModePodcast:
		return append(
			r.GetDefinitionsByCategory(CategoryBlog),
			r.GetDefinitionsByCategory(CategoryResearch)...,
		)
	case ModeResearch:
		return r.GetDefinitionsByCategory(CategoryResearch)
	case ModeDB:
		return r.GetDefinitionsByCategory(CategoryDB)
	case ModeAll:
		return r.GetDefinitions()
	default:
		return r.GetDefinitionsByCategory(CategoryCode)
	}
}

// DefaultRegistry is the global default registry
var DefaultRegistry = NewRegistry()

// Register adds a tool to the default registry
func Register(tool *ToolDefinition) error {
	return DefaultRegistry.Register(tool)
}

// Get returns a tool from the default registry
func Get(name string) (*ToolDefinition, bool) {
	return DefaultRegistry.Get(name)
}

// Execute executes a tool from the default registry
func Execute(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error) {
	return DefaultRegistry.Execute(ctx, name, args)
}

// AgentInterface is an interface that agents must implement for registration
// This avoids importing the agents package and creating an import cycle
type AgentInterface interface {
	Name() string
	Description() string
	Execute(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// RegisterAgent registers an agent as a tool in the registry
// This implements the adapter.AgentRegistry interface
func (r *Registry) RegisterAgent(agent AgentInterface) error {
	handler := func(args map[string]interface{}) (*ToolResult, error) {
		job, err := agent.Execute(context.Background(), args)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		// Extract job ID if possible
		jobID := "unknown"
		if j, ok := job.(interface{ GetID() string }); ok {
			jobID = j.GetID()
		} else if j, ok := job.(struct{ ID string }); ok {
			jobID = j.ID
		}

		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Job %s started", jobID),
		}, nil
	}

	schema := getAgentSchema(agent.Name())

	def := &ToolDefinition{
		Name:        agent.Name(),
		Description: agent.Description(),
		Category:    CategoryAgent,
		Parameters:  schema,
		Handler:     handler,
	}

	return r.Register(def)
}

// getAgentSchema returns the parameter schema for an agent
func getAgentSchema(name string) ParameterSchema {
	switch name {
	case "builder":
		return ParameterSchema{
			Type:     "object",
			Required: []string{"description"},
			Properties: map[string]PropertySchema{
				"description": {Type: "string", Description: "Description of the feature to build"},
				"issue":       {Type: "string", Description: "GitHub issue number (optional)"},
			},
		}
	case "debugger":
		return ParameterSchema{
			Type:     "object",
			Required: []string{"description"},
			Properties: map[string]PropertySchema{
				"description": {Type: "string", Description: "Description of the bug symptoms"},
				"error_log":   {Type: "string", Description: "Path to error log file (optional)"},
			},
		}
	case "reviewer":
		return ParameterSchema{
			Type:     "object",
			Required: []string{"branch"},
			Properties: map[string]PropertySchema{
				"branch":    {Type: "string", Description: "Branch name to review"},
				"pr_number": {Type: "string", Description: "PR number (optional)"},
			},
		}
	case "triager":
		return ParameterSchema{
			Type:     "object",
			Required: []string{"description"},
			Properties: map[string]PropertySchema{
				"description": {Type: "string", Description: "Description of the issue to triage"},
				"error_log":   {Type: "string", Description: "Path to error log file (optional)"},
			},
		}
	case "blog_orchestrator":
		return ParameterSchema{
			Type:     "object",
			Required: []string{"prompt"},
			Properties: map[string]PropertySchema{
				"prompt":  {Type: "string", Description: "Blog post prompt or dictation"},
				"title":   {Type: "string", Description: "Blog post title (optional)"},
				"publish": {Type: "boolean", Description: "Auto-publish to Notion"},
			},
		}
	default:
		return ParameterSchema{
			Type:       "object",
			Properties: map[string]PropertySchema{},
		}
	}
}
