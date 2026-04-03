package prompts

import (
	"fmt"
	"strings"
	"text/template"
)

type Layer int

const (
	LayerIdentity Layer = iota + 1
	LayerMode
	LayerPhase
	LayerTask
	LayerSkills
	LayerOutputContract
)

var layerNames = map[Layer]string{
	LayerIdentity:       "Identity",
	LayerMode:           "Mode",
	LayerPhase:          "Phase",
	LayerTask:           "Task",
	LayerSkills:         "Skills",
	LayerOutputContract: "Output Contract",
}

type LayerData struct {
	Identity        string
	Mode            string
	ModeConstraints string
	Phase           string
	PhaseGoal       string
	PhaseTools      string
	Task            string
	Skills          string
	OutputSchema    string
}

var layerTemplate = template.Must(template.New("layered").Parse(`
{{if .Identity}}# Identity (Layer 1)
{{.Identity}}
{{end}}

{{if .Mode}}# Mode: {{.Mode}} (Layer 2)
{{.ModeConstraints}}
{{end}}

{{if .Phase}}# Phase: {{.Phase}} (Layer 3)
Goal: {{.PhaseGoal}}
Available tools: {{.PhaseTools}}
{{end}}

{{if .Task}}# Task (Layer 4)
{{.Task}}
{{end}}

{{if .Skills}}# Project Context (Layer 5)
{{.Skills}}
{{end}}

{{if .OutputSchema}}# Output Format (Layer 6)
{{.OutputSchema}}
{{end}}
`))

type PromptBuilder struct {
	data LayerData

	identitySource string
	modeSource     string
	phaseSource    string
	skillsSource   string
}

func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{
		data: LayerData{},
	}
}

func (pb *PromptBuilder) SetIdentity(prompt string) *PromptBuilder {
	pb.identitySource = prompt
	pb.data.Identity = prompt
	return pb
}

func (pb *PromptBuilder) SetMode(mode string, constraints string) *PromptBuilder {
	pb.modeSource = mode
	pb.data.Mode = mode
	pb.data.ModeConstraints = constraints
	return pb
}

func (pb *PromptBuilder) SetPhase(name string, goal string, tools string) *PromptBuilder {
	pb.phaseSource = name
	pb.data.Phase = name
	pb.data.PhaseGoal = goal
	pb.data.PhaseTools = tools
	return pb
}

func (pb *PromptBuilder) SetTask(task string) *PromptBuilder {
	pb.data.Task = task
	return pb
}

func (pb *PromptBuilder) SetSkills(skills string) *PromptBuilder {
	pb.skillsSource = skills
	pb.data.Skills = skills
	return pb
}

func (pb *PromptBuilder) SetOutputSchema(schema string) *PromptBuilder {
	pb.data.OutputSchema = schema
	return pb
}

func (pb *PromptBuilder) Build() string {
	var sb strings.Builder
	err := layerTemplate.Execute(&sb, pb.data)
	if err != nil {
		return fmt.Sprintf("Error building prompt: %v", err)
	}
	return sb.String()
}

func (pb *PromptBuilder) BuildWithToolSection(toolSection string) string {
	base := pb.Build()
	return base + "\n\n# Tools\n\n" + toolSection + "\n\nFormat: {\"tool\": \"name\", \"args\": {...}}\nWhen done: TASK_COMPLETE"
}

func (pb *PromptBuilder) LayerCount() int {
	count := 0
	if pb.data.Identity != "" {
		count++
	}
	if pb.data.Mode != "" {
		count++
	}
	if pb.data.Phase != "" {
		count++
	}
	if pb.data.Task != "" {
		count++
	}
	if pb.data.Skills != "" {
		count++
	}
	if pb.data.OutputSchema != "" {
		count++
	}
	return count
}

type PromptLayerConfig struct {
	IdentityPath string
	ModePath     string
	PhasePath    string
	SkillsPath   string
}

func DefaultLayerConfig() PromptLayerConfig {
	return PromptLayerConfig{
		IdentityPath: "prompts/layers/identity.md",
		ModePath:     "prompts/layers/mode_%s.md",
		PhasePath:    "prompts/layers/phase_%s.md",
		SkillsPath:   "prompts/layers/skills.md",
	}
}

type ModeConstraints struct {
	AllowedTools     []string
	ForbiddenTools   []string
	RequiresApproval bool
	MaxRounds        int
	CanWrite         bool
}

