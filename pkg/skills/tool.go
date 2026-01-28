package skills

import (
	"context"
	"fmt"

	"github.com/soypete/pedrocli/pkg/tools"
)

// SkillTool provides on-demand skill loading for agents
type SkillTool struct {
	registry *SkillRegistry
}

// NewSkillTool creates a new skill tool
func NewSkillTool(registry *SkillRegistry) *SkillTool {
	return &SkillTool{
		registry: registry,
	}
}

// Name returns the tool name
func (t *SkillTool) Name() string {
	return "skill"
}

// Description returns the tool description including available skills
func (t *SkillTool) Description() string {
	base := `Load a skill to get detailed instructions and context for a specific task.
Skills provide specialized knowledge and workflows that can help you complete tasks more effectively.
Use this tool when you need guidance on how to approach a specific type of task.

`
	return base + t.registry.GetToolDescription()
}

// Execute loads a skill by name
func (t *SkillTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return &tools.Result{
			Success: false,
			Error:   "skill name is required",
		}, nil
	}

	content, err := t.registry.Load(name)
	if err != nil {
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("failed to load skill: %v", err),
		}, nil
	}

	return &tools.Result{
		Success: true,
		Output:  content,
		Data: map[string]interface{}{
			"skill_name": name,
		},
	}, nil
}

// Metadata returns the tool metadata for the skill tool
func (t *SkillTool) Metadata() *tools.ToolMetadata {
	return &tools.ToolMetadata{
		Category:  tools.CategoryUtility,
		UsageHint: "Use this tool when you need specialized guidance for a task. Skills provide workflows and best practices.",
		Examples: []tools.ToolExample{
			{
				Description: "Load the git-release skill",
				Input: map[string]interface{}{
					"name": "git-release",
				},
			},
			{
				Description: "Load the api-design skill",
				Input: map[string]interface{}{
					"name": "api-design",
				},
			},
		},
	}
}

// SkillListTool provides skill discovery for agents
type SkillListTool struct {
	registry *SkillRegistry
}

// NewSkillListTool creates a new skill list tool
func NewSkillListTool(registry *SkillRegistry) *SkillListTool {
	return &SkillListTool{
		registry: registry,
	}
}

// Name returns the tool name
func (t *SkillListTool) Name() string {
	return "skill_list"
}

// Description returns the tool description
func (t *SkillListTool) Description() string {
	return "List available skills that can be loaded for specialized guidance. Optionally filter by category or search query."
}

// Execute lists available skills
func (t *SkillListTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	var skills []*Skill

	// Check for category filter
	if category, ok := args["category"].(string); ok && category != "" {
		skills = t.registry.ListByCategory(category)
	} else if query, ok := args["query"].(string); ok && query != "" {
		skills = t.registry.Search(query)
	} else {
		skills = t.registry.List()
	}

	if len(skills) == 0 {
		return &tools.Result{
			Success: true,
			Output:  "No skills found.",
		}, nil
	}

	var output string
	output = fmt.Sprintf("Found %d skill(s):\n\n", len(skills))
	for _, skill := range skills {
		output += fmt.Sprintf("- **%s**", skill.Name)
		if skill.Description != "" {
			output += fmt.Sprintf(": %s", skill.Description)
		}
		if skill.Category != "" {
			output += fmt.Sprintf(" [%s]", skill.Category)
		}
		output += "\n"
	}

	return &tools.Result{
		Success: true,
		Output:  output,
		Data: map[string]interface{}{
			"count": len(skills),
		},
	}, nil
}

// Metadata returns the tool metadata
func (t *SkillListTool) Metadata() *tools.ToolMetadata {
	return &tools.ToolMetadata{
		Category:  tools.CategoryUtility,
		UsageHint: "List available skills that can be loaded for specialized guidance.",
		Examples: []tools.ToolExample{
			{
				Description: "List all skills",
				Input:       map[string]interface{}{},
			},
			{
				Description: "List skills in the 'git' category",
				Input: map[string]interface{}{
					"category": "git",
				},
			},
		},
	}
}
