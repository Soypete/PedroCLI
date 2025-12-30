package agents

import (
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/prompts"
)

// CodingBaseAgent provides common functionality for coding agents
type CodingBaseAgent struct {
	*BaseAgent
	promptMgr *prompts.Manager
}

// NewCodingBaseAgent creates a new coding base agent
func NewCodingBaseAgent(name, description string, cfg *config.Config, backend llm.Backend, jobMgr *jobs.Manager) *CodingBaseAgent {
	base := NewBaseAgent(name, description, cfg, backend, jobMgr)
	return &CodingBaseAgent{
		BaseAgent: base,
		promptMgr: prompts.NewManager(cfg),
	}
}

// buildCodingSystemPrompt builds the system prompt for coding agents
func (a *CodingBaseAgent) buildCodingSystemPrompt() string {
	return a.promptMgr.GetCodingSystemPrompt() + `

Available tools:
- file: Read, write, and modify entire files
- code_edit: Precise line-based editing (edit/insert/delete specific lines)
- search: Search code (grep patterns, find files, find definitions)
- navigate: Navigate code structure (list directories, get file outlines, find imports)
- git: Execute git commands (status, diff, commit, push, etc.)
- bash: Run safe shell commands (limited to allowed commands)
- test: Run tests and parse results (Go, npm, Python)
- repo: Manage repositories (clone, list, switch)

Tool usage format:
{"tool": "tool_name", "args": {"action": "action_name", ...}}

When you have completed all tasks successfully, respond with "TASK_COMPLETE".`
}

// GetPromptManager returns the prompt manager for sub-agents
func (a *CodingBaseAgent) GetPromptManager() *prompts.Manager {
	return a.promptMgr
}
