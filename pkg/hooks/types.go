// Package hooks provides git hook management for local CI/CD validation.
// It enables pre-commit, pre-push, and commit-msg hooks to validate code
// before it leaves the developer's machine.
package hooks

import (
	"time"
)

// HookType represents the type of git hook
type HookType string

const (
	HookTypePreCommit HookType = "pre-commit"
	HookTypePrePush   HookType = "pre-push"
	HookTypeCommitMsg HookType = "commit-msg"
)

// ProjectType represents the type of project based on its primary language
type ProjectType string

const (
	ProjectTypeGo      ProjectType = "go"
	ProjectTypeNode    ProjectType = "node"
	ProjectTypePython  ProjectType = "python"
	ProjectTypeRust    ProjectType = "rust"
	ProjectTypeJava    ProjectType = "java"
	ProjectTypeRuby    ProjectType = "ruby"
	ProjectTypePHP     ProjectType = "php"
	ProjectTypeDotnet  ProjectType = "dotnet"
	ProjectTypeUnknown ProjectType = "unknown"
)

// Check represents a single validation check to run
type Check struct {
	Name         string        `json:"name"`
	Command      string        `json:"command"`
	Args         []string      `json:"args,omitempty"`
	Required     bool          `json:"required"`      // Fail hook if this fails
	FailOnOutput bool          `json:"fail_on_output"` // Fail if command produces output (e.g., gofmt -l)
	Timeout      time.Duration `json:"timeout,omitempty"`
	Optional     bool          `json:"optional"` // Don't fail if command doesn't exist
}

// CommitMsgConfig configures commit message validation
type CommitMsgConfig struct {
	// Pattern is a regex pattern the commit message must match
	Pattern string `json:"pattern,omitempty"`
	// MinLength is the minimum length of the commit message
	MinLength int `json:"min_length,omitempty"`
	// MaxLength is the maximum length of the first line
	MaxLength int `json:"max_length,omitempty"`
	// RequireConventional requires conventional commit format
	RequireConventional bool `json:"require_conventional,omitempty"`
}

// HooksConfig contains the full hooks configuration for a repository
type HooksConfig struct {
	ProjectType  ProjectType      `json:"project_type"`
	PreCommit    []Check          `json:"pre_commit,omitempty"`
	PrePush      []Check          `json:"pre_push,omitempty"`
	CommitMsg    *CommitMsgConfig `json:"commit_msg,omitempty"`
	CustomChecks []Check          `json:"custom_checks,omitempty"`

	// Timeouts
	PreCommitTimeout time.Duration `json:"pre_commit_timeout,omitempty"`
	PrePushTimeout   time.Duration `json:"pre_push_timeout,omitempty"`

	// Source of the configuration
	Source string `json:"source,omitempty"` // auto, manual, ci_parsed
}

// HookResult represents the result of running a hook
type HookResult struct {
	HookName  string        `json:"hook_name"`
	CheckName string        `json:"check_name,omitempty"`
	Passed    bool          `json:"passed"`
	Output    string        `json:"output"`
	ErrorMsg  string        `json:"error_msg,omitempty"`
	Duration  time.Duration `json:"duration"`
	Skipped   bool          `json:"skipped,omitempty"`
	SkipReason string       `json:"skip_reason,omitempty"`
}

// ValidationResult represents the result of running all pre-push validation
type ValidationResult struct {
	AllPassed bool          `json:"all_passed"`
	Results   []HookResult  `json:"results"`
	Summary   string        `json:"summary"` // Human readable summary for agent
	Duration  time.Duration `json:"duration"`
}

