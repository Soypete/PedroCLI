package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Harness orchestrates evaluation runs.
type Harness struct {
	config        *EvalConfig
	client        ModelClient
	graderFactory *GraderFactory
	mu            sync.Mutex
	progress      ProgressCallback
}

// ProgressCallback is called to report progress during evaluation.
type ProgressCallback func(taskID string, trialNum int, status string, result *GradeResult)

// NewHarness creates a new evaluation harness.
func NewHarness(config *EvalConfig) (*Harness, error) {
	// Create model client for evaluation target
	client, err := NewClient(config.Provider, config.Endpoint, config.Model)
	if err != nil {
		return nil, fmt.Errorf("create model client: %w", err)
	}

	// Create grader client (may be different model/provider)
	var graderClient ModelClient
	if config.LLMGraderModel != "" {
		graderProvider := config.LLMGraderProvider
		if graderProvider == "" {
			graderProvider = config.Provider
		}
		graderEndpoint := config.LLMGraderEndpoint
		if graderEndpoint == "" {
			graderEndpoint = config.Endpoint
		}
		graderClient, err = NewClient(graderProvider, graderEndpoint, config.LLMGraderModel)
		if err != nil {
			return nil, fmt.Errorf("create grader client: %w", err)
		}
	} else {
		graderClient = client
	}

	return &Harness{
		config:        config,
		client:        client,
		graderFactory: NewGraderFactory(graderClient),
	}, nil
}

// SetProgressCallback sets a callback for progress updates.
func (h *Harness) SetProgressCallback(cb ProgressCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.progress = cb
}

// LoadSuite loads an evaluation suite from a YAML file.
func LoadSuite(path string) (*Suite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read suite file: %w", err)
	}

	var suite Suite
	if err := yaml.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("parse suite YAML: %w", err)
	}

	// Validate suite
	if suite.Name == "" {
		suite.Name = filepath.Base(path)
	}
	if len(suite.Tasks) == 0 {
		return nil, fmt.Errorf("suite has no tasks")
	}

	return &suite, nil
}

// Run executes an evaluation suite and returns results.
func (h *Harness) Run(ctx context.Context, suite *Suite) (*EvalRun, error) {
	startTime := time.Now()
	runID := fmt.Sprintf("run-%s-%s", suite.Name, startTime.Format("20060102-150405"))

	run := &EvalRun{
		ID:        runID,
		StartedAt: startTime,
		Config:    h.config,
		Suite:     suite,
		Trials:    make([]*Trial, 0),
	}

	// Ensure output directory exists
	if h.config.OutputDir != "" {
		if err := os.MkdirAll(h.config.OutputDir, 0755); err != nil {
			return nil, fmt.Errorf("create output dir: %w", err)
		}
		if h.config.SaveTranscripts {
			transcriptDir := filepath.Join(h.config.OutputDir, "transcripts", runID)
			if err := os.MkdirAll(transcriptDir, 0755); err != nil {
				return nil, fmt.Errorf("create transcript dir: %w", err)
			}
		}
	}

	trialsPerTask := h.config.TrialsPerTask
	if trialsPerTask <= 0 {
		trialsPerTask = 1
	}

	concurrency := h.config.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	// Create work queue
	type work struct {
		task     *Task
		trialNum int
	}
	workChan := make(chan work, len(suite.Tasks)*trialsPerTask)
	resultsChan := make(chan *Trial, len(suite.Tasks)*trialsPerTask)

	// Queue all work
	for i := range suite.Tasks {
		task := &suite.Tasks[i]
		for t := 1; t <= trialsPerTask; t++ {
			workChan <- work{task: task, trialNum: t}
		}
	}
	close(workChan)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for w := range workChan {
				trial := h.runTrial(ctx, w.task, w.trialNum)
				resultsChan <- trial
			}
		}()
	}

	// Collect results in background
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect all trials
	for trial := range resultsChan {
		run.Trials = append(run.Trials, trial)

		// Save transcript if enabled
		if h.config.SaveTranscripts && h.config.OutputDir != "" && trial.Transcript != nil {
			if err := h.saveTranscript(runID, trial); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save transcript for %s trial %d: %v\n",
					trial.TaskID, trial.TrialNumber, err)
			}
		}
	}

	// Sort trials by task ID and trial number for consistent output
	sort.Slice(run.Trials, func(i, j int) bool {
		if run.Trials[i].TaskID != run.Trials[j].TaskID {
			return run.Trials[i].TaskID < run.Trials[j].TaskID
		}
		return run.Trials[i].TrialNumber < run.Trials[j].TrialNumber
	})

	run.CompletedAt = time.Now()
	run.Summary = h.computeSummary(run)

	return run, nil
}

