// Package repos provides repository management with GOPATH-style local storage.
// It enables persistent repo storage, git operations, and hook-based validation.
package repos

import (
	"time"
)

// LocalRepo represents a locally managed repository
type LocalRepo struct {
	ID            string    `json:"id"`
	Provider      string    `json:"provider"`    // e.g., "github.com", "gitlab.com"
	Owner         string    `json:"owner"`       // e.g., "soypete"
	Name          string    `json:"name"`        // e.g., "pedro-cli"
	LocalPath     string    `json:"local_path"`  // Full path on disk
	CurrentRef    string    `json:"current_ref"` // Current branch or commit
	DefaultBranch string    `json:"default_branch"`
	ProjectType   string    `json:"project_type"` // go, node, python, rust, etc.
	LastFetched   time.Time `json:"last_fetched"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// FullName returns the full repository path like "github.com/owner/repo"
func (r *LocalRepo) FullName() string {
	return r.Provider + "/" + r.Owner + "/" + r.Name
}

// CloneURL returns the git clone URL for the repository
func (r *LocalRepo) CloneURL() string {
	// Default to HTTPS, can be overridden by credentials config
	return "https://" + r.Provider + "/" + r.Owner + "/" + r.Name + ".git"
}

// SSHCloneURL returns the SSH clone URL for the repository
func (r *LocalRepo) SSHCloneURL() string {
	return "git@" + r.Provider + ":" + r.Owner + "/" + r.Name + ".git"
}

// RepoStatus represents the current status of a repository
type RepoStatus struct {
	IsClean        bool     `json:"is_clean"`
	CurrentBranch  string   `json:"current_branch"`
	HeadCommit     string   `json:"head_commit"`
	ModifiedFiles  []string `json:"modified_files,omitempty"`
	UntrackedFiles []string `json:"untracked_files,omitempty"`
	StagedFiles    []string `json:"staged_files,omitempty"`
	Ahead          int      `json:"ahead"`  // Commits ahead of remote
	Behind         int      `json:"behind"` // Commits behind remote
}

// Branch represents a git branch
type Branch struct {
	Name         string    `json:"name"`
	IsRemote     bool      `json:"is_remote"`
	IsCurrent    bool      `json:"is_current"`
	Commit       string    `json:"commit"`
	LastCommitAt time.Time `json:"last_commit_at,omitempty"`
}

// Commit represents a git commit
type Commit struct {
	Hash        string    `json:"hash"`
	ShortHash   string    `json:"short_hash"`
	Author      string    `json:"author"`
	AuthorEmail string    `json:"author_email"`
	Date        time.Time `json:"date"`
	Message     string    `json:"message"`
	Subject     string    `json:"subject"` // First line of message
}

// TrackedPR represents a pull request being tracked
type TrackedPR struct {
	ID               string     `json:"id"`
	RepoID           string     `json:"repo_id"`
	PRNumber         int        `json:"pr_number"`
	BranchName       string     `json:"branch_name"`
	BaseBranch       string     `json:"base_branch"`
	Title            string     `json:"title"`
	Body             string     `json:"body,omitempty"`
	Status           PRStatus   `json:"status"`
	LocalCommitHash  string     `json:"local_commit_hash"`
	RemoteCommitHash string     `json:"remote_commit_hash"`
	HTMLURL          string     `json:"html_url,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	MergedAt         *time.Time `json:"merged_at,omitempty"`
}

// PRStatus represents the status of a pull request
type PRStatus string

const (
	PRStatusOpen    PRStatus = "open"
	PRStatusClosed  PRStatus = "closed"
	PRStatusMerged  PRStatus = "merged"
	PRStatusDraft   PRStatus = "draft"
	PRStatusUnknown PRStatus = "unknown"
)

// RepoOperation represents an operation performed on a repository
type RepoOperation struct {
	ID            string                 `json:"id"`
	RepoID        string                 `json:"repo_id"`
	OperationType string                 `json:"operation_type"`
	RefBefore     string                 `json:"ref_before,omitempty"`
	RefAfter      string                 `json:"ref_after,omitempty"`
	Details       map[string]interface{} `json:"details,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

// RepoJob represents a job associated with a repository
type RepoJob struct {
	ID                   string                 `json:"id"`
	RepoID               string                 `json:"repo_id"`
	JobType              string                 `json:"job_type"`
	BranchName           string                 `json:"branch_name"`
	Status               string                 `json:"status"`
	InputPayload         map[string]interface{} `json:"input_payload,omitempty"`
	OutputPayload        map[string]interface{} `json:"output_payload,omitempty"`
	ValidationAttempts   int                    `json:"validation_attempts"`
	LastValidationResult map[string]interface{} `json:"last_validation_result,omitempty"`
	CreatedAt            time.Time              `json:"created_at"`
	CompletedAt          *time.Time             `json:"completed_at,omitempty"`
}

// CredentialType defines how to authenticate with a git provider
type CredentialType string

const (
	CredentialTypeSSH   CredentialType = "ssh"
	CredentialTypeHTTPS CredentialType = "https"
	CredentialTypeToken CredentialType = "token"
)

// GitCredential represents authentication credentials for a git provider
type GitCredential struct {
	Provider   string         `json:"provider"`
	Type       CredentialType `json:"type"`
	SSHKeyPath string         `json:"ssh_key_path,omitempty"`
	Username   string         `json:"username,omitempty"`
	Token      string         `json:"token,omitempty"` // TODO: encrypt at rest
}
