package orchestration

import (
	"context"
	"time"

	"github.com/soypete/pedrocli/pkg/jobs"
)

type IntentType string

const (
	IntentChat            IntentType = "chat"
	IntentPlan            IntentType = "plan"
	IntentBuild           IntentType = "build"
	IntentDebug           IntentType = "debug"
	IntentReview          IntentType = "review"
	IntentTriage          IntentType = "triage"
	IntentBlog            IntentType = "blog"
	IntentPodcast         IntentType = "podcast"
	IntentTechnicalWriter IntentType = "technical_writer"
	IntentUnknown         IntentType = "unknown"
)

type QueryRequest struct {
	Input     string
	Mode      Mode
	Intent    IntentType
	Workspace string
	Context   map[string]interface{}
}

type QueryResult struct {
	JobID      string
	Success    bool
	Output     string
	Error      string
	Intent     IntentType
	Mode       Mode
	Finished   bool
	FinishedAt *time.Time
}

type QueryEngine interface {
	Execute(ctx context.Context, req QueryRequest) (*QueryResult, error)
	ExecuteWithMode(ctx context.Context, input string, mode Mode) (*QueryResult, error)
	ClassifyIntent(ctx context.Context, input string) (IntentType, float64, error)
	GetSupportedIntents() []IntentType
	GetSupportedModes() []Mode
}

type AgentExecutor interface {
	Execute(ctx context.Context, args map[string]interface{}) (*jobs.Job, error)
}

type AgentFactory interface {
	NewBuilderAgent() AgentExecutor
	NewDebuggerAgent() AgentExecutor
	NewReviewerAgent() AgentExecutor
	NewTriagerAgent() AgentExecutor
	NewDynamicBlogAgent() AgentExecutor
	NewTechnicalWriterAgent() AgentExecutor
}

type QueryEngineConfig struct {
	AgentFactory     AgentFactory
	JobManager       *jobs.Manager
	WorkspaceDir     string
	DefaultMode      Mode
	EnableAutoIntent bool
}
