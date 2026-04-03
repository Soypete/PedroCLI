package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/llm"
)

type Consolidator struct {
	llmBackend llm.Backend
}

func NewConsolidator(backend llm.Backend) *Consolidator {
	return &Consolidator{
		llmBackend: backend,
	}
}

func (c *Consolidator) Consolidate(ctx context.Context, input ConsolidationInput) (*ConsolidationResult, error) {
	sessionSummary, err := c.generateSessionSummary(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to generate session summary: %w", err)
	}

	facts, err := c.extractFacts(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to extract facts: %w", err)
	}

	openTasks, err := c.extractOpenTasks(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to extract open tasks: %w", err)
	}

	resumePacket, err := c.generateResumePacket(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to generate resume packet: %w", err)
	}

	prunedIDs := c.identifyPrunedArtifacts(input.Artifacts)

	return &ConsolidationResult{
		Summary:           *sessionSummary,
		Facts:             facts,
		OpenTasks:         openTasks,
		ResumePacket:      *resumePacket,
		PrunedArtifactIDs: prunedIDs,
	}, nil
}

func (c *Consolidator) generateSessionSummary(ctx context.Context, input ConsolidationInput) (*SessionSummary, error) {
	prompt := buildSessionSummaryPrompt(input)

	req := &llm.InferenceRequest{
		SystemPrompt: `You are a session summarizer. Generate a concise summary of what happened in this coding session. Focus on:
- What the user wanted to accomplish
- What was accomplished vs what was not
- Key files changed
- Any blockers or issues encountered
- Test results if any

Output a JSON object with these fields:
- id: unique summary ID (e.g., "sum_<session_id>")
- session_id: the session ID
- user_goal: what the user was trying to do
- summary: 2-3 sentence summary of what happened
- files_read: array of file paths that were read
- files_changed: array of files that were modified
- tests_run: number of tests run
- blockers: array of blocker descriptions (empty if none)
- result_status: one of completed, partial_success, failed, cancelled
- created_at: current timestamp in ISO 8601 format`,
		UserPrompt:  prompt,
		Temperature: 0.3,
		MaxTokens:   2000,
	}

	resp, err := c.llmBackend.Infer(ctx, req)
	if err != nil {
		return c.fallbackSessionSummary(input), nil
	}

	var summary SessionSummary
	if err := json.Unmarshal([]byte(resp.Text), &summary); err == nil {
		summary.ID = fmt.Sprintf("sum_%s", input.Session.SessionID)
		summary.SessionID = input.Session.SessionID
		summary.CreatedAt = time.Now()
		return &summary, nil
	}

	return c.fallbackSessionSummary(input), nil
}

func (c *Consolidator) fallbackSessionSummary(input ConsolidationInput) *SessionSummary {
	return &SessionSummary{
		ID:           fmt.Sprintf("sum_%s", input.Session.SessionID),
		SessionID:    input.Session.SessionID,
		UserGoal:     input.Session.UserGoal,
		Summary:      fmt.Sprintf("Session %s completed with status %s", input.Session.SessionID, input.Session.Status),
		FilesRead:    input.Session.FilesRead,
		FilesChanged: input.Session.FilesChanged,
		TestsRun:     input.Session.TestsRun,
		Blockers:     input.Session.Blockers,
		ResultStatus: input.Session.Status,
		CreatedAt:    time.Now(),
	}
}

func (c *Consolidator) extractFacts(ctx context.Context, input ConsolidationInput) ([]MemoryFact, error) {
	if len(input.Artifacts) == 0 && len(input.PriorFacts) == 0 {
		return nil, nil
	}

	prompt := buildFactsExtractionPrompt(input)

	req := &llm.InferenceRequest{
		SystemPrompt: `You are a fact extractor. Analyze the session artifacts and extract durable facts about the codebase.

For each fact, output a JSON object with these fields:
- id: unique fact ID (e.g., "mem_<number>")
- type: one of repo_fact, architecture_notes, tool_hints, user_preferences, coding_conventions, known_risks, failure_note
- scope: one of repo, file, function, global
- subject: what this fact is about (e.g., file path, module name)
- fact: the factual statement
- confidence: high, medium, or low
- evidence_artifacts: array of artifact IDs that support this fact
- created_at: current timestamp in ISO 8601 format

Only output facts that are:
1. Verifiable from the artifacts
2. Likely to be useful in future sessions
3. Not speculative or uncertain

Output as a JSON array of fact objects. If no useful facts, output an empty array [].`,
		UserPrompt:  prompt,
		Temperature: 0.3,
		MaxTokens:   3000,
	}

	resp, err := c.llmBackend.Infer(ctx, req)
	if err != nil {
		return nil, nil
	}

	var facts []MemoryFact
	if err := json.Unmarshal([]byte(resp.Text), &facts); err != nil {
		return nil, nil
	}

	for i := range facts {
		if facts[i].ID == "" {
			facts[i].ID = fmt.Sprintf("mem_%d", time.Now().UnixNano()%1000000+int64(i))
		}
		if facts[i].CreatedAt.IsZero() {
			facts[i].CreatedAt = time.Now()
		}
		if facts[i].LastValidatedAt.IsZero() {
			facts[i].LastValidatedAt = time.Now()
		}
	}

	return facts, nil
}

