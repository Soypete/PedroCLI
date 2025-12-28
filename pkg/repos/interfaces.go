package repos

import (
	"context"
	"time"
)

// Manager provides repository storage management with GOPATH-style pathing
type Manager interface {
	// EnsureRepo clones or fetches a repository and returns its local info
	EnsureRepo(ctx context.Context, provider, owner, repo string) (*LocalRepo, error)

	// GetRepoPath returns the local path for a repository without fetching
	GetRepoPath(provider, owner, repo string) string

	// FreshClone removes existing repo and does a fresh clone
	FreshClone(ctx context.Context, provider, owner, repo string) (*LocalRepo, error)

	// ListRepos returns all managed repositories
	ListRepos(ctx context.Context) ([]LocalRepo, error)

	// GetRepo returns a specific repo by provider/owner/name
	GetRepo(ctx context.Context, provider, owner, repo string) (*LocalRepo, error)

	// RemoveRepo removes a repository from local storage
	RemoveRepo(ctx context.Context, provider, owner, repo string) error

	// UpdateRepoMeta updates repository metadata (project type, etc.)
	UpdateRepoMeta(ctx context.Context, repo *LocalRepo) error

	// SetBasePath changes the base storage path (for testing)
	SetBasePath(path string)

	// GetBasePath returns the current base storage path
	GetBasePath() string
}

// GitOps provides pure git operations on a repository
type GitOps interface {
	// Clone clones a repository to the specified path
	Clone(ctx context.Context, url, path string) error

	// CloneWithDepth clones with a specific depth (shallow clone)
	CloneWithDepth(ctx context.Context, url, path string, depth int) error

	// Fetch fetches all refs from remotes
	Fetch(ctx context.Context, path string) error

	// FetchRemote fetches from a specific remote
	FetchRemote(ctx context.Context, path, remote string) error

	// Checkout checks out a ref (branch, tag, or commit)
	Checkout(ctx context.Context, path, ref string) error

	// Pull pulls changes from the remote
	Pull(ctx context.Context, path string) error

	// PullRemote pulls from a specific remote/branch
	PullRemote(ctx context.Context, path, remote, branch string) error

	// CreateBranch creates a new branch
	CreateBranch(ctx context.Context, path, name string) error

	// CreateBranchFrom creates a new branch from a specific ref
	CreateBranchFrom(ctx context.Context, path, name, from string) error

	// DeleteBranch deletes a branch
	DeleteBranch(ctx context.Context, path, name string, force bool) error

	// ListBranches lists all branches
	ListBranches(ctx context.Context, path string) ([]Branch, error)

	// CurrentBranch returns the current branch name
	CurrentBranch(ctx context.Context, path string) (string, error)

	// Status returns the repository status
	Status(ctx context.Context, path string) (*RepoStatus, error)

	// Diff returns the diff output
	Diff(ctx context.Context, path string, staged bool) (string, error)

	// DiffBranches returns diff between two refs
	DiffBranches(ctx context.Context, path, base, head string) (string, error)

	// Add stages files for commit
	Add(ctx context.Context, path string, files ...string) error

	// AddAll stages all changes
	AddAll(ctx context.Context, path string) error

	// Commit creates a commit with the given message
	Commit(ctx context.Context, path, message string) error

	// CommitWithAuthor creates a commit with a specific author
	CommitWithAuthor(ctx context.Context, path, message, author, email string) error

	// Push pushes to the remote
	Push(ctx context.Context, path, remote, branch string) error

	// PushForce force pushes to the remote
	PushForce(ctx context.Context, path, remote, branch string) error

	// GetHeadHash returns the current HEAD commit hash
	GetHeadHash(ctx context.Context, path string) (string, error)

	// GetRemoteHash returns the hash of a remote branch
	GetRemoteHash(ctx context.Context, path, remote, branch string) (string, error)

	// IsUpToDate checks if the local branch is up to date with remote
	IsUpToDate(ctx context.Context, path string) (bool, error)

	// Log returns recent commits
	Log(ctx context.Context, path string, limit int) ([]Commit, error)

	// LogBranch returns commits on a specific branch
	LogBranch(ctx context.Context, path, branch string, limit int) ([]Commit, error)

	// GetDefaultBranch returns the default branch name
	GetDefaultBranch(ctx context.Context, path string) (string, error)

	// GetRemoteURL returns the URL for a remote
	GetRemoteURL(ctx context.Context, path, remote string) (string, error)

	// SetRemoteURL sets the URL for a remote
	SetRemoteURL(ctx context.Context, path, remote, url string) error

	// Stash stashes current changes
	Stash(ctx context.Context, path string) error

	// StashPop pops the most recent stash
	StashPop(ctx context.Context, path string) error

	// Clean removes untracked files
	Clean(ctx context.Context, path string, force, directories bool) error

	// Reset resets to a specific ref
	Reset(ctx context.Context, path, ref string, hard bool) error
}

