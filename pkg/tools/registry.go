package tools

import (
	"fmt"
	"sync"

	"github.com/soypete/pedrocli/pkg/logits"
)

// RegistryEventType represents the type of registry event
type RegistryEventType string

const (
	EventToolRegistered   RegistryEventType = "registered"
	EventToolUnregistered RegistryEventType = "unregistered"
)

// RegistryEvent is emitted when the registry changes
type RegistryEvent struct {
	Type     RegistryEventType
	ToolName string
	Tool     ExtendedTool
}

// RegistryEventListener is called when registry events occur
type RegistryEventListener func(event RegistryEvent)

// ToolRegistry provides centralized tool management with discovery,
// filtering, and event notification capabilities
type ToolRegistry struct {
	mu        sync.RWMutex
	tools     map[string]ExtendedTool
	listeners []RegistryEventListener
}

// NewToolRegistry creates a new empty tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:     make(map[string]ExtendedTool),
		listeners: make([]RegistryEventListener, 0),
	}
}

// Register adds a tool to the registry. If the tool doesn't implement
// ExtendedTool, it will be wrapped with SimpleExtendedTool.
func (r *ToolRegistry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}

	// Wrap if needed
	var extTool ExtendedTool
	if et, ok := tool.(ExtendedTool); ok {
		extTool = et
	} else {
		extTool = NewSimpleExtendedTool(tool)
	}

	r.tools[name] = extTool
	r.notifyListeners(RegistryEvent{
		Type:     EventToolRegistered,
		ToolName: name,
		Tool:     extTool,
	})

	return nil
}

// RegisterExtended adds an ExtendedTool to the registry
func (r *ToolRegistry) RegisterExtended(tool ExtendedTool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}

	r.tools[name] = tool
	r.notifyListeners(RegistryEvent{
		Type:     EventToolRegistered,
		ToolName: name,
		Tool:     tool,
	})

	return nil
}

// Unregister removes a tool from the registry
func (r *ToolRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tool, exists := r.tools[name]
	if !exists {
		return fmt.Errorf("tool %q not found", name)
	}

	delete(r.tools, name)
	r.notifyListeners(RegistryEvent{
		Type:     EventToolUnregistered,
		ToolName: name,
		Tool:     tool,
	})

	return nil
}

// Get retrieves a tool by name
func (r *ToolRegistry) Get(name string) (ExtendedTool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	return tool, exists
}

// List returns all registered tools
func (r *ToolRegistry) List() []ExtendedTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]ExtendedTool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ListNames returns the names of all registered tools
func (r *ToolRegistry) ListNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Count returns the number of registered tools
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.tools)
}

// FilterByCategory returns tools matching the given category
func (r *ToolRegistry) FilterByCategory(category ToolCategory) []ExtendedTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []ExtendedTool
	for _, tool := range r.tools {
		meta := tool.Metadata()
		if meta != nil && meta.Category == category {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

// FilterByOptionality returns tools matching the given optionality
func (r *ToolRegistry) FilterByOptionality(optionality ToolOptionality) []ExtendedTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []ExtendedTool
	for _, tool := range r.tools {
		meta := tool.Metadata()
		if meta != nil && meta.Optionality == optionality {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

// FilterRequired returns all required tools
func (r *ToolRegistry) FilterRequired() []ExtendedTool {
	return r.FilterByOptionality(ToolRequired)
}

// FilterOptional returns all optional tools
func (r *ToolRegistry) FilterOptional() []ExtendedTool {
	return r.FilterByOptionality(ToolOptional)
}

// AddListener registers an event listener that will be called on registry changes
func (r *ToolRegistry) AddListener(listener RegistryEventListener) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.listeners = append(r.listeners, listener)
}

// notifyListeners calls all registered listeners with the given event
// Must be called with lock held
func (r *ToolRegistry) notifyListeners(event RegistryEvent) {
	for _, listener := range r.listeners {
		listener(event)
	}
}

// GenerateToolCallGrammar creates a GBNF grammar for valid tool calls
// based on all registered tools with schemas
func (r *ToolRegistry) GenerateToolCallGrammar() (*logits.GBNF, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	schemas := make(map[string]*logits.JSONSchema)
	for name, tool := range r.tools {
		meta := tool.Metadata()
		if meta != nil && meta.Schema != nil {
			schemas[name] = meta.Schema
		}
	}

	if len(schemas) == 0 {
		return nil, fmt.Errorf("no tools with schemas registered")
	}

	// Use MultiToolCallSchema from logits package
	multiSchema := logits.MultiToolCallSchema(schemas)
	grammarStr, err := logits.SchemaToGBNF(multiSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to generate grammar: %w", err)
	}

	return logits.ParseGBNF(grammarStr)
}

// GetToolDefinitions returns tool definitions suitable for LLM prompts
func (r *ToolRegistry) GetToolDefinitions() []*logits.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var defs []*logits.ToolDefinition
	for _, tool := range r.tools {
		meta := tool.Metadata()
		var schema *logits.JSONSchema
		if meta != nil {
			schema = meta.Schema
		}
		defs = append(defs, &logits.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  schema,
		})
	}
	return defs
}

// Clone creates a copy of the registry with the same tools
func (r *ToolRegistry) Clone() *ToolRegistry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clone := NewToolRegistry()
	for name, tool := range r.tools {
		clone.tools[name] = tool
	}
	return clone
}

// Merge adds all tools from another registry to this one
// Returns an error if any tool names conflict
func (r *ToolRegistry) Merge(other *ToolRegistry) error {
	other.mu.RLock()
	defer other.mu.RUnlock()

	// First check for conflicts
	r.mu.RLock()
	for name := range other.tools {
		if _, exists := r.tools[name]; exists {
			r.mu.RUnlock()
			return fmt.Errorf("tool %q already exists in target registry", name)
		}
	}
	r.mu.RUnlock()

	// Now add all tools
	for _, tool := range other.tools {
		if err := r.RegisterExtended(tool); err != nil {
			return err
		}
	}

	return nil
}
