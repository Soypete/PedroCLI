package orchestration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
)

type DefaultQueryEngine struct {
	config     QueryEngineConfig
	intentRank intentClassifier
}

func NewDefaultQueryEngine(cfg QueryEngineConfig) *DefaultQueryEngine {
	return &DefaultQueryEngine{
		config: cfg,
		intentRank: intentClassifier{
			keywords: map[IntentType][]string{
				IntentBuild:           {"build", "implement", "add", "create", "new feature", "write code", "make"},
				IntentDebug:           {"debug", "fix", "bug", "error", "issue", "crash", "broken", "fails", "doesn't work"},
				IntentReview:          {"review", "review code", "pr review", "check", "look at", "audit"},
				IntentTriage:          {"triage", "diagnose", "investigate", "analyze", "find cause", "understand"},
				IntentPlan:            {"plan", "design", "architecture", "how to", "approach", "roadmap"},
				IntentChat:            {"explain", "what is", "how does", "tell me about", "describe"},
				IntentBlog:            {"blog", "post", "article", "write about", "draft"},
				IntentPodcast:         {"podcast", "episode", "outline", "script", "interview"},
				IntentTechnicalWriter: {"write", "document", "tutorial", "guide", "explain", "documentation", "technical article"},
			},
		},
	}
}

func (e *DefaultQueryEngine) Execute(ctx context.Context, req QueryRequest) (*QueryResult, error) {
	input := req.Input
	mode := req.Mode

	if mode == "" {
		mode = e.config.DefaultMode
	}

	intent := req.Intent
	if intent == "" {
		var err error
		intent, _, err = e.ClassifyIntent(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to classify intent: %w", err)
		}
	}

	now := time.Now()

	switch intent {
	case IntentBuild:
		return e.executeBuilder(ctx, input, mode, now)
	case IntentDebug:
		return e.executeDebugger(ctx, input, mode, now)
	case IntentReview:
		return e.executeReviewer(ctx, input, mode, now)
	case IntentTriage:
		return e.executeTriager(ctx, input, mode, now)
	case IntentBlog:
		return e.executeBlog(ctx, input, mode, now)
	case IntentPodcast:
		return e.executePodcast(ctx, input, mode, now)
	case IntentTechnicalWriter:
		return e.executeTechnicalWriter(ctx, input, mode, now)
	case IntentChat, IntentPlan:
		return &QueryResult{
			Success:    true,
			Output:     "Chat/Plan mode - returning to interactive REPL",
			Intent:     intent,
			Mode:       mode,
			Finished:   true,
			FinishedAt: &now,
		}, nil
	default:
		return &QueryResult{
			Success:    true,
			Output:     "Unknown intent - defaulting to build mode",
			Intent:     IntentBuild,
			Mode:       mode,
			Finished:   true,
			FinishedAt: &now,
		}, nil
	}
}

func (e *DefaultQueryEngine) ExecuteWithMode(ctx context.Context, input string, mode Mode) (*QueryResult, error) {
	req := QueryRequest{
		Input:     input,
		Mode:      mode,
		Workspace: e.config.WorkspaceDir,
	}
	return e.Execute(ctx, req)
}

func (e *DefaultQueryEngine) ClassifyIntent(ctx context.Context, input string) (IntentType, float64, error) {
	inputLower := strings.ToLower(input)
	return e.intentRank.classify(inputLower)
}

func (e *DefaultQueryEngine) GetSupportedIntents() []IntentType {
	return GetSupportedIntents()
}

func (e *DefaultQueryEngine) GetSupportedModes() []Mode {
	return GetSupportedModes()
}

func (e *DefaultQueryEngine) executeBuilder(ctx context.Context, input string, mode Mode, now time.Time) (*QueryResult, error) {
	agent := e.config.AgentFactory.NewBuilderAgent()
	job, err := agent.Execute(ctx, map[string]interface{}{
		"description": input,
		"workspace":   e.config.WorkspaceDir,
	})
	if err != nil {
		return &QueryResult{
			Success:    false,
			Error:      fmt.Sprintf("builder failed: %v", err),
			Intent:     IntentBuild,
			Mode:       mode,
			Finished:   true,
			FinishedAt: &now,
		}, nil
	}
	return &QueryResult{
		JobID:      job.ID,
		Success:    true,
		Output:     fmt.Sprintf("Build job %s started", job.ID),
		Intent:     IntentBuild,
		Mode:       mode,
		Finished:   false,
		FinishedAt: nil,
	}, nil
}

