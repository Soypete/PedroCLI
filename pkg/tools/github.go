// Package tools provides the GitHub tool for interacting with GitHub via API or CLI.
//
// # Usage
//
// The GitHub tool supports two authentication methods:
//
//  1. gh CLI (default): Uses the authenticated gh session. Requires `gh auth login`.
//     - Automatically used when no token is provided
//     - Supports both public and private repos the user has access to
//
//  2. GitHub Token: Direct API access with a personal access token or GitHub App token.
//     - Use NewGitHubToolWithToken(workDir, token) to enable
//     - Useful for CI/automation or fine-grained access control
//
// # Actions
//
// The tool supports the following actions via the "action" argument:
//
//   - pr_fetch: Fetch pull request details (title, body, diff, files changed)
//     Required: repo (owner/repo) or url (https://github.com/.../pull/123)
//     Optional: pr_number, include_diff (bool)
//
//   - pr_checkout: Checkout a PR locally using gh CLI
//     Required: pr_number
//     Optional: repo
//
//   - issue_fetch: Fetch issue details (title, body, labels, comments)
//     Required: repo, issue_number
//
//   - pr_create: Create a new pull request (requires gh CLI)
//     Required: title
//     Optional: body, head, base, draft (default: true)
//
//   - pr_comment: Add a comment to a PR
//     Required: pr_number, body
//     Optional: repo
//
// # Examples
//
//	// Using gh CLI (requires authentication)
//	tool := NewGitHubTool("/path/to/repo")
//	result, _ := tool.Execute(ctx, map[string]interface{}{
//	    "action":    "pr_fetch",
//	    "repo":      "owner/repo",
//	    "pr_number": 123,
//	})
//
//	// Using token for API access
//	tool := NewGitHubToolWithToken("/path/to/repo", "ghp_...")
//
//	// Fetch by URL
//	result, _ := tool.Execute(ctx, map[string]interface{}{
//	    "action": "pr_fetch",
//	    "url":    "https://github.com/owner/repo/pull/123",
//	})
//
// # Capabilities
//
// The tool requires the "gh" capability for CLI-based actions.
// HTTP client mode works with any network-capable environment.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/soypete/pedrocli/pkg/logits"
	"github.com/soypete/pedrocli/pkg/tools/github"
)

// GitHubTool provides GitHub API operations for fetching PRs and issues
type GitHubTool struct {
	workDir     string
	githubToken string
	httpClient  *github.Client
}

// NewGitHubTool creates a new GitHub tool (legacy CLI-based)
func NewGitHubTool(workDir string) *GitHubTool {
	return &GitHubTool{
		workDir: workDir,
	}
}

// NewGitHubToolWithToken creates a new GitHub tool with HTTP API client
func NewGitHubToolWithToken(workDir, token string) *GitHubTool {
	var httpClient *github.Client
	if token != "" {
		httpClient = github.NewClient(token)
	}
	return &GitHubTool{
		workDir:     workDir,
		githubToken: token,
		httpClient:  httpClient,
	}
}

// Name returns the tool name
func (g *GitHubTool) Name() string {
	return "github"
}

// Description returns the tool description
func (g *GitHubTool) Description() string {
	return `Interact with GitHub using the API.

Actions:
- pr_fetch: Fetch pull request details
  Args: repo (string, "owner/repo"), pr_number (int) OR url (string, "https://github.com/owner/repo/pull/123")
  Returns: PR title, body, diff, files changed, review status

- pr_checkout: Checkout a pull request locally (requires gh CLI)
  Args: pr_number (int), repo (string, optional)
  Returns: Local branch name

- issue_fetch: Fetch issue details
  Args: repo (string), issue_number (int)
  Returns: Issue title, body, labels, comments

- pr_create: Create a pull request
  Args: repo (string), title (string), body (string), head (string), base (string), draft (bool)
  Returns: PR URL

- pr_comment: Add a comment to a PR
  Args: repo (string), pr_number (int), body (string)
  Returns: Comment URL

Examples:
{"tool": "github", "args": {"action": "pr_fetch", "repo": "owner/repo", "pr_number": 123}}
{"tool": "github", "args": {"action": "pr_fetch", "url": "https://github.com/owner/repo/pull/123"}}
{"tool": "github", "args": {"action": "issue_fetch", "repo": "owner/repo", "issue_number": 456}}`
}

