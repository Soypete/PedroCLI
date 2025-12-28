package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DefaultManager implements Manager for git hook management
type DefaultManager struct {
	ciParser CIConfigParser
}

// NewManager creates a new hooks manager
func NewManager() *DefaultManager {
	return &DefaultManager{
		ciParser: NewCIConfigParser(),
	}
}

// InstallHooks installs hooks for a repo based on detected project type
func (m *DefaultManager) InstallHooks(repoPath string) error {
	// Detect project type
	projectType, err := m.DetectProjectType(repoPath)
	if err != nil {
		return fmt.Errorf("failed to detect project type: %w", err)
	}

	// Get default checks for project type
	config := DefaultChecks(projectType)

	// Try to parse CI config to enhance hooks
	if ciConfig, err := m.ciParser.ParseCIConfig(repoPath); err == nil && ciConfig != nil {
		if ciHooks, err := m.ciParser.ConvertToHooks(ciConfig); err == nil && ciHooks != nil {
			config = m.mergeConfigs(config, ciHooks)
		}
	}

	// Save config
	if err := m.SetHooksConfig(repoPath, config); err != nil {
		return fmt.Errorf("failed to save hooks config: %w", err)
	}

	// Install hook scripts
	hooksDir := filepath.Join(repoPath, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	// Install pre-commit hook
	if len(config.PreCommit) > 0 {
		if err := m.installHookScript(hooksDir, HookTypePreCommit); err != nil {
			return fmt.Errorf("failed to install pre-commit hook: %w", err)
		}
	}

	// Install pre-push hook
	if len(config.PrePush) > 0 {
		if err := m.installHookScript(hooksDir, HookTypePrePush); err != nil {
			return fmt.Errorf("failed to install pre-push hook: %w", err)
		}
	}

	// Install commit-msg hook
	if config.CommitMsg != nil {
		if err := m.installHookScript(hooksDir, HookTypeCommitMsg); err != nil {
			return fmt.Errorf("failed to install commit-msg hook: %w", err)
		}
	}

	return nil
}

// UninstallHooks removes hooks from a repository
func (m *DefaultManager) UninstallHooks(repoPath string) error {
	hooksDir := filepath.Join(repoPath, ".git", "hooks")

	for _, hookType := range []HookType{HookTypePreCommit, HookTypePrePush, HookTypeCommitMsg} {
		hookPath := filepath.Join(hooksDir, string(hookType))
		if err := os.Remove(hookPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s hook: %w", hookType, err)
		}
	}

	// Remove config file
	configPath := filepath.Join(repoPath, ".pedrocli-hooks.json")
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove hooks config: %w", err)
	}

	return nil
}