func (e *DefaultQueryEngine) executeDebugger(ctx context.Context, input string, mode Mode, now time.Time) (*QueryResult, error) {
	agent := e.config.AgentFactory.NewDebuggerAgent()
	job, err := agent.Execute(ctx, map[string]interface{}{
		"description": input,
		"workspace":   e.config.WorkspaceDir,
	})
	if err != nil {
		return &QueryResult{
			Success:    false,
			Error:      fmt.Sprintf("debugger failed: %v", err),
			Intent:     IntentDebug,
			Mode:       mode,
			Finished:   true,
			FinishedAt: &now,
		}, nil
	}
	return &QueryResult{
		JobID:      job.ID,
		Success:    true,
		Output:     fmt.Sprintf("Debug job %s started", job.ID),
		Intent:     IntentDebug,
		Mode:       mode,
		Finished:   false,
		FinishedAt: nil,
	}, nil
}

func (e *DefaultQueryEngine) executeReviewer(ctx context.Context, input string, mode Mode, now time.Time) (*QueryResult, error) {
	agent := e.config.AgentFactory.NewReviewerAgent()
	job, err := agent.Execute(ctx, map[string]interface{}{
		"description": input,
		"workspace":   e.config.WorkspaceDir,
	})
	if err != nil {
		return &QueryResult{
			Success:    false,
			Error:      fmt.Sprintf("reviewer failed: %v", err),
			Intent:     IntentReview,
			Mode:       mode,
			Finished:   true,
			FinishedAt: &now,
		}, nil
	}
	return &QueryResult{
		JobID:      job.ID,
		Success:    true,
		Output:     fmt.Sprintf("Review job %s started", job.ID),
		Intent:     IntentReview,
		Mode:       mode,
		Finished:   false,
		FinishedAt: nil,
	}, nil
}

func (e *DefaultQueryEngine) executeTriager(ctx context.Context, input string, mode Mode, now time.Time) (*QueryResult, error) {
	agent := e.config.AgentFactory.NewTriagerAgent()
	job, err := agent.Execute(ctx, map[string]interface{}{
		"description": input,
		"workspace":   e.config.WorkspaceDir,
	})
	if err != nil {
		return &QueryResult{
			Success:    false,
			Error:      fmt.Sprintf("triager failed: %v", err),
			Intent:     IntentTriage,
			Mode:       mode,
			Finished:   true,
			FinishedAt: &now,
		}, nil
	}
	return &QueryResult{
		JobID:      job.ID,
		Success:    true,
		Output:     fmt.Sprintf("Triage job %s started", job.ID),
		Intent:     IntentTriage,
		Mode:       mode,
		Finished:   false,
		FinishedAt: nil,
	}, nil
}

func (e *DefaultQueryEngine) executeBlog(ctx context.Context, input string, mode Mode, now time.Time) (*QueryResult, error) {
	agent := e.config.AgentFactory.NewDynamicBlogAgent()
	job, err := agent.Execute(ctx, map[string]interface{}{
		"content": input,
		"title":   "",
	})
	if err != nil {
		return &QueryResult{
			Success:    false,
			Error:      fmt.Sprintf("blog agent failed: %v", err),
			Intent:     IntentBlog,
			Mode:       mode,
			Finished:   true,
			FinishedAt: &now,
		}, nil
	}
	return &QueryResult{
		JobID:      job.ID,
		Success:    true,
		Output:     fmt.Sprintf("Blog job %s started", job.ID),
		Intent:     IntentBlog,
		Mode:       mode,
		Finished:   false,
		FinishedAt: nil,
	}, nil
}

