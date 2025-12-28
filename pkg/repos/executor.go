package repos

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultExecutor implements Executor for running commands in repo context
type DefaultExecutor struct {
	defaultTimeout time.Duration
}

// NewExecutor creates a new executor
func NewExecutor() *DefaultExecutor {
	return &DefaultExecutor{
		defaultTimeout: 5 * time.Minute,
	}
}

// Exec runs a command in the repository directory
func (e *DefaultExecutor) Exec(ctx context.Context, repoPath, cmd string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, cmd, args...)
	command.Dir = repoPath
	return command.CombinedOutput()
}

// ExecWithEnv runs a command with additional environment variables
func (e *DefaultExecutor) ExecWithEnv(ctx context.Context, repoPath string, env []string, cmd string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, cmd, args...)
	command.Dir = repoPath
	command.Env = append(command.Environ(), env...)
	return command.CombinedOutput()
}

// ExecWithTimeout runs a command with a specific timeout
func (e *DefaultExecutor) ExecWithTimeout(ctx context.Context, repoPath string, timeout time.Duration, cmd string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return e.Exec(ctx, repoPath, cmd, args...)
}

// RunFormatter runs the appropriate formatter for the project type
func (e *DefaultExecutor) RunFormatter(ctx context.Context, repoPath, language string) error {
	var cmd string
	var args []string

	switch language {
	case "go":
		cmd = "gofmt"
		args = []string{"-w", "."}
	case "node", "javascript", "typescript":
		// Check if prettier is available
		if e.commandExists("npx") {
			cmd = "npx"
			args = []string{"prettier", "--write", "."}
		} else {
			return fmt.Errorf("prettier not available")
		}
	case "python":
		// Try black first, then autopep8
		if e.commandExists("black") {
			cmd = "black"
			args = []string{"."}
		} else if e.commandExists("autopep8") {
			cmd = "autopep8"
			args = []string{"--in-place", "--recursive", "."}
		} else {
			return fmt.Errorf("no python formatter available (black or autopep8)")
		}
	case "rust":
		cmd = "cargo"
		args = []string{"fmt"}
	default:
		return fmt.Errorf("no formatter configured for language: %s", language)
	}

	output, err := e.Exec(ctx, repoPath, cmd, args...)
	if err != nil {
		return fmt.Errorf("formatter failed: %w: %s", err, string(output))
	}
	return nil
}

// RunLinter runs the appropriate linter for the project type
func (e *DefaultExecutor) RunLinter(ctx context.Context, repoPath, language string) ([]LintResult, error) {
	var cmd string
	var args []string
	var parser func([]byte) []LintResult

	switch language {
	case "go":
		if e.commandExists("golangci-lint") {
			cmd = "golangci-lint"
			args = []string{"run", "--out-format=json"}
			parser = parseGolangciLintOutput
		} else {
			cmd = "go"
			args = []string{"vet", "./..."}
			parser = parseGoVetOutput
		}
	case "node", "javascript", "typescript":
		if e.commandExists("npx") {
			cmd = "npx"
			args = []string{"eslint", ".", "--format=json"}
			parser = parseEslintOutput
		} else {
			return nil, fmt.Errorf("eslint not available")
		}
	case "python":
		if e.commandExists("ruff") {
			cmd = "ruff"
			args = []string{"check", ".", "--output-format=json"}
			parser = parseRuffOutput
		} else if e.commandExists("pylint") {
			cmd = "pylint"
			args = []string{".", "--output-format=json"}
			parser = parsePylintOutput
		} else {
			return nil, fmt.Errorf("no python linter available (ruff or pylint)")
		}
	case "rust":
		cmd = "cargo"
		args = []string{"clippy", "--message-format=json"}
		parser = parseCargoClippyOutput
	default:
		return nil, fmt.Errorf("no linter configured for language: %s", language)
	}

	output, err := e.Exec(ctx, repoPath, cmd, args...)
	// Linters often return non-zero exit codes when they find issues
	// So we parse output even if there's an error
	if parser != nil {
		return parser(output), nil
	}

	if err != nil {
		return nil, fmt.Errorf("linter failed: %w: %s", err, string(output))
	}

	return nil, nil
}