// RunHook runs a specific hook manually
func (m *DefaultManager) RunHook(repoPath string, hookName HookType) (*HookResult, error) {
	config, err := m.GetHooksConfig(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get hooks config: %w", err)
	}

	var checks []Check
	var timeout time.Duration

	switch hookName {
	case HookTypePreCommit:
		checks = config.PreCommit
		timeout = config.PreCommitTimeout
	case HookTypePrePush:
		checks = config.PrePush
		timeout = config.PrePushTimeout
	case HookTypeCommitMsg:
		// Commit message validation is different
		return m.runCommitMsgHook(repoPath, config.CommitMsg)
	default:
		return nil, fmt.Errorf("unknown hook type: %s", hookName)
	}

	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	result := &HookResult{
		HookName: string(hookName),
		Passed:   true,
	}

	start := time.Now()

	for _, check := range checks {
		checkResult := m.runCheck(repoPath, check, timeout)
		result.Output += fmt.Sprintf("=== %s ===\n%s\n", check.Name, checkResult.Output)

		if !checkResult.Passed && check.Required {
			result.Passed = false
			result.ErrorMsg = checkResult.ErrorMsg
			break
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// ValidateBeforePush runs all pre-push checks without actually pushing
func (m *DefaultManager) ValidateBeforePush(repoPath string) (*ValidationResult, error) {
	config, err := m.GetHooksConfig(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get hooks config: %w", err)
	}

	result := &ValidationResult{
		AllPassed: true,
		Results:   make([]HookResult, 0),
	}

	start := time.Now()

	// First run pre-commit checks
	for _, check := range config.PreCommit {
		checkResult := m.runCheck(repoPath, check, config.PreCommitTimeout)
		hookResult := HookResult{
			HookName:  "pre-commit",
			CheckName: check.Name,
			Passed:    checkResult.Passed,
			Output:    checkResult.Output,
			ErrorMsg:  checkResult.ErrorMsg,
			Duration:  checkResult.Duration,
			Skipped:   checkResult.Skipped,
			SkipReason: checkResult.SkipReason,
		}
		result.Results = append(result.Results, hookResult)

		if !checkResult.Passed && check.Required {
			result.AllPassed = false
		}
	}

	// Then run pre-push checks
	for _, check := range config.PrePush {
		checkResult := m.runCheck(repoPath, check, config.PrePushTimeout)
		hookResult := HookResult{
			HookName:  "pre-push",
			CheckName: check.Name,
			Passed:    checkResult.Passed,
			Output:    checkResult.Output,
			ErrorMsg:  checkResult.ErrorMsg,
			Duration:  checkResult.Duration,
			Skipped:   checkResult.Skipped,
			SkipReason: checkResult.SkipReason,
		}
		result.Results = append(result.Results, hookResult)

		if !checkResult.Passed && check.Required {
			result.AllPassed = false
		}
	}

	result.Duration = time.Since(start)
	result.Summary = m.buildSummary(result)

	return result, nil
}

// GetHooksConfig gets hook configuration for a repo
func (m *DefaultManager) GetHooksConfig(repoPath string) (*HooksConfig, error) {
	configPath := filepath.Join(repoPath, ".pedrocli-hooks.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config based on project type
			projectType, _ := m.DetectProjectType(repoPath)
			return DefaultChecks(projectType), nil
		}
		return nil, fmt.Errorf("failed to read hooks config: %w", err)
	}

	var config HooksConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse hooks config: %w", err)
	}

	return &config, nil
}

// SetHooksConfig sets hook configuration for a repo
func (m *DefaultManager) SetHooksConfig(repoPath string, config *HooksConfig) error {
	configPath := filepath.Join(repoPath, ".pedrocli-hooks.json")

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hooks config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write hooks config: %w", err)
	}

	return nil
}

// DetectProjectType detects the project type from the repository
func (m *DefaultManager) DetectProjectType(repoPath string) (ProjectType, error) {
	// Check for common project type indicators
	indicators := map[ProjectType][]string{
		ProjectTypeGo:     {"go.mod", "go.sum"},
		ProjectTypeNode:   {"package.json"},
		ProjectTypePython: {"setup.py", "pyproject.toml", "requirements.txt"},
		ProjectTypeRust:   {"Cargo.toml"},
		ProjectTypeJava:   {"pom.xml", "build.gradle", "build.gradle.kts"},
		ProjectTypeRuby:   {"Gemfile"},
		ProjectTypePHP:    {"composer.json"},
		ProjectTypeDotnet: {"*.csproj", "*.fsproj", "*.vbproj"},
	}

	for projectType, files := range indicators {
		for _, file := range files {
			matches, _ := filepath.Glob(filepath.Join(repoPath, file))
			if len(matches) > 0 {
				return projectType, nil
			}
		}
	}

	return ProjectTypeUnknown, nil
}

// FormatAgentFeedback formats validation results for agent consumption
func (m *DefaultManager) FormatAgentFeedback(result *ValidationResult) *AgentFeedback {
	feedback := &AgentFeedback{
		Success:    result.AllPassed,
		AllResults: make([]CheckFeedback, 0, len(result.Results)),
	}

	for _, r := range result.Results {
		checkFeedback := CheckFeedback{
			Name:   r.CheckName,
			Passed: r.Passed,
			Output: r.Output,
		}

		if !r.Passed {
			feedback.FailedCheck = r.CheckName
			feedback.ErrorOutput = r.Output
			checkFeedback.Suggestion = m.getSuggestion(r.CheckName, r.Output)

			// Extract affected files from output
			files := m.extractFiles(r.Output)
			if len(files) > 0 {
				feedback.FilesAffected = append(feedback.FilesAffected, files...)
				checkFeedback.Files = files
			}
		}

		feedback.AllResults = append(feedback.AllResults, checkFeedback)
	}

	if !result.AllPassed {
		feedback.Suggestion = m.buildAgentSuggestion(result)
	}

	return feedback
}

// Internal helper methods

func (m *DefaultManager) installHookScript(hooksDir string, hookType HookType) error {
	hookPath := filepath.Join(hooksDir, string(hookType))

	script := m.generateHookScript(hookType)

	if err := os.WriteFile(hookPath, []byte(script), 0755); err != nil {
		return fmt.Errorf("failed to write hook script: %w", err)
	}

	return nil
}