// Execute executes the GitHub tool
func (g *GitHubTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	action, ok := args["action"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'action' parameter"}, nil
	}

	switch action {
	case "pr_fetch":
		return g.prFetch(ctx, args)
	case "pr_checkout":
		return g.prCheckout(ctx, args)
	case "issue_fetch":
		return g.issueFetch(ctx, args)
	case "pr_create":
		return g.prCreate(ctx, args)
	case "pr_comment":
		return g.prComment(ctx, args)
	default:
		return &Result{Success: false, Error: fmt.Sprintf("unknown action: %s", action)}, nil
	}
}

// PRInfo contains pull request information
type PRInfo struct {
	Number     int      `json:"number"`
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	State      string   `json:"state"`
	HeadBranch string   `json:"head_branch"`
	BaseBranch string   `json:"base_branch"`
	Author     string   `json:"author"`
	URL        string   `json:"url"`
	Files      []string `json:"files"`
	Additions  int      `json:"additions"`
	Deletions  int      `json:"deletions"`
	Diff       string   `json:"diff,omitempty"`
}

// IssueInfo contains issue information
type IssueInfo struct {
	Number   int            `json:"number"`
	Title    string         `json:"title"`
	Body     string         `json:"body"`
	State    string         `json:"state"`
	Author   string         `json:"author"`
	Labels   []string       `json:"labels"`
	URL      string         `json:"url"`
	Comments []IssueComment `json:"comments,omitempty"`
}