// RunTests runs tests for the project
func (e *DefaultExecutor) RunTests(ctx context.Context, repoPath string) (*TestResult, error) {
	projectType := e.detectProjectType(repoPath)

	var cmd string
	var args []string

	switch projectType {
	case "go":
		cmd = "go"
		args = []string{"test", "-v", "-race", "./..."}
	case "node":
		cmd = "npm"
		args = []string{"test"}
	case "python":
		if e.commandExists("pytest") {
			cmd = "pytest"
			args = []string{"-v"}
		} else {
			cmd = "python"
			args = []string{"-m", "unittest", "discover", "-v"}
		}
	case "rust":
		cmd = "cargo"
		args = []string{"test"}
	default:
		return nil, fmt.Errorf("no test runner configured for project type: %s", projectType)
	}

	start := time.Now()
	output, err := e.Exec(ctx, repoPath, cmd, args...)
	duration := time.Since(start)

	result := &TestResult{
		Output:   string(output),
		Duration: duration,
	}

	// Parse output based on project type
	switch projectType {
	case "go":
		e.parseGoTestOutput(output, result)
	case "node":
		e.parseNpmTestOutput(output, result)
	case "python":
		e.parsePytestOutput(output, result)
	case "rust":
		e.parseCargoTestOutput(output, result)
	}

	// If command failed and we haven't detected failures, mark as failed
	if err != nil && result.FailedTests == 0 {
		result.Passed = false
		result.Failures = append(result.Failures, TestFailure{
			TestName: "unknown",
			Message:  err.Error(),
		})
	}

	return result, nil
}

// Helper methods

func (e *DefaultExecutor) commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func (e *DefaultExecutor) detectProjectType(repoPath string) string {
	// Quick detection based on common files
	checks := map[string]string{
		"go.mod":           "go",
		"package.json":     "node",
		"requirements.txt": "python",
		"pyproject.toml":   "python",
		"Cargo.toml":       "rust",
	}

	for file, projectType := range checks {
		cmd := exec.Command("test", "-f", file)
		cmd.Dir = repoPath
		if cmd.Run() == nil {
			return projectType
		}
	}

	return "unknown"
}

// Test output parsers

func (e *DefaultExecutor) parseGoTestOutput(output []byte, result *TestResult) {
	scanner := bufio.NewScanner(bytes.NewReader(output))

	passPattern := regexp.MustCompile(`^--- PASS: (\S+)`)
	failPattern := regexp.MustCompile(`^--- FAIL: (\S+)`)
	skipPattern := regexp.MustCompile(`^--- SKIP: (\S+)`)
	resultPattern := regexp.MustCompile(`^(ok|FAIL)\s+(\S+)`)

	for scanner.Scan() {
		line := scanner.Text()

		if matches := passPattern.FindStringSubmatch(line); len(matches) > 1 {
			result.PassedTests++
			result.TotalTests++
		} else if matches := failPattern.FindStringSubmatch(line); len(matches) > 1 {
			result.FailedTests++
			result.TotalTests++
			result.Failures = append(result.Failures, TestFailure{
				TestName: matches[1],
			})
		} else if matches := skipPattern.FindStringSubmatch(line); len(matches) > 1 {
			result.SkippedTests++
			result.TotalTests++
		} else if matches := resultPattern.FindStringSubmatch(line); len(matches) > 2 {
			if matches[1] == "FAIL" {
				result.Passed = false
			}
		}
	}

	if result.FailedTests == 0 && result.TotalTests > 0 {
		result.Passed = true
	}
}