// runTrial executes a single trial for a task.
func (h *Harness) runTrial(ctx context.Context, task *Task, trialNum int) *Trial {
	trialID := fmt.Sprintf("%s-trial-%d-%d", task.ID, trialNum, time.Now().UnixNano())

	trial := &Trial{
		ID:          trialID,
		TaskID:      task.ID,
		TrialNumber: trialNum,
		StartedAt:   time.Now(),
		Transcript:  &Transcript{Turns: []Turn{}},
		Metrics:     &TrialMetrics{},
	}

	h.reportProgress(task.ID, trialNum, "started", nil)

	// Apply timeout if configured
	if h.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(h.config.Timeout)*time.Second)
		defer cancel()
	}

	// Build messages for the task
	messages := h.buildMessages(task)

	// Record start time for metrics
	inferenceStart := time.Now()

	// Call model
	temperature := h.config.Temperature
	if temperature == 0 {
		temperature = 0.2 // Default low temperature for evals
	}
	maxTokens := h.config.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	resp, err := h.client.Complete(ctx, &CompletionRequest{
		Model:       h.config.Model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
	})

	trial.CompletedAt = time.Now()

	if err != nil {
		trial.Error = err.Error()
		trial.Outcome = &Outcome{
			ExitReason: "error",
		}
		h.reportProgress(task.ID, trialNum, "error", nil)
		return trial
	}

	// Record metrics
	trial.Metrics.NTotalTokens = resp.TotalTokens
	trial.Metrics.NPromptTokens = resp.PromptTokens
	trial.Metrics.NCompletionTokens = resp.CompletionTokens
	trial.Metrics.TimeToFirstToken = resp.TimeToFirstToken
	trial.Metrics.TotalLatency = resp.TotalTime
	if resp.TotalTime > 0 && resp.CompletionTokens > 0 {
		trial.Metrics.TokensPerSecond = float64(resp.CompletionTokens) / resp.TotalTime.Seconds()
	}
	trial.Metrics.NTurns = 1 // Single turn for now

	// Record transcript
	for _, msg := range messages {
		trial.Transcript.Turns = append(trial.Transcript.Turns, Turn{
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: inferenceStart,
		})
	}
	trial.Transcript.Turns = append(trial.Transcript.Turns, Turn{
		Role:       "assistant",
		Content:    resp.Content,
		TokensUsed: resp.TotalTokens,
		Timestamp:  trial.CompletedAt,
	})

	// Create outcome
	trial.Outcome = &Outcome{
		FinalOutput: resp.Content,
		ExitReason:  "completed",
	}

	// Run graders
	trial.GradeResults = h.runGraders(ctx, task, trial)

	// Calculate composite score and pass status
	trial.Score, trial.Passed = CompositeScore(trial.GradeResults, task.Graders)

	h.reportProgress(task.ID, trialNum, "completed", nil)

	return trial
}

