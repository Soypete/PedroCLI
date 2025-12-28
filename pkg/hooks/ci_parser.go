package hooks

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultCIConfigParser implements CIConfigParser
type DefaultCIConfigParser struct{}

// NewCIConfigParser creates a new CI config parser
func NewCIConfigParser() *DefaultCIConfigParser {
	return &DefaultCIConfigParser{}
}

// ParseCIConfig detects and parses CI config from a repo
func (p *DefaultCIConfigParser) ParseCIConfig(repoPath string) (*CIConfig, error) {
	// Check for CI configs in order of preference
	parsers := []struct {
		path   string
		source string
		parse  func([]byte) ([]CIStep, error)
	}{
		{".github/workflows/*.yml", "github_actions", p.parseGitHubActions},
		{".github/workflows/*.yaml", "github_actions", p.parseGitHubActions},
		{".gitlab-ci.yml", "gitlab_ci", p.parseGitLabCI},
		{".circleci/config.yml", "circle_ci", p.parseCircleCI},
		{"Makefile", "makefile", p.parseMakefile},
		{"justfile", "justfile", p.parseJustfile},
	}

	for _, parser := range parsers {
		matches, _ := filepath.Glob(filepath.Join(repoPath, parser.path))
		if len(matches) == 0 {
			continue
		}

		// For glob patterns, read the first matching file
		configPath := matches[0]
		data, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}

		steps, err := parser.parse(data)
		if err != nil {
			continue
		}

		return &CIConfig{
			Source:      parser.source,
			RawConfig:   data,
			ParsedSteps: steps,
		}, nil
	}

	return nil, fmt.Errorf("no CI configuration found")
}

// ConvertToHooks converts CI steps to local hooks
func (p *DefaultCIConfigParser) ConvertToHooks(ciConfig *CIConfig) (*HooksConfig, error) {
	if ciConfig == nil || len(ciConfig.ParsedSteps) == 0 {
		return nil, fmt.Errorf("no CI steps to convert")
	}

	config := &HooksConfig{
		Source: "ci_parsed",
	}

	for _, step := range ciConfig.ParsedSteps {
		checks := p.stepToChecks(step)

		// Categorize checks based on name/content
		for _, check := range checks {
			if p.isPreCommitCheck(check) {
				config.PreCommit = append(config.PreCommit, check)
			} else {
				config.PrePush = append(config.PrePush, check)
			}
		}
	}

	return config, nil
}

// SupportsFormat checks if a CI format is supported
func (p *DefaultCIConfigParser) SupportsFormat(format string) bool {
	supported := []string{
		"github_actions",
		"gitlab_ci",
		"circle_ci",
		"makefile",
		"justfile",
	}

	for _, f := range supported {
		if f == format {
			return true
		}
	}
	return false
}

// GitHub Actions parser

