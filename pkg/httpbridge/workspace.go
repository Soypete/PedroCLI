package httpbridge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/soypete/pedrocli/pkg/config"
)

// WorkspaceManager manages isolated workspace directories for HTTP bridge jobs.
// Each job gets its own workspace at ~/.cache/pedrocli/jobs/<job-id>/
type WorkspaceManager struct {
	basePath string // Base directory for all workspaces (e.g., ~/.cache/pedrocli/jobs)
}

// NewWorkspaceManager creates a new workspace manager.
func NewWorkspaceManager(basePath string) *WorkspaceManager {
	return &WorkspaceManager{
		basePath: basePath,
	}
}

// SetupWorkspace creates or updates a workspace for a job.
// If the workspace already exists, it performs git fetch/pull instead of re-cloning.
// Returns the absolute path to the workspace directory.
func (wm *WorkspaceManager) SetupWorkspace(ctx context.Context, jobID string, repoPath string) (string, error) {
	// Create job directory
	jobDir := filepath.Join(wm.basePath, jobID)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create job directory: %w", err)
	}

	workspacePath := filepath.Join(jobDir, "workspace")

	// Check if workspace already exists (has .git directory)
	gitDir := filepath.Join(workspacePath, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		// Workspace exists - update it
		if err := wm.updateWorkspace(ctx, workspacePath); err != nil {
			return "", fmt.Errorf("failed to update existing workspace: %w", err)
		}
		return workspacePath, nil
	}

	// Workspace doesn't exist - clone it
	// Convert HTTPS URL to SSH
	sshURL := wm.ConvertToSSH(repoPath)

	// Clone repo to workspace
	cmd := exec.CommandContext(ctx, "git", "clone", sshURL, workspacePath)
	cmd.Dir = jobDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to clone repo: %w\nOutput: %s", err, string(output))
	}

	return workspacePath, nil
}

// updateWorkspace updates an existing workspace with git fetch and pull.
func (wm *WorkspaceManager) updateWorkspace(ctx context.Context, workspacePath string) error {
	// Fetch latest changes
	fetchCmd := exec.CommandContext(ctx, "git", "fetch", "origin")
	fetchCmd.Dir = workspacePath
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed: %w\nOutput: %s", err, string(output))
	}

	// Pull latest changes (fast-forward only to avoid conflicts)
	pullCmd := exec.CommandContext(ctx, "git", "pull", "--ff-only")
	pullCmd.Dir = workspacePath
	if output, err := pullCmd.CombinedOutput(); err != nil {
		// If pull fails, that's okay - might be on a detached HEAD or have local changes
		// Just log it and continue
		fmt.Printf("Warning: git pull failed (continuing anyway): %v\nOutput: %s\n", err, string(output))
	}

	return nil
}

// ConvertToSSH converts HTTPS git URLs to SSH format.
// Examples:
//   - https://github.com/user/repo.git → git@github.com:user/repo.git
//   - https://github.com/user/repo → git@github.com:user/repo.git
//   - git@github.com:user/repo.git → git@github.com:user/repo.git (unchanged)
func (wm *WorkspaceManager) ConvertToSSH(url string) string {
	// Already SSH format
	if strings.HasPrefix(url, "git@") {
		return url
	}

	// Convert GitHub HTTPS to SSH
	if strings.HasPrefix(url, "https://github.com/") {
		parts := strings.TrimPrefix(url, "https://github.com/")
		parts = strings.TrimSuffix(parts, ".git")
		return fmt.Sprintf("git@github.com:%s.git", parts)
	}

	// Convert GitLab HTTPS to SSH
	if strings.HasPrefix(url, "https://gitlab.com/") {
		parts := strings.TrimPrefix(url, "https://gitlab.com/")
		parts = strings.TrimSuffix(parts, ".git")
		return fmt.Sprintf("git@gitlab.com:%s.git", parts)
	}

	// Convert Bitbucket HTTPS to SSH
	if strings.HasPrefix(url, "https://bitbucket.org/") {
		parts := strings.TrimPrefix(url, "https://bitbucket.org/")
		parts = strings.TrimSuffix(parts, ".git")
		return fmt.Sprintf("git@bitbucket.org:%s.git", parts)
	}

	// If local path or unknown format, return as-is
	return url
}

// CreateBranchInWorkspace creates a new git branch in the workspace.
func (wm *WorkspaceManager) CreateBranchInWorkspace(ctx context.Context, workspacePath string, branchName string) error {
	// Ensure we're on the default branch first
	checkoutCmd := exec.CommandContext(ctx, "git", "checkout", "main")
	checkoutCmd.Dir = workspacePath
	if _, err := checkoutCmd.CombinedOutput(); err != nil {
		// Try master if main doesn't exist
		checkoutCmd = exec.CommandContext(ctx, "git", "checkout", "master")
		checkoutCmd.Dir = workspacePath
		if output, err := checkoutCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to checkout default branch: %w\nOutput: %s", err, string(output))
		}
	}

	// Create and checkout new branch
	branchCmd := exec.CommandContext(ctx, "git", "checkout", "-b", branchName)
	branchCmd.Dir = workspacePath
	if output, err := branchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create branch %s: %w\nOutput: %s", branchName, err, string(output))
	}

	return nil
}

// PushAndCreatePR pushes the branch and creates a GitHub PR.
func (wm *WorkspaceManager) PushAndCreatePR(ctx context.Context, workspacePath string, branchName string, title string, body string) error {
	// Push branch to remote
	pushCmd := exec.CommandContext(ctx, "git", "push", "-u", "origin", branchName)
	pushCmd.Dir = workspacePath
	if output, err := pushCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to push branch: %w\nOutput: %s", err, string(output))
	}

	// Create PR using gh CLI
	prCmd := exec.CommandContext(ctx, "gh", "pr", "create",
		"--title", title,
		"--body", body,
		"--base", "main")
	prCmd.Dir = workspacePath
	if _, err := prCmd.CombinedOutput(); err != nil {
		// Try master if main doesn't exist
		prCmd = exec.CommandContext(ctx, "gh", "pr", "create",
			"--title", title,
			"--body", body,
			"--base", "master")
		prCmd.Dir = workspacePath
		if output, err := prCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create PR: %w\nOutput: %s", err, string(output))
		}
	}

	return nil
}

// CleanupWorkspace removes a workspace directory if cleanup is enabled in config.
func (wm *WorkspaceManager) CleanupWorkspace(ctx context.Context, jobID string, config *config.HTTPBridgeConfig) error {
	if !config.CleanupOnComplete {
		return nil // Preserve workspace for debugging
	}

	workspacePath := filepath.Join(wm.basePath, jobID)
	if err := os.RemoveAll(workspacePath); err != nil {
		return fmt.Errorf("failed to remove workspace: %w", err)
	}

	return nil
}