func (c *Consolidator) extractOpenTasks(ctx context.Context, input ConsolidationInput) ([]OpenTask, error) {
	prompt := buildOpenTasksPrompt(input)

	req := &llm.InferenceRequest{
		SystemPrompt: `You are a task extractor. Analyze the session artifacts and identify unfinished work that needs to be done in future sessions.

For each open task, output a JSON object with these fields:
- id: unique task ID (e.g., "task_<number>")
- title: brief description of the task
- scope: relevant file or module path
- status: open, in_progress, or blocked
- priority: high, medium, or low
- depends_on: array of task IDs this depends on (can be empty)
- evidence_artifacts: array of artifact IDs that mention this task
- created_at: current timestamp in ISO 8601 format
- last_updated_at: current timestamp in ISO 8601 format
- description: more detailed description of what needs to be done

Only include tasks that are:
1. Clearly not completed in this session
2. Actionable (not vague)
3. Worth remembering

Output as a JSON array of task objects. If no open tasks, output an empty array [].`,
		UserPrompt:  prompt,
		Temperature: 0.3,
		MaxTokens:   2000,
	}

	resp, err := c.llmBackend.Infer(ctx, req)
	if err != nil {
		return nil, nil
	}

	var tasks []OpenTask
	if err := json.Unmarshal([]byte(resp.Text), &tasks); err != nil {
		return nil, nil
	}

	for i := range tasks {
		if tasks[i].ID == "" {
			tasks[i].ID = fmt.Sprintf("task_%d", time.Now().UnixNano()%1000000+int64(i))
		}
		if tasks[i].CreatedAt.IsZero() {
			tasks[i].CreatedAt = time.Now()
		}
		if tasks[i].LastUpdatedAt.IsZero() {
			tasks[i].LastUpdatedAt = time.Now()
		}
	}

	return tasks, nil
}

func (c *Consolidator) generateResumePacket(ctx context.Context, input ConsolidationInput) (*ResumePacket, error) {
	prompt := buildResumePacketPrompt(input)

	req := &llm.InferenceRequest{
		SystemPrompt: `You are a resume packet generator. Create a compact handoff package for the next session.

Output a JSON object with these fields:
- repo_id: the repository identifier
- branch: current git branch
- goal: what the user was trying to accomplish
- next_step: recommended next step to continue the work
- changed_files: array of files that were modified
- warnings: array of warnings or concerns to be aware of
- created_at: current timestamp in ISO 8601 format
- session_id: the session ID
- validated: false (will be validated on load)
- validation_errors: empty array

Make sure next_step is specific and actionable.`,
		UserPrompt:  prompt,
		Temperature: 0.3,
		MaxTokens:   1000,
	}

	resp, err := c.llmBackend.Infer(ctx, req)
	if err != nil {
		return c.fallbackResumePacket(input), nil
	}

	var packet ResumePacket
	if err := json.Unmarshal([]byte(resp.Text), &packet); err == nil {
		packet.CreatedAt = time.Now()
		packet.SessionID = input.Session.SessionID
		packet.Validated = false
		packet.ValidationErrors = []string{}
		return &packet, nil
	}

	return c.fallbackResumePacket(input), nil
}

func (c *Consolidator) fallbackResumePacket(input ConsolidationInput) *ResumePacket {
	return &ResumePacket{
		RepoID:           input.Session.RepoID,
		Branch:           input.GitState.Branch,
		Goal:             input.Session.UserGoal,
		NextStep:         "Resume work from previous session",
		ChangedFiles:     input.Session.FilesChanged,
		Warnings:         input.Session.Blockers,
		CreatedAt:        time.Now(),
		SessionID:        input.Session.SessionID,
		Validated:        false,
		ValidationErrors: []string{},
	}
}

func (c *Consolidator) identifyPrunedArtifacts(artifacts []Artifact) []string {
	var pruned []string
	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)

	for _, artifact := range artifacts {
		if artifact.CreatedAt.Before(weekAgo) {
			pruned = append(pruned, artifact.ID)
		}
	}

	return pruned
}