// IssueComment represents a comment on an issue
type IssueComment struct {
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// prFetch fetches PR details
func (g *GitHubTool) prFetch(ctx context.Context, args map[string]interface{}) (*Result, error) {
	repo, _ := args["repo"].(string)
	urlStr, _ := args["url"].(string)

	// Handle URL parameter - extract repo and PR number
	if urlStr != "" {
		owner, repoName, prNum, err := parsePRURL(urlStr)
		if err != nil {
			return &Result{Success: false, Error: fmt.Sprintf("failed to parse PR URL: %v", err)}, nil
		}
		repo = fmt.Sprintf("%s/%s", owner, repoName)
		args["pr_number"] = prNum
	}

	// Use HTTP client if available
	if g.httpClient != nil && repo != "" {
		return g.prFetchHTTP(ctx, args)
	}

	// Fallback to CLI
	return g.prFetchCLI(ctx, args)
}

// prFetchHTTP fetches PR using GitHub API
func (g *GitHubTool) prFetchHTTP(ctx context.Context, args map[string]interface{}) (*Result, error) {
	repo, _ := args["repo"].(string)
	owner, repoName, err := github.ParseRepo(repo)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("invalid repo format: %v", err)}, nil
	}

	var prNum int
	if prNumFloat, ok := args["pr_number"].(float64); ok {
		prNum = int(prNumFloat)
	} else if prNumInt, ok := args["pr_number"].(int); ok {
		prNum = prNumInt
	} else {
		return &Result{Success: false, Error: "missing 'pr_number' parameter"}, nil
	}

	// Fetch PR details
	pr, err := g.httpClient.FetchPR(ctx, owner, repoName, prNum)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to fetch PR: %v", err)}, nil
	}

	// Fetch files
	files, err := g.httpClient.FetchPRFiles(ctx, owner, repoName, prNum)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to fetch PR files: %v", err)}, nil
	}

	filePaths := make([]string, len(files))
	for i, f := range files {
		filePaths[i] = f.Filename
	}

	// Optionally fetch diff
	var diff string
	if includeDiff, ok := args["include_diff"].(bool); ok && includeDiff {
		diff, err = g.httpClient.FetchPRDiff(ctx, owner, repoName, prNum)
		if err != nil {
			return &Result{Success: false, Error: fmt.Sprintf("failed to fetch diff: %v", err)}, nil
		}
	}

	prInfo := PRInfo{
		Number:     pr.Number,
		Title:      pr.Title,
		Body:       pr.Body,
		State:      pr.State,
		HeadBranch: pr.Head.Ref,
		BaseBranch: pr.Base.Ref,
		Author:     pr.User.Login,
		URL:        pr.URL,
		Files:      filePaths,
		Additions:  pr.Additions,
		Deletions:  pr.Deletions,
		Diff:       diff,
	}

	// Format output
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("PR #%d: %s\n", prInfo.Number, prInfo.Title))
	sb.WriteString(fmt.Sprintf("Author: %s | State: %s\n", prInfo.Author, prInfo.State))
	sb.WriteString(fmt.Sprintf("Branch: %s → %s\n", prInfo.HeadBranch, prInfo.BaseBranch))
	sb.WriteString(fmt.Sprintf("Changes: +%d -%d in %d files\n", prInfo.Additions, prInfo.Deletions, len(prInfo.Files)))
	sb.WriteString(fmt.Sprintf("URL: %s\n", prInfo.URL))
	sb.WriteString("\n## Description\n")
	sb.WriteString(prInfo.Body)
	sb.WriteString("\n\n## Files Changed\n")
	for _, f := range prInfo.Files {
		sb.WriteString(fmt.Sprintf("- %s\n", f))
	}

	if diff != "" {
		sb.WriteString("\n## Diff\n```diff\n")
		sb.WriteString(diff)
		sb.WriteString("\n```")
	}

	return &Result{
		Success: true,
		Output:  sb.String(),
		Data: map[string]interface{}{
			"pr_info": prInfo,
		},
	}, nil
}

