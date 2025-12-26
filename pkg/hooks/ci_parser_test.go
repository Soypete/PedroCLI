package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGitHubActions(t *testing.T) {
	parser := NewCIConfigParser()

	// Create temp directory with GitHub Actions workflow
	tmpDir, err := os.MkdirTemp("", "test-ci-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	workflowDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	workflowContent := `
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Run tests
        run: go test ./...
      - name: Build
        run: go build ./...
`

	if err := os.WriteFile(filepath.Join(workflowDir, "ci.yml"), []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := parser.ParseCIConfig(tmpDir)
	if err != nil {
		t.Fatalf("failed to parse CI config: %v", err)
	}

	if config.Source != "github_actions" {
		t.Errorf("expected source 'github_actions', got '%s'", config.Source)
	}

	if len(config.ParsedSteps) == 0 {
		t.Error("expected parsed steps")
	}

	// Should have "Run tests" and "Build" steps
	foundTest := false
	foundBuild := false
	for _, step := range config.ParsedSteps {
		if step.Name == "Run tests" {
			foundTest = true
		}
		if step.Name == "Build" {
			foundBuild = true
		}
	}

	if !foundTest {
		t.Error("expected 'Run tests' step")
	}
	if !foundBuild {
		t.Error("expected 'Build' step")
	}
}

func TestParseMakefile(t *testing.T) {
	parser := NewCIConfigParser()

	// Create temp directory with Makefile
	tmpDir, err := os.MkdirTemp("", "test-makefile-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	makefileContent := `
.PHONY: test build lint

test:
	go test ./...

build:
	go build -o bin/app ./...

lint:
	golangci-lint run

format:
	gofmt -w .
`

	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte(makefileContent), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := parser.ParseCIConfig(tmpDir)
	if err != nil {
		t.Fatalf("failed to parse CI config: %v", err)
	}

	if config.Source != "makefile" {
		t.Errorf("expected source 'makefile', got '%s'", config.Source)
	}

	if len(config.ParsedSteps) == 0 {
		t.Error("expected parsed steps")
	}

	// Should have test, build, lint targets
	targets := make(map[string]bool)
	for _, step := range config.ParsedSteps {
		targets[step.Name] = true
	}

	if !targets["test"] {
		t.Error("expected 'test' target")
	}
	if !targets["build"] {
		t.Error("expected 'build' target")
	}
	if !targets["lint"] {
		t.Error("expected 'lint' target")
	}
}

func TestConvertToHooks(t *testing.T) {
	parser := NewCIConfigParser()

	ciConfig := &CIConfig{
		Source: "test",
		ParsedSteps: []CIStep{
			{
				Name:     "lint",
				Commands: []string{"golangci-lint run"},
			},
			{
				Name:     "test",
				Commands: []string{"go test ./..."},
			},
			{
				Name:     "format",
				Commands: []string{"gofmt -w ."},
			},
		},
	}

	hooksConfig, err := parser.ConvertToHooks(ciConfig)
	if err != nil {
		t.Fatalf("failed to convert to hooks: %v", err)
	}

	if hooksConfig.Source != "ci_parsed" {
		t.Errorf("expected source 'ci_parsed', got '%s'", hooksConfig.Source)
	}

	// Lint and format should be pre-commit, test should be pre-push
	totalChecks := len(hooksConfig.PreCommit) + len(hooksConfig.PrePush)
	if totalChecks < 3 {
		t.Errorf("expected at least 3 checks, got %d", totalChecks)
	}
}

func TestSupportsFormat(t *testing.T) {
	parser := NewCIConfigParser()

	supportedFormats := []string{
		"github_actions",
		"gitlab_ci",
		"circle_ci",
		"makefile",
		"justfile",
	}

	for _, format := range supportedFormats {
		if !parser.SupportsFormat(format) {
			t.Errorf("expected format '%s' to be supported", format)
		}
	}

	if parser.SupportsFormat("unknown_format") {
		t.Error("expected 'unknown_format' to be unsupported")
	}
}

func TestParseGitLabCI(t *testing.T) {
	parser := NewCIConfigParser()

	// Create temp directory with GitLab CI config
	tmpDir, err := os.MkdirTemp("", "test-gitlab-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	gitlabContent := `
stages:
  - test
  - build

test:
  stage: test
  script:
    - go test ./...
    - go vet ./...

build:
  stage: build
  script:
    - go build -o app ./...
`

	if err := os.WriteFile(filepath.Join(tmpDir, ".gitlab-ci.yml"), []byte(gitlabContent), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := parser.ParseCIConfig(tmpDir)
	if err != nil {
		t.Fatalf("failed to parse CI config: %v", err)
	}

	if config.Source != "gitlab_ci" {
		t.Errorf("expected source 'gitlab_ci', got '%s'", config.Source)
	}

	if len(config.ParsedSteps) < 2 {
		t.Errorf("expected at least 2 steps, got %d", len(config.ParsedSteps))
	}
}

func TestNoCIConfig(t *testing.T) {
	parser := NewCIConfigParser()

	// Create empty temp directory
	tmpDir, err := os.MkdirTemp("", "test-empty-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	_, err = parser.ParseCIConfig(tmpDir)
	if err == nil {
		t.Error("expected error for missing CI config")
	}
}