var DefaultModeConstraints = map[string]ModeConstraints{
	"chat": {
		AllowedTools:     []string{"search", "navigate", "file", "context"},
		RequiresApproval: false,
		MaxRounds:        10,
		CanWrite:         false,
	},
	"plan": {
		AllowedTools:     []string{"search", "navigate", "file", "context", "git"},
		RequiresApproval: false,
		MaxRounds:        5,
		CanWrite:         false,
	},
	"build": {
		AllowedTools:     []string{"file", "code_edit", "search", "navigate", "bash", "git", "test", "github"},
		RequiresApproval: true,
		MaxRounds:        30,
		CanWrite:         true,
	},
	"review": {
		AllowedTools:     []string{"search", "navigate", "file", "git", "github", "test"},
		RequiresApproval: true,
		MaxRounds:        15,
		CanWrite:         false,
	},
}

func (c ModeConstraints) String() string {
	var sb strings.Builder
	sb.WriteString("Constraints:\n")
	sb.WriteString(fmt.Sprintf("- Can write files: %v\n", c.CanWrite))
	sb.WriteString(fmt.Sprintf("- Max inference rounds: %d\n", c.MaxRounds))
	sb.WriteString(fmt.Sprintf("- Requires approval: %v\n", c.RequiresApproval))
	if len(c.AllowedTools) > 0 {
		sb.WriteString(fmt.Sprintf("- Allowed tools: %s\n", strings.Join(c.AllowedTools, ", ")))
	}
	if len(c.ForbiddenTools) > 0 {
		sb.WriteString(fmt.Sprintf("- Forbidden tools: %s\n", strings.Join(c.ForbiddenTools, ", ")))
	}
	return sb.String()
}

const DefaultIdentityPrompt = `# Identity

You are **Pedro**, an autonomous coding assistant built with self-hosted LLMs. You are practical, reliable, and focused on delivering working code.

## Core Principles

1. **Working code over perfect code** - Get something functional first, then improve
2. **Ask for clarification when needed** - Don't make assumptions that could lead to wasted work
3. **Be transparent about limitations** - If you're unsure, say so
4. **Respect project conventions** - Follow existing patterns and styles in the codebase

## Communication Style

- Be concise and direct
- Explain *why* you're doing something when it's not obvious
- Show relevant code changes, not just describe them
- Use progress indicators for long-running tasks

## Safety Rules

- Never commit secrets, keys, or credentials
- Always verify before destructive operations (git push, rm -rf, etc.)
- Ask for confirmation before making changes outside the current directory
- Respect .gitignore and don't commit generated files`

var phaseGoalTemplates = map[string]string{
	"analyze":     "Understand the codebase and gather relevant context for the task",
	"plan":        "Create a detailed implementation plan with steps and file changes",
	"implement":   "Write the code changes according to the plan",
	"validate":    "Verify the implementation works and tests pass",
	"deliver":     "Finalize changes and prepare for review",
	"reproduce":   "Reproduce the issue to understand its nature",
	"investigate": "Find the root cause of the bug",
	"isolate":     "Identify the specific code causing the issue",
	"fix":         "Apply the fix for the identified issue",
	"verify":      "Confirm the fix resolves the issue",
	"commit":      "Commit the changes with appropriate message",
	"gather":      "Gather information about the code to review",
	"security":    "Check for security vulnerabilities",
	"quality":     "Review code quality and best practices",
	"compile":     "Verify the code compiles without errors",
	"publish":     "Finalize and publish the review",
}

func GetPhaseGoal(phaseName string) string {
	if goal, ok := phaseGoalTemplates[phaseName]; ok {
		return goal
	}
	return fmt.Sprintf("Execute the %s phase", phaseName)
}

func GetDefaultOutputSchema(phaseName string) string {
	schemas := map[string]string{
		"analyze":     "Return JSON with 'analysis' field containing code understanding",
		"plan":        "Return JSON with 'plan' field containing implementation steps",
		"implement":   "Return JSON with 'files_modified' and 'summary' fields",
		"validate":    "Return JSON with 'test_results' and 'success' fields",
		"deliver":     "Return JSON with 'commit_url' and 'summary' fields",
		"reproduce":   "Return JSON with 'reproduction_steps' and 'observed' fields",
		"investigate": "Return JSON with 'root_cause' and 'evidence' fields",
		"fix":         "Return JSON with 'fix_description' and 'files_changed' fields",
		"verify":      "Return JSON with 'verification_result' and 'tests_passed' fields",
		"gather":      "Return JSON with 'code_summary' and 'key_findings' fields",
		"security":    "Return JSON with 'vulnerabilities' and 'severity' fields",
		"quality":     "Return JSON with 'issues' and 'suggestions' fields",
		"compile":     "Return JSON with 'success' and 'errors' fields",
	}
	if schema, ok := schemas[phaseName]; ok {
		return schema
	}
	return "Return JSON with 'result' field containing the phase output"
}
