// pedro-eval is a CLI tool for running evaluations on Pedro CLI agents.
// It supports testing coding, blog post, and podcast agents against various models.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/soypete/pedrocli/pkg/evals"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	configFile string
	verbose    bool
	noColor    bool

	// Run command flags
	provider        string
	endpoint        string
	model           string
	llmGraderModel  string
	outputDir       string
	saveTranscripts bool
	concurrency     int
	trialsPerTask   int
	temperature     float64
	maxTokens       int
	timeout         int
	format          string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "pedro-eval",
		Short: "Evaluation system for Pedro CLI agents",
		Long: `Pedro Eval is a comprehensive evaluation system for testing Pedro CLI agents.

It supports evaluating:
  - Coding agents (code generation, debugging, refactoring)
  - Blog post agents (content structure, readability, SEO)
  - Podcast agents (script writing, interview questions, show notes)

Models can be hosted on Ollama or llama.cpp servers.`,
	}

	// Add global flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Path to config file (default: pedro-eval.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	// Add commands
	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(modelsCmd())
	rootCmd.AddCommand(compareCmd())
	rootCmd.AddCommand(reportCmd())
	rootCmd.AddCommand(listCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run evaluations against a model",
		Long: `Run evaluations against a model hosted on Ollama or llama.cpp.

Examples:
  # Run coding agent evals with Ollama
  pedro-eval run --agent coding --model llama3:8b --provider ollama

  # Run specific suite
  pedro-eval run --suite suites/coding/suite.yaml --provider llama_cpp --endpoint http://localhost:8080

  # Run single task
  pedro-eval run --task code-gen-001 --model codellama:13b`,
		RunE: runEvals,
	}

	// Agent/suite selection
	cmd.Flags().StringP("agent", "a", "", "Agent type to evaluate (coding, blog, podcast, all)")
	cmd.Flags().StringP("suite", "s", "", "Path to evaluation suite YAML file")
	cmd.Flags().StringP("task", "t", "", "Run single task by ID")

	// Model configuration
	cmd.Flags().StringVarP(&provider, "provider", "p", "ollama", "Model provider (ollama, llama_cpp)")
	cmd.Flags().StringVarP(&endpoint, "endpoint", "e", "", "Provider endpoint URL")
	cmd.Flags().StringVarP(&model, "model", "m", "", "Model name to evaluate")
	cmd.Flags().StringVar(&llmGraderModel, "grader-model", "", "Model for LLM-based grading (defaults to same as model)")

	// Execution options
	cmd.Flags().StringVarP(&outputDir, "output", "o", "./results", "Output directory for results")
	cmd.Flags().BoolVar(&saveTranscripts, "save-transcripts", true, "Save full trial transcripts")
	cmd.Flags().IntVar(&concurrency, "concurrency", 2, "Number of concurrent trials")
	cmd.Flags().IntVar(&trialsPerTask, "trials", 3, "Number of trials per task")
	cmd.Flags().Float64Var(&temperature, "temperature", 0.2, "Model temperature")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 4096, "Max tokens per response")
	cmd.Flags().IntVar(&timeout, "timeout", 300, "Timeout per trial in seconds")

	// Output format
	cmd.Flags().StringVarP(&format, "format", "f", "console", "Output format (console, json, html, all)")

	return cmd
}

func runEvals(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupts
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nInterrupted, canceling...")
		cancel()
	}()

	// Build config
	config := buildConfig(cmd)

	// Validate model is specified
	if config.Model == "" {
		return fmt.Errorf("model is required (use --model flag)")
	}

	// Load or create suite
	suite, err := loadOrCreateSuite(cmd)
	if err != nil {
		return err
	}

	// Create harness
	harness, err := evals.NewHarness(config)
	if err != nil {
		return fmt.Errorf("create harness: %w", err)
	}

	// Set up progress callback
	if verbose {
		harness.SetProgressCallback(func(taskID string, trialNum int, status string, result *evals.GradeResult) {
			timestamp := time.Now().Format("15:04:05")
			fmt.Printf("[%s] %s trial %d: %s\n", timestamp, taskID, trialNum, status)
		})
	}

	fmt.Printf("Running evaluation suite: %s\n", suite.Name)
	fmt.Printf("Model: %s (%s)\n", config.Model, config.Provider)
	fmt.Printf("Tasks: %d, Trials per task: %d\n", len(suite.Tasks), config.TrialsPerTask)
	fmt.Println()

	// Run evaluation
	startTime := time.Now()
	run, err := harness.Run(ctx, suite)
	if err != nil {
		return fmt.Errorf("run evaluation: %w", err)
	}
	duration := time.Since(startTime)

	fmt.Printf("\nCompleted in %s\n", duration.Round(time.Second))

	// Report results
	reporters := buildReporters(cmd, run)
	for _, reporter := range reporters {
		if err := reporter.Report(run); err != nil {
			fmt.Fprintf(os.Stderr, "Reporter error: %v\n", err)
		}
	}

	// Exit with error if pass rate is below threshold
	if run.Summary.OverallPassRate < 0.5 {
		os.Exit(1)
	}

	return nil
}