// HookRun represents a recorded hook execution
type HookRun struct {
	ID            string                 `json:"id"`
	RepoID        string                 `json:"repo_id"`
	HookType      HookType               `json:"hook_type"`
	TriggeredBy   string                 `json:"triggered_by"` // commit, push, manual, agent
	Passed        bool                   `json:"passed"`
	Results       []HookResult           `json:"results"`
	AgentFeedback string                 `json:"agent_feedback,omitempty"`
	DurationMs    int64                  `json:"duration_ms"`
	CreatedAt     time.Time              `json:"created_at"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// AgentFeedback provides structured feedback for the AI agent
type AgentFeedback struct {
	Success       bool           `json:"success"`
	FailedCheck   string         `json:"failed_check,omitempty"`
	ErrorOutput   string         `json:"error_output,omitempty"`
	Suggestion    string         `json:"suggestion,omitempty"`
	FilesAffected []string       `json:"files_affected,omitempty"`
	AllResults    []CheckFeedback `json:"all_results,omitempty"`
}

// CheckFeedback provides feedback for a single check
type CheckFeedback struct {
	Name       string   `json:"name"`
	Passed     bool     `json:"passed"`
	Output     string   `json:"output,omitempty"`
	Suggestion string   `json:"suggestion,omitempty"`
	Files      []string `json:"files,omitempty"`
}

// CIConfig represents a parsed CI configuration
type CIConfig struct {
	Source      string    `json:"source"` // github_actions, gitlab_ci, circle_ci, etc.
	RawConfig   []byte    `json:"raw_config,omitempty"`
	ParsedSteps []CIStep  `json:"parsed_steps"`
}

// CIStep represents a step from a CI configuration
type CIStep struct {
	Name     string            `json:"name"`
	Commands []string          `json:"commands"`
	Env      map[string]string `json:"env,omitempty"`
	WorkDir  string            `json:"workdir,omitempty"`
}

// Manager provides hook installation and management
type Manager interface {
	// InstallHooks installs hooks for a repo based on detected project type
	InstallHooks(repoPath string) error

	// UninstallHooks removes hooks from a repository
	UninstallHooks(repoPath string) error

	// RunHook runs a specific hook manually
	RunHook(repoPath string, hookName HookType) (*HookResult, error)

	// ValidateBeforePush runs all pre-push checks without actually pushing
	ValidateBeforePush(repoPath string) (*ValidationResult, error)

	// GetHooksConfig gets hook configuration for a repo
	GetHooksConfig(repoPath string) (*HooksConfig, error)

	// SetHooksConfig sets hook configuration for a repo
	SetHooksConfig(repoPath string, config *HooksConfig) error

	// DetectProjectType detects the project type from the repository
	DetectProjectType(repoPath string) (ProjectType, error)

	// FormatAgentFeedback formats validation results for agent consumption
	FormatAgentFeedback(result *ValidationResult) *AgentFeedback
}

// CIConfigParser parses CI configurations from various systems
type CIConfigParser interface {
	// ParseCIConfig detects and parses CI config from a repo
	ParseCIConfig(repoPath string) (*CIConfig, error)

	// ConvertToHooks converts CI steps to local hooks
	ConvertToHooks(ciConfig *CIConfig) (*HooksConfig, error)

	// SupportsFormat checks if a CI format is supported
	SupportsFormat(format string) bool
}

// DefaultChecks returns the default checks for a project type
func DefaultChecks(projectType ProjectType) *HooksConfig {
	switch projectType {
	case ProjectTypeGo:
		return defaultGoChecks()
	case ProjectTypeNode:
		return defaultNodeChecks()
	case ProjectTypePython:
		return defaultPythonChecks()
	case ProjectTypeRust:
		return defaultRustChecks()
	default:
		return &HooksConfig{
			ProjectType: projectType,
			Source:      "auto",
		}
	}
}

func defaultGoChecks() *HooksConfig {
	return &HooksConfig{
		ProjectType: ProjectTypeGo,
		Source:      "auto",
		PreCommit: []Check{
			{
				Name:         "gofmt",
				Command:      "gofmt",
				Args:         []string{"-l", "."},
				Required:     true,
				FailOnOutput: true,
			},
			{
				Name:     "go_vet",
				Command:  "go",
				Args:     []string{"vet", "./..."},
				Required: true,
			},
		},
		PrePush: []Check{
			{
				Name:     "go_test",
				Command:  "go",
				Args:     []string{"test", "-race", "./..."},
				Required: true,
				Timeout:  5 * time.Minute,
			},
			{
				Name:     "go_build",
				Command:  "go",
				Args:     []string{"build", "./..."},
				Required: true,
			},
			{
				Name:     "golangci_lint",
				Command:  "golangci-lint",
				Args:     []string{"run"},
				Required: false,
				Optional: true,
			},
		},
		PreCommitTimeout: 30 * time.Second,
		PrePushTimeout:   5 * time.Minute,
	}
}

func defaultNodeChecks() *HooksConfig {
	return &HooksConfig{
		ProjectType: ProjectTypeNode,
		Source:      "auto",
		PreCommit: []Check{
			{
				Name:     "eslint",
				Command:  "npm",
				Args:     []string{"run", "lint"},
				Required: true,
				Optional: true,
			},
			{
				Name:         "prettier_check",
				Command:      "npx",
				Args:         []string{"prettier", "--check", "."},
				Required:     true,
				FailOnOutput: false,
				Optional:     true,
			},
		},
		PrePush: []Check{
			{
				Name:     "test",
				Command:  "npm",
				Args:     []string{"test"},
				Required: true,
				Timeout:  5 * time.Minute,
			},
			{
				Name:     "build",
				Command:  "npm",
				Args:     []string{"run", "build"},
				Required: true,
				Optional: true,
			},
		},
		PreCommitTimeout: 30 * time.Second,
		PrePushTimeout:   5 * time.Minute,
	}
}

func defaultPythonChecks() *HooksConfig {
	return &HooksConfig{
		ProjectType: ProjectTypePython,
		Source:      "auto",
		PreCommit: []Check{
			{
				Name:     "black_check",
				Command:  "black",
				Args:     []string{"--check", "."},
				Required: true,
				Optional: true,
			},
			{
				Name:     "ruff",
				Command:  "ruff",
				Args:     []string{"check", "."},
				Required: true,
				Optional: true,
			},
		},
		PrePush: []Check{
			{
				Name:     "pytest",
				Command:  "pytest",
				Args:     []string{},
				Required: true,
				Timeout:  5 * time.Minute,
			},
			{
				Name:     "mypy",
				Command:  "mypy",
				Args:     []string{"."},
				Required: false,
				Optional: true,
			},
		},
		PreCommitTimeout: 30 * time.Second,
		PrePushTimeout:   5 * time.Minute,
	}
}

func defaultRustChecks() *HooksConfig {
	return &HooksConfig{
		ProjectType: ProjectTypeRust,
		Source:      "auto",
		PreCommit: []Check{
			{
				Name:     "cargo_fmt_check",
				Command:  "cargo",
				Args:     []string{"fmt", "--", "--check"},
				Required: true,
			},
			{
				Name:     "cargo_clippy",
				Command:  "cargo",
				Args:     []string{"clippy", "--", "-D", "warnings"},
				Required: true,
			},
		},
		PrePush: []Check{
			{
				Name:     "cargo_test",
				Command:  "cargo",
				Args:     []string{"test"},
				Required: true,
				Timeout:  5 * time.Minute,
			},
			{
				Name:     "cargo_build",
				Command:  "cargo",
				Args:     []string{"build"},
				Required: true,
			},
		},
		PreCommitTimeout: 30 * time.Second,
		PrePushTimeout:   5 * time.Minute,
	}
}