func (e *DefaultQueryEngine) executePodcast(ctx context.Context, input string, mode Mode, now time.Time) (*QueryResult, error) {
	return &QueryResult{
		Success:    true,
		Output:     "Podcast mode - use /podcast-outline or /podcast-script commands",
		Intent:     IntentPodcast,
		Mode:       mode,
		Finished:   true,
		FinishedAt: &now,
	}, nil
}

func (e *DefaultQueryEngine) executeTechnicalWriter(ctx context.Context, input string, mode Mode, now time.Time) (*QueryResult, error) {
	agent := e.config.AgentFactory.NewTechnicalWriterAgent()
	job, err := agent.Execute(ctx, map[string]interface{}{
		"content": input,
		"title":   "",
		"type":    "technical",
	})
	if err != nil {
		return &QueryResult{
			Success:    false,
			Error:      fmt.Sprintf("technical writer failed: %v", err),
			Intent:     IntentTechnicalWriter,
			Mode:       mode,
			Finished:   true,
			FinishedAt: &now,
		}, nil
	}
	return &QueryResult{
		JobID:      job.ID,
		Success:    true,
		Output:     fmt.Sprintf("Technical writer job %s started", job.ID),
		Intent:     IntentTechnicalWriter,
		Mode:       mode,
		Finished:   false,
		FinishedAt: nil,
	}, nil
}

type intentClassifier struct {
	keywords map[IntentType][]string
}

func (c *intentClassifier) classify(input string) (IntentType, float64, error) {
	bestIntent := IntentUnknown
	bestScore := 0.0

	for intent, keywords := range c.keywords {
		score := 0.0
		for _, kw := range keywords {
			if strings.Contains(input, kw) {
				score += 1.0
			}
		}
		normalizedScore := score / float64(len(keywords))
		if normalizedScore > bestScore {
			bestScore = normalizedScore
			bestIntent = intent
		}
	}

	if bestScore == 0 {
		return IntentUnknown, 0.0, nil
	}

	return bestIntent, bestScore, nil
}

func NewAppContextAgentFactory(appCtx interface {
	NewBuilderAgent() *agents.BuilderPhasedAgent
	NewDebuggerAgent() *agents.DebuggerPhasedAgent
	NewReviewerAgent() *agents.ReviewerPhasedAgent
	NewTriagerAgent() *agents.TriagerAgent
	NewDynamicBlogAgent() *agents.DynamicBlogAgent
}, cfg *config.Config, backend llm.Backend, jobManager jobs.JobManager) AgentFactory {
	return &concreteAgentFactory{
		appCtx:     appCtx,
		cfg:        cfg,
		backend:    backend,
		jobManager: jobManager,
	}
}

type concreteAgentFactory struct {
	appCtx interface {
		NewBuilderAgent() *agents.BuilderPhasedAgent
		NewDebuggerAgent() *agents.DebuggerPhasedAgent
		NewReviewerAgent() *agents.ReviewerPhasedAgent
		NewTriagerAgent() *agents.TriagerAgent
		NewDynamicBlogAgent() *agents.DynamicBlogAgent
	}
	cfg        *config.Config
	backend    llm.Backend
	jobManager jobs.JobManager
}

func (f *concreteAgentFactory) NewBuilderAgent() AgentExecutor {
	return f.appCtx.NewBuilderAgent()
}

func (f *concreteAgentFactory) NewDebuggerAgent() AgentExecutor {
	return f.appCtx.NewDebuggerAgent()
}

func (f *concreteAgentFactory) NewReviewerAgent() AgentExecutor {
	return f.appCtx.NewReviewerAgent()
}

func (f *concreteAgentFactory) NewTriagerAgent() AgentExecutor {
	return f.appCtx.NewTriagerAgent()
}

func (f *concreteAgentFactory) NewDynamicBlogAgent() AgentExecutor {
	return f.appCtx.NewDynamicBlogAgent()
}

func (f *concreteAgentFactory) NewTechnicalWriterAgent() AgentExecutor {
	return agents.NewTechnicalWriterAgent(f.cfg, f.backend, f.jobManager)
}