func buildConfig(cmd *cobra.Command) *evals.EvalConfig {
	config := evals.DefaultConfig()

	// Override from config file
	if configFile != "" {
		if loaded, err := evals.LoadConfig(configFile); err == nil {
			config = loaded
		}
	}

	// Override from flags
	if provider != "" {
		config.Provider = provider
	}
	if endpoint != "" {
		config.Endpoint = endpoint
	} else {
		// Set default endpoints
		if config.Provider == "ollama" && config.Endpoint == "" {
			config.Endpoint = "http://localhost:11434"
		} else if config.Provider == "llama_cpp" && config.Endpoint == "" {
			config.Endpoint = "http://localhost:8080"
		}
	}
	if model != "" {
		config.Model = model
	}
	if llmGraderModel != "" {
		config.LLMGraderModel = llmGraderModel
	}
	if outputDir != "" {
		config.OutputDir = outputDir
	}
	config.SaveTranscripts = saveTranscripts
	if concurrency > 0 {
		config.Concurrency = concurrency
	}
	if trialsPerTask > 0 {
		config.TrialsPerTask = trialsPerTask
	}
	if temperature > 0 {
		config.Temperature = temperature
	}
	if maxTokens > 0 {
		config.MaxTokens = maxTokens
	}
	if timeout > 0 {
		config.Timeout = timeout
	}

	return config
}

func loadOrCreateSuite(cmd *cobra.Command) (*evals.Suite, error) {
	suitePath, _ := cmd.Flags().GetString("suite")
	agentType, _ := cmd.Flags().GetString("agent")
	taskID, _ := cmd.Flags().GetString("task")

	// If suite path specified, load it
	if suitePath != "" {
		return evals.LoadSuite(suitePath)
	}

	// If agent type specified, find default suite
	if agentType != "" {
		switch agentType {
		case "coding":
			return loadDefaultSuite("coding")
		case "blog":
			return loadDefaultSuite("blog")
		case "podcast":
			return loadDefaultSuite("podcast")
		case "all":
			return loadAllSuites()
		default:
			return nil, fmt.Errorf("unknown agent type: %s", agentType)
		}
	}

	// If task ID specified, create single-task suite
	if taskID != "" {
		return createSingleTaskSuite(taskID)
	}

	return nil, fmt.Errorf("must specify --suite, --agent, or --task")
}

func loadDefaultSuite(agentType string) (*evals.Suite, error) {
	// Look for suite in standard locations
	paths := []string{
		fmt.Sprintf("suites/%s/suite.yaml", agentType),
		fmt.Sprintf("./suites/%s/suite.yaml", agentType),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return evals.LoadSuite(path)
		}
	}

	return nil, fmt.Errorf("default suite not found for agent type: %s", agentType)
}

func loadAllSuites() (*evals.Suite, error) {
	combined := &evals.Suite{
		Name:        "all-agents",
		Description: "Combined evaluation suite for all agent types",
		Tasks:       []evals.Task{},
	}

	for _, agentType := range []string{"coding", "blog", "podcast"} {
		suite, err := loadDefaultSuite(agentType)
		if err != nil {
			fmt.Printf("Warning: could not load %s suite: %v\n", agentType, err)
			continue
		}
		combined.Tasks = append(combined.Tasks, suite.Tasks...)
	}

	if len(combined.Tasks) == 0 {
		return nil, fmt.Errorf("no tasks found in any suite")
	}

	return combined, nil
}