// prFetchCLI fetches PR using gh CLI (fallback)
func (g *GitHubTool) prFetchCLI(ctx context.Context, args map[string]interface{}) (*Result, error) {
	var prIdentifier string
	repo, _ := args["repo"].(string)

	// Accept either pr_number or branch
	if prNum, ok := args["pr_number"].(float64); ok {
		prIdentifier = fmt.Sprintf("%d", int(prNum))
	} else if prNum, ok := args["pr_number"].(int); ok {
		prIdentifier = fmt.Sprintf("%d", prNum)
	} else if branch, ok := args["branch"].(string); ok {
		// Find PR by branch
		cmdArgs := []string{"pr", "list", "--head", branch, "--json", "number", "--jq", ".[0].number"}
		cmd := exec.CommandContext(ctx, "gh", cmdArgs...)
		if repo != "" {
			cmd = exec.CommandContext(ctx, "gh", append([]string{"-R", repo}, cmdArgs...)...)
		}
		cmd.Dir = g.workDir
		output, err := cmd.CombinedOutput()
		if err != nil || strings.TrimSpace(string(output)) == "" {
			return &Result{Success: false, Error: fmt.Sprintf("no PR found for branch %s", branch)}, nil
		}
		prIdentifier = strings.TrimSpace(string(output))
	} else {
		return &Result{Success: false, Error: "missing 'pr_number' or 'branch' parameter"}, nil
	}

	// Fetch PR details
	cmdArgs := []string{"pr", "view", prIdentifier,
		"--json", "number,title,body,state,headRefName,baseRefName,author,url,files,additions,deletions"}
	var cmd *exec.Cmd
	if repo != "" {
		cmd = exec.CommandContext(ctx, "gh", append([]string{"-R", repo}, cmdArgs...)...)
	} else {
		cmd = exec.CommandContext(ctx, "gh", cmdArgs...)
	}
	cmd.Dir = g.workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to fetch PR: %s", string(output))}, nil
	}

	// Parse JSON output
	var prData struct {
		Number      int    `json:"number"`
		Title       string `json:"title"`
		Body        string `json:"body"`
		State       string `json:"state"`
		HeadRefName string `json:"headRefName"`
		BaseRefName string `json:"baseRefName"`
		Author      struct {
			Login string `json:"login"`
		} `json:"author"`
		URL       string `json:"url"`
		Additions int    `json:"additions"`
		Deletions int    `json:"deletions"`
		Files     []struct {
			Path string `json:"path"`
		} `json:"files"`
	}

	if err := json.Unmarshal(output, &prData); err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to parse PR data: %v", err)}, nil
	}

	// Extract file paths
	files := make([]string, len(prData.Files))
	for i, f := range prData.Files {
		files[i] = f.Path
	}

	// Optionally fetch diff
	var diff string
	if includeDiff, ok := args["include_diff"].(bool); ok && includeDiff {
		diffCmd := exec.CommandContext(ctx, "gh", "pr", "diff", prIdentifier)
		diffCmd.Dir = g.workDir
		diffOutput, _ := diffCmd.CombinedOutput()
		diff = string(diffOutput)
	}

	prInfo := PRInfo{
		Number:     prData.Number,
		Title:      prData.Title,
		Body:       prData.Body,
		State:      prData.State,
		HeadBranch: prData.HeadRefName,
		BaseBranch: prData.BaseRefName,
		Author:     prData.Author.Login,
		URL:        prData.URL,
		Files:      files,
		Additions:  prData.Additions,
		Deletions:  prData.Deletions,
		Diff:       diff,
	}

	// Format output
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("PR #%d: %s\n", prInfo.Number, prInfo.Title))
	sb.WriteString(fmt.Sprintf("Author: %s | State: %s\n", prInfo.Author, prInfo.State))
	sb.WriteString(fmt.Sprintf("Branch: %s → %s\n", prInfo.HeadBranch, prInfo.BaseBranch))
	sb.WriteString(fmt.Sprintf("Changes: +%d -%d in %d files\n", prInfo.Additions, prInfo.Deletions, len(prInfo.Files)))
	sb.WriteString(fmt.Sprintf("URL: %s\n", prInfo.URL))
	sb.WriteString("\n## Description\n")
	sb.WriteString(prInfo.Body)
	sb.WriteString("\n\n## Files Changed\n")
	for _, f := range prInfo.Files {
		sb.WriteString(fmt.Sprintf("- %s\n", f))
	}

	if diff != "" {
		sb.WriteString("\n## Diff\n```diff\n")
		sb.WriteString(diff)
		sb.WriteString("\n```")
	}

	return &Result{
		Success: true,
		Output:  sb.String(),
		Data: map[string]interface{}{
			"pr_info": prInfo,
		},
	}, nil
}

// parsePRURL parses a GitHub PR URL and extracts owner, repo, and PR number
func parsePRURL(urlStr string) (owner, repo string, prNum int, err error) {
	// Support formats:
	// https://github.com/owner/repo/pull/123
	// https://github.com/owner/repo/pull/123/files
	// github.com/owner/repo/pull/123

	urlStr = strings.TrimPrefix(urlStr, "https://")
	urlStr = strings.TrimPrefix(urlStr, "http://")
	urlStr = strings.TrimPrefix(urlStr, "github.com/")
	urlStr = strings.TrimPrefix(urlStr, "github.com")
	urlStr = strings.TrimSuffix(urlStr, "/")
	urlStr = strings.TrimSuffix(urlStr, "/files")
	urlStr = strings.TrimSuffix(urlStr, "/diff")

	parts := strings.Split(urlStr, "/")
	if len(parts) != 4 {
		return "", "", 0, fmt.Errorf("invalid PR URL format: %s", urlStr)
	}

	if parts[2] != "pull" {
		return "", "", 0, fmt.Errorf("URL does not point to a pull request: %s", urlStr)
	}

	owner = parts[0]
	repo = parts[1]
	prNumStr := parts[3]

	if _, err := fmt.Sscanf(prNumStr, "%d", &prNum); err != nil {
		return "", "", 0, fmt.Errorf("invalid PR number format: %s", prNumStr)
	}
	if prNum == 0 {
		return "", "", 0, fmt.Errorf("invalid PR number: %s", prNumStr)
	}

	return owner, repo, prNum, nil
}

