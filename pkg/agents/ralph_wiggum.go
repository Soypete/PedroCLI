package agents

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
)

//go:embed prompts/ralph_base.md
var ralphBasePrompt string

//go:embed prompts/ralph_code.md
var ralphCodePrompt string

//go:embed prompts/ralph_blog.md
var ralphBlogPrompt string

//go:embed prompts/ralph_podcast.md
var ralphPodcastPrompt string

// RalphWiggumAgent implements an iterative autonomous loop that works through
// a PRD (Product Requirements Document) of user stories. It supports three modes:
// code, blog, and podcast — each with mode-specific quality standards.
//
// Inspired by the Ralph Wiggum methodology: iteration over perfection,
// persistent memory between loops, and the ability to walk away while the agent works.
type RalphWiggumAgent struct {
	*CodingBaseAgent
	prd           *PRD
	maxIterations int
	progress      *ProgressTracker
	learnings     []string // Accumulated learnings across iterations
}

// RalphWiggumConfig configures the Ralph Wiggum agent
type RalphWiggumConfig struct {
	Config        *config.Config
	Backend       llm.Backend
	JobManager    jobs.JobManager
	PRD           *PRD
	MaxIterations int // Default: 10
}

// NewRalphWiggumAgent creates a new Ralph Wiggum agent
func NewRalphWiggumAgent(cfg RalphWiggumConfig) *RalphWiggumAgent {
	maxIter := cfg.MaxIterations
	if maxIter <= 0 {
		maxIter = 10
	}

	base := NewCodingBaseAgent(
		"ralph_wiggum",
		fmt.Sprintf("Ralph Wiggum iterative agent (%s mode) — works through PRD stories autonomously", cfg.PRD.Mode),
		cfg.Config,
		cfg.Backend,
		cfg.JobManager,
	)

	// Build progress tracker from PRD stories
	progress := NewProgressTracker()
	for _, story := range cfg.PRD.UserStories {
		progress.AddPhase(story.ID)
		if story.Passes {
			progress.UpdatePhase(story.ID, PhaseStatusDone, "previously completed")
		}
	}

	return &RalphWiggumAgent{
		CodingBaseAgent: base,
		prd:             cfg.PRD,
		maxIterations:   maxIter,
		progress:        progress,
		learnings:       []string{},
	}
}

// Execute runs the Ralph Wiggum iteration loop
func (r *RalphWiggumAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	// Create job
	description := fmt.Sprintf("Ralph Wiggum [%s] — %s (%d stories)",
		r.prd.Mode, r.prd.ProjectName, len(r.prd.UserStories))

	job, err := r.jobManager.Create(ctx, "ralph_wiggum", description, input)
	if err != nil {
		return nil, err
	}

	r.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	go func() {
		bgCtx := context.Background()
		result, err := r.executeLoop(bgCtx, job.ID)
		if err != nil {
			r.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, result, err)
			return
		}
		r.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, result, nil)
	}()

	return job, nil
}

// ExecuteSync runs the Ralph Wiggum loop synchronously (for CLI usage)
func (r *RalphWiggumAgent) ExecuteSync(ctx context.Context) error {
	fmt.Println()
	fmt.Println("============================================")
	fmt.Println("  Ralph Wiggum — Autonomous Iteration Loop")
	fmt.Println("============================================")
	fmt.Printf("  Mode:           %s\n", r.prd.Mode)
	fmt.Printf("  Project:        %s\n", r.prd.ProjectName)
	fmt.Printf("  Max iterations: %d\n", r.maxIterations)
	fmt.Printf("  Stories:        %d (%d complete)\n", len(r.prd.UserStories), r.prd.CompletedCount())
	fmt.Println("============================================")
	fmt.Println()
	r.progress.PrintTree()

	_, err := r.executeLoop(ctx, "cli-sync")
	if err != nil {
		return err
	}

	return nil
}