func (e *DefaultExecutor) parseNpmTestOutput(output []byte, result *TestResult) {
	// Basic npm test output parsing
	// This is simplified - real parsing depends on test framework (jest, mocha, etc.)
	outputStr := string(output)

	if strings.Contains(outputStr, "FAIL") || strings.Contains(outputStr, "failed") {
		result.Passed = false
		result.FailedTests++
	} else if strings.Contains(outputStr, "PASS") || strings.Contains(outputStr, "passed") {
		result.Passed = true
	}

	// Try to extract counts from common patterns
	passPattern := regexp.MustCompile(`(\d+)\s+pass(?:ing|ed)?`)
	failPattern := regexp.MustCompile(`(\d+)\s+fail(?:ing|ed|ures?)?`)

	if matches := passPattern.FindStringSubmatch(outputStr); len(matches) > 1 {
		result.PassedTests, _ = strconv.Atoi(matches[1])
	}
	if matches := failPattern.FindStringSubmatch(outputStr); len(matches) > 1 {
		result.FailedTests, _ = strconv.Atoi(matches[1])
	}

	result.TotalTests = result.PassedTests + result.FailedTests + result.SkippedTests
}

func (e *DefaultExecutor) parsePytestOutput(output []byte, result *TestResult) {
	outputStr := string(output)

	// pytest summary line: "== 5 passed, 2 failed, 1 skipped =="
	summaryPattern := regexp.MustCompile(`(\d+)\s+passed(?:.*?(\d+)\s+failed)?(?:.*?(\d+)\s+skipped)?`)

	if matches := summaryPattern.FindStringSubmatch(outputStr); len(matches) > 1 {
		result.PassedTests, _ = strconv.Atoi(matches[1])
		if len(matches) > 2 && matches[2] != "" {
			result.FailedTests, _ = strconv.Atoi(matches[2])
		}
		if len(matches) > 3 && matches[3] != "" {
			result.SkippedTests, _ = strconv.Atoi(matches[3])
		}
	}

	result.TotalTests = result.PassedTests + result.FailedTests + result.SkippedTests
	result.Passed = result.FailedTests == 0
}

func (e *DefaultExecutor) parseCargoTestOutput(output []byte, result *TestResult) {
	outputStr := string(output)

	// Cargo test summary: "test result: ok. 10 passed; 0 failed; 0 ignored"
	summaryPattern := regexp.MustCompile(`test result: (ok|FAILED)\. (\d+) passed; (\d+) failed; (\d+) ignored`)

	if matches := summaryPattern.FindStringSubmatch(outputStr); len(matches) > 4 {
		result.Passed = matches[1] == "ok"
		result.PassedTests, _ = strconv.Atoi(matches[2])
		result.FailedTests, _ = strconv.Atoi(matches[3])
		result.SkippedTests, _ = strconv.Atoi(matches[4])
		result.TotalTests = result.PassedTests + result.FailedTests + result.SkippedTests
	}
}

// Linter output parsers

func parseGolangciLintOutput(output []byte) []LintResult {
	var report struct {
		Issues []struct {
			FromLinter string `json:"FromLinter"`
			Text       string `json:"Text"`
			Pos        struct {
				Filename string `json:"Filename"`
				Line     int    `json:"Line"`
				Column   int    `json:"Column"`
			} `json:"Pos"`
		} `json:"Issues"`
	}

	if err := json.Unmarshal(output, &report); err != nil {
		return nil
	}

	var results []LintResult
	for _, issue := range report.Issues {
		results = append(results, LintResult{
			File:     issue.Pos.Filename,
			Line:     issue.Pos.Line,
			Column:   issue.Pos.Column,
			Message:  issue.Text,
			Severity: "error",
			RuleID:   issue.FromLinter,
		})
	}
	return results
}