// prCheckout checks out a PR locally
func (g *GitHubTool) prCheckout(ctx context.Context, args map[string]interface{}) (*Result, error) {
	var prNum int
	if num, ok := args["pr_number"].(float64); ok {
		prNum = int(num)
	} else if num, ok := args["pr_number"].(int); ok {
		prNum = num
	} else {
		return &Result{Success: false, Error: "missing 'pr_number' parameter"}, nil
	}

	repo, _ := args["repo"].(string)
	cmdArgs := []string{"pr", "checkout", fmt.Sprintf("%d", prNum)}
	var cmd *exec.Cmd
	if repo != "" {
		cmd = exec.CommandContext(ctx, "gh", append([]string{"-R", repo}, cmdArgs...)...)
	} else {
		cmd = exec.CommandContext(ctx, "gh", cmdArgs...)
	}
	cmd.Dir = g.workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to checkout PR: %s", string(output))}, nil
	}

	// Get current branch name
	branchCmd := exec.CommandContext(ctx, "git", "branch", "--show-current")
	branchCmd.Dir = g.workDir
	branchOutput, _ := branchCmd.CombinedOutput()
	branch := strings.TrimSpace(string(branchOutput))

	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Checked out PR #%d to branch: %s", prNum, branch),
		Data: map[string]interface{}{
			"branch": branch,
		},
	}, nil
}

// issueFetch fetches issue details
func (g *GitHubTool) issueFetch(ctx context.Context, args map[string]interface{}) (*Result, error) {
	repo, _ := args["repo"].(string)

	// Use HTTP client if available and repo specified
	if g.httpClient != nil && repo != "" {
		return g.issueFetchHTTP(ctx, args)
	}

	// Fallback to CLI
	return g.issueFetchCLI(ctx, args)
}

// issueFetchHTTP fetches issue using GitHub API
func (g *GitHubTool) issueFetchHTTP(ctx context.Context, args map[string]interface{}) (*Result, error) {
	repo, _ := args["repo"].(string)
	owner, repoName, err := github.ParseRepo(repo)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("invalid repo format: %v", err)}, nil
	}

	var issueNum int
	if num, ok := args["issue_number"].(float64); ok {
		issueNum = int(num)
	} else if num, ok := args["issue_number"].(int); ok {
		issueNum = num
	} else {
		return &Result{Success: false, Error: "missing 'issue_number' parameter"}, nil
	}

	// Fetch issue
	issue, err := g.httpClient.FetchIssue(ctx, owner, repoName, issueNum)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to fetch issue: %v", err)}, nil
	}

	// Fetch comments
	comments, err := g.httpClient.FetchIssueComments(ctx, owner, repoName, issueNum)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to fetch comments: %v", err)}, nil
	}

	// Extract labels
	labels := make([]string, len(issue.Labels))
	for i, l := range issue.Labels {
		labels[i] = l.Name
	}

	// Convert comments
	issueComments := make([]IssueComment, len(comments))
	for i, c := range comments {
		issueComments[i] = IssueComment{
			Author:    c.User.Login,
			Body:      c.Body,
			CreatedAt: c.CreatedAt,
		}
	}

	issueInfo := IssueInfo{
		Number:   issue.Number,
		Title:    issue.Title,
		Body:     issue.Body,
		State:    issue.State,
		Author:   issue.User.Login,
		Labels:   labels,
		URL:      issue.URL,
		Comments: issueComments,
	}

	// Format output
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Issue #%d: %s\n", issueInfo.Number, issueInfo.Title))
	sb.WriteString(fmt.Sprintf("Author: %s | State: %s\n", issueInfo.Author, issueInfo.State))
	if len(issueInfo.Labels) > 0 {
		sb.WriteString(fmt.Sprintf("Labels: %s\n", strings.Join(issueInfo.Labels, ", ")))
	}
	sb.WriteString(fmt.Sprintf("URL: %s\n", issueInfo.URL))
	sb.WriteString("\n## Description\n")
	sb.WriteString(issueInfo.Body)

	if len(issueInfo.Comments) > 0 {
		sb.WriteString("\n\n## Comments\n")
		for _, c := range issueInfo.Comments {
			sb.WriteString(fmt.Sprintf("\n### @%s (%s)\n%s\n", c.Author, c.CreatedAt, c.Body))
		}
	}

	return &Result{
		Success: true,
		Output:  sb.String(),
		Data: map[string]interface{}{
			"issue_info": issueInfo,
		},
	}, nil
}

