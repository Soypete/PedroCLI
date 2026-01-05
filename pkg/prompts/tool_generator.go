package prompts

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/soypete/pedrocli/pkg/logits"
	"github.com/soypete/pedrocli/pkg/tools"
)

// ToolPromptGenerator generates tool descriptions for system prompts
// from registry metadata, replacing static hardcoded tool lists.
type ToolPromptGenerator struct {
	registry *tools.ToolRegistry
}

// NewToolPromptGenerator creates a new tool prompt generator
func NewToolPromptGenerator(registry *tools.ToolRegistry) *ToolPromptGenerator {
	return &ToolPromptGenerator{
		registry: registry,
	}
}

// GenerateToolSection creates the "Available Tools" section for system prompts.
// It iterates through all registered tools and formats them with their
// parameters, usage hints, and examples.
func (g *ToolPromptGenerator) GenerateToolSection() string {
	toolList := g.registry.List()
	return g.formatTools(toolList)
}

// GenerateForBundle creates a tool section for a specific bundle.
// Returns tools that are part of the bundle (both required and optional).
func (g *ToolPromptGenerator) GenerateForBundle(bundleName string) string {
	bundle := tools.GetBundle(bundleName)
	if bundle == nil {
		return ""
	}

	var bundleTools []tools.ExtendedTool
	for _, toolName := range bundle.AllToolNames() {
		if tool, ok := g.registry.Get(toolName); ok {
			bundleTools = append(bundleTools, tool)
		}
	}

	return g.formatTools(bundleTools)
}

// GenerateForCategory creates a tool section for a specific category.
func (g *ToolPromptGenerator) GenerateForCategory(category tools.ToolCategory) string {
	toolList := g.registry.FilterByCategory(category)
	return g.formatTools(toolList)
}

// GenerateAvailable creates a tool section for tools that have all their
// required capabilities available.
func (g *ToolPromptGenerator) GenerateAvailable(checker tools.CapabilityChecker) string {
	toolList := g.registry.ListAvailable(checker)
	return g.formatTools(toolList)
}

// formatTools formats a list of tools into markdown documentation
func (g *ToolPromptGenerator) formatTools(toolList []tools.ExtendedTool) string {
	if len(toolList) == 0 {
		return "No tools available."
	}

	// Sort tools by name for consistent output
	sort.Slice(toolList, func(i, j int) bool {
		return toolList[i].Name() < toolList[j].Name()
	})

	var sections []string
	for _, tool := range toolList {
		section := g.FormatTool(tool)
		if section != "" {
			sections = append(sections, section)
		}
	}

	return strings.Join(sections, "\n\n")
}

// FormatTool creates a formatted description for a single tool
func (g *ToolPromptGenerator) FormatTool(tool tools.ExtendedTool) string {
	var sb strings.Builder

	meta := tool.Metadata()

	// Tool header - just name, no optionality marker
	sb.WriteString(fmt.Sprintf("## %s\n", tool.Name()))
	sb.WriteString(tool.Description())
	sb.WriteString("\n")

	// ONE example only (skip parameters - they're in the example)
	if meta != nil && len(meta.Examples) > 0 {
		sb.WriteString(g.formatExample(tool.Name(), meta.Examples[0]))
	}

	return sb.String()
}

// formatParameters formats the schema properties as a parameter list
// TODO: Part of old GBNF manual tool formatting system, superseded by native API
//
//nolint:unused // Kept for reference
func (g *ToolPromptGenerator) formatParameters(schema *logits.JSONSchema) string {
	if schema == nil || schema.Properties == nil || len(schema.Properties) == 0 {
		return ""
	}

	// Create required set for quick lookup
	requiredSet := make(map[string]bool)
	for _, name := range schema.Required {
		requiredSet[name] = true
	}

	// Sort property names for consistent output
	propNames := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		propNames = append(propNames, name)
	}
	sort.Strings(propNames)

	var lines []string
	for _, name := range propNames {
		prop := schema.Properties[name]
		line := g.formatParameter(name, prop, requiredSet[name])
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// formatParameter formats a single parameter
//
//nolint:unused // Part of old GBNF system
func (g *ToolPromptGenerator) formatParameter(name string, prop *logits.JSONSchema, required bool) string {
	var sb strings.Builder

	// Parameter name and type
	paramType := prop.Type
	if paramType == "" {
		paramType = "any"
	}

	requiredStr := "optional"
	if required {
		requiredStr = "required"
	}

	sb.WriteString(fmt.Sprintf("- `%s` (%s, %s)", name, paramType, requiredStr))

	// Description
	if prop.Description != "" {
		sb.WriteString(": ")
		sb.WriteString(prop.Description)
	}

	// Enum values
	if len(prop.Enum) > 0 {
		enumStrs := make([]string, 0, len(prop.Enum))
		for _, v := range prop.Enum {
			enumStrs = append(enumStrs, fmt.Sprintf("%q", v))
		}
		sb.WriteString(fmt.Sprintf("\n  Valid values: [%s]", strings.Join(enumStrs, ", ")))
	}

	// Default value
	if prop.Default != nil {
		sb.WriteString(fmt.Sprintf(" (default: %v)", prop.Default))
	}

	return sb.String()
}

// formatExample formats a tool example
func (g *ToolPromptGenerator) formatExample(toolName string, example tools.ToolExample) string {
	// Format the tool call JSON - no description, just the example
	call := map[string]interface{}{
		"tool": toolName,
		"args": example.Input,
	}
	jsonBytes, err := json.MarshalIndent(call, "", "  ")
	if err != nil {
		return ""
	}
	return string(jsonBytes) + "\n"
}

// GenerateToolCallFormat returns the standard tool call format instruction
func (g *ToolPromptGenerator) GenerateToolCallFormat() string {
	return `## Tool Call Format

Use tools by providing JSON objects with the following structure:
{"tool": "tool_name", "args": {"key": "value"}}

You can call multiple tools in sequence by providing multiple JSON objects.
Always use the exact parameter names shown in the tool documentation.`
}

// GenerateCompletionSignal returns the task completion instruction
func (g *ToolPromptGenerator) GenerateCompletionSignal() string {
	return `## Task Completion

When all tasks are complete and verified, respond with "TASK_COMPLETE".
Do not mark the task complete until you have verified your changes work correctly.`
}

// GenerateFullSection generates a complete tool section including
// the tool list, call format, and completion signal.
func (g *ToolPromptGenerator) GenerateFullSection() string {
	var parts []string

	parts = append(parts, "# Available Tools")
	parts = append(parts, g.GenerateToolSection())
	parts = append(parts, g.GenerateToolCallFormat())
	parts = append(parts, g.GenerateCompletionSignal())

	return strings.Join(parts, "\n\n")
}

// GenerateSummary creates a brief summary of available tools (name and one-line description)
func (g *ToolPromptGenerator) GenerateSummary() string {
	toolList := g.registry.List()
	if len(toolList) == 0 {
		return "No tools available."
	}

	// Sort by name
	sort.Slice(toolList, func(i, j int) bool {
		return toolList[i].Name() < toolList[j].Name()
	})

	var lines []string
	for _, tool := range toolList {
		lines = append(lines, fmt.Sprintf("- %s: %s", tool.Name(), tool.Description()))
	}

	return strings.Join(lines, "\n")
}
