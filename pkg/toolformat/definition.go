// Package toolformat provides model-agnostic tool definitions and model-specific formatters
// for generating tool call prompts and parsing tool call responses.
package toolformat

// ToolDefinition represents a tool with its schema for LLM consumption.
// This is the canonical format that all tools should provide.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    ToolCategory    `json:"category"`
	Parameters  ParameterSchema `json:"parameters"`
	Handler     ToolHandler     `json:"-"` // Not serialized
}

// ToolCategory categorizes tools for filtering and mode selection
type ToolCategory string

const (
	CategoryCode     ToolCategory = "code"     // Code manipulation tools
	CategoryResearch ToolCategory = "research" // Research/web tools
	CategoryBlog     ToolCategory = "blog"     // Blog/content tools
	CategoryJob      ToolCategory = "job"      // Job management tools
	CategoryAgent    ToolCategory = "agent"    // Agent invocation tools
	CategoryDB       ToolCategory = "db"       // Database migration tools
)

// ToolHandler is the function signature for tool execution
type ToolHandler func(args map[string]interface{}) (*ToolResult, error)

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Success       bool                   `json:"success"`
	Output        string                 `json:"output"`
	Error         string                 `json:"error,omitempty"`
	ModifiedFiles []string               `json:"modified_files,omitempty"`
	Data          map[string]interface{} `json:"data,omitempty"`
}

// ParameterSchema describes the parameters a tool accepts using JSON Schema
type ParameterSchema struct {
	Type       string                    `json:"type"` // Always "object" for tool params
	Properties map[string]PropertySchema `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

// PropertySchema describes a single parameter property
type PropertySchema struct {
	Type        string                    `json:"type"` // "string", "number", "boolean", "array", "object"
	Description string                    `json:"description"`
	Enum        []string                  `json:"enum,omitempty"`       // For constrained string values
	Items       *PropertySchema           `json:"items,omitempty"`      // For array types
	Properties  map[string]PropertySchema `json:"properties,omitempty"` // For nested objects
	Default     interface{}               `json:"default,omitempty"`
}

// NewParameterSchema creates a new empty parameter schema
func NewParameterSchema() ParameterSchema {
	return ParameterSchema{
		Type:       "object",
		Properties: make(map[string]PropertySchema),
		Required:   []string{},
	}
}

// AddProperty adds a property to the schema
func (p *ParameterSchema) AddProperty(name string, prop PropertySchema, required bool) {
	p.Properties[name] = prop
	if required {
		p.Required = append(p.Required, name)
	}
}

// StringProperty creates a string property schema
func StringProperty(description string) PropertySchema {
	return PropertySchema{
		Type:        "string",
		Description: description,
	}
}

// StringEnumProperty creates a string property with allowed values
func StringEnumProperty(description string, values ...string) PropertySchema {
	return PropertySchema{
		Type:        "string",
		Description: description,
		Enum:        values,
	}
}

// NumberProperty creates a number property schema
func NumberProperty(description string) PropertySchema {
	return PropertySchema{
		Type:        "number",
		Description: description,
	}
}

// BoolProperty creates a boolean property schema
func BoolProperty(description string) PropertySchema {
	return PropertySchema{
		Type:        "boolean",
		Description: description,
	}
}

// ArrayProperty creates an array property schema
func ArrayProperty(description string, itemType PropertySchema) PropertySchema {
	return PropertySchema{
		Type:        "array",
		Description: description,
		Items:       &itemType,
	}
}

// ObjectProperty creates an object property schema
func ObjectProperty(description string, properties map[string]PropertySchema) PropertySchema {
	return PropertySchema{
		Type:        "object",
		Description: description,
		Properties:  properties,
	}
}