// issueFetchCLI fetches issue using gh CLI (fallback)
func (g *GitHubTool) issueFetchCLI(ctx context.Context, args map[string]interface{}) (*Result, error) {
	var issueNum int
	if num, ok := args["issue_number"].(float64); ok {
		issueNum = int(num)
	} else if num, ok := args["issue_number"].(int); ok {
		issueNum = num
	} else {
		return &Result{Success: false, Error: "missing 'issue_number' parameter"}, nil
	}

	// Fetch issue details
	cmd := exec.CommandContext(ctx, "gh", "issue", "view", fmt.Sprintf("%d", issueNum),
		"--json", "number,title,body,state,author,labels,url,comments")
	cmd.Dir = g.workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to fetch issue: %s", string(output))}, nil
	}

	// Parse JSON output
	var issueData struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		State  string `json:"state"`
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		URL      string `json:"url"`
		Comments []struct {
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
			Body      string `json:"body"`
			CreatedAt string `json:"createdAt"`
		} `json:"comments"`
	}

	if err := json.Unmarshal(output, &issueData); err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to parse issue data: %v", err)}, nil
	}

	// Extract labels
	labels := make([]string, len(issueData.Labels))
	for i, l := range issueData.Labels {
		labels[i] = l.Name
	}

	// Extract comments
	comments := make([]IssueComment, len(issueData.Comments))
	for i, c := range issueData.Comments {
		comments[i] = IssueComment{
			Author:    c.Author.Login,
			Body:      c.Body,
			CreatedAt: c.CreatedAt,
		}
	}

	issueInfo := IssueInfo{
		Number:   issueData.Number,
		Title:    issueData.Title,
		Body:     issueData.Body,
		State:    issueData.State,
		Author:   issueData.Author.Login,
		Labels:   labels,
		URL:      issueData.URL,
		Comments: comments,
	}

	// Format output
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Issue #%d: %s\n", issueInfo.Number, issueInfo.Title))
	sb.WriteString(fmt.Sprintf("Author: %s | State: %s\n", issueInfo.Author, issueInfo.State))
	if len(issueInfo.Labels) > 0 {
		sb.WriteString(fmt.Sprintf("Labels: %s\n", strings.Join(issueInfo.Labels, ", ")))
	}
	sb.WriteString(fmt.Sprintf("URL: %s\n", issueInfo.URL))
	sb.WriteString("\n## Description\n")
	sb.WriteString(issueInfo.Body)

	if len(issueInfo.Comments) > 0 {
		sb.WriteString("\n\n## Comments\n")
		for _, c := range issueInfo.Comments {
			sb.WriteString(fmt.Sprintf("\n### @%s (%s)\n%s\n", c.Author, c.CreatedAt, c.Body))
		}
	}

	return &Result{
		Success: true,
		Output:  sb.String(),
		Data: map[string]interface{}{
			"issue_info": issueInfo,
		},
	}, nil
}