func createSingleTaskSuite(taskID string) (*evals.Suite, error) {
	// Search all suites for the task
	for _, agentType := range []string{"coding", "blog", "podcast"} {
		suite, err := loadDefaultSuite(agentType)
		if err != nil {
			continue
		}
		for _, task := range suite.Tasks {
			if task.ID == taskID {
				return &evals.Suite{
					Name:        fmt.Sprintf("single-task-%s", taskID),
					Description: fmt.Sprintf("Single task: %s", task.Description),
					AgentType:   task.AgentType,
					Tasks:       []evals.Task{task},
				}, nil
			}
		}
	}
	return nil, fmt.Errorf("task not found: %s", taskID)
}

func buildReporters(cmd *cobra.Command, run *evals.EvalRun) []evals.Reporter {
	var reporters []evals.Reporter
	outputFormats := strings.Split(format, ",")

	for _, f := range outputFormats {
		f = strings.TrimSpace(f)
		switch f {
		case "console":
			reporters = append(reporters, evals.NewConsoleReporter(verbose, !noColor))
		case "json":
			path := filepath.Join(outputDir, fmt.Sprintf("%s.json", run.ID))
			reporters = append(reporters, evals.NewJSONReporter(path, true))
		case "html":
			path := filepath.Join(outputDir, fmt.Sprintf("%s.html", run.ID))
			reporters = append(reporters, evals.NewHTMLReporter(path))
		case "all":
			reporters = append(reporters, evals.NewConsoleReporter(verbose, !noColor))
			reporters = append(reporters, evals.NewJSONReporter(
				filepath.Join(outputDir, fmt.Sprintf("%s.json", run.ID)), true))
			reporters = append(reporters, evals.NewHTMLReporter(
				filepath.Join(outputDir, fmt.Sprintf("%s.html", run.ID))))
		}
	}

	if len(reporters) == 0 {
		reporters = append(reporters, evals.NewConsoleReporter(verbose, !noColor))
	}

	return reporters
}

func modelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "List available models",
		Long: `List available models from the specified provider.

Examples:
  pedro-eval models --provider ollama
  pedro-eval models --provider llama_cpp --endpoint http://localhost:8080`,
		RunE: listModels,
	}

	cmd.Flags().StringVarP(&provider, "provider", "p", "ollama", "Model provider")
	cmd.Flags().StringVarP(&endpoint, "endpoint", "e", "", "Provider endpoint URL")

	return cmd
}

func listModels(cmd *cobra.Command, args []string) error {
	if endpoint == "" {
		if provider == "ollama" {
			endpoint = "http://localhost:11434"
		} else {
			endpoint = "http://localhost:8080"
		}
	}

	client, err := evals.NewClient(provider, endpoint, "")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	models, err := client.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("list models: %w", err)
	}

	fmt.Printf("Available models (%s at %s):\n\n", provider, endpoint)
	for _, m := range models {
		fmt.Printf("  - %s\n", m.Name)
	}

	return nil
}

func compareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare two models on the same suite",
		Long: `Run the same evaluation suite against two models and compare results.

Examples:
  pedro-eval compare --model1 llama3:8b --model2 codellama:13b --suite coding
  pedro-eval compare --model1 qwen2.5:32b --model2 llama3.1:70b --agent all`,
		RunE: compareModels,
	}

	cmd.Flags().String("model1", "", "First model to compare")
	cmd.Flags().String("model2", "", "Second model to compare")
	cmd.Flags().StringP("suite", "s", "", "Suite path or agent type")
	cmd.Flags().StringP("agent", "a", "", "Agent type (coding, blog, podcast)")
	cmd.Flags().StringVarP(&provider, "provider", "p", "ollama", "Model provider")
	cmd.Flags().StringVarP(&endpoint, "endpoint", "e", "", "Provider endpoint URL")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "./results", "Output directory")
	cmd.Flags().IntVar(&trialsPerTask, "trials", 3, "Trials per task")

	_ = cmd.MarkFlagRequired("model1")
	_ = cmd.MarkFlagRequired("model2")

	return cmd
}

