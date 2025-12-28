package repos

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultGitOps implements GitOps using git command line
type DefaultGitOps struct{}

// NewGitOps creates a new GitOps implementation
func NewGitOps() *DefaultGitOps {
	return &DefaultGitOps{}
}

// Clone clones a repository to the specified path
func (g *DefaultGitOps) Clone(ctx context.Context, url, path string) error {
	cmd := exec.CommandContext(ctx, "git", "clone", url, path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w: %s", err, string(output))
	}
	return nil
}

// CloneWithDepth clones with a specific depth (shallow clone)
func (g *DefaultGitOps) CloneWithDepth(ctx context.Context, url, path string, depth int) error {
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", strconv.Itoa(depth), url, path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w: %s", err, string(output))
	}
	return nil
}

// Fetch fetches all refs from remotes
func (g *DefaultGitOps) Fetch(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "git", "fetch", "--all", "--prune")
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch failed: %w: %s", err, string(output))
	}
	return nil
}

// FetchRemote fetches from a specific remote
func (g *DefaultGitOps) FetchRemote(ctx context.Context, path, remote string) error {
	cmd := exec.CommandContext(ctx, "git", "fetch", remote, "--prune")
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch failed: %w: %s", err, string(output))
	}
	return nil
}

// Checkout checks out a ref (branch, tag, or commit)
func (g *DefaultGitOps) Checkout(ctx context.Context, path, ref string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", ref)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout failed: %w: %s", err, string(output))
	}
	return nil
}

// Pull pulls changes from the remote
func (g *DefaultGitOps) Pull(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "git", "pull")
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull failed: %w: %s", err, string(output))
	}
	return nil
}

// PullRemote pulls from a specific remote/branch
func (g *DefaultGitOps) PullRemote(ctx context.Context, path, remote, branch string) error {
	cmd := exec.CommandContext(ctx, "git", "pull", remote, branch)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull failed: %w: %s", err, string(output))
	}
	return nil
}

// CreateBranch creates a new branch
func (g *DefaultGitOps) CreateBranch(ctx context.Context, path, name string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", "-b", name)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout -b failed: %w: %s", err, string(output))
	}
	return nil
}

// CreateBranchFrom creates a new branch from a specific ref
func (g *DefaultGitOps) CreateBranchFrom(ctx context.Context, path, name, from string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", "-b", name, from)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout -b failed: %w: %s", err, string(output))
	}
	return nil
}

// DeleteBranch deletes a branch
func (g *DefaultGitOps) DeleteBranch(ctx context.Context, path, name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	cmd := exec.CommandContext(ctx, "git", "branch", flag, name)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git branch delete failed: %w: %s", err, string(output))
	}
	return nil
}

// ListBranches lists all branches
func (g *DefaultGitOps) ListBranches(ctx context.Context, path string) ([]Branch, error) {
	// Get all branches with their commit info
	cmd := exec.CommandContext(ctx, "git", "branch", "-a", "--format=%(refname:short)|%(objectname:short)|%(HEAD)")
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git branch failed: %w: %s", err, string(output))
	}

	var branches []Branch
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}

		name := parts[0]
		commit := parts[1]
		isCurrent := parts[2] == "*"

		// Check if it's a remote branch
		isRemote := strings.HasPrefix(name, "remotes/") || strings.HasPrefix(name, "origin/")
		if strings.HasPrefix(name, "remotes/") {
			name = strings.TrimPrefix(name, "remotes/")
		}

		branches = append(branches, Branch{
			Name:      name,
			IsRemote:  isRemote,
			IsCurrent: isCurrent,
			Commit:    commit,
		})
	}

	return branches, nil
}