// prCreate creates a pull request
func (g *GitHubTool) prCreate(ctx context.Context, args map[string]interface{}) (*Result, error) {
	title, ok := args["title"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'title' parameter"}, nil
	}

	body, ok := args["body"].(string)
	if !ok {
		body = ""
	}

	cmdArgs := []string{"pr", "create", "--title", title, "--body", body}

	// Default to draft
	draft := true
	if d, ok := args["draft"].(bool); ok {
		draft = d
	}
	if draft {
		cmdArgs = append(cmdArgs, "--draft")
	}

	cmd := exec.CommandContext(ctx, "gh", cmdArgs...)
	cmd.Dir = g.workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to create PR: %s", string(output))}, nil
	}

	prURL := strings.TrimSpace(string(output))

	return &Result{
		Success: true,
		Output:  fmt.Sprintf("PR created: %s", prURL),
		Data: map[string]interface{}{
			"pr_url": prURL,
		},
	}, nil
}

// prComment adds a comment to a PR
func (g *GitHubTool) prComment(ctx context.Context, args map[string]interface{}) (*Result, error) {
	var prNum int
	if num, ok := args["pr_number"].(float64); ok {
		prNum = int(num)
	} else if num, ok := args["pr_number"].(int); ok {
		prNum = num
	} else {
		return &Result{Success: false, Error: "missing 'pr_number' parameter"}, nil
	}

	body, ok := args["body"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'body' parameter"}, nil
	}

	cmd := exec.CommandContext(ctx, "gh", "pr", "comment", fmt.Sprintf("%d", prNum), "--body", body)
	cmd.Dir = g.workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to comment on PR: %s", string(output))}, nil
	}

	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Comment added to PR #%d", prNum),
	}, nil
}

// Metadata returns rich tool metadata
func (g *GitHubTool) Metadata() *ToolMetadata {
	return &ToolMetadata{
		Schema: &logits.JSONSchema{
			Type: "object",
			Properties: map[string]*logits.JSONSchema{
				"action": {
					Type:        "string",
					Enum:        []interface{}{"pr_fetch", "pr_checkout", "issue_fetch", "pr_create", "pr_comment"},
					Description: "The GitHub operation to perform",
				},
				"pr_number": {
					Type:        "integer",
					Description: "Pull request number",
				},
				"issue_number": {
					Type:        "integer",
					Description: "Issue number",
				},
				"branch": {
					Type:        "string",
					Description: "Branch name to find PR for",
				},
				"title": {
					Type:        "string",
					Description: "PR title (for pr_create)",
				},
				"body": {
					Type:        "string",
					Description: "PR/comment body",
				},
				"draft": {
					Type:        "boolean",
					Description: "Create as draft PR (default: true)",
				},
				"include_diff": {
					Type:        "boolean",
					Description: "Include full diff in pr_fetch (default: false)",
				},
			},
			Required: []string{"action"},
		},
		Category:             CategoryVCS,
		Optionality:          ToolOptional,
		UsageHint:            "Use for fetching PR/issue details, checking out PRs, and creating PRs",
		RequiresCapabilities: []string{"gh"},
		Examples: []ToolExample{
			{
				Description: "Fetch PR details",
				Input:       map[string]interface{}{"action": "pr_fetch", "pr_number": 123},
			},
			{
				Description: "Fetch issue with comments",
				Input:       map[string]interface{}{"action": "issue_fetch", "issue_number": 456},
			},
			{
				Description: "Create draft PR",
				Input:       map[string]interface{}{"action": "pr_create", "title": "Add feature", "body": "Description", "draft": true},
			},
		},
		Produces: []string{"pr_info", "issue_info"},
	}
}