// PRTracker tracks pull requests without relying on external CLIs
type PRTracker interface {
	// CreatePR creates a new pull request using the GitHub API
	CreatePR(ctx context.Context, repo *LocalRepo, title, body, head, base string, draft bool) (*TrackedPR, error)

	// GetPR retrieves a tracked PR by number
	GetPR(ctx context.Context, repoID string, prNumber int) (*TrackedPR, error)

	// UpdatePRStatus updates the status of a tracked PR
	UpdatePRStatus(ctx context.Context, pr *TrackedPR) error

	// ListPRs lists all tracked PRs for a repo
	ListPRs(ctx context.Context, repoID string) ([]TrackedPR, error)

	// SyncPRStatus syncs PR status from remote
	SyncPRStatus(ctx context.Context, repo *LocalRepo, prNumber int) (*TrackedPR, error)

	// IsMerged checks if a PR is merged by comparing commits
	IsMerged(ctx context.Context, repo *LocalRepo, pr *TrackedPR) (bool, error)
}

// Executor runs commands in a repository context
type Executor interface {
	// Exec runs a command in the repository directory
	Exec(ctx context.Context, repoPath, cmd string, args ...string) ([]byte, error)

	// ExecWithEnv runs a command with additional environment variables
	ExecWithEnv(ctx context.Context, repoPath string, env []string, cmd string, args ...string) ([]byte, error)

	// ExecWithTimeout runs a command with a specific timeout
	ExecWithTimeout(ctx context.Context, repoPath string, timeout time.Duration, cmd string, args ...string) ([]byte, error)

	// RunFormatter runs the appropriate formatter for the project type
	RunFormatter(ctx context.Context, repoPath, language string) error

	// RunLinter runs the appropriate linter for the project type
	RunLinter(ctx context.Context, repoPath, language string) ([]LintResult, error)

	// RunTests runs tests for the project
	RunTests(ctx context.Context, repoPath string) (*TestResult, error)
}

// LintResult represents a linting issue
type LintResult struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Message    string `json:"message"`
	Severity   string `json:"severity"` // error, warning, info
	RuleID     string `json:"rule_id,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// TestResult represents test execution results
type TestResult struct {
	Passed       bool          `json:"passed"`
	TotalTests   int           `json:"total_tests"`
	PassedTests  int           `json:"passed_tests"`
	FailedTests  int           `json:"failed_tests"`
	SkippedTests int           `json:"skipped_tests"`
	Duration     time.Duration `json:"duration"`
	Output       string        `json:"output"`
	Failures     []TestFailure `json:"failures,omitempty"`
}

// TestFailure represents a single test failure
type TestFailure struct {
	TestName string `json:"test_name"`
	Package  string `json:"package,omitempty"`
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
}

// Store provides persistent storage for repository metadata
type Store interface {
	// SaveRepo saves or updates a repository record
	SaveRepo(ctx context.Context, repo *LocalRepo) error

	// GetRepo retrieves a repository by provider/owner/name
	GetRepo(ctx context.Context, provider, owner, name string) (*LocalRepo, error)

	// GetRepoByID retrieves a repository by ID
	GetRepoByID(ctx context.Context, id string) (*LocalRepo, error)

	// ListRepos lists all repositories
	ListRepos(ctx context.Context) ([]LocalRepo, error)

	// DeleteRepo deletes a repository record
	DeleteRepo(ctx context.Context, id string) error

	// SaveOperation saves a repository operation record
	SaveOperation(ctx context.Context, op *RepoOperation) error

	// ListOperations lists operations for a repository
	ListOperations(ctx context.Context, repoID string, limit int) ([]RepoOperation, error)

	// SavePR saves or updates a tracked PR
	SavePR(ctx context.Context, pr *TrackedPR) error

	// GetPR retrieves a PR by repo ID and number
	GetPR(ctx context.Context, repoID string, prNumber int) (*TrackedPR, error)

	// ListPRs lists PRs for a repository
	ListPRs(ctx context.Context, repoID string) ([]TrackedPR, error)

	// SaveJob saves or updates a repo job
	SaveJob(ctx context.Context, job *RepoJob) error

	// GetJob retrieves a job by ID
	GetJob(ctx context.Context, id string) (*RepoJob, error)

	// ListJobs lists jobs for a repository
	ListJobs(ctx context.Context, repoID string) ([]RepoJob, error)

	// Close closes the store connection
	Close() error
}