// executeLoop is the core iteration loop
func (r *RalphWiggumAgent) executeLoop(ctx context.Context, jobID string) (map[string]interface{}, error) {
	startTime := time.Now()
	var completedThisRun []string

	for iteration := 1; iteration <= r.maxIterations; iteration++ {
		// Check if all stories are complete
		if r.prd.AllComplete() {
			fmt.Fprintf(os.Stderr, "\n✅ All stories complete! Ralph did it!\n")
			break
		}

		// Get next story
		story := r.prd.NextStory()
		if story == nil {
			break
		}

		fmt.Fprintf(os.Stderr, "\n--------------------------------------------\n")
		fmt.Fprintf(os.Stderr, "  Iteration %d/%d\n", iteration, r.maxIterations)
		fmt.Fprintf(os.Stderr, "  Story: [%s] %s\n", story.ID, story.Title)
		fmt.Fprintf(os.Stderr, "  Remaining: %d stories\n", len(r.prd.IncompleteStories()))
		fmt.Fprintf(os.Stderr, "--------------------------------------------\n\n")

		r.progress.UpdatePhase(story.ID, PhaseStatusInProgress, fmt.Sprintf("iteration %d", iteration))
		r.progress.PrintTree()

		// Create context manager for this iteration
		contextMgr, err := llmcontext.NewManager(
			fmt.Sprintf("ralph-%s-iter%d-%s", r.prd.Mode, iteration, story.ID),
			r.config.Debug.Enabled,
			r.config.Model.ContextSize,
		)
		if err != nil {
			r.progress.UpdatePhase(story.ID, PhaseStatusFailed, err.Error())
			return nil, fmt.Errorf("failed to create context manager: %w", err)
		}

		// Build the prompt for this story
		prompt := r.buildStoryPrompt(story, iteration)
		systemPrompt := r.buildSystemPrompt()

		// Execute inference loop for this story
		executor := NewInferenceExecutor(r.BaseAgent, contextMgr)
		executor.SetSystemPrompt(systemPrompt)

		executor.SetProgressCallback(func(event ProgressEvent) {
			if event.Type == ProgressEventToolCall {
				r.progress.IncrementToolUse(story.ID)
			}
		})

		iterStart := time.Now()
		err = executor.Execute(ctx, prompt)
		iterDuration := time.Since(iterStart)

		contextMgr.Cleanup()

		// Check if story was completed (look for STORY_COMPLETE in response)
		storyCompleted := err == nil
		if storyCompleted {
			r.prd.MarkStoryComplete(story.ID)
			r.progress.UpdatePhase(story.ID, PhaseStatusDone, fmt.Sprintf("completed in %s", iterDuration.Round(time.Second)))
			completedThisRun = append(completedThisRun, story.ID)
			fmt.Fprintf(os.Stderr, "  ✅ Story %s: COMPLETED\n", story.ID)
		} else {
			r.progress.UpdatePhase(story.ID, PhaseStatusFailed, fmt.Sprintf("iteration %d failed: %v", iteration, err))
			fmt.Fprintf(os.Stderr, "  ❌ Story %s: NOT COMPLETE (will retry if iterations remain)\n", story.ID)
			// Reset to pending so it can be retried
			r.progress.UpdatePhase(story.ID, PhaseStatusPending, "awaiting retry")
		}

		// Record learnings
		learning := fmt.Sprintf("--- Iteration %d: %s (%s) ---\n[Duration: %s]\n[Status: %s]",
			iteration, story.ID, time.Now().Format(time.RFC3339),
			iterDuration.Round(time.Second),
			map[bool]string{true: "Complete", false: "Incomplete"}[storyCompleted])
		r.learnings = append(r.learnings, learning)

		r.progress.PrintTree()
	}

	// Build final result
	totalDuration := time.Since(startTime)
	result := map[string]interface{}{
		"status":              "completed",
		"workflow_type":       "ralph_wiggum",
		"mode":                string(r.prd.Mode),
		"project":             r.prd.ProjectName,
		"total_stories":       len(r.prd.UserStories),
		"completed_stories":   r.prd.CompletedCount(),
		"remaining_stories":   len(r.prd.IncompleteStories()),
		"completed_this_run":  completedThisRun,
		"total_duration":      totalDuration.String(),
		"all_complete":        r.prd.AllComplete(),
		"learnings":           r.learnings,
	}

	// Print summary
	r.printSummary(completedThisRun, totalDuration)

	if !r.prd.AllComplete() {
		return result, fmt.Errorf("not all stories completed (%d/%d done)", r.prd.CompletedCount(), len(r.prd.UserStories))
	}

	return result, nil
}

// buildSystemPrompt assembles the full system prompt from base + mode prompts
func (r *RalphWiggumAgent) buildSystemPrompt() string {
	var sb strings.Builder

	sb.WriteString(ralphBasePrompt)
	sb.WriteString("\n\n---\n\n")

	switch r.prd.Mode {
	case PRDModeCode:
		sb.WriteString(ralphCodePrompt)
	case PRDModeBlog:
		sb.WriteString(ralphBlogPrompt)
	case PRDModePodcast:
		sb.WriteString(ralphPodcastPrompt)
	}

	// Add tool instructions (reuse dynamic prompt if available)
	if r.toolPromptGen != nil {
		sb.WriteString("\n\n# Available Tools\n\n")
		sb.WriteString(r.toolPromptGen.GenerateToolSection())
		sb.WriteString("\n\nFormat: {\"tool\": \"name\", \"args\": {...}}")
	}

	return sb.String()
}