func buildSessionSummaryPrompt(input ConsolidationInput) string {
	var sb strings.Builder
	sb.WriteString("Session Information:\n")
	sb.WriteString(fmt.Sprintf("- Session ID: %s\n", input.Session.SessionID))
	sb.WriteString(fmt.Sprintf("- Mode: %s\n", input.Session.Mode))
	sb.WriteString(fmt.Sprintf("- Status: %s\n", input.Session.Status))
	sb.WriteString(fmt.Sprintf("- User Goal: %s\n", input.Session.UserGoal))
	sb.WriteString(fmt.Sprintf("- Files Read: %v\n", input.Session.FilesRead))
	sb.WriteString(fmt.Sprintf("- Files Changed: %v\n", input.Session.FilesChanged))
	sb.WriteString(fmt.Sprintf("- Tests Run: %d\n", input.Session.TestsRun))
	sb.WriteString(fmt.Sprintf("- Blockers: %v\n", input.Session.Blockers))

	sb.WriteString("\nArtifacts:\n")
	for i, artifact := range input.Artifacts {
		if i > 20 {
			sb.WriteString(fmt.Sprintf("... and %d more artifacts\n", len(input.Artifacts)-20))
			break
		}
		sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", artifact.ID, artifact.Type, truncate(artifact.Content, 500)))
	}

	sb.WriteString("\nGit State:\n")
	sb.WriteString(fmt.Sprintf("- Branch: %s\n", input.GitState.Branch))
	sb.WriteString(fmt.Sprintf("- Commit: %s\n", input.GitState.CommitHash))
	sb.WriteString(fmt.Sprintf("- Modified: %v\n", input.GitState.ModifiedFiles))

	return sb.String()
}

func buildFactsExtractionPrompt(input ConsolidationInput) string {
	var sb strings.Builder

	sb.WriteString("Extract durable facts from the following session data:\n\n")
	sb.WriteString(fmt.Sprintf("Prior Facts (for reference, may still be valid):\n"))
	for _, fact := range input.PriorFacts {
		sb.WriteString(fmt.Sprintf("- [%s] %s: %s (confidence: %s)\n", fact.ID, fact.Type, fact.Fact, fact.Confidence))
	}

	sb.WriteString("\nCurrent Session Artifacts:\n")
	for i, artifact := range input.Artifacts {
		if i > 30 {
			sb.WriteString(fmt.Sprintf("... and %d more artifacts\n", len(input.Artifacts)-30))
			break
		}
		sb.WriteString(fmt.Sprintf("Artifact %s (%s):\n%s\n\n", artifact.ID, artifact.Type, truncate(artifact.Content, 1000)))
	}

	return sb.String()
}

func buildOpenTasksPrompt(input ConsolidationInput) string {
	var sb strings.Builder

	sb.WriteString("Identify unfinished work from the following session:\n\n")
	sb.WriteString(fmt.Sprintf("Session Goal: %s\n", input.Session.UserGoal))
	sb.WriteString(fmt.Sprintf("Status: %s\n", input.Session.Status))
	sb.WriteString(fmt.Sprintf("Blockers: %v\n", input.Session.Blockers))
	sb.WriteString(fmt.Sprintf("Changed Files: %v\n", input.Session.FilesChanged))

	sb.WriteString("\nArtifacts:\n")
	for i, artifact := range input.Artifacts {
		if i > 20 {
			break
		}
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", artifact.ID, truncate(artifact.Content, 500)))
	}

	return sb.String()
}

func buildResumePacketPrompt(input ConsolidationInput) string {
	var sb strings.Builder

	sb.WriteString("Generate a resume packet for the next session:\n\n")
	sb.WriteString(fmt.Sprintf("Repo ID: %s\n", input.Session.RepoID))
	sb.WriteString(fmt.Sprintf("Current Branch: %s\n", input.GitState.Branch))
	sb.WriteString(fmt.Sprintf("User Goal: %s\n", input.Session.UserGoal))
	sb.WriteString(fmt.Sprintf("Session Status: %s\n", input.Session.Status))
	sb.WriteString(fmt.Sprintf("Changed Files: %v\n", input.Session.FilesChanged))
	sb.WriteString(fmt.Sprintf("Blockers: %v\n", input.Session.Blockers))
	sb.WriteString(fmt.Sprintf("Errors: %v\n", input.Session.Errors))

	if len(input.Artifacts) > 0 {
		sb.WriteString("\nRecent Artifact Summaries:\n")
		for i, a := range input.Artifacts {
			if i > 5 {
				break
			}
			sb.WriteString(fmt.Sprintf("- %s: %s\n", a.Type, truncate(a.Content, 200)))
		}
	}

	return sb.String()
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
