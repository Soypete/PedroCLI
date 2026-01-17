// Package evals provides a comprehensive evaluation system for Pedro CLI agents.
// It supports testing coding, blog post, and podcast agents with various grading strategies.
package evals

import (
	"time"
)

// AgentType represents the type of agent being evaluated.
type AgentType string

const (
	AgentTypeCoding  AgentType = "coding"
	AgentTypeBlog    AgentType = "blog"
	AgentTypePodcast AgentType = "podcast"
)

// GraderType represents the type of grader to use for evaluation.
type GraderType string

const (
	GraderTypeStringMatch  GraderType = "string_match"
	GraderTypeRegex        GraderType = "regex"
	GraderTypeJSONSchema   GraderType = "json_schema"
	GraderTypeLLMRubric    GraderType = "llm_rubric"
	GraderTypeMarkdownLint GraderType = "markdown_lint"
	GraderTypeReadability  GraderType = "readability"
	GraderTypeCodeExec     GraderType = "code_exec"
	GraderTypeToolCalls    GraderType = "tool_calls"
)

// MatchType specifies how string matching should be performed.
type MatchType string

const (
	MatchTypeExact    MatchType = "exact"
	MatchTypeContains MatchType = "contains"
	MatchTypePrefix   MatchType = "prefix"
	MatchTypeSuffix   MatchType = "suffix"
)

// Task represents a single evaluation task with inputs and grading criteria.
type Task struct {
	ID          string                 `yaml:"id" json:"id"`
	Description string                 `yaml:"description" json:"description"`
	AgentType   AgentType              `yaml:"agent_type" json:"agent_type"`
	Input       TaskInput              `yaml:"input" json:"input"`
	Graders     []GraderConfig         `yaml:"graders" json:"graders"`
	Metadata    map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Tags        []string               `yaml:"tags,omitempty" json:"tags,omitempty"`
	Weight      float64                `yaml:"weight,omitempty" json:"weight,omitempty"` // For weighted scoring
}

// TaskInput contains the input for a task.
type TaskInput struct {
	Prompt      string                 `yaml:"prompt" json:"prompt"`
	Context     map[string]interface{} `yaml:"context,omitempty" json:"context,omitempty"`
	Files       map[string]string      `yaml:"files,omitempty" json:"files,omitempty"`               // filename -> content
	SystemHints []string               `yaml:"system_hints,omitempty" json:"system_hints,omitempty"` // Additional system-level hints
}

// GraderConfig configures how to grade a task's output.
type GraderConfig struct {
	Type       GraderType             `yaml:"type" json:"type"`
	Required   bool                   `yaml:"required,omitempty" json:"required,omitempty"` // If true, must pass for task to pass
	Weight     float64                `yaml:"weight,omitempty" json:"weight,omitempty"`     // Weight in composite score
	Config     map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
	Assertions []string               `yaml:"assertions,omitempty" json:"assertions,omitempty"` // For regex/string graders
}

// Trial represents a single attempt at a task.
type Trial struct {
	ID           string         `json:"id"`
	TaskID       string         `json:"task_id"`
	TrialNumber  int            `json:"trial_number"`
	StartedAt    time.Time      `json:"started_at"`
	CompletedAt  time.Time      `json:"completed_at"`
	Outcome      *Outcome       `json:"outcome"`
	Transcript   *Transcript    `json:"transcript"`
	Metrics      *TrialMetrics  `json:"metrics"`
	GradeResults []*GradeResult `json:"grade_results"`
	Passed       bool           `json:"passed"`
	Score        float64        `json:"score"` // 0.0 to 1.0
	Error        string         `json:"error,omitempty"`
}

// Outcome represents the final state after a trial.
type Outcome struct {
	FinalOutput   string                 `json:"final_output"`
	ToolCallsUsed []ToolCallRecord       `json:"tool_calls_used,omitempty"`
	FilesModified []string               `json:"files_modified,omitempty"`
	ExitReason    string                 `json:"exit_reason"` // "completed", "max_turns", "error", "timeout"
	Data          map[string]interface{} `json:"data,omitempty"`
}

// ToolCallRecord records a single tool call during a trial.
type ToolCallRecord struct {
	Tool      string                 `json:"tool"`
	Args      map[string]interface{} `json:"args"`
	Result    string                 `json:"result"`
	Success   bool                   `json:"success"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration"`
}

// Transcript contains the complete record of a trial.
type Transcript struct {
	Turns []Turn `json:"turns"`
}

// Turn represents a single conversation turn.
type Turn struct {
	Role       string           `json:"role"` // "user", "assistant", "system", "tool"
	Content    string           `json:"content"`
	ToolCalls  []ToolCallRecord `json:"tool_calls,omitempty"`
	TokensUsed int              `json:"tokens_used"`
	Timestamp  time.Time        `json:"timestamp"`
}

// TrialMetrics contains performance metrics for a trial.
type TrialMetrics struct {
	NTurns            int           `json:"n_turns"`
	NToolCalls        int           `json:"n_tool_calls"`
	NTotalTokens      int           `json:"n_total_tokens"`
	NPromptTokens     int           `json:"n_prompt_tokens"`
	NCompletionTokens int           `json:"n_completion_tokens"`
	TimeToFirstToken  time.Duration `json:"time_to_first_token"`
	TotalLatency      time.Duration `json:"total_latency"`
	TokensPerSecond   float64       `json:"tokens_per_second"`
}

