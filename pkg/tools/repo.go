package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/soypete/pedrocli/pkg/hooks"
	"github.com/soypete/pedrocli/pkg/repos"
)

// RepoTool provides repository management operations
type RepoTool struct {
	manager      repos.Manager
	gitOps       repos.GitOps
	hooksManager hooks.Manager
	executor     repos.Executor
}

// NewRepoTool creates a new repo management tool
func NewRepoTool(manager repos.Manager, gitOps repos.GitOps, hooksManager hooks.Manager, executor repos.Executor) *RepoTool {
	return &RepoTool{
		manager:      manager,
		gitOps:       gitOps,
		hooksManager: hooksManager,
		executor:     executor,
	}
}

// Name returns the tool name
func (r *RepoTool) Name() string {
	return "repo"
}

// Description returns the tool description
func (r *RepoTool) Description() string {
	return `Manage repositories with GOPATH-style local storage.

Actions:
- ensure_repo: Clone or fetch repository, install hooks
  Args: provider (string), owner (string), repo (string)

- list_repos: List all managed repositories
  Args: none

- get_repo: Get info about a specific repository
  Args: provider (string), owner (string), repo (string)

- remove_repo: Remove a repository from local storage
  Args: provider (string), owner (string), repo (string)

- checkout_branch: Checkout or create a branch
  Args: provider (string), owner (string), repo (string), branch (string), create (bool, optional)

- status: Get repository status (modified files, branch, etc.)
  Args: provider (string), owner (string), repo (string)

- validate_changes: Run all hooks/checks without committing
  Args: provider (string), owner (string), repo (string)

- commit_changes: Commit with pre-commit hooks
  Args: provider (string), owner (string), repo (string), message (string), files ([]string, optional)

- push_changes: Push with pre-push validation
  Args: provider (string), owner (string), repo (string), branch (string), force (bool, optional)

- configure_hooks: Update hook configuration for repo
  Args: provider (string), owner (string), repo (string), config (object)

- run_hook: Run specific hook manually
  Args: provider (string), owner (string), repo (string), hook_type (string: pre-commit, pre-push, commit-msg)

- run_command: Run command in repo directory
  Args: provider (string), owner (string), repo (string), command (string), args ([]string)
`
}

// Execute executes the repo tool
func (r *RepoTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	action, ok := args["action"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'action' parameter"}, nil
	}

	switch action {
	case "ensure_repo":
		return r.ensureRepo(ctx, args)
	case "list_repos":
		return r.listRepos(ctx)
	case "get_repo":
		return r.getRepo(ctx, args)
	case "remove_repo":
		return r.removeRepo(ctx, args)
	case "checkout_branch":
		return r.checkoutBranch(ctx, args)
	case "status":
		return r.status(ctx, args)
	case "validate_changes":
		return r.validateChanges(ctx, args)
	case "commit_changes":
		return r.commitChanges(ctx, args)
	case "push_changes":
		return r.pushChanges(ctx, args)
	case "configure_hooks":
		return r.configureHooks(ctx, args)
	case "run_hook":
		return r.runHook(ctx, args)
	case "run_command":
		return r.runCommand(ctx, args)
	default:
		return &Result{Success: false, Error: fmt.Sprintf("unknown action: %s", action)}, nil
	}
}

// Helper to extract provider/owner/repo from args
func (r *RepoTool) getRepoArgs(args map[string]interface{}) (provider, owner, repo string, err error) {
	provider, _ = args["provider"].(string)
	owner, _ = args["owner"].(string)
	repo, _ = args["repo"].(string)

	if provider == "" || owner == "" || repo == "" {
		return "", "", "", fmt.Errorf("missing required parameters: provider, owner, repo")
	}

	return provider, owner, repo, nil
}

func (r *RepoTool) ensureRepo(ctx context.Context, args map[string]interface{}) (*Result, error) {
	provider, owner, repoName, err := r.getRepoArgs(args)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	localRepo, err := r.manager.EnsureRepo(ctx, provider, owner, repoName)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to ensure repo: %v", err)}, nil
	}

	output, _ := json.MarshalIndent(localRepo, "", "  ")
	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Repository ready at %s\n\n%s", localRepo.LocalPath, string(output)),
	}, nil
}