// CurrentBranch returns the current branch name
func (g *DefaultGitOps) CurrentBranch(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %w: %s", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// Status returns the repository status
func (g *DefaultGitOps) Status(ctx context.Context, path string) (*RepoStatus, error) {
	status := &RepoStatus{}

	// Get current branch
	branch, err := g.CurrentBranch(ctx, path)
	if err != nil {
		return nil, err
	}
	status.CurrentBranch = branch

	// Get HEAD commit
	headHash, err := g.GetHeadHash(ctx, path)
	if err != nil {
		return nil, err
	}
	status.HeadCommit = headHash

	// Get porcelain status
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w: %s", err, string(output))
	}

	// Parse status output
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 3 {
			continue
		}

		indexStatus := line[0]
		workTreeStatus := line[1]
		file := strings.TrimSpace(line[3:])

		// Staged changes
		if indexStatus != ' ' && indexStatus != '?' {
			status.StagedFiles = append(status.StagedFiles, file)
		}

		// Modified but not staged
		if workTreeStatus == 'M' || workTreeStatus == 'D' {
			status.ModifiedFiles = append(status.ModifiedFiles, file)
		}

		// Untracked files
		if indexStatus == '?' && workTreeStatus == '?' {
			status.UntrackedFiles = append(status.UntrackedFiles, file)
		}
	}

	status.IsClean = len(status.ModifiedFiles) == 0 &&
		len(status.StagedFiles) == 0 &&
		len(status.UntrackedFiles) == 0

	// Get ahead/behind counts
	revListCmd := exec.CommandContext(ctx, "git", "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	revListCmd.Dir = path
	revOutput, err := revListCmd.CombinedOutput()
	if err == nil {
		parts := strings.Fields(strings.TrimSpace(string(revOutput)))
		if len(parts) == 2 {
			status.Ahead, _ = strconv.Atoi(parts[0])
			status.Behind, _ = strconv.Atoi(parts[1])
		}
	}
	// Ignore error - upstream might not be set

	return status, nil
}

// Diff returns the diff output
func (g *DefaultGitOps) Diff(ctx context.Context, path string, staged bool) (string, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--staged")
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w: %s", err, string(output))
	}
	return string(output), nil
}

// DiffBranches returns diff between two refs
func (g *DefaultGitOps) DiffBranches(ctx context.Context, path, base, head string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", base+"..."+head)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w: %s", err, string(output))
	}
	return string(output), nil
}

// Add stages files for commit
func (g *DefaultGitOps) Add(ctx context.Context, path string, files ...string) error {
	args := append([]string{"add"}, files...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add failed: %w: %s", err, string(output))
	}
	return nil
}

// AddAll stages all changes
func (g *DefaultGitOps) AddAll(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "git", "add", "-A")
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add failed: %w: %s", err, string(output))
	}
	return nil
}

// Commit creates a commit with the given message
func (g *DefaultGitOps) Commit(ctx context.Context, path, message string) error {
	cmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit failed: %w: %s", err, string(output))
	}
	return nil
}

// CommitWithAuthor creates a commit with a specific author
func (g *DefaultGitOps) CommitWithAuthor(ctx context.Context, path, message, author, email string) error {
	authorStr := fmt.Sprintf("%s <%s>", author, email)
	cmd := exec.CommandContext(ctx, "git", "commit", "-m", message, "--author", authorStr)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit failed: %w: %s", err, string(output))
	}
	return nil
}

// Push pushes to the remote
func (g *DefaultGitOps) Push(ctx context.Context, path, remote, branch string) error {
	cmd := exec.CommandContext(ctx, "git", "push", "-u", remote, branch)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push failed: %w: %s", err, string(output))
	}
	return nil
}

// PushForce force pushes to the remote
func (g *DefaultGitOps) PushForce(ctx context.Context, path, remote, branch string) error {
	cmd := exec.CommandContext(ctx, "git", "push", "--force-with-lease", "-u", remote, branch)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push failed: %w: %s", err, string(output))
	}
	return nil
}