// GradeResult contains the result of a single grader.
type GradeResult struct {
	GraderType GraderType             `json:"grader_type"`
	Passed     bool                   `json:"passed"`
	Score      float64                `json:"score"` // 0.0 to 1.0
	Feedback   string                 `json:"feedback"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

// Suite represents a collection of evaluation tasks.
type Suite struct {
	Name        string                 `yaml:"name" json:"name"`
	Description string                 `yaml:"description" json:"description"`
	AgentType   AgentType              `yaml:"agent_type" json:"agent_type"`
	Version     string                 `yaml:"version" json:"version"`
	Tasks       []Task                 `yaml:"tasks" json:"tasks"`
	Metadata    map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// EvalConfig configures an evaluation run.
type EvalConfig struct {
	Provider          string  `yaml:"provider" json:"provider"`                       // "ollama" or "llama_cpp"
	Endpoint          string  `yaml:"endpoint" json:"endpoint"`                       // Server URL
	Model             string  `yaml:"model" json:"model"`                             // Model to evaluate
	LLMGraderModel    string  `yaml:"llm_grader_model" json:"llm_grader_model"`       // Model for LLM-based grading
	LLMGraderProvider string  `yaml:"llm_grader_provider" json:"llm_grader_provider"` // Provider for grader model
	LLMGraderEndpoint string  `yaml:"llm_grader_endpoint" json:"llm_grader_endpoint"` // Endpoint for grader model
	OutputDir         string  `yaml:"output_dir" json:"output_dir"`                   // Results directory
	SaveTranscripts   bool    `yaml:"save_transcripts" json:"save_transcripts"`       // Save full transcripts
	Concurrency       int     `yaml:"concurrency" json:"concurrency"`                 // Parallel trials
	TrialsPerTask     int     `yaml:"trials_per_task" json:"trials_per_task"`         // Trials per task
	Temperature       float64 `yaml:"temperature" json:"temperature"`                 // Model temperature
	MaxTokens         int     `yaml:"max_tokens" json:"max_tokens"`                   // Max tokens per response
	Timeout           int     `yaml:"timeout" json:"timeout"`                         // Timeout in seconds per trial
}

// EvalRun represents a complete evaluation run with results.
type EvalRun struct {
	ID          string      `json:"id"`
	StartedAt   time.Time   `json:"started_at"`
	CompletedAt time.Time   `json:"completed_at"`
	Config      *EvalConfig `json:"config"`
	Suite       *Suite      `json:"suite"`
	Trials      []*Trial    `json:"trials"`
	Summary     *RunSummary `json:"summary"`
}

// RunSummary contains aggregate statistics for an evaluation run.
type RunSummary struct {
	TotalTasks      int                        `json:"total_tasks"`
	TotalTrials     int                        `json:"total_trials"`
	PassedTrials    int                        `json:"passed_trials"`
	FailedTrials    int                        `json:"failed_trials"`
	ErrorTrials     int                        `json:"error_trials"`
	OverallPassRate float64                    `json:"overall_pass_rate"`
	PassAtK         map[int]float64            `json:"pass_at_k"`    // pass@1, pass@3, pass@5, etc.
	PassPowerK      map[int]float64            `json:"pass_power_k"` // pass^1, pass^3, pass^5, etc.
	AvgScore        float64                    `json:"avg_score"`
	AvgTokensUsed   float64                    `json:"avg_tokens_used"`
	AvgLatency      time.Duration              `json:"avg_latency"`
	AvgTurns        float64                    `json:"avg_turns"`
	AvgToolCalls    float64                    `json:"avg_tool_calls"`
	ByGraderType    map[GraderType]GraderStats `json:"by_grader_type"`
	ByTag           map[string]TagStats        `json:"by_tag,omitempty"`
}

// GraderStats contains statistics for a specific grader type.
type GraderStats struct {
	TotalRuns int     `json:"total_runs"`
	Passed    int     `json:"passed"`
	Failed    int     `json:"failed"`
	PassRate  float64 `json:"pass_rate"`
	AvgScore  float64 `json:"avg_score"`
}

// TagStats contains statistics for tasks with a specific tag.
type TagStats struct {
	TotalTasks  int     `json:"total_tasks"`
	TotalTrials int     `json:"total_trials"`
	Passed      int     `json:"passed"`
	PassRate    float64 `json:"pass_rate"`
	AvgScore    float64 `json:"avg_score"`
}

// ComparisonResult contains results from comparing two models.
type ComparisonResult struct {
	Model1        string            `json:"model1"`
	Model2        string            `json:"model2"`
	Run1          *EvalRun          `json:"run1"`
	Run2          *EvalRun          `json:"run2"`
	WinnerByTask  map[string]string `json:"winner_by_task"` // task_id -> model name
	Model1Wins    int               `json:"model1_wins"`
	Model2Wins    int               `json:"model2_wins"`
	Ties          int               `json:"ties"`
	SignificanceP float64           `json:"significance_p"` // Statistical significance
}