func (r *RepoTool) listRepos(ctx context.Context) (*Result, error) {
	repoList, err := r.manager.ListRepos(ctx)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to list repos: %v", err)}, nil
	}

	if len(repoList) == 0 {
		return &Result{Success: true, Output: "No repositories managed"}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Managed repositories: %d\n\n", len(repoList)))

	for _, repo := range repoList {
		sb.WriteString(fmt.Sprintf("- %s/%s/%s\n", repo.Provider, repo.Owner, repo.Name))
		sb.WriteString(fmt.Sprintf("  Path: %s\n", repo.LocalPath))
		sb.WriteString(fmt.Sprintf("  Branch: %s\n", repo.CurrentRef))
		sb.WriteString(fmt.Sprintf("  Type: %s\n\n", repo.ProjectType))
	}

	return &Result{Success: true, Output: sb.String()}, nil
}

func (r *RepoTool) getRepo(ctx context.Context, args map[string]interface{}) (*Result, error) {
	provider, owner, repoName, err := r.getRepoArgs(args)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	localRepo, err := r.manager.GetRepo(ctx, provider, owner, repoName)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to get repo: %v", err)}, nil
	}

	output, _ := json.MarshalIndent(localRepo, "", "  ")
	return &Result{Success: true, Output: string(output)}, nil
}

func (r *RepoTool) removeRepo(ctx context.Context, args map[string]interface{}) (*Result, error) {
	provider, owner, repoName, err := r.getRepoArgs(args)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	if err := r.manager.RemoveRepo(ctx, provider, owner, repoName); err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to remove repo: %v", err)}, nil
	}

	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Removed repository %s/%s/%s", provider, owner, repoName),
	}, nil
}

func (r *RepoTool) checkoutBranch(ctx context.Context, args map[string]interface{}) (*Result, error) {
	provider, owner, repoName, err := r.getRepoArgs(args)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	branch, _ := args["branch"].(string)
	if branch == "" {
		return &Result{Success: false, Error: "missing 'branch' parameter"}, nil
	}

	create, _ := args["create"].(bool)

	// Get repo path
	repoPath := r.manager.GetRepoPath(provider, owner, repoName)

	if create {
		if err := r.gitOps.CreateBranch(ctx, repoPath, branch); err != nil {
			return &Result{Success: false, Error: fmt.Sprintf("failed to create branch: %v", err)}, nil
		}
	} else {
		if err := r.gitOps.Checkout(ctx, repoPath, branch); err != nil {
			return &Result{Success: false, Error: fmt.Sprintf("failed to checkout branch: %v", err)}, nil
		}
	}

	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Checked out branch: %s", branch),
	}, nil
}

func (r *RepoTool) status(ctx context.Context, args map[string]interface{}) (*Result, error) {
	provider, owner, repoName, err := r.getRepoArgs(args)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	repoPath := r.manager.GetRepoPath(provider, owner, repoName)
	status, err := r.gitOps.Status(ctx, repoPath)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to get status: %v", err)}, nil
	}

	output, _ := json.MarshalIndent(status, "", "  ")
	return &Result{Success: true, Output: string(output)}, nil
}

func (r *RepoTool) validateChanges(ctx context.Context, args map[string]interface{}) (*Result, error) {
	provider, owner, repoName, err := r.getRepoArgs(args)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	repoPath := r.manager.GetRepoPath(provider, owner, repoName)

	result, err := r.hooksManager.ValidateBeforePush(repoPath)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("validation error: %v", err)}, nil
	}

	// Format for agent consumption
	feedback := r.hooksManager.FormatAgentFeedback(result)
	output, _ := json.MarshalIndent(feedback, "", "  ")

	return &Result{
		Success: result.AllPassed,
		Output:  result.Summary + "\n\n" + string(output),
	}, nil
}

func (r *RepoTool) commitChanges(ctx context.Context, args map[string]interface{}) (*Result, error) {
	provider, owner, repoName, err := r.getRepoArgs(args)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	message, _ := args["message"].(string)
	if message == "" {
		return &Result{Success: false, Error: "missing 'message' parameter"}, nil
	}

	repoPath := r.manager.GetRepoPath(provider, owner, repoName)

	// Run pre-commit hook
	hookResult, err := r.hooksManager.RunHook(repoPath, hooks.HookTypePreCommit)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("pre-commit hook error: %v", err)}, nil
	}

	if !hookResult.Passed {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("pre-commit hook failed: %s\n\n%s", hookResult.ErrorMsg, hookResult.Output),
		}, nil
	}

	// Stage files if specified, otherwise stage all
	files, ok := args["files"].([]interface{})
	if ok && len(files) > 0 {
		var fileStrings []string
		for _, f := range files {
			if fs, ok := f.(string); ok {
				fileStrings = append(fileStrings, fs)
			}
		}
		if err := r.gitOps.Add(ctx, repoPath, fileStrings...); err != nil {
			return &Result{Success: false, Error: fmt.Sprintf("failed to stage files: %v", err)}, nil
		}
	} else {
		if err := r.gitOps.AddAll(ctx, repoPath); err != nil {
			return &Result{Success: false, Error: fmt.Sprintf("failed to stage files: %v", err)}, nil
		}
	}

	// Commit
	if err := r.gitOps.Commit(ctx, repoPath, message); err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to commit: %v", err)}, nil
	}

	hash, _ := r.gitOps.GetHeadHash(ctx, repoPath)

	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Committed: %s\nCommit hash: %s", message, hash),
	}, nil
}

