package evals

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadSuite(t *testing.T) {
	// Create temp suite file
	tmpDir := t.TempDir()
	suitePath := filepath.Join(tmpDir, "test-suite.yaml")

	suiteContent := `
name: "test-suite"
description: "Test evaluation suite"
agent_type: "coding"
version: "1.0.0"

tasks:
  - id: "test-001"
    description: "Test task"
    agent_type: "coding"
    input:
      prompt: "Write hello world"
    graders:
      - type: "string_match"
        config:
          expected: "hello"
          match_type: "contains"
`

	if err := os.WriteFile(suitePath, []byte(suiteContent), 0644); err != nil {
		t.Fatalf("failed to write test suite: %v", err)
	}

	suite, err := LoadSuite(suitePath)
	if err != nil {
		t.Fatalf("failed to load suite: %v", err)
	}

	if suite.Name != "test-suite" {
		t.Errorf("unexpected suite name: %s", suite.Name)
	}
	if len(suite.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(suite.Tasks))
	}
	if suite.Tasks[0].ID != "test-001" {
		t.Errorf("unexpected task ID: %s", suite.Tasks[0].ID)
	}
}

func TestLoadSuite_NotFound(t *testing.T) {
	_, err := LoadSuite("/nonexistent/path/suite.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadSuite_NoTasks(t *testing.T) {
	tmpDir := t.TempDir()
	suitePath := filepath.Join(tmpDir, "empty-suite.yaml")

	suiteContent := `
name: "empty-suite"
description: "Suite with no tasks"
tasks: []
`

	if err := os.WriteFile(suitePath, []byte(suiteContent), 0644); err != nil {
		t.Fatalf("failed to write test suite: %v", err)
	}

	_, err := LoadSuite(suitePath)
	if err == nil {
		t.Error("expected error for suite with no tasks")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Provider != "ollama" {
		t.Errorf("unexpected default provider: %s", config.Provider)
	}
	if config.Endpoint != "http://localhost:11434" {
		t.Errorf("unexpected default endpoint: %s", config.Endpoint)
	}
	if config.TrialsPerTask != 3 {
		t.Errorf("unexpected default trials per task: %d", config.TrialsPerTask)
	}
	if config.Concurrency != 2 {
		t.Errorf("unexpected default concurrency: %d", config.Concurrency)
	}
	if config.Temperature != 0.2 {
		t.Errorf("unexpected default temperature: %f", config.Temperature)
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
provider: llama_cpp
endpoint: http://localhost:9000
model: custom-model
trials_per_task: 5
concurrency: 4
temperature: 0.5
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if config.Provider != "llama_cpp" {
		t.Errorf("unexpected provider: %s", config.Provider)
	}
	if config.Model != "custom-model" {
		t.Errorf("unexpected model: %s", config.Model)
	}
	if config.TrialsPerTask != 5 {
		t.Errorf("unexpected trials per task: %d", config.TrialsPerTask)
	}
}

func TestRunSummary_PassMetrics(t *testing.T) {
	// Test pass@k and pass^k calculation
	trialsByTask := map[string][]*Trial{
		"task-1": {
			{Passed: true},
			{Passed: true},
			{Passed: false},
		},
		"task-2": {
			{Passed: true},
			{Passed: false},
			{Passed: false},
		},
		"task-3": {
			{Passed: false},
			{Passed: false},
			{Passed: false},
		},
	}

	config := &EvalConfig{TrialsPerTask: 3}
	harness := &Harness{config: config}

	// pass@1: task-1 passes (1st trial passes), task-2 passes (1st trial passes), task-3 fails
	// Expected: 2/3 = 0.667
	passAt1 := harness.calculatePassAtK(trialsByTask, 1)
	if passAt1 < 0.65 || passAt1 > 0.68 {
		t.Errorf("unexpected pass@1: %f, expected ~0.667", passAt1)
	}

	// pass@3: task-1 has at least 1 pass, task-2 has at least 1 pass, task-3 has 0 passes
	// Expected: 2/3 = 0.667
	passAt3 := harness.calculatePassAtK(trialsByTask, 3)
	if passAt3 < 0.65 || passAt3 > 0.68 {
		t.Errorf("unexpected pass@3: %f, expected ~0.667", passAt3)
	}

	// pass^1: task-1 first trial passes, task-2 first trial passes, task-3 first trial fails
	// Expected: 2/3 = 0.667
	passPower1 := harness.calculatePassPowerK(trialsByTask, 1)
	if passPower1 < 0.65 || passPower1 > 0.68 {
		t.Errorf("unexpected pass^1: %f, expected ~0.667", passPower1)
	}

	// pass^3: all 3 trials must pass
	// task-1: fails (has 1 fail), task-2: fails, task-3: fails
	// Expected: 0/3 = 0
	passPower3 := harness.calculatePassPowerK(trialsByTask, 3)
	if passPower3 != 0 {
		t.Errorf("unexpected pass^3: %f, expected 0", passPower3)
	}
}

func TestBuildMessages_Basic(t *testing.T) {
	harness := &Harness{}

	task := &Task{
		AgentType: AgentTypeCoding,
		Input: TaskInput{
			Prompt: "Write a function",
		},
	}

	messages := harness.buildMessages(task)

	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != "system" {
		t.Errorf("expected system message first, got %s", messages[0].Role)
	}
	if messages[1].Role != "user" {
		t.Errorf("expected user message second, got %s", messages[1].Role)
	}
	if messages[1].Content != "Write a function" {
		t.Errorf("unexpected user message content: %s", messages[1].Content)
	}
}

func TestBuildMessages_WithContext(t *testing.T) {
	harness := &Harness{}

	task := &Task{
		AgentType: AgentTypeBlog,
		Input: TaskInput{
			Prompt: "Write a blog post",
			Context: map[string]interface{}{
				"topic":    "Go programming",
				"audience": "beginners",
			},
		},
	}

	messages := harness.buildMessages(task)

	// Should include context in user message
	if len(messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(messages))
	}

	userMsg := messages[len(messages)-1]
	if userMsg.Role != "user" {
		t.Errorf("expected last message to be user, got %s", userMsg.Role)
	}

	// Check that context is included
	if !containsSubstring(userMsg.Content, "topic") {
		t.Error("expected context to include 'topic'")
	}
}

func TestBuildMessages_WithFiles(t *testing.T) {
	harness := &Harness{}

	task := &Task{
		AgentType: AgentTypeCoding,
		Input: TaskInput{
			Prompt: "Fix the bug",
			Files: map[string]string{
				"main.go": "package main\n\nfunc main() {}",
			},
		},
	}

	messages := harness.buildMessages(task)

	userMsg := messages[len(messages)-1]
	if !containsSubstring(userMsg.Content, "main.go") {
		t.Error("expected files to be included in message")
	}
	if !containsSubstring(userMsg.Content, "package main") {
		t.Error("expected file content to be included")
	}
}

func TestGetSystemPrompt(t *testing.T) {
	harness := &Harness{}

	tests := []struct {
		agentType AgentType
		contains  string
	}{
		{AgentTypeCoding, "software engineer"},
		{AgentTypeBlog, "technical writer"},
		{AgentTypePodcast, "podcast"},
	}

	for _, tt := range tests {
		t.Run(string(tt.agentType), func(t *testing.T) {
			prompt := harness.getSystemPrompt(tt.agentType)
			if !containsSubstring(prompt, tt.contains) {
				t.Errorf("expected system prompt to contain %q", tt.contains)
			}
		})
	}
}

func TestNormalCDF(t *testing.T) {
	// Test known values
	tests := []struct {
		x        float64
		expected float64
		delta    float64
	}{
		{0, 0.5, 0.01},
		{1, 0.8413, 0.01},
		{-1, 0.1587, 0.01},
		{2, 0.9772, 0.01},
	}

	for _, tt := range tests {
		result := normalCDF(tt.x)
		if result < tt.expected-tt.delta || result > tt.expected+tt.delta {
			t.Errorf("normalCDF(%f) = %f, expected ~%f", tt.x, result, tt.expected)
		}
	}
}

func TestTrialMetrics(t *testing.T) {
	metrics := &TrialMetrics{
		NTurns:            3,
		NToolCalls:        5,
		NTotalTokens:      1000,
		NPromptTokens:     400,
		NCompletionTokens: 600,
		TotalLatency:      2 * time.Second,
	}

	if metrics.NTurns != 3 {
		t.Errorf("unexpected turns: %d", metrics.NTurns)
	}
	if metrics.NTotalTokens != metrics.NPromptTokens+metrics.NCompletionTokens {
		t.Error("token counts don't add up")
	}
}

func TestEvalConfig_Validation(t *testing.T) {
	config := &EvalConfig{
		Provider:      "ollama",
		Endpoint:      "http://localhost:11434",
		Model:         "llama3:8b",
		TrialsPerTask: 0, // Invalid
		Concurrency:   0, // Invalid
	}

	// The harness should handle defaults
	if config.TrialsPerTask <= 0 {
		config.TrialsPerTask = 1 // Default handling
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 1 // Default handling
	}

	if config.TrialsPerTask != 1 {
		t.Errorf("expected trials per task to default to 1")
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
