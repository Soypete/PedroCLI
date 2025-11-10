package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GitTool provides git operations
type GitTool struct {
	workDir string
}

// NewGitTool creates a new git tool
func NewGitTool(workDir string) *GitTool {
	return &GitTool{
		workDir: workDir,
	}
}

// Name returns the tool name
func (g *GitTool) Name() string {
	return "git"
}

// Description returns the tool description
func (g *GitTool) Description() string {
	return "Execute git commands for version control"
}

// Execute executes the git tool
func (g *GitTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	action, ok := args["action"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'action' parameter"}, nil
	}

	switch action {
	case "status":
		return g.status(ctx)
	case "diff":
		return g.diff(ctx, args)
	case "add":
		return g.add(ctx, args)
	case "commit":
		return g.commit(ctx, args)
	case "push":
		return g.push(ctx, args)
	case "checkout":
		return g.checkout(ctx, args)
	case "create_branch":
		return g.createBranch(ctx, args)
	default:
		return &Result{Success: false, Error: fmt.Sprintf("unknown action: %s", action)}, nil
	}
}

// status runs git status
func (g *GitTool) status(ctx context.Context) (*Result, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--short")
	cmd.Dir = g.workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// diff runs git diff
func (g *GitTool) diff(ctx context.Context, args map[string]interface{}) (*Result, error) {
	cmdArgs := []string{"diff"}

	// Optional: compare branch against base
	if base, ok := args["base"].(string); ok {
		if branch, ok := args["branch"].(string); ok {
			// Diff between base and branch: git diff base...branch
			cmdArgs = append(cmdArgs, fmt.Sprintf("%s...%s", base, branch))
		} else {
			// Diff against base: git diff base
			cmdArgs = append(cmdArgs, base)
		}
	}

	// Optional: specific files
	if fileList, ok := args["files"].([]interface{}); ok {
		files := []string{}
		for _, f := range fileList {
			if file, ok := f.(string); ok {
				files = append(files, file)
			}
		}
		if len(files) > 0 {
			cmdArgs = append(cmdArgs, "--")
			cmdArgs = append(cmdArgs, files...)
		}
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Dir = g.workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// add runs git add
func (g *GitTool) add(ctx context.Context, args map[string]interface{}) (*Result, error) {
	files, ok := args["files"].([]interface{})
	if !ok || len(files) == 0 {
		return &Result{Success: false, Error: "missing 'files' parameter"}, nil
	}

	var fileStrings []string
	for _, f := range files {
		if file, ok := f.(string); ok {
			fileStrings = append(fileStrings, file)
		}
	}

	cmdArgs := append([]string{"add"}, fileStrings...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Dir = g.workDir

	_, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Added %d files", len(fileStrings)),
	}, nil
}

// commit runs git commit
func (g *GitTool) commit(ctx context.Context, args map[string]interface{}) (*Result, error) {
	message, ok := args["message"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'message' parameter"}, nil
	}

	cmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	cmd.Dir = g.workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("%s: %s", err, string(output))}, nil
	}

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// push runs git push
func (g *GitTool) push(ctx context.Context, args map[string]interface{}) (*Result, error) {
	remote := "origin"
	if r, ok := args["remote"].(string); ok {
		remote = r
	}

	branch, ok := args["branch"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'branch' parameter"}, nil
	}

	cmd := exec.CommandContext(ctx, "git", "push", "-u", remote, branch)
	cmd.Dir = g.workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("%s: %s", err, string(output))}, nil
	}

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// checkout runs git checkout
func (g *GitTool) checkout(ctx context.Context, args map[string]interface{}) (*Result, error) {
	branch, ok := args["branch"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'branch' parameter"}, nil
	}

	cmd := exec.CommandContext(ctx, "git", "checkout", branch)
	cmd.Dir = g.workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("%s: %s", err, string(output))}, nil
	}

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// createBranch creates a new branch
func (g *GitTool) createBranch(ctx context.Context, args map[string]interface{}) (*Result, error) {
	branch, ok := args["branch"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'branch' parameter"}, nil
	}

	cmd := exec.CommandContext(ctx, "git", "checkout", "-b", branch)
	cmd.Dir = g.workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("%s: %s", err, string(output))}, nil
	}

	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Created and checked out branch: %s", branch),
	}, nil
}

// CreatePR creates a pull request using GitHub CLI
func (g *GitTool) CreatePR(ctx context.Context, title, body string, draft bool) (*Result, error) {
	args := []string{"pr", "create", "--title", title, "--body", body}
	if draft {
		args = append(args, "--draft")
	}

	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = g.workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("%s: %s", err, string(output))}, nil
	}

	return &Result{
		Success: true,
		Output:  strings.TrimSpace(string(output)),
	}, nil
}
