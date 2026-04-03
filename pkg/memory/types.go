package memory

import (
	"time"
)

type SessionStatus string

const (
	SessionStatusPending        SessionStatus = "pending"
	SessionStatusRunning        SessionStatus = "running"
	SessionStatusCompleted      SessionStatus = "completed"
	SessionStatusFailed         SessionStatus = "failed"
	SessionStatusPartialSuccess SessionStatus = "partial_success"
	SessionStatusCancelled      SessionStatus = "cancelled"
)

type SessionRecord struct {
	SessionID    string        `json:"session_id"`
	RepoID       string        `json:"repo_id"`
	StartedAt    time.Time     `json:"started_at"`
	EndedAt      time.Time     `json:"ended_at,omitempty"`
	Mode         string        `json:"mode"`
	Status       SessionStatus `json:"status"`
	SummaryID    string        `json:"summary_id,omitempty"`
	Artifacts    []string      `json:"artifacts,omitempty"`
	UserGoal     string        `json:"user_goal,omitempty"`
	FilesRead    []string      `json:"files_read,omitempty"`
	FilesChanged []string      `json:"files_changed,omitempty"`
	TestsRun     int           `json:"tests_run,omitempty"`
	Blockers     []string      `json:"blockers,omitempty"`
	Errors       []string      `json:"errors,omitempty"`
}

type FactType string

const (
	FactTypeRepoFact         FactType = "repo_fact"
	FactTypeArchitectureNote FactType = "architecture_notes"
	FactTypeToolHint         FactType = "tool_hints"
	FactTypeUserPreference   FactType = "user_preferences"
	FactTypeCodingConvention FactType = "coding_conventions"
	FactTypeKnownRisk        FactType = "known_risks"
	FactTypeFailureNote      FactType = "failure_note"
)

type FactConfidence string

const (
	FactConfidenceHigh   FactConfidence = "high"
	FactConfidenceMedium FactConfidence = "medium"
	FactConfidenceLow    FactConfidence = "low"
)

type FactScope string

const (
	FactScopeRepo     FactScope = "repo"
	FactScopeFile     FactScope = "file"
	FactScopeFunction FactScope = "function"
	FactScopeGlobal   FactScope = "global"
)

type MemoryFact struct {
	ID                string         `json:"id"`
	Type              FactType       `json:"type"`
	Scope             FactScope      `json:"scope"`
	Subject           string         `json:"subject"`
	Fact              string         `json:"fact"`
	Confidence        FactConfidence `json:"confidence"`
	EvidenceArtifacts []string       `json:"evidence_artifacts"`
	LastValidatedAt   time.Time      `json:"last_validated_at"`
	CreatedAt         time.Time      `json:"created_at"`
	Tags              []string       `json:"tags,omitempty"`
}

type TaskStatus string

const (
	TaskStatusOpen       TaskStatus = "open"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusBlocked    TaskStatus = "blocked"
	TaskStatusCompleted  TaskStatus = "completed"
)

type TaskPriority string

const (
	TaskPriorityHigh   TaskPriority = "high"
	TaskPriorityMedium TaskPriority = "medium"
	TaskPriorityLow    TaskPriority = "low"
)

type OpenTask struct {
	ID                string       `json:"id"`
	Title             string       `json:"title"`
	Scope             string       `json:"scope"`
	Status            TaskStatus   `json:"status"`
	Priority          TaskPriority `json:"priority"`
	DependsOn         []string     `json:"depends_on,omitempty"`
	EvidenceArtifacts []string     `json:"evidence_artifacts,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
	LastUpdatedAt     time.Time    `json:"last_updated_at"`
	Description       string       `json:"description,omitempty"`
}

type ResumePacket struct {
	RepoID           string    `json:"repo_id"`
	Branch           string    `json:"branch"`
	Goal             string    `json:"goal"`
	NextStep         string    `json:"next_step"`
	ChangedFiles     []string  `json:"changed_files"`
	Warnings         []string  `json:"warnings,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	SessionID        string    `json:"session_id,omitempty"`
	Validated        bool      `json:"validated"`
	ValidationErrors []string  `json:"validation_errors,omitempty"`
}

type SessionSummary struct {
	ID           string        `json:"id"`
	SessionID    string        `json:"session_id"`
	UserGoal     string        `json:"user_goal"`
	Summary      string        `json:"summary"`
	FilesRead    []string      `json:"files_read"`
	FilesChanged []string      `json:"files_changed"`
	TestsRun     int           `json:"tests_run"`
	Blockers     []string      `json:"blockers"`
	ResultStatus SessionStatus `json:"result_status"`
	CreatedAt    time.Time     `json:"created_at"`
}

type GitSnapshot struct {
	Branch         string   `json:"branch"`
	CommitHash     string   `json:"commit_hash"`
	Status         string   `json:"status"`
	StagedFiles    []string `json:"staged_files"`
	ModifiedFiles  []string `json:"modified_files"`
	UntrackedFiles []string `json:"untracked_files"`
}

type Artifact struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Content   string                 `json:"content"`
	SessionID string                 `json:"session_id"`
	CreatedAt time.Time              `json:"created_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type ConsolidationInput struct {
	Session    SessionRecord
	Artifacts  []Artifact
	GitState   GitSnapshot
	PriorFacts []MemoryFact
}

type ConsolidationResult struct {
	Summary           SessionSummary
	Facts             []MemoryFact
	OpenTasks         []OpenTask
	ResumePacket      ResumePacket
	PrunedArtifactIDs []string
}