// buildMessages creates the message array for a task.
func (h *Harness) buildMessages(task *Task) []Message {
	messages := []Message{}

	// Add system message based on agent type
	systemPrompt := h.getSystemPrompt(task.AgentType)
	if systemPrompt != "" {
		messages = append(messages, Message{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	// Add task prompt
	userPrompt := task.Input.Prompt

	// Add context if provided
	if len(task.Input.Context) > 0 {
		contextJSON, _ := json.MarshalIndent(task.Input.Context, "", "  ")
		userPrompt = fmt.Sprintf("%s\n\nContext:\n```json\n%s\n```", userPrompt, string(contextJSON))
	}

	// Add files if provided
	if len(task.Input.Files) > 0 {
		userPrompt += "\n\nFiles:"
		for name, content := range task.Input.Files {
			userPrompt += fmt.Sprintf("\n\n--- %s ---\n%s", name, content)
		}
	}

	messages = append(messages, Message{
		Role:    "user",
		Content: userPrompt,
	})

	return messages
}

// getSystemPrompt returns the system prompt for an agent type.
func (h *Harness) getSystemPrompt(agentType AgentType) string {
	switch agentType {
	case AgentTypeCoding:
		return `You are an expert software engineer. Your task is to help with coding tasks including:
- Writing new code
- Debugging and fixing issues
- Refactoring and improving code
- Explaining code concepts

Provide clear, well-structured code with appropriate comments. Follow best practices for the language being used.`

	case AgentTypeBlog:
		return `You are an expert technical writer. Your task is to create engaging, well-structured blog posts about technology topics.

Guidelines:
- Use clear, accessible language
- Include code examples where appropriate
- Structure content with headers and sections
- Aim for readability scores suitable for the target audience
- Use markdown formatting`

	case AgentTypePodcast:
		return `You are an expert podcast content creator. Your task is to help create engaging podcast content including:
- Episode scripts and outlines
- Interview questions
- Show notes
- Segment transitions

Keep the tone conversational and engaging. Include timing cues and speaker annotations where appropriate.`

	default:
		return ""
	}
}

// runGraders runs all graders for a task.
func (h *Harness) runGraders(ctx context.Context, task *Task, trial *Trial) []*GradeResult {
	results := make([]*GradeResult, 0, len(task.Graders))

	for i := range task.Graders {
		config := &task.Graders[i]

		grader, err := h.graderFactory.GetGrader(config.Type)
		if err != nil {
			results = append(results, &GradeResult{
				GraderType: config.Type,
				Passed:     false,
				Score:      0,
				Feedback:   fmt.Sprintf("Failed to get grader: %s", err),
				Error:      err.Error(),
			})
			continue
		}

		result, err := grader.Grade(ctx, task, trial, config)
		if err != nil {
			results = append(results, &GradeResult{
				GraderType: config.Type,
				Passed:     false,
				Score:      0,
				Feedback:   fmt.Sprintf("Grading failed: %s", err),
				Error:      err.Error(),
			})
			continue
		}

		results = append(results, result)
	}

	return results
}

// computeSummary calculates aggregate statistics for a run.
func (h *Harness) computeSummary(run *EvalRun) *RunSummary {
	summary := &RunSummary{
		ByGraderType: make(map[GraderType]GraderStats),
		ByTag:        make(map[string]TagStats),
		PassAtK:      make(map[int]float64),
		PassPowerK:   make(map[int]float64),
	}

	if len(run.Trials) == 0 {
		return summary
	}

	// Group trials by task
	trialsByTask := make(map[string][]*Trial)
	for _, trial := range run.Trials {
		trialsByTask[trial.TaskID] = append(trialsByTask[trial.TaskID], trial)
	}

	summary.TotalTasks = len(trialsByTask)
	summary.TotalTrials = len(run.Trials)

	// Calculate basic stats
	var totalScore float64
	var totalTokens int
	var totalLatency time.Duration
	var totalTurns int
	var totalToolCalls int

	for _, trial := range run.Trials {
		totalScore += trial.Score
		if trial.Passed {
			summary.PassedTrials++
		} else if trial.Error != "" {
			summary.ErrorTrials++
		} else {
			summary.FailedTrials++
		}

		if trial.Metrics != nil {
			totalTokens += trial.Metrics.NTotalTokens
			totalLatency += trial.Metrics.TotalLatency
			totalTurns += trial.Metrics.NTurns
			totalToolCalls += trial.Metrics.NToolCalls
		}

		// Aggregate grader stats
		for _, gr := range trial.GradeResults {
			stats := summary.ByGraderType[gr.GraderType]
			stats.TotalRuns++
			if gr.Passed {
				stats.Passed++
			} else {
				stats.Failed++
			}
			stats.AvgScore = (stats.AvgScore*float64(stats.TotalRuns-1) + gr.Score) / float64(stats.TotalRuns)
			stats.PassRate = float64(stats.Passed) / float64(stats.TotalRuns)
			summary.ByGraderType[gr.GraderType] = stats
		}
	}

	summary.OverallPassRate = float64(summary.PassedTrials) / float64(summary.TotalTrials)
	summary.AvgScore = totalScore / float64(summary.TotalTrials)
	summary.AvgTokensUsed = float64(totalTokens) / float64(summary.TotalTrials)
	summary.AvgLatency = totalLatency / time.Duration(summary.TotalTrials)
	summary.AvgTurns = float64(totalTurns) / float64(summary.TotalTrials)
	summary.AvgToolCalls = float64(totalToolCalls) / float64(summary.TotalTrials)

	// Calculate pass@k and pass^k for various k values
	kValues := []int{1, 3, 5, 10}
	for _, k := range kValues {
		if k > h.config.TrialsPerTask {
			continue
		}
		summary.PassAtK[k] = h.calculatePassAtK(trialsByTask, k)
		summary.PassPowerK[k] = h.calculatePassPowerK(trialsByTask, k)
	}

	// Calculate tag-based stats
	taskByID := make(map[string]*Task)
	for i := range run.Suite.Tasks {
		taskByID[run.Suite.Tasks[i].ID] = &run.Suite.Tasks[i]
	}

	for taskID, trials := range trialsByTask {
		task := taskByID[taskID]
		if task == nil {
			continue
		}
		for _, tag := range task.Tags {
			stats := summary.ByTag[tag]
			stats.TotalTasks++
			stats.TotalTrials += len(trials)
			for _, trial := range trials {
				if trial.Passed {
					stats.Passed++
				}
				stats.AvgScore = (stats.AvgScore*float64(stats.TotalTrials-len(trials)) + trial.Score) / float64(stats.TotalTrials)
			}
			stats.PassRate = float64(stats.Passed) / float64(stats.TotalTrials)
			summary.ByTag[tag] = stats
		}
	}

	return summary
}

// calculatePassAtK calculates the probability of at least 1 success in k trials.
// pass@k = 1 - (1-p)^k where p is the single-trial pass rate
func (h *Harness) calculatePassAtK(trialsByTask map[string][]*Trial, k int) float64 {
	if len(trialsByTask) == 0 {
		return 0
	}

	totalPassAtK := 0.0
	for _, trials := range trialsByTask {
		if len(trials) < k {
			continue
		}
		// Count passes in first k trials
		passes := 0
		for i := 0; i < k && i < len(trials); i++ {
			if trials[i].Passed {
				passes++
			}
		}
		// pass@k for this task: at least 1 pass in k trials
		if passes > 0 {
			totalPassAtK++
		}
	}

	return totalPassAtK / float64(len(trialsByTask))
}

// calculatePassPowerK calculates the probability of ALL k trials succeeding.
// pass^k = p^k where p is the single-trial pass rate
func (h *Harness) calculatePassPowerK(trialsByTask map[string][]*Trial, k int) float64 {
	if len(trialsByTask) == 0 {
		return 0
	}

	totalPassPowerK := 0.0
	for _, trials := range trialsByTask {
		if len(trials) < k {
			continue
		}
		// Check if all first k trials passed
		allPassed := true
		for i := 0; i < k && i < len(trials); i++ {
			if !trials[i].Passed {
				allPassed = false
				break
			}
		}
		if allPassed {
			totalPassPowerK++
		}
	}

	return totalPassPowerK / float64(len(trialsByTask))
}

// saveTranscript saves a trial transcript to disk.
func (h *Harness) saveTranscript(runID string, trial *Trial) error {
	transcriptDir := filepath.Join(h.config.OutputDir, "transcripts", runID)
	filename := fmt.Sprintf("%s-trial-%d.json", trial.TaskID, trial.TrialNumber)
	path := filepath.Join(transcriptDir, filename)

	data, err := json.MarshalIndent(trial.Transcript, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal transcript: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// reportProgress reports progress via callback if set.
func (h *Harness) reportProgress(taskID string, trialNum int, status string, result *GradeResult) {
	h.mu.Lock()
	cb := h.progress
	h.mu.Unlock()

	if cb != nil {
		cb(taskID, trialNum, status, result)
	}
}

// RunSingleTask runs a single task and returns the trial result.
func (h *Harness) RunSingleTask(ctx context.Context, task *Task) (*Trial, error) {
	trial := h.runTrial(ctx, task, 1)
	if trial.Error != "" {
		return trial, fmt.Errorf("trial failed: %s", trial.Error)
	}
	return trial, nil
}

// CompareModels runs the same suite against two models and compares results.
func CompareModels(ctx context.Context, suite *Suite, config1, config2 *EvalConfig) (*ComparisonResult, error) {
	// Run first model
	harness1, err := NewHarness(config1)
	if err != nil {
		return nil, fmt.Errorf("create harness for model 1: %w", err)
	}
	run1, err := harness1.Run(ctx, suite)
	if err != nil {
		return nil, fmt.Errorf("run model 1: %w", err)
	}

	// Run second model
	harness2, err := NewHarness(config2)
	if err != nil {
		return nil, fmt.Errorf("create harness for model 2: %w", err)
	}
	run2, err := harness2.Run(ctx, suite)
	if err != nil {
		return nil, fmt.Errorf("run model 2: %w", err)
	}

	// Compare results
	result := &ComparisonResult{
		Model1:       config1.Model,
		Model2:       config2.Model,
		Run1:         run1,
		Run2:         run2,
		WinnerByTask: make(map[string]string),
	}

	// Group trials by task
	run1ByTask := make(map[string]float64)
	run2ByTask := make(map[string]float64)

	for _, trial := range run1.Trials {
		if existing, ok := run1ByTask[trial.TaskID]; !ok || trial.Score > existing {
			run1ByTask[trial.TaskID] = trial.Score
		}
	}
	for _, trial := range run2.Trials {
		if existing, ok := run2ByTask[trial.TaskID]; !ok || trial.Score > existing {
			run2ByTask[trial.TaskID] = trial.Score
		}
	}

	// Compare per-task
	for taskID := range run1ByTask {
		score1 := run1ByTask[taskID]
		score2 := run2ByTask[taskID]

		if math.Abs(score1-score2) < 0.01 { // Within 1% is a tie
			result.WinnerByTask[taskID] = "tie"
			result.Ties++
		} else if score1 > score2 {
			result.WinnerByTask[taskID] = config1.Model
			result.Model1Wins++
		} else {
			result.WinnerByTask[taskID] = config2.Model
			result.Model2Wins++
		}
	}

	// Calculate significance (simplified binomial test approximation)
	total := result.Model1Wins + result.Model2Wins
	if total > 0 {
		// Two-tailed binomial test approximation
		p := float64(result.Model1Wins) / float64(total)
		// z-score for difference from 0.5
		z := math.Abs(p-0.5) / math.Sqrt(0.25/float64(total))
		// Approximate p-value (two-tailed)
		result.SignificanceP = 2 * (1 - normalCDF(z))
	}

	return result, nil
}

// normalCDF approximates the normal CDF using the error function approximation.
func normalCDF(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

// DefaultConfig returns a default evaluation configuration.
func DefaultConfig() *EvalConfig {
	return &EvalConfig{
		Provider:        "ollama",
		Endpoint:        "http://localhost:11434",
		Model:           "llama3:8b",
		OutputDir:       "./results",
		SaveTranscripts: true,
		Concurrency:     2,
		TrialsPerTask:   3,
		Temperature:     0.2,
		MaxTokens:       4096,
		Timeout:         300, // 5 minutes
	}
}

// LoadConfig loads configuration from a YAML file.
func LoadConfig(path string) (*EvalConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	config := DefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("parse config YAML: %w", err)
	}

	return config, nil
}