// GetHeadHash returns the current HEAD commit hash
func (g *DefaultGitOps) GetHeadHash(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %w: %s", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// GetRemoteHash returns the hash of a remote branch
func (g *DefaultGitOps) GetRemoteHash(ctx context.Context, path, remote, branch string) (string, error) {
	ref := remote + "/" + branch
	cmd := exec.CommandContext(ctx, "git", "rev-parse", ref)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed for %s: %w: %s", ref, err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// IsUpToDate checks if the local branch is up to date with remote
func (g *DefaultGitOps) IsUpToDate(ctx context.Context, path string) (bool, error) {
	status, err := g.Status(ctx, path)
	if err != nil {
		return false, err
	}
	return status.Ahead == 0 && status.Behind == 0 && status.IsClean, nil
}

// Log returns recent commits
func (g *DefaultGitOps) Log(ctx context.Context, path string, limit int) ([]Commit, error) {
	return g.LogBranch(ctx, path, "HEAD", limit)
}

// LogBranch returns commits on a specific branch
func (g *DefaultGitOps) LogBranch(ctx context.Context, path, branch string, limit int) ([]Commit, error) {
	// Format: hash|short|author|email|date|subject
	format := "%H|%h|%an|%ae|%aI|%s"
	cmd := exec.CommandContext(ctx, "git", "log", branch,
		fmt.Sprintf("-n%d", limit),
		"--format="+format)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w: %s", err, string(output))
	}

	var commits []Commit
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 6)
		if len(parts) < 6 {
			continue
		}

		date, _ := time.Parse(time.RFC3339, parts[4])
		commits = append(commits, Commit{
			Hash:        parts[0],
			ShortHash:   parts[1],
			Author:      parts[2],
			AuthorEmail: parts[3],
			Date:        date,
			Subject:     parts[5],
			Message:     parts[5], // For full message, would need separate call
		})
	}

	return commits, nil
}

// GetDefaultBranch returns the default branch name
func (g *DefaultGitOps) GetDefaultBranch(ctx context.Context, path string) (string, error) {
	// Try to get the default branch from remote HEAD
	cmd := exec.CommandContext(ctx, "git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err == nil {
		ref := strings.TrimSpace(string(output))
		// Format: refs/remotes/origin/main
		parts := strings.Split(ref, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	// Fallback: check common default branch names
	for _, branch := range []string{"main", "master"} {
		checkCmd := exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
		checkCmd.Dir = path
		if err := checkCmd.Run(); err == nil {
			return branch, nil
		}
	}

	// Last resort: return current branch
	return g.CurrentBranch(ctx, path)
}

// GetRemoteURL returns the URL for a remote
func (g *DefaultGitOps) GetRemoteURL(ctx context.Context, path, remote string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", remote)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git remote get-url failed: %w: %s", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// SetRemoteURL sets the URL for a remote
func (g *DefaultGitOps) SetRemoteURL(ctx context.Context, path, remote, url string) error {
	cmd := exec.CommandContext(ctx, "git", "remote", "set-url", remote, url)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git remote set-url failed: %w: %s", err, string(output))
	}
	return nil
}

// Stash stashes current changes
func (g *DefaultGitOps) Stash(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "git", "stash", "push", "-m", "pedrocli-auto-stash")
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git stash failed: %w: %s", err, string(output))
	}
	return nil
}

// StashPop pops the most recent stash
func (g *DefaultGitOps) StashPop(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "git", "stash", "pop")
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git stash pop failed: %w: %s", err, string(output))
	}
	return nil
}

// Clean removes untracked files
func (g *DefaultGitOps) Clean(ctx context.Context, path string, force, directories bool) error {
	args := []string{"clean"}
	if force {
		args = append(args, "-f")
	}
	if directories {
		args = append(args, "-d")
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clean failed: %w: %s", err, string(output))
	}
	return nil
}

// Reset resets to a specific ref
func (g *DefaultGitOps) Reset(ctx context.Context, path, ref string, hard bool) error {
	args := []string{"reset"}
	if hard {
		args = append(args, "--hard")
	}
	args = append(args, ref)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset failed: %w: %s", err, string(output))
	}
	return nil
}

// ParseRepoURL parses a git URL and returns provider, owner, repo
func ParseRepoURL(url string) (provider, owner, repo string, err error) {
	// SSH format: git@github.com:owner/repo.git
	sshRegex := regexp.MustCompile(`git@([^:]+):([^/]+)/(.+?)(?:\.git)?$`)
	if matches := sshRegex.FindStringSubmatch(url); len(matches) == 4 {
		return matches[1], matches[2], matches[3], nil
	}

	// HTTPS format: https://github.com/owner/repo.git
	httpsRegex := regexp.MustCompile(`https?://([^/]+)/([^/]+)/(.+?)(?:\.git)?$`)
	if matches := httpsRegex.FindStringSubmatch(url); len(matches) == 4 {
		return matches[1], matches[2], matches[3], nil
	}

	return "", "", "", fmt.Errorf("unable to parse git URL: %s", url)
}

// Ensure DefaultGitOps implements GitOps
var _ GitOps = (*DefaultGitOps)(nil)