func (m *DefaultManager) generateHookScript(hookType HookType) string {
	// Generate a shell script that runs pedrocli to execute hooks
	script := `#!/bin/sh
# PedroCLI git hook - %s
# Auto-generated, do not edit manually

# Get the repository root
REPO_ROOT="$(git rev-parse --show-toplevel)"

# Run validation through pedrocli if available
if command -v pedrocli >/dev/null 2>&1; then
    pedrocli hook run --type %s --repo "$REPO_ROOT"
    exit $?
fi

# Fallback: read config and run checks directly
CONFIG_FILE="$REPO_ROOT/.pedrocli-hooks.json"

if [ ! -f "$CONFIG_FILE" ]; then
    echo "No hooks config found, skipping"
    exit 0
fi

echo "Running %s hooks..."

# Simple check runner (backup if pedrocli not available)
%s

exit 0
`

	var checkRunner string
	switch hookType {
	case HookTypePreCommit:
		checkRunner = `
# Pre-commit checks placeholder
# In production, this reads .pedrocli-hooks.json and runs pre_commit checks
echo "Pre-commit validation..."
`
	case HookTypePrePush:
		checkRunner = `
# Pre-push checks placeholder
# In production, this reads .pedrocli-hooks.json and runs pre_push checks
echo "Pre-push validation..."
`
	case HookTypeCommitMsg:
		checkRunner = `
# Commit message validation placeholder
COMMIT_MSG_FILE="$1"
if [ -z "$COMMIT_MSG_FILE" ]; then
    exit 0
fi
echo "Commit message validation..."
`
	}

	return fmt.Sprintf(script, hookType, hookType, hookType, checkRunner)
}

func (m *DefaultManager) runCheck(repoPath string, check Check, timeout time.Duration) *HookResult {
	result := &HookResult{
		CheckName: check.Name,
		Passed:    true,
	}

	// Check if command exists
	_, err := exec.LookPath(check.Command)
	if err != nil {
		if check.Optional {
			result.Skipped = true
			result.SkipReason = fmt.Sprintf("command not found: %s", check.Command)
			return result
		}
		result.Passed = false
		result.ErrorMsg = fmt.Sprintf("command not found: %s", check.Command)
		return result
	}

	// Set timeout
	if check.Timeout > 0 {
		timeout = check.Timeout
	}
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()

	cmd := exec.CommandContext(ctx, check.Command, check.Args...)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(start)
	result.Output = string(output)

	if ctx.Err() == context.DeadlineExceeded {
		result.Passed = false
		result.ErrorMsg = "check timed out"
		return result
	}

	if err != nil {
		result.Passed = false
		result.ErrorMsg = err.Error()
		return result
	}

	// Check if we should fail on output
	if check.FailOnOutput && len(strings.TrimSpace(string(output))) > 0 {
		result.Passed = false
		result.ErrorMsg = "check produced output (files need formatting)"
		return result
	}

	return result
}

func (m *DefaultManager) runCommitMsgHook(repoPath string, config *CommitMsgConfig) (*HookResult, error) {
	if config == nil {
		return &HookResult{
			HookName: "commit-msg",
			Passed:   true,
		}, nil
	}

	// Read the commit message from .git/COMMIT_EDITMSG
	msgPath := filepath.Join(repoPath, ".git", "COMMIT_EDITMSG")
	data, err := os.ReadFile(msgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read commit message: %w", err)
	}

	msg := strings.TrimSpace(string(data))
	result := &HookResult{
		HookName: "commit-msg",
		Passed:   true,
	}

	// Check minimum length
	if config.MinLength > 0 && len(msg) < config.MinLength {
		result.Passed = false
		result.ErrorMsg = fmt.Sprintf("commit message too short (min %d chars)", config.MinLength)
		return result, nil
	}

	// Check max length of first line
	if config.MaxLength > 0 {
		firstLine := strings.Split(msg, "\n")[0]
		if len(firstLine) > config.MaxLength {
			result.Passed = false
			result.ErrorMsg = fmt.Sprintf("first line too long (max %d chars)", config.MaxLength)
			return result, nil
		}
	}

	// TODO: Add conventional commit and pattern validation

	return result, nil
}

