package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/prompts"
	"github.com/soypete/pedrocli/pkg/tools"
)

// CodingBaseAgent provides common functionality for coding agents
type CodingBaseAgent struct {
	*BaseAgent
	promptMgr *prompts.Manager
}

// NewCodingBaseAgent creates a new coding base agent
func NewCodingBaseAgent(name, description string, cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager) *CodingBaseAgent {
	base := NewBaseAgent(name, description, cfg, backend, jobMgr)
	return &CodingBaseAgent{
		BaseAgent: base,
		promptMgr: prompts.NewManager(cfg),
	}
}

// NewCodingBaseAgentWithRegistry creates a new coding base agent with a tool registry
func NewCodingBaseAgentWithRegistry(name, description string, cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager, registry *tools.ToolRegistry) *CodingBaseAgent {
	base := NewBaseAgentWithRegistry(name, description, cfg, backend, jobMgr, registry)
	return &CodingBaseAgent{
		BaseAgent: base,
		promptMgr: prompts.NewManager(cfg),
	}
}

// buildCodingSystemPrompt builds the system prompt for coding agents
func (a *CodingBaseAgent) buildCodingSystemPrompt() string {
	// Use dynamic tool prompt if registry is available
	if a.toolPromptGen != nil {
		return a.buildDynamicCodingSystemPrompt()
	}

	// Fall back to static prompt for backward compatibility
	return a.buildStaticCodingSystemPrompt()
}

// buildDynamicCodingSystemPrompt builds a system prompt with dynamically generated tool descriptions
func (a *CodingBaseAgent) buildDynamicCodingSystemPrompt() string {
	// Use the code_agent bundle for coding-specific tools
	toolSection := a.toolPromptGen.GenerateForBundle("code_agent")
	if toolSection == "" {
		// Fall back to all registry tools if bundle not found or empty
		toolSection = a.toolPromptGen.GenerateToolSection()
	}

	return a.promptMgr.GetCodingSystemPrompt() + `

# Available Tools

` + toolSection + `

## Tool Call Format
Use tools by providing JSON objects: {"tool": "tool_name", "args": {"key": "value"}}

When you have completed all tasks successfully, respond with "TASK_COMPLETE".`
}

// buildStaticCodingSystemPrompt returns the legacy static system prompt
func (a *CodingBaseAgent) buildStaticCodingSystemPrompt() string {
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

// setupWorkDirectory sets up the working directory for the job.
// If repo info (provider, owner, repo) is provided, it uses the repo tool to ensure the repo exists.
// Otherwise, it uses the current working directory.
func (a *CodingBaseAgent) setupWorkDirectory(ctx context.Context, jobID string, input map[string]interface{}) (string, error) {
	// Check if repo information is provided
	provider, hasProvider := input["provider"].(string)
	owner, hasOwner := input["owner"].(string)
	repo, hasRepo := input["repo"].(string)

	if hasProvider && hasOwner && hasRepo {
		// Use repo tool to ensure repo exists
		repoTool := a.tools["repo"]
		if repoTool == nil {
			return "", fmt.Errorf("repo tool not available")
		}

		result, err := repoTool.Execute(ctx, map[string]interface{}{
			"action":   "ensure_repo",
			"provider": provider,
			"owner":    owner,
			"repo":     repo,
		})
		if err != nil {
			return "", fmt.Errorf("failed to ensure repo: %w", err)
		}

		if !result.Success {
			return "", fmt.Errorf("repo tool failed: %s", result.Error)
		}

		// Parse the output to get LocalPath
		var repoInfo struct {
			LocalPath string `json:"local_path"`
		}

		// Try to extract LocalPath from the output
		lines := result.Output
		if jsonStart := findJSON(lines); jsonStart != -1 {
			if err := json.Unmarshal([]byte(lines[jsonStart:]), &repoInfo); err == nil && repoInfo.LocalPath != "" {
				return repoInfo.LocalPath, nil
			}
		}

		return "", fmt.Errorf("failed to extract local_path from repo tool output")
	}

	// No repo info provided, use current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	return cwd, nil
}

// findJSON finds the start of JSON in a string (looks for opening brace)
func findJSON(s string) int {
	for i, c := range s {
		if c == '{' {
			return i
		}
	}
	return -1
}