func (r *RepoTool) pushChanges(ctx context.Context, args map[string]interface{}) (*Result, error) {
	provider, owner, repoName, err := r.getRepoArgs(args)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	branch, _ := args["branch"].(string)
	if branch == "" {
		return &Result{Success: false, Error: "missing 'branch' parameter"}, nil
	}

	force, _ := args["force"].(bool)

	repoPath := r.manager.GetRepoPath(provider, owner, repoName)

	// Run pre-push validation
	result, err := r.hooksManager.ValidateBeforePush(repoPath)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("validation error: %v", err)}, nil
	}

	if !result.AllPassed {
		feedback := r.hooksManager.FormatAgentFeedback(result)
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("pre-push validation failed:\n%s", feedback.Suggestion),
		}, nil
	}

	// Push
	if force {
		if err := r.gitOps.PushForce(ctx, repoPath, "origin", branch); err != nil {
			return &Result{Success: false, Error: fmt.Sprintf("failed to push: %v", err)}, nil
		}
	} else {
		if err := r.gitOps.Push(ctx, repoPath, "origin", branch); err != nil {
			return &Result{Success: false, Error: fmt.Sprintf("failed to push: %v", err)}, nil
		}
	}

	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Pushed to origin/%s", branch),
	}, nil
}

func (r *RepoTool) configureHooks(ctx context.Context, args map[string]interface{}) (*Result, error) {
	provider, owner, repoName, err := r.getRepoArgs(args)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	configArg, ok := args["config"].(map[string]interface{})
	if !ok {
		return &Result{Success: false, Error: "missing 'config' parameter"}, nil
	}

	repoPath := r.manager.GetRepoPath(provider, owner, repoName)

	// Get existing config
	config, err := r.hooksManager.GetHooksConfig(repoPath)
	if err != nil {
		config = &hooks.HooksConfig{}
	}

	// Update config from args
	configJSON, _ := json.Marshal(configArg)
	json.Unmarshal(configJSON, config)

	// Save config
	if err := r.hooksManager.SetHooksConfig(repoPath, config); err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to save config: %v", err)}, nil
	}

	output, _ := json.MarshalIndent(config, "", "  ")
	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Updated hooks configuration:\n%s", string(output)),
	}, nil
}

func (r *RepoTool) runHook(ctx context.Context, args map[string]interface{}) (*Result, error) {
	provider, owner, repoName, err := r.getRepoArgs(args)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	hookType, _ := args["hook_type"].(string)
	if hookType == "" {
		return &Result{Success: false, Error: "missing 'hook_type' parameter"}, nil
	}

	repoPath := r.manager.GetRepoPath(provider, owner, repoName)

	result, err := r.hooksManager.RunHook(repoPath, hooks.HookType(hookType))
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("hook error: %v", err)}, nil
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return &Result{
		Success: result.Passed,
		Output:  string(output),
	}, nil
}

func (r *RepoTool) runCommand(ctx context.Context, args map[string]interface{}) (*Result, error) {
	provider, owner, repoName, err := r.getRepoArgs(args)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	command, _ := args["command"].(string)
	if command == "" {
		return &Result{Success: false, Error: "missing 'command' parameter"}, nil
	}

	var cmdArgs []string
	if argsInterface, ok := args["args"].([]interface{}); ok {
		for _, a := range argsInterface {
			if s, ok := a.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	}

	repoPath := r.manager.GetRepoPath(provider, owner, repoName)

	output, err := r.executor.Exec(ctx, repoPath, command, cmdArgs...)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("command failed: %v\n%s", err, string(output))}, nil
	}

	return &Result{Success: true, Output: string(output)}, nil
}