// buildStoryPrompt builds the user prompt for a specific story iteration
func (r *RalphWiggumAgent) buildStoryPrompt(story *UserStory, iteration int) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Project: %s\n\n", r.prd.ProjectName))

	if r.prd.OutputFile != "" {
		sb.WriteString(fmt.Sprintf("## Output File: %s\n\n", r.prd.OutputFile))
	}

	sb.WriteString(fmt.Sprintf("## Current Story (Iteration %d)\n\n", iteration))
	sb.WriteString(fmt.Sprintf("**ID:** %s\n", story.ID))
	sb.WriteString(fmt.Sprintf("**Title:** %s\n", story.Title))
	sb.WriteString(fmt.Sprintf("**Description:** %s\n\n", story.Description))

	if len(story.AcceptanceCriteria) > 0 {
		sb.WriteString("**Acceptance Criteria:**\n")
		for _, ac := range story.AcceptanceCriteria {
			sb.WriteString(fmt.Sprintf("- %s\n", ac))
		}
		sb.WriteString("\n")
	}

	// All stories status
	sb.WriteString("## All Stories Status\n\n")
	sb.WriteString(r.prd.StatusSummary())
	sb.WriteString("\n")

	// Progress from previous iterations
	if len(r.learnings) > 0 {
		sb.WriteString("## Progress from Previous Iterations\n\n")
		// Include last 10 learnings (most recent context)
		start := 0
		if len(r.learnings) > 10 {
			start = len(r.learnings) - 10
		}
		for _, l := range r.learnings[start:] {
			sb.WriteString(l)
			sb.WriteString("\n\n")
		}
	}

	// Instructions
	sb.WriteString(fmt.Sprintf("## Instructions\n\n"))
	sb.WriteString(fmt.Sprintf("Work on story **%s** now. When complete:\n", story.ID))
	sb.WriteString(fmt.Sprintf("1. Run quality checks appropriate for %s mode\n", r.prd.Mode))

	switch r.prd.Mode {
	case PRDModeCode:
		sb.WriteString(fmt.Sprintf("2. Commit your changes with message: feat(%s): <description>\n", story.ID))
	case PRDModeBlog:
		sb.WriteString("2. Save your work to the output file\n")
	case PRDModePodcast:
		sb.WriteString("2. Save your work to the output file\n")
	}

	sb.WriteString("3. End with: STORY_COMPLETE\n")

	return sb.String()
}

// printSummary prints a final summary of the run
func (r *RalphWiggumAgent) printSummary(completedThisRun []string, duration time.Duration) {
	fmt.Fprintf(os.Stderr, "\n============================================\n")
	fmt.Fprintf(os.Stderr, "  Ralph Wiggum — Final Summary\n")
	fmt.Fprintf(os.Stderr, "============================================\n")
	fmt.Fprintf(os.Stderr, "  Mode:       %s\n", r.prd.Mode)
	fmt.Fprintf(os.Stderr, "  Duration:   %s\n", duration.Round(time.Second))
	fmt.Fprintf(os.Stderr, "  Total:      %d stories\n", len(r.prd.UserStories))
	fmt.Fprintf(os.Stderr, "  Completed:  %d\n", r.prd.CompletedCount())
	fmt.Fprintf(os.Stderr, "  Remaining:  %d\n", len(r.prd.IncompleteStories()))
	fmt.Fprintf(os.Stderr, "\n")

	if len(completedThisRun) > 0 {
		fmt.Fprintf(os.Stderr, "  Completed this run:\n")
		for _, id := range completedThisRun {
			fmt.Fprintf(os.Stderr, "    [DONE] %s\n", id)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	incomplete := r.prd.IncompleteStories()
	if len(incomplete) > 0 {
		fmt.Fprintf(os.Stderr, "  Still incomplete:\n")
		for _, s := range incomplete {
			fmt.Fprintf(os.Stderr, "    [TODO] %s: %s\n", s.ID, s.Title)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	if r.prd.AllComplete() {
		fmt.Fprintf(os.Stderr, "  All stories complete! Ralph did it!\n")
	} else {
		fmt.Fprintf(os.Stderr, "  Some stories remain. Run again to continue iterating.\n")
	}
	fmt.Fprintf(os.Stderr, "============================================\n")
}

// GetPRD returns the current PRD state
func (r *RalphWiggumAgent) GetPRD() *PRD {
	return r.prd
}

// GetProgress returns the progress tracker
func (r *RalphWiggumAgent) GetProgress() *ProgressTracker {
	return r.progress
}

// GetLearnings returns accumulated learnings
func (r *RalphWiggumAgent) GetLearnings() []string {
	return r.learnings
}