func parseGoVetOutput(output []byte) []LintResult {
	var results []LintResult

	// go vet output format: filename:line:column: message
	pattern := regexp.MustCompile(`^([^:]+):(\d+):(\d+):\s*(.+)$`)

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if matches := pattern.FindStringSubmatch(line); len(matches) > 4 {
			lineNum, _ := strconv.Atoi(matches[2])
			colNum, _ := strconv.Atoi(matches[3])
			results = append(results, LintResult{
				File:     matches[1],
				Line:     lineNum,
				Column:   colNum,
				Message:  matches[4],
				Severity: "error",
			})
		}
	}

	return results
}

func parseEslintOutput(output []byte) []LintResult {
	var report []struct {
		FilePath string `json:"filePath"`
		Messages []struct {
			RuleId   string `json:"ruleId"`
			Severity int    `json:"severity"`
			Message  string `json:"message"`
			Line     int    `json:"line"`
			Column   int    `json:"column"`
		} `json:"messages"`
	}

	if err := json.Unmarshal(output, &report); err != nil {
		return nil
	}

	var results []LintResult
	for _, file := range report {
		for _, msg := range file.Messages {
			severity := "warning"
			if msg.Severity == 2 {
				severity = "error"
			}
			results = append(results, LintResult{
				File:     file.FilePath,
				Line:     msg.Line,
				Column:   msg.Column,
				Message:  msg.Message,
				Severity: severity,
				RuleID:   msg.RuleId,
			})
		}
	}
	return results
}

func parseRuffOutput(output []byte) []LintResult {
	var report []struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Location struct {
			File   string `json:"file"`
			Row    int    `json:"row"`
			Column int    `json:"column"`
		} `json:"location"`
	}

	if err := json.Unmarshal(output, &report); err != nil {
		return nil
	}

	var results []LintResult
	for _, issue := range report {
		results = append(results, LintResult{
			File:     issue.Location.File,
			Line:     issue.Location.Row,
			Column:   issue.Location.Column,
			Message:  issue.Message,
			Severity: "error",
			RuleID:   issue.Code,
		})
	}
	return results
}

func parsePylintOutput(output []byte) []LintResult {
	var report []struct {
		Type    string `json:"type"`
		Module  string `json:"module"`
		Line    int    `json:"line"`
		Column  int    `json:"column"`
		Message string `json:"message"`
		Symbol  string `json:"symbol"`
	}

	if err := json.Unmarshal(output, &report); err != nil {
		return nil
	}

	var results []LintResult
	for _, issue := range report {
		severity := "warning"
		if issue.Type == "error" || issue.Type == "fatal" {
			severity = "error"
		}
		results = append(results, LintResult{
			File:     issue.Module,
			Line:     issue.Line,
			Column:   issue.Column,
			Message:  issue.Message,
			Severity: severity,
			RuleID:   issue.Symbol,
		})
	}
	return results
}

func parseCargoClippyOutput(output []byte) []LintResult {
	var results []LintResult

	// Cargo clippy JSON output is newline-delimited JSON
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		var msg struct {
			Reason  string `json:"reason"`
			Message struct {
				Code    *struct{ Code string } `json:"code"`
				Level   string                 `json:"level"`
				Message string                 `json:"message"`
				Spans   []struct {
					FileName    string `json:"file_name"`
					LineStart   int    `json:"line_start"`
					ColumnStart int    `json:"column_start"`
				} `json:"spans"`
			} `json:"message"`
		}

		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}

		if msg.Reason != "compiler-message" {
			continue
		}

		if len(msg.Message.Spans) > 0 {
			span := msg.Message.Spans[0]
			ruleID := ""
			if msg.Message.Code != nil {
				ruleID = msg.Message.Code.Code
			}
			results = append(results, LintResult{
				File:     span.FileName,
				Line:     span.LineStart,
				Column:   span.ColumnStart,
				Message:  msg.Message.Message,
				Severity: msg.Message.Level,
				RuleID:   ruleID,
			})
		}
	}

	return results
}

// Ensure DefaultExecutor implements Executor
var _ Executor = (*DefaultExecutor)(nil)