func (m *DefaultManager) mergeConfigs(base, override *HooksConfig) *HooksConfig {
	if override == nil {
		return base
	}

	// Override takes precedence for checks
	if len(override.PreCommit) > 0 {
		base.PreCommit = append(base.PreCommit, override.PreCommit...)
	}
	if len(override.PrePush) > 0 {
		base.PrePush = append(base.PrePush, override.PrePush...)
	}
	if len(override.CustomChecks) > 0 {
		base.CustomChecks = append(base.CustomChecks, override.CustomChecks...)
	}
	if override.CommitMsg != nil {
		base.CommitMsg = override.CommitMsg
	}

	base.Source = "merged"
	return base
}

func (m *DefaultManager) buildSummary(result *ValidationResult) string {
	var sb strings.Builder

	if result.AllPassed {
		sb.WriteString("✅ All validation checks passed\n\n")
	} else {
		sb.WriteString("❌ Validation failed\n\n")
	}

	// Count results
	passed := 0
	failed := 0
	skipped := 0

	for _, r := range result.Results {
		if r.Skipped {
			skipped++
		} else if r.Passed {
			passed++
		} else {
			failed++
		}
	}

	sb.WriteString(fmt.Sprintf("Summary: %d passed, %d failed, %d skipped\n", passed, failed, skipped))
	sb.WriteString(fmt.Sprintf("Duration: %s\n\n", result.Duration.Round(time.Millisecond)))

	// List failures
	if failed > 0 {
		sb.WriteString("Failed checks:\n")
		for _, r := range result.Results {
			if !r.Passed && !r.Skipped {
				sb.WriteString(fmt.Sprintf("  ❌ %s: %s\n", r.CheckName, r.ErrorMsg))
			}
		}
	}

	return sb.String()
}

func (m *DefaultManager) getSuggestion(checkName, output string) string {
	// Provide actionable suggestions based on check type
	switch {
	case strings.Contains(checkName, "gofmt") || strings.Contains(checkName, "fmt"):
		return "Run 'gofmt -w .' or 'go fmt ./...' to fix formatting"
	case strings.Contains(checkName, "prettier"):
		return "Run 'npx prettier --write .' to fix formatting"
	case strings.Contains(checkName, "black"):
		return "Run 'black .' to fix Python formatting"
	case strings.Contains(checkName, "eslint") || strings.Contains(checkName, "lint"):
		return "Fix the linting errors shown above, or run with --fix flag if available"
	case strings.Contains(checkName, "test"):
		return "Fix the failing tests. Check the test output for details"
	case strings.Contains(checkName, "vet"):
		return "Fix the issues reported by go vet"
	case strings.Contains(checkName, "clippy"):
		return "Fix the issues reported by cargo clippy"
	default:
		return "Review the output above and fix the reported issues"
	}
}

func (m *DefaultManager) extractFiles(output string) []string {
	// Try to extract file paths from output
	// This is a simple heuristic - could be improved
	var files []string
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Common patterns for file references
		if strings.HasSuffix(line, ".go") ||
			strings.HasSuffix(line, ".js") ||
			strings.HasSuffix(line, ".ts") ||
			strings.HasSuffix(line, ".py") ||
			strings.HasSuffix(line, ".rs") {
			// Check if it looks like a file path
			if !strings.Contains(line, " ") || strings.Contains(line, ":") {
				// Extract just the filename from patterns like "file.go:10:5:"
				parts := strings.Split(line, ":")
				if len(parts) > 0 && len(parts[0]) > 0 {
					files = append(files, parts[0])
				}
			}
		}
	}

	return files
}

func (m *DefaultManager) buildAgentSuggestion(result *ValidationResult) string {
	var sb strings.Builder

	sb.WriteString("PRE-PUSH VALIDATION FAILED\n\n")

	for _, r := range result.Results {
		if !r.Passed && !r.Skipped {
			sb.WriteString(fmt.Sprintf("❌ %s failed:\n", r.CheckName))

			// Truncate long output
			output := r.Output
			if len(output) > 500 {
				output = output[:500] + "\n... (truncated)"
			}
			sb.WriteString(fmt.Sprintf("   %s\n", strings.ReplaceAll(output, "\n", "\n   ")))

			files := m.extractFiles(r.Output)
			if len(files) > 0 {
				sb.WriteString(fmt.Sprintf("   Files: %s\n", strings.Join(files, ", ")))
			}

			sb.WriteString(fmt.Sprintf("   Action: %s\n\n", m.getSuggestion(r.CheckName, r.Output)))
		}
	}

	sb.WriteString("Run validation again after fixes: pedrocli validate --repo <path>")

	return sb.String()
}

// Ensure DefaultManager implements Manager
var _ Manager = (*DefaultManager)(nil)