type githubWorkflow struct {
	Jobs map[string]struct {
		Steps []struct {
			Name string                 `yaml:"name"`
			Run  string                 `yaml:"run"`
			With map[string]interface{} `yaml:"with"`
			Uses string                 `yaml:"uses"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func (p *DefaultCIConfigParser) parseGitHubActions(data []byte) ([]CIStep, error) {
	var workflow githubWorkflow
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub Actions workflow: %w", err)
	}

	var steps []CIStep

	for _, job := range workflow.Jobs {
		for _, step := range job.Steps {
			if step.Run == "" {
				continue // Skip non-run steps (uses actions)
			}

			// Parse multi-line run commands
			commands := strings.Split(step.Run, "\n")
			var cleanCommands []string
			for _, cmd := range commands {
				cmd = strings.TrimSpace(cmd)
				if cmd != "" && !strings.HasPrefix(cmd, "#") {
					cleanCommands = append(cleanCommands, cmd)
				}
			}

			if len(cleanCommands) > 0 {
				steps = append(steps, CIStep{
					Name:     step.Name,
					Commands: cleanCommands,
				})
			}
		}
	}

	return steps, nil
}

// GitLab CI parser

type gitlabCI struct {
	Jobs map[string]struct {
		Script       []string          `yaml:"script"`
		BeforeScript []string          `yaml:"before_script"`
		Variables    map[string]string `yaml:"variables"`
	} `yaml:",inline"`
}

func (p *DefaultCIConfigParser) parseGitLabCI(data []byte) ([]CIStep, error) {
	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse GitLab CI config: %w", err)
	}

	var steps []CIStep

	for name, jobData := range config {
		// Skip special keys
		if strings.HasPrefix(name, ".") || name == "stages" || name == "variables" || name == "default" {
			continue
		}

		job, ok := jobData.(map[string]interface{})
		if !ok {
			continue
		}

		var commands []string

		// Get script commands
		if script, ok := job["script"].([]interface{}); ok {
			for _, cmd := range script {
				if cmdStr, ok := cmd.(string); ok {
					commands = append(commands, cmdStr)
				}
			}
		}

		if len(commands) > 0 {
			env := make(map[string]string)
			if vars, ok := job["variables"].(map[string]interface{}); ok {
				for k, v := range vars {
					if vStr, ok := v.(string); ok {
						env[k] = vStr
					}
				}
			}

			steps = append(steps, CIStep{
				Name:     name,
				Commands: commands,
				Env:      env,
			})
		}
	}

	return steps, nil
}

// CircleCI parser

type circleCI struct {
	Jobs map[string]struct {
		Steps []interface{} `yaml:"steps"`
	} `yaml:"jobs"`
}

func (p *DefaultCIConfigParser) parseCircleCI(data []byte) ([]CIStep, error) {
	var config circleCI
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse CircleCI config: %w", err)
	}

	var steps []CIStep

	for jobName, job := range config.Jobs {
		for i, step := range job.Steps {
			stepMap, ok := step.(map[string]interface{})
			if !ok {
				continue
			}

			if run, ok := stepMap["run"]; ok {
				var commands []string
				var name string

				switch r := run.(type) {
				case string:
					commands = []string{r}
					name = fmt.Sprintf("%s_step_%d", jobName, i)
				case map[string]interface{}:
					if cmd, ok := r["command"].(string); ok {
						commands = strings.Split(cmd, "\n")
					}
					if n, ok := r["name"].(string); ok {
						name = n
					}
				}

				if len(commands) > 0 {
					steps = append(steps, CIStep{
						Name:     name,
						Commands: commands,
					})
				}
			}
		}
	}

	return steps, nil
}

// Makefile parser

func (p *DefaultCIConfigParser) parseMakefile(data []byte) ([]CIStep, error) {
	var steps []CIStep

	// Simple Makefile target parser
	// Looks for common targets: test, lint, build, fmt, check
	targetRegex := regexp.MustCompile(`^([a-zA-Z_-]+):.*$`)
	commonTargets := map[string]bool{
		"test":   true,
		"lint":   true,
		"build":  true,
		"fmt":    true,
		"format": true,
		"check":  true,
		"vet":    true,
		"verify": true,
		"all":    true,
		"ci":     true,
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var currentTarget string
	var currentCommands []string

	for scanner.Scan() {
		line := scanner.Text()

		// Check for target definition
		if matches := targetRegex.FindStringSubmatch(line); len(matches) > 1 {
			// Save previous target if it had commands
			if currentTarget != "" && len(currentCommands) > 0 {
				steps = append(steps, CIStep{
					Name:     currentTarget,
					Commands: currentCommands,
				})
			}

			// Start new target
			target := matches[1]
			if commonTargets[target] {
				currentTarget = target
				currentCommands = nil
			} else {
				currentTarget = ""
			}
			continue
		}

		// Check for command (starts with tab)
		if currentTarget != "" && strings.HasPrefix(line, "\t") {
			cmd := strings.TrimPrefix(line, "\t")
			cmd = strings.TrimSpace(cmd)
			if cmd != "" && !strings.HasPrefix(cmd, "@echo") {
				// Remove @ prefix if present
				cmd = strings.TrimPrefix(cmd, "@")
				currentCommands = append(currentCommands, cmd)
			}
		}
	}

	// Save last target
	if currentTarget != "" && len(currentCommands) > 0 {
		steps = append(steps, CIStep{
			Name:     currentTarget,
			Commands: currentCommands,
		})
	}

	return steps, nil
}

// Justfile parser

func (p *DefaultCIConfigParser) parseJustfile(data []byte) ([]CIStep, error) {
	var steps []CIStep

	// Simple justfile parser
	recipeRegex := regexp.MustCompile(`^([a-zA-Z_-]+):.*$`)
	commonRecipes := map[string]bool{
		"test":   true,
		"lint":   true,
		"build":  true,
		"fmt":    true,
		"format": true,
		"check":  true,
		"verify": true,
		"ci":     true,
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var currentRecipe string
	var currentCommands []string

	for scanner.Scan() {
		line := scanner.Text()

		// Check for recipe definition
		if matches := recipeRegex.FindStringSubmatch(line); len(matches) > 1 {
			// Save previous recipe
			if currentRecipe != "" && len(currentCommands) > 0 {
				steps = append(steps, CIStep{
					Name:     currentRecipe,
					Commands: currentCommands,
				})
			}

			recipe := matches[1]
			if commonRecipes[recipe] {
				currentRecipe = recipe
				currentCommands = nil
			} else {
				currentRecipe = ""
			}
			continue
		}

		// Check for command (indented)
		if currentRecipe != "" && (strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t")) {
			cmd := strings.TrimSpace(line)
			if cmd != "" && !strings.HasPrefix(cmd, "#") && !strings.HasPrefix(cmd, "@echo") {
				// Remove @ prefix if present
				cmd = strings.TrimPrefix(cmd, "@")
				currentCommands = append(currentCommands, cmd)
			}
		}
	}

	// Save last recipe
	if currentRecipe != "" && len(currentCommands) > 0 {
		steps = append(steps, CIStep{
			Name:     currentRecipe,
			Commands: currentCommands,
		})
	}

	return steps, nil
}

// Helper methods

func (p *DefaultCIConfigParser) stepToChecks(step CIStep) []Check {
	var checks []Check

	for _, cmd := range step.Commands {
		check := p.commandToCheck(step.Name, cmd)
		if check != nil {
			checks = append(checks, *check)
		}
	}

	return checks
}

func (p *DefaultCIConfigParser) commandToCheck(stepName, cmd string) *Check {
	// Clean up command
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}

	// Parse command and args
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil
	}

	command := parts[0]
	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}

	// Determine if this is a required check
	required := true
	optional := false

	// Some commands should be optional if not installed
	optionalCommands := map[string]bool{
		"golangci-lint": true,
		"prettier":      true,
		"eslint":        true,
		"ruff":          true,
		"mypy":          true,
		"clippy":        true,
	}

	if optionalCommands[command] {
		optional = true
	}

	// Determine timeout based on command type
	timeout := 30 * time.Second
	if strings.Contains(cmd, "test") || strings.Contains(cmd, "build") {
		timeout = 5 * time.Minute
	}

	return &Check{
		Name:     stepName,
		Command:  command,
		Args:     args,
		Required: required,
		Optional: optional,
		Timeout:  timeout,
	}
}

func (p *DefaultCIConfigParser) isPreCommitCheck(check Check) bool {
	// Checks that are fast and should run on every commit
	preCommitPatterns := []string{
		"fmt",
		"format",
		"gofmt",
		"prettier",
		"black",
		"lint",
		"eslint",
		"ruff",
		"vet",
		"clippy",
	}

	name := strings.ToLower(check.Name)
	cmd := strings.ToLower(check.Command)

	for _, pattern := range preCommitPatterns {
		if strings.Contains(name, pattern) || strings.Contains(cmd, pattern) {
			return true
		}
	}

	return false
}

// Ensure DefaultCIConfigParser implements CIConfigParser
var _ CIConfigParser = (*DefaultCIConfigParser)(nil)