func compareModels(cmd *cobra.Command, args []string) error {
	model1, _ := cmd.Flags().GetString("model1")
	model2, _ := cmd.Flags().GetString("model2")

	// Load suite
	suite, err := loadOrCreateSuite(cmd)
	if err != nil {
		return err
	}

	// Set default endpoint
	if endpoint == "" {
		if provider == "ollama" {
			endpoint = "http://localhost:11434"
		} else {
			endpoint = "http://localhost:8080"
		}
	}

	config1 := &evals.EvalConfig{
		Provider:        provider,
		Endpoint:        endpoint,
		Model:           model1,
		OutputDir:       outputDir,
		SaveTranscripts: true,
		Concurrency:     2,
		TrialsPerTask:   trialsPerTask,
		Temperature:     0.2,
		MaxTokens:       4096,
		Timeout:         300,
	}

	config2 := &evals.EvalConfig{
		Provider:        provider,
		Endpoint:        endpoint,
		Model:           model2,
		OutputDir:       outputDir,
		SaveTranscripts: true,
		Concurrency:     2,
		TrialsPerTask:   trialsPerTask,
		Temperature:     0.2,
		MaxTokens:       4096,
		Timeout:         300,
	}

	fmt.Printf("Comparing models on suite: %s\n", suite.Name)
	fmt.Printf("  Model 1: %s\n", model1)
	fmt.Printf("  Model 2: %s\n", model2)
	fmt.Printf("  Tasks: %d, Trials: %d\n", len(suite.Tasks), trialsPerTask)
	fmt.Println()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupts
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nInterrupted, canceling...")
		cancel()
	}()

	result, err := evals.CompareModels(ctx, suite, config1, config2)
	if err != nil {
		return err
	}

	// Report comparison
	return evals.ReportComparison(result, "console", "")
}

func reportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report [results-file]",
		Short: "Generate report from saved results",
		Long: `Generate a report from previously saved evaluation results.

Examples:
  pedro-eval report results/run-123.json --format html
  pedro-eval report results/run-123.json --format console`,
		Args: cobra.ExactArgs(1),
		RunE: generateReport,
	}

	cmd.Flags().StringVarP(&format, "format", "f", "console", "Output format (console, html)")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output file (for html format)")

	return cmd
}

func generateReport(cmd *cobra.Command, args []string) error {
	resultsPath := args[0]

	// Load results
	data, err := os.ReadFile(resultsPath)
	if err != nil {
		return fmt.Errorf("read results: %w", err)
	}

	var run evals.EvalRun
	if err := json.Unmarshal(data, &run); err != nil {
		return fmt.Errorf("parse results: %w", err)
	}

	// Generate report
	switch format {
	case "console":
		reporter := evals.NewConsoleReporter(verbose, !noColor)
		return reporter.Report(&run)
	case "html":
		outputPath := outputDir
		if outputPath == "" {
			outputPath = strings.TrimSuffix(resultsPath, ".json") + ".html"
		}
		reporter := evals.NewHTMLReporter(outputPath)
		if err := reporter.Report(&run); err != nil {
			return err
		}
		fmt.Printf("HTML report written to: %s\n", outputPath)
		return nil
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available evaluation tasks",
		Long: `List all available evaluation tasks across suites.

Examples:
  pedro-eval list
  pedro-eval list --agent coding
  pedro-eval list --tags "basic,generation"`,
		RunE: listTasks,
	}

	cmd.Flags().StringP("agent", "a", "", "Filter by agent type")
	cmd.Flags().StringSlice("tags", nil, "Filter by tags")

	return cmd
}

func listTasks(cmd *cobra.Command, args []string) error {
	agentFilter, _ := cmd.Flags().GetString("agent")
	tagFilters, _ := cmd.Flags().GetStringSlice("tags")

	agentTypes := []string{"coding", "blog", "podcast"}
	if agentFilter != "" {
		agentTypes = []string{agentFilter}
	}

	fmt.Println("Available evaluation tasks:")
	fmt.Println()

	for _, agentType := range agentTypes {
		suite, err := loadDefaultSuite(agentType)
		if err != nil {
			continue
		}

		// Capitalize first letter of agent type
		title := agentType
		if len(agentType) > 0 {
			title = strings.ToUpper(agentType[:1]) + agentType[1:]
		}
		fmt.Printf("=== %s Agent (%d tasks) ===\n", title, len(suite.Tasks))

		for _, task := range suite.Tasks {
			// Filter by tags if specified
			if len(tagFilters) > 0 {
				hasTag := false
				for _, filter := range tagFilters {
					for _, tag := range task.Tags {
						if tag == filter {
							hasTag = true
							break
						}
					}
				}
				if !hasTag {
					continue
				}
			}

			tagsStr := ""
			if len(task.Tags) > 0 {
				tagsStr = fmt.Sprintf(" [%s]", strings.Join(task.Tags, ", "))
			}
			fmt.Printf("  %-25s %s%s\n", task.ID, truncateStr(task.Description, 45), tagsStr)
		}
		fmt.Println()
	}

	return nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
