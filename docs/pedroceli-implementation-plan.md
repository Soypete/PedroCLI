# Pedroceli Implementation Plan

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     MCP CLIENTS                              │
│  ┌──────────────────┐           ┌──────────────────┐        │
│  │   CLI Client     │           │   Web Client     │        │
│  │                  │           │  (Speech-to-Text)│        │
│  │  pedroceli build │           │                  │        │
│  │  pedroceli debug │           │  Whisper.cpp STT │        │
│  │  pedroceli review│           │  Voice Recording │        │
│  └──────────────────┘           └──────────────────┘        │
└─────────────────────────────────────────────────────────────┘
                            │
                    MCP Protocol (stdio)
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│              PEDROCELI MCP SERVER                            │
│                                                              │
│  MCP Tools:                                                  │
│  ├─ build_feature      → Background building                │
│  ├─ debug_issue        → Debugging & fixing                 │
│  ├─ review_pr          → Code review / eval                 │
│  ├─ triage_issue       → Test & diagnose issues             │
│  ├─ get_job_status     → Job monitoring                     │
│  └─ list_jobs          → Job listing                        │
│                                                              │
│  Backend: llama.cpp (primary) / Ollama (secondary)          │
└─────────────────────────────────────────────────────────────┘
                            │
                    One-shot inference
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│           INFERENCE BACKEND                                  │
│                                                              │
│  Primary:   llama.cpp with CUDA on DGX Spark                │
│  Secondary: Ollama (easier setup)                           │
└─────────────────────────────────────────────────────────────┘
```

## Project Structure

```
pedroceli/
├── cmd/
│   ├── mcp-server.go      # MCP server entrypoint
│   ├── cli.go             # CLI client (wraps MCP calls)
│   └── web.go             # Web server entrypoint
├── pkg/
│   ├── mcp/
│   │   ├── server.go      # MCP protocol handler
│   │   ├── tools.go       # Tool definitions
│   │   └── client.go      # MCP client library
│   ├── agents/
│   │   ├── builder.go     # Background building agent
│   │   ├── debugger.go    # Debugging agent
│   │   ├── reviewer.go    # PR review agent
│   │   └── triager.go     # Issue triage agent
│   ├── llm/
│   │   ├── llamacpp.go    # llama.cpp backend (PRIMARY)
│   │   ├── ollama.go      # Ollama backend (SECONDARY)
│   │   └── interface.go   # Backend interface
│   ├── tools/
│   │   ├── bash.go        # Shell commands
│   │   ├── git.go         # Git operations
│   │   ├── file.go        # File operations
│   │   └── test.go        # Test execution
│   ├── jobs/
│   │   └── manager.go     # Job state management
│   ├── stt/               # (Web client only)
│   │   └── whisper.go     # Whisper.cpp integration
│   └── config/
│       └── config.go      # Configuration
├── web/
│   ├── static/
│   │   ├── index.html     # Voice UI
│   │   ├── app.js         # Frontend logic
│   │   └── styles.css
│   └── api/
│       └── handlers.go    # HTTP handlers
└── .pedroceli.json        # Config file
```

## Context Management Strategy

### File-Based Context (Not In-Memory)

**Problem**: Managing context across one-shot inferences without memory bloat or losing history.

**Solution**: Write everything to temp files as we go.

```
/tmp/pedroceli-jobs/
└── job-1699401234/
    ├── 001-initial-prompt.txt        # First inference prompt
    ├── 002-response.txt               # First inference response
    ├── 003-tool-calls.json            # Tools that were called
    ├── 004-tool-results.json          # Tool execution results
    ├── 005-next-prompt.txt            # Second inference prompt
    ├── 006-response.txt               # Second inference response
    ├── ...
    └── job-metadata.json              # Job state, config, timestamps
```

### Implementation

```go
// pkg/context/manager.go
type ContextManager struct {
    jobID      string
    jobDir     string
    counter    int
    debugMode  bool
}

func NewContextManager(jobID string, debugMode bool) *ContextManager {
    timestamp := time.Now().Format("20060102-150405")
    jobDir := filepath.Join("/tmp/pedroceli-jobs", fmt.Sprintf("%s-%s", jobID, timestamp))
    os.MkdirAll(jobDir, 0755)
    
    return &ContextManager{
        jobID:     jobID,
        jobDir:    jobDir,
        counter:   0,
        debugMode: debugMode,
    }
}

func (cm *ContextManager) SavePrompt(prompt string) error {
    cm.counter++
    filename := fmt.Sprintf("%03d-prompt.txt", cm.counter)
    return os.WriteFile(filepath.Join(cm.jobDir, filename), []byte(prompt), 0644)
}

func (cm *ContextManager) SaveResponse(response string) error {
    cm.counter++
    filename := fmt.Sprintf("%03d-response.txt", cm.counter)
    return os.WriteFile(filepath.Join(cm.jobDir, filename), []byte(response), 0644)
}

func (cm *ContextManager) SaveToolCalls(calls []ToolCall) error {
    cm.counter++
    filename := fmt.Sprintf("%03d-tool-calls.json", cm.counter)
    data, _ := json.MarshalIndent(calls, "", "  ")
    return os.WriteFile(filepath.Join(cm.jobDir, filename), data, 0644)
}

func (cm *ContextManager) SaveToolResults(results []ToolResult) error {
    cm.counter++
    filename := fmt.Sprintf("%03d-tool-results.json", cm.counter)
    data, _ := json.MarshalIndent(results, "", "  ")
    return os.WriteFile(filepath.Join(cm.jobDir, filename), data, 0644)
}

func (cm *ContextManager) GetHistory() (string, error) {
    // Read all files in order and concatenate
    files, _ := filepath.Glob(filepath.Join(cm.jobDir, "*.txt"))
    sort.Strings(files)
    
    var history strings.Builder
    for _, file := range files {
        content, _ := os.ReadFile(file)
        history.WriteString(fmt.Sprintf("\n=== %s ===\n", filepath.Base(file)))
        history.Write(content)
        history.WriteString("\n")
    }
    
    return history.String(), nil
}

func (cm *ContextManager) Cleanup() error {
    if cm.debugMode {
        log.Printf("Debug mode: keeping temp files in %s", cm.jobDir)
        return nil
    }
    
    return os.RemoveAll(cm.jobDir)
}

// For compaction - summarize old history
func (cm *ContextManager) CompactHistory(keepRecentFiles int) (string, error) {
    files, _ := filepath.Glob(filepath.Join(cm.jobDir, "*-prompt.txt"))
    sort.Strings(files)
    
    if len(files) <= keepRecentFiles {
        return cm.GetHistory()
    }
    
    // Keep recent files as-is
    recentFiles := files[len(files)-keepRecentFiles:]
    
    // Summarize older files
    oldFiles := files[:len(files)-keepRecentFiles]
    var summary strings.Builder
    summary.WriteString("=== Previous Work Summary ===\n")
    
    for _, file := range oldFiles {
        // Extract key information: files modified, tests run, errors
        content, _ := os.ReadFile(file)
        // Simple extraction - could be enhanced with LLM summarization
        summary.WriteString(fmt.Sprintf("Step %s: ", filepath.Base(file)))
        
        // Extract tool calls from corresponding json
        toolCallsFile := strings.Replace(file, "-prompt.txt", "-tool-calls.json", 1)
        if toolData, err := os.ReadFile(toolCallsFile); err == nil {
            var calls []ToolCall
            json.Unmarshal(toolData, &calls)
            summary.WriteString(fmt.Sprintf("%d tool calls\n", len(calls)))
        }
    }
    
    // Combine summary + recent history
    var fullContext strings.Builder
    fullContext.WriteString(summary.String())
    fullContext.WriteString("\n=== Recent Context ===\n")
    
    for _, file := range recentFiles {
        content, _ := os.ReadFile(file)
        fullContext.Write(content)
        fullContext.WriteString("\n")
    }
    
    return fullContext.String(), nil
}
```

### Usage in Agent

```go
func (a *Agent) ExecuteTask(ctx context.Context, task Task) (*Job, error) {
    job := a.createJob(task)
    
    // Create context manager
    contextMgr := context.NewContextManager(job.ID, a.config.DebugMode)
    defer contextMgr.Cleanup()
    
    // Build initial prompt
    initialPrompt := a.buildPrompt(task)
    contextMgr.SavePrompt(initialPrompt)
    
    for inferenceCount := 0; inferenceCount < maxInferences; inferenceCount++ {
        // Run inference
        response, err := a.llm.Infer(ctx, InferenceRequest{
            SystemPrompt: systemPrompt,
            UserPrompt:   initialPrompt,
        })
        
        if err != nil {
            // Save error for retry context
            contextMgr.SaveResponse(fmt.Sprintf("ERROR: %v", err))
            
            // For retry: load history and try again
            if inferenceCount < retryLimit {
                history, _ := contextMgr.GetHistory()
                initialPrompt = a.buildRetryPrompt(history, err)
                contextMgr.SavePrompt(initialPrompt)
                continue
            }
            return job, err
        }
        
        // Save response
        contextMgr.SaveResponse(response.Text)
        
        // Parse and save tool calls
        contextMgr.SaveToolCalls(response.ToolCalls)
        
        // Execute tools
        results := a.executeToolCalls(ctx, response.ToolCalls)
        contextMgr.SaveToolResults(results)
        
        // Check if done
        if response.NextAction == "COMPLETE" {
            break
        }
        
        // Build next prompt with context
        var promptContext string
        if a.shouldCompact() {
            promptContext, _ = contextMgr.CompactHistory(5)
        } else {
            promptContext, _ = contextMgr.GetHistory()
        }
        
        initialPrompt = a.buildNextPrompt(task, promptContext, results)
        contextMgr.SavePrompt(initialPrompt)
    }
    
    return job, nil
}
```

### Debug Mode

```json
// .pedroceli.json
{
  "debug": {
    "enabled": false,
    "keep_temp_files": false,
    "log_level": "info"
  }
}
```

```bash
# Enable debug mode
export PEDROCELI_DEBUG=true
pedroceli build --description "Add rate limiting"

# Temp files kept in /tmp/pedroceli-jobs/job-123-20250107-143022/
```

### Benefits

1. **No Memory Bloat**: Write to disk, not RAM
2. **Full History**: Never lose context, can search later
3. **Retry Support**: Load history to retry with context
4. **Debugging**: Inspect exact prompts/responses
5. **Compaction**: Summarize old files, keep recent ones
6. **Simple Format**: Plain text files, easy to read
7. **Auto Cleanup**: Remove on success (unless debug mode)

### Optional: Vector Embeddings (Future Enhancement)

If you want to add vector search later:

```go
// pkg/context/embeddings.go
func (cm *ContextManager) CreateEmbeddings() error {
    // Read all text files
    history, _ := cm.GetHistory()
    
    // Call embedding model (could be local with sentence-transformers)
    embeddings := generateEmbeddings(history)
    
    // Save to .embeddings file
    data, _ := json.Marshal(embeddings)
    return os.WriteFile(filepath.Join(cm.jobDir, "embeddings.json"), data, 0644)
}

func (cm *ContextManager) SearchHistory(query string) ([]string, error) {
    // Load embeddings
    // Find most similar chunks
    // Return relevant context
}
```

But for now, plain text files are simpler and work great for one-shot inference!

## Initialization & Dependency Checks

### Pre-Flight Validation

Before starting any job, Pedroceli validates all dependencies are available and exits immediately with helpful messages if anything is missing.

```go
// pkg/init/checker.go
package init

type DependencyChecker struct {
    config *config.Config
}

type CheckResult struct {
    Name     string
    Required bool
    Found    bool
    Path     string
    Version  string
    Error    string
}

func (dc *DependencyChecker) CheckAll() ([]CheckResult, error) {
    results := []CheckResult{}
    
    // Check inference backend
    if dc.config.Model.Type == "llamacpp" {
        results = append(results, dc.checkLlamaCpp())
        results = append(results, dc.checkModelFile())
    } else if dc.config.Model.Type == "ollama" {
        results = append(results, dc.checkOllama())
    }
    
    // Check required CLI tools
    results = append(results, dc.checkGit())
    results = append(results, dc.checkGitHubCLI())
    
    // Check optional but recommended tools
    results = append(results, dc.checkGo())
    
    // Check SSH access if using remote Spark
    if dc.config.Execution.RunOnSpark {
        results = append(results, dc.checkSparkSSH())
    }
    
    // Check for Whisper (web client only)
    if dc.checkingWebDeps {
        results = append(results, dc.checkWhisper())
        results = append(results, dc.checkFFmpeg())
    }
    
    // Validate any failures
    var failures []CheckResult
    for _, result := range results {
        if result.Required && !result.Found {
            failures = append(failures, result)
        }
    }
    
    return results, dc.formatErrors(failures)
}

func (dc *DependencyChecker) checkLlamaCpp() CheckResult {
    path := dc.config.Model.LlamaCppPath
    
    // Check if file exists
    if _, err := os.Stat(path); err != nil {
        return CheckResult{
            Name:     "llama.cpp",
            Required: true,
            Found:    false,
            Error:    fmt.Sprintf("llama-cli not found at %s", path),
        }
    }
    
    // Check if executable
    if !isExecutable(path) {
        return CheckResult{
            Name:     "llama.cpp",
            Required: true,
            Found:    false,
            Error:    fmt.Sprintf("%s is not executable", path),
        }
    }
    
    // Get version
    cmd := exec.Command(path, "--version")
    output, _ := cmd.CombinedOutput()
    version := strings.TrimSpace(string(output))
    
    return CheckResult{
        Name:     "llama.cpp",
        Required: true,
        Found:    true,
        Path:     path,
        Version:  version,
    }
}

func (dc *DependencyChecker) checkModelFile() CheckResult {
    modelPath := dc.config.Model.ModelPath
    
    if _, err := os.Stat(modelPath); err != nil {
        return CheckResult{
            Name:     "Model file",
            Required: true,
            Found:    false,
            Error:    fmt.Sprintf("Model not found at %s", modelPath),
        }
    }
    
    // Check file size (should be > 1GB for any real model)
    info, _ := os.Stat(modelPath)
    sizeMB := info.Size() / (1024 * 1024)
    
    if sizeMB < 100 {
        return CheckResult{
            Name:     "Model file",
            Required: true,
            Found:    false,
            Error:    fmt.Sprintf("Model file suspiciously small: %dMB", sizeMB),
        }
    }
    
    return CheckResult{
        Name:     "Model file",
        Required: true,
        Found:    true,
        Path:     modelPath,
        Version:  fmt.Sprintf("%dMB", sizeMB),
    }
}

func (dc *DependencyChecker) checkOllama() CheckResult {
    path, err := exec.LookPath("ollama")
    if err != nil {
        return CheckResult{
            Name:     "Ollama",
            Required: true,
            Found:    false,
            Error:    "ollama not found in PATH. Install: curl -fsSL https://ollama.com/install.sh | sh",
        }
    }
    
    // Check if model is pulled
    cmd := exec.Command("ollama", "list")
    output, _ := cmd.CombinedOutput()
    
    modelName := dc.config.Model.ModelName
    if !strings.Contains(string(output), modelName) {
        return CheckResult{
            Name:     "Ollama",
            Required: true,
            Found:    false,
            Error:    fmt.Sprintf("Model %s not found. Run: ollama pull %s", modelName, modelName),
        }
    }
    
    return CheckResult{
        Name:     "Ollama",
        Required: true,
        Found:    true,
        Path:     path,
        Version:  "OK (model available)",
    }
}

func (dc *DependencyChecker) checkGit() CheckResult {
    path, err := exec.LookPath("git")
    if err != nil {
        return CheckResult{
            Name:     "Git",
            Required: true,
            Found:    false,
            Error:    "git not found. Install git to manage code changes.",
        }
    }
    
    cmd := exec.Command("git", "--version")
    output, _ := cmd.CombinedOutput()
    version := strings.TrimSpace(string(output))
    
    return CheckResult{
        Name:     "Git",
        Required: true,
        Found:    true,
        Path:     path,
        Version:  version,
    }
}

func (dc *DependencyChecker) checkGitHubCLI() CheckResult {
    path, err := exec.LookPath("gh")
    if err != nil {
        return CheckResult{
            Name:     "GitHub CLI",
            Required: true,
            Found:    false,
            Error:    "gh not found. Install: https://cli.github.com/",
        }
    }
    
    // Check if authenticated
    cmd := exec.Command("gh", "auth", "status")
    if err := cmd.Run(); err != nil {
        return CheckResult{
            Name:     "GitHub CLI",
            Required: true,
            Found:    false,
            Error:    "gh not authenticated. Run: gh auth login",
        }
    }
    
    cmd = exec.Command("gh", "--version")
    output, _ := cmd.CombinedOutput()
    version := strings.TrimSpace(strings.Split(string(output), "\n")[0])
    
    return CheckResult{
        Name:     "GitHub CLI",
        Required: true,
        Found:    true,
        Path:     path,
        Version:  version,
    }
}

func (dc *DependencyChecker) checkGo() CheckResult {
    path, err := exec.LookPath("go")
    if err != nil {
        return CheckResult{
            Name:     "Go",
            Required: false,
            Found:    false,
            Error:    "go not found (needed for Go projects)",
        }
    }
    
    cmd := exec.Command("go", "version")
    output, _ := cmd.CombinedOutput()
    version := strings.TrimSpace(string(output))
    
    return CheckResult{
        Name:     "Go",
        Required: false,
        Found:    true,
        Path:     path,
        Version:  version,
    }
}

func (dc *DependencyChecker) checkSparkSSH() CheckResult {
    sshHost := dc.config.Execution.SparkSSH
    
    cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", sshHost, "echo OK")
    if err := cmd.Run(); err != nil {
        return CheckResult{
            Name:     "Spark SSH",
            Required: true,
            Found:    false,
            Error:    fmt.Sprintf("Cannot SSH to %s. Check SSH keys and config.", sshHost),
        }
    }
    
    return CheckResult{
        Name:     "Spark SSH",
        Required: true,
        Found:    true,
        Version:  "Connected",
    }
}

func (dc *DependencyChecker) checkWhisper() CheckResult {
    whisperPath := "/usr/local/bin/whisper-cpp"
    
    if _, err := os.Stat(whisperPath); err != nil {
        return CheckResult{
            Name:     "Whisper.cpp",
            Required: true,
            Found:    false,
            Error:    "whisper-cpp not found (needed for voice interface)",
        }
    }
    
    return CheckResult{
        Name:     "Whisper.cpp",
        Required: true,
        Found:    true,
        Path:     whisperPath,
    }
}

func (dc *DependencyChecker) checkFFmpeg() CheckResult {
    path, err := exec.LookPath("ffmpeg")
    if err != nil {
        return CheckResult{
            Name:     "FFmpeg",
            Required: true,
            Found:    false,
            Error:    "ffmpeg not found (needed for audio conversion)",
        }
    }
    
    return CheckResult{
        Name:     "FFmpeg",
        Required: true,
        Found:    true,
        Path:     path,
    }
}

func (dc *DependencyChecker) formatErrors(failures []CheckResult) error {
    if len(failures) == 0 {
        return nil
    }
    
    var msg strings.Builder
    msg.WriteString("\n❌ Dependency check failed:\n\n")
    
    for _, failure := range failures {
        msg.WriteString(fmt.Sprintf("  ✗ %s: %s\n", failure.Name, failure.Error))
    }
    
    msg.WriteString("\nPlease install missing dependencies and try again.\n")
    
    return fmt.Errorf(msg.String())
}

func isExecutable(path string) bool {
    info, err := os.Stat(path)
    if err != nil {
        return false
    }
    return info.Mode()&0111 != 0
}
```

### Usage in CLI

```go
// cmd/cli.go
func main() {
    // Load config
    config := loadConfig()
    
    // Check dependencies before doing anything
    checker := init.NewDependencyChecker(config)
    results, err := checker.CheckAll()
    
    if err != nil {
        // Print pretty error and exit
        fmt.Println(err)
        os.Exit(1)
    }
    
    // Optional: print successful checks in verbose mode
    if verbose {
        fmt.Println("✓ All dependencies OK")
        for _, result := range results {
            if result.Found {
                fmt.Printf("  ✓ %s: %s\n", result.Name, result.Version)
            }
        }
    }
    
    // Continue with normal execution
    rootCmd.Execute()
}
```

### Example Output

**Success:**
```
$ pedroceli build --description "Add rate limiting"
✓ All dependencies OK
  ✓ llama.cpp: v1.2.3
  ✓ Model file: 21349MB
  ✓ Git: git version 2.39.0
  ✓ GitHub CLI: gh version 2.40.0
  ✓ Go: go version go1.21.5

Job started: job-1699401234
```

**Failure:**
```
$ pedroceli build --description "Add rate limiting"

❌ Dependency check failed:

  ✗ llama.cpp: llama-cli not found at /usr/local/bin/llama-cli
  ✗ GitHub CLI: gh not authenticated. Run: gh auth login

Please install missing dependencies and try again.
```

### Config Flag

```json
// .pedroceli.json
{
  "init": {
    "skip_checks": false,
    "verbose": true
  }
}
```

```bash
# Skip checks (not recommended)
pedroceli build --skip-checks --description "Add feature"
```

### Benefits

1. **Fail Fast**: Catch missing dependencies before starting work
2. **Clear Errors**: Tell user exactly what's missing and how to fix it
3. **Version Info**: Show what versions are installed
4. **SSH Validation**: Test Spark connectivity before trying inference
5. **Model Validation**: Ensure model file exists and is correct size
6. **Auth Checks**: Verify GitHub CLI is authenticated

This prevents frustrating failures 10 minutes into a job because `gh` wasn't authenticated!

## Context Window Management

### The Challenge

Different models have different context windows, and we need to be **acutely aware** of limits:

| Model | Context Window | Notes |
|-------|----------------|-------|
| Qwen 2.5 Coder 7B | 32k tokens | ~24k usable |
| Qwen 2.5 Coder 32B | 32k tokens | ~24k usable |
| Qwen 2.5 Coder 72B | 128k tokens | ~96k usable |
| Llama 3.1 70B | 128k tokens | ~96k usable |
| DeepSeek Coder 33B | 16k tokens | ~12k usable |

**Rule of Thumb**: Use 75% of stated context window (leave room for response)

### Configuration

#### llama.cpp (User Configurable)

```json
// .pedroceli.json
{
  "model": {
    "type": "llamacpp",
    "model_path": "/models/qwen2.5-coder-32b.gguf",
    "llamacpp_path": "/usr/local/bin/llama-cli",
    "context_size": 32768,
    "usable_context": 24576,
    "temperature": 0.2
  }
}
```

**For llama.cpp**: Assume user knows their model's limits.

#### Ollama (Predefined per Model)

```go
// pkg/llm/ollama.go

var ollamaModelContexts = map[string]int{
    "qwen2.5-coder:7b":      32768,
    "qwen2.5-coder:32b":     32768,
    "qwen2.5-coder:72b":     131072,
    "deepseek-coder:33b":    16384,
    "codellama:34b":         16384,
    "llama3.1:70b":          131072,
}

func (o *OllamaClient) GetContextWindow() int {
    if ctx, ok := ollamaModelContexts[o.modelName]; ok {
        return ctx
    }
    // Default conservative estimate
    return 8192
}

func (o *OllamaClient) GetUsableContext() int {
    // Use 75% of context window
    return o.GetContextWindow() * 3 / 4
}
```

```json
// .pedroceli.json (Ollama)
{
  "model": {
    "type": "ollama",
    "model_name": "qwen2.5-coder:32b"
    // context_size inferred from model
  }
}
```

### Token Estimation

```go
// pkg/llm/tokens.go
package llm

// Rough token estimation (1 token ≈ 4 characters for English)
func EstimateTokens(text string) int {
    return len(text) / 4
}

// More accurate: count words and punctuation
func EstimateTokensAccurate(text string) int {
    words := len(strings.Fields(text))
    // Average: 1.3 tokens per word
    return int(float64(words) * 1.3)
}

type ContextBudget struct {
    Total          int
    Usable         int
    SystemPrompt   int
    TaskPrompt     int
    History        int
    ToolDefinitions int
    Available      int
}

func (cb *ContextBudget) Calculate(config *Config) *ContextBudget {
    total := config.Model.ContextSize
    usable := total * 3 / 4  // 75% usable
    
    systemPrompt := EstimateTokens(buildSystemPrompt())
    taskPrompt := EstimateTokens(buildTaskPrompt())
    toolDefs := EstimateTokens(buildToolDefinitions())
    
    available := usable - systemPrompt - taskPrompt - toolDefs
    
    return &ContextBudget{
        Total:           total,
        Usable:          usable,
        SystemPrompt:    systemPrompt,
        TaskPrompt:      taskPrompt,
        ToolDefinitions: toolDefs,
        Available:       available,
    }
}

func (cb *ContextBudget) CanFitHistory(historyTokens int) bool {
    return historyTokens <= cb.Available
}

func (cb *ContextBudget) MaxFilesSize() int {
    // Leave room for history and responses
    return cb.Available - 2000  // Reserve 2k for history/responses
}
```

### Strategic Code Loading

```go
// pkg/agent/context.go

type ContextLoader struct {
    budget   *ContextBudget
    repoPath string
}

func (cl *ContextLoader) LoadRelevantFiles(task Task) map[string]string {
    files := make(map[string]string)
    tokenCount := 0
    maxTokens := cl.budget.MaxFilesSize()
    
    // Strategy 1: Load explicitly mentioned files
    for _, path := range task.Files {
        content, _ := os.ReadFile(filepath.Join(cl.repoPath, path))
        tokens := EstimateTokens(string(content))
        
        if tokenCount+tokens > maxTokens {
            // File too large, include summary or key parts
            files[path] = cl.summarizeFile(path, content)
        } else {
            files[path] = string(content)
            tokenCount += tokens
        }
    }
    
    // Strategy 2: Load related files based on imports/references
    related := cl.findRelatedFiles(task.Files)
    for _, path := range related {
        if tokenCount >= maxTokens {
            break
        }
        
        content, _ := os.ReadFile(filepath.Join(cl.repoPath, path))
        tokens := EstimateTokens(string(content))
        
        if tokenCount+tokens <= maxTokens {
            files[path] = string(content)
            tokenCount += tokens
        }
    }
    
    log.Printf("Loaded %d files, %d tokens (budget: %d)", 
        len(files), tokenCount, maxTokens)
    
    return files
}

func (cl *ContextLoader) summarizeFile(path string, content []byte) string {
    // For large files, extract key parts:
    // - Package declaration
    // - Imports
    // - Function signatures
    // - Type definitions
    
    lines := strings.Split(string(content), "\n")
    var summary []string
    
    summary = append(summary, fmt.Sprintf("// %s (summarized - too large for context)", path))
    
    for _, line := range lines {
        trimmed := strings.TrimSpace(line)
        
        // Keep important declarations
        if strings.HasPrefix(trimmed, "package ") ||
           strings.HasPrefix(trimmed, "import ") ||
           strings.HasPrefix(trimmed, "type ") ||
           strings.HasPrefix(trimmed, "func ") ||
           strings.HasPrefix(trimmed, "const ") ||
           strings.HasPrefix(trimmed, "var ") {
            summary = append(summary, line)
        }
    }
    
    return strings.Join(summary, "\n")
}

func (cl *ContextLoader) findRelatedFiles(files []string) []string {
    // Parse Go imports, find related files
    // This is project-specific logic
    var related []string
    
    for _, file := range files {
        content, _ := os.ReadFile(filepath.Join(cl.repoPath, file))
        
        // Extract imports
        imports := extractImports(string(content))
        
        for _, imp := range imports {
            // Find files that match import path
            if localFile := resolveImport(imp); localFile != "" {
                related = append(related, localFile)
            }
        }
    }
    
    return related
}
```

### History Compaction Strategy

```go
// pkg/context/manager.go

func (cm *ContextManager) GetHistoryWithinBudget(budget int) (string, error) {
    files, _ := filepath.Glob(filepath.Join(cm.jobDir, "*.txt"))
    sort.Strings(files)
    
    // Start with most recent files
    var selected []string
    totalTokens := 0
    
    // Always keep last N inferences
    keepRecent := 3
    recentFiles := files
    if len(files) > keepRecent*2 {
        recentFiles = files[len(files)-keepRecent*2:]
    }
    
    // Estimate tokens for recent files
    for _, file := range recentFiles {
        content, _ := os.ReadFile(file)
        tokens := EstimateTokens(string(content))
        
        if totalTokens+tokens > budget {
            // Can't fit all recent - summarize older
            break
        }
        
        selected = append(selected, file)
        totalTokens += tokens
    }
    
    // If we have room and older files exist
    if totalTokens < budget && len(files) > len(recentFiles) {
        oldFiles := files[:len(files)-len(recentFiles)]
        summary := cm.summarizeHistory(oldFiles)
        summaryTokens := EstimateTokens(summary)
        
        if totalTokens+summaryTokens <= budget {
            // Prepend summary
            var result strings.Builder
            result.WriteString("=== Previous Work Summary ===\n")
            result.WriteString(summary)
            result.WriteString("\n\n=== Recent Context ===\n")
            
            for _, file := range selected {
                content, _ := os.ReadFile(file)
                result.Write(content)
                result.WriteString("\n")
            }
            
            return result.String(), nil
        }
    }
    
    // Just return recent files
    var result strings.Builder
    for _, file := range selected {
        content, _ := os.ReadFile(file)
        result.Write(content)
        result.WriteString("\n")
    }
    
    return result.String(), nil
}

func (cm *ContextManager) summarizeHistory(files []string) string {
    var summary strings.Builder
    
    for _, file := range files {
        // Extract key facts:
        // - What files were modified
        // - What tests were run
        // - Any errors encountered
        
        if strings.Contains(file, "tool-calls.json") {
            data, _ := os.ReadFile(file)
            var calls []ToolCall
            json.Unmarshal(data, &calls)
            
            summary.WriteString(fmt.Sprintf("Step %s: %d tool calls\n", 
                filepath.Base(file), len(calls)))
            
            // List files modified
            for _, call := range calls {
                if call.Name == "write_file" {
                    summary.WriteString(fmt.Sprintf("  - Modified: %s\n", 
                        call.Args["path"]))
                }
            }
        }
    }
    
    return summary.String()
}
```

### Config Validation

```go
// pkg/config/validate.go

func (c *Config) Validate() error {
    // Validate context size is reasonable
    if c.Model.Type == "llamacpp" {
        if c.Model.ContextSize < 2048 {
            return fmt.Errorf("context_size too small: %d (minimum 2048)", 
                c.Model.ContextSize)
        }
        
        if c.Model.ContextSize > 200000 {
            return fmt.Errorf("context_size suspiciously large: %d", 
                c.Model.ContextSize)
        }
        
        // Warn if usable_context not set
        if c.Model.UsableContext == 0 {
            c.Model.UsableContext = c.Model.ContextSize * 3 / 4
            log.Printf("⚠️  usable_context not set, using 75%%: %d tokens", 
                c.Model.UsableContext)
        }
    }
    
    if c.Model.Type == "ollama" {
        // Check if model is known
        if _, ok := ollamaModelContexts[c.Model.ModelName]; !ok {
            log.Printf("⚠️  Unknown Ollama model: %s (using conservative 8k context)", 
                c.Model.ModelName)
        }
    }
    
    return nil
}
```

### CLI Warnings

```bash
$ pedroceli build --description "Refactor entire codebase"

⚠️  Context Budget:
  Total: 32768 tokens
  Usable: 24576 tokens (75%)
  System Prompt: 2048 tokens
  Task Prompt: 512 tokens
  Tool Definitions: 1024 tokens
  Available for code: 20992 tokens (~84KB of text)

ℹ️  Large repository detected. Will load files strategically.

Job started: job-1699401234
```

### Web UI (Future)

```html
<!-- Model selector with context info -->
<select id="modelSelect">
  <option value="qwen2.5-coder:7b">
    Qwen 2.5 Coder 7B (32k context) - Fast
  </option>
  <option value="qwen2.5-coder:32b" selected>
    Qwen 2.5 Coder 32B (32k context) - Balanced
  </option>
  <option value="qwen2.5-coder:72b">
    Qwen 2.5 Coder 72B (128k context) - Large context
  </option>
</select>

<div id="contextInfo">
  📊 Context Budget: ~21k tokens available for code
</div>
```

### Repository Size Considerations

```go
// pkg/agent/estimator.go

type RepoEstimate struct {
    TotalFiles    int
    TotalLines    int
    TotalTokens   int
    CanFitInContext bool
    Strategy      string
}

func EstimateRepo(repoPath string, budget int) *RepoEstimate {
    var totalLines, totalTokens int
    fileCount := 0
    
    filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
        if info.IsDir() || !isCodeFile(path) {
            return nil
        }
        
        content, _ := os.ReadFile(path)
        lines := len(strings.Split(string(content), "\n"))
        tokens := EstimateTokens(string(content))
        
        totalLines += lines
        totalTokens += tokens
        fileCount++
        
        return nil
    })
    
    canFit := totalTokens < budget
    
    var strategy string
    if canFit {
        strategy = "Load all files"
    } else if totalTokens < budget*2 {
        strategy = "Load most files, summarize largest"
    } else {
        strategy = "Load strategically based on task"
    }
    
    return &RepoEstimate{
        TotalFiles:      fileCount,
        TotalLines:      totalLines,
        TotalTokens:     totalTokens,
        CanFitInContext: canFit,
        Strategy:        strategy,
    }
}
```

### Key Principles

1. **Always track token usage** - Budget is sacred
2. **Load strategically** - Task-relevant files first
3. **Summarize large files** - Signatures > implementations
4. **Compact history** - Keep recent, summarize old
5. **Warn users** - Show context limits upfront
6. **Model-aware** - Different limits per model
7. **Leave room** - Reserve tokens for responses

### Future Enhancement: Hugging Face API

```go
// pkg/llm/hf_info.go (Future)

// Query Hugging Face for model context window
func GetModelContextFromHF(modelPath string) (int, error) {
    // Parse model name from path
    // Query HF API
    // Return context_length from config
    
    // This would require HF MCP or API access
    // Stretch goal for future
}
```

### Config Examples

**Small model (16k context):**
```json
{
  "model": {
    "type": "llamacpp",
    "model_path": "/models/deepseek-coder-33b.gguf",
    "context_size": 16384,
    "usable_context": 12288
  }
}
```

**Large model (128k context):**
```json
{
  "model": {
    "type": "llamacpp",
    "model_path": "/models/qwen2.5-coder-72b.gguf",
    "context_size": 131072,
    "usable_context": 98304
  }
}
```

**Ollama (auto-detected):**
```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen2.5-coder:32b"
    // context_size = 32768 (auto)
    // usable_context = 24576 (auto)
  }
}
```

## Cross-Platform Compatibility

### Target Platforms

**Primary:**
- 🍎 **macOS** - Development environment
- 🐧 **Ubuntu 24.04** - Production environment (DGX Spark)

**Future:**
- Windows (if needed)

### OS-Specific Tool Differences

#### Text Processing Tools

| Tool | macOS (BSD) | Linux (GNU) | Solution |
|------|-------------|-------------|----------|
| `sed` | BSD sed | GNU sed | Use Go strings package instead |
| `grep` | BSD grep | GNU grep | Use Go regexp package instead |
| `find` | BSD find | GNU find | Use Go filepath.Walk instead |
| `xargs` | BSD xargs | GNU xargs | Build commands in Go |

**Problem**: BSD and GNU versions have different flags and behavior.

**Solution**: Don't shell out to these tools - use Go standard library.

#### File Editing Strategy

**❌ Don't Use:**
```bash
# sed is different on Mac vs Linux
sed -i '' 's/old/new/g' file.txt  # macOS
sed -i 's/old/new/g' file.txt     # Linux
```

**✅ Use Go Instead:**
```go
// pkg/tools/file.go
func (f *FileTool) ReplaceInFile(path, old, new string) error {
    content, err := os.ReadFile(path)
    if err != nil {
        return err
    }
    
    newContent := strings.ReplaceAll(string(content), old, new)
    
    return os.WriteFile(path, []byte(newContent), 0644)
}
```

#### Safe Cross-Platform Tools

These work the same on both:
- ✅ `git` - Same everywhere
- ✅ `go` - Same everywhere  
- ✅ `gh` (GitHub CLI) - Same everywhere
- ✅ Go stdlib - file operations, strings, regexp

### OS Detection

```go
// pkg/platform/detect.go
package platform

import (
    "runtime"
)

type OS string

const (
    macOS  OS = "darwin"
    Linux  OS = "linux"
    Windows OS = "windows"
)

func Current() OS {
    return OS(runtime.GOOS)
}

func IsMac() bool {
    return runtime.GOOS == "darwin"
}

func IsLinux() bool {
    return runtime.GOOS == "linux"
}

func IsWindows() bool {
    return runtime.GOOS == "windows"
}
```

### Tool Implementations

#### File Tool (Cross-Platform)

```go
// pkg/tools/file.go
type FileTool struct {
    maxFileSize int64
}

func (f *FileTool) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
    action := args["action"].(string)
    
    switch action {
    case "read":
        return f.read(args)
    case "write":
        return f.write(args)
    case "replace":
        return f.replace(args)  // Pure Go, no sed
    case "append":
        return f.append(args)
    default:
        return ToolResult{Success: false}, fmt.Errorf("unknown action: %s", action)
    }
}

func (f *FileTool) read(args map[string]interface{}) (ToolResult, error) {
    path := args["path"].(string)
    
    content, err := os.ReadFile(path)
    if err != nil {
        return ToolResult{Success: false, Error: err}, nil
    }
    
    return ToolResult{
        Success: true,
        Output:  string(content),
    }, nil
}

func (f *FileTool) write(args map[string]interface{}) (ToolResult, error) {
    path := args["path"].(string)
    content := args["content"].(string)
    
    // Ensure directory exists
    dir := filepath.Dir(path)
    os.MkdirAll(dir, 0755)
    
    err := os.WriteFile(path, []byte(content), 0644)
    
    return ToolResult{
        Success:       err == nil,
        Error:         err,
        ModifiedFiles: []string{path},
    }, nil
}

func (f *FileTool) replace(args map[string]interface{}) (ToolResult, error) {
    path := args["path"].(string)
    old := args["old"].(string)
    new := args["new"].(string)
    
    content, err := os.ReadFile(path)
    if err != nil {
        return ToolResult{Success: false, Error: err}, nil
    }
    
    // Use Go strings, not sed
    newContent := strings.ReplaceAll(string(content), old, new)
    
    err = os.WriteFile(path, []byte(newContent), 0644)
    
    return ToolResult{
        Success:       err == nil,
        Error:         err,
        ModifiedFiles: []string{path},
    }, nil
}
```

#### Bash Tool (Restricted Set)

```go
// pkg/tools/bash.go
type BashTool struct {
    allowedCommands   []string
    forbiddenCommands []string
}

var safeCrossplatformCommands = []string{
    "git",
    "gh", 
    "go",
    "cat",    // Same on both
    "ls",     // Same on both
    "head",   // Same on both
    "tail",   // Same on both
    "wc",     // Same on both
    "sort",   // Same on both
    "uniq",   // Same on both
}

// Don't allow sed, grep, find, xargs - do it in Go instead
var forbiddenCommands = []string{
    "sed",    // Different on Mac/Linux
    "grep",   // Use Go regexp
    "find",   // Use Go filepath.Walk
    "xargs",  // Build commands in Go
    "rm",
    "mv",
    "dd",
}

func (b *BashTool) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
    command := args["command"].(string)
    
    // Parse command to check first word
    fields := strings.Fields(command)
    if len(fields) == 0 {
        return ToolResult{Success: false}, fmt.Errorf("empty command")
    }
    
    baseCmd := fields[0]
    
    // Check if allowed
    if !b.isAllowed(baseCmd) {
        return ToolResult{
            Success: false,
            Error:   fmt.Errorf("command not allowed: %s", baseCmd),
        }, nil
    }
    
    // Execute
    cmd := exec.CommandContext(ctx, "sh", "-c", command)
    output, err := cmd.CombinedOutput()
    
    return ToolResult{
        Success: err == nil,
        Output:  string(output),
        Error:   err,
    }, nil
}
```

### Build Configuration

#### Makefile

```makefile
.PHONY: build build-mac build-linux test install clean

# Default build for current platform
build:
	go build -o pedroceli cmd/cli.go

# Build for macOS
build-mac:
	GOOS=darwin GOARCH=arm64 go build -o pedroceli-mac-arm64 cmd/cli.go
	GOOS=darwin GOARCH=amd64 go build -o pedroceli-mac-amd64 cmd/cli.go

# Build for Linux (Ubuntu on Spark)
build-linux:
	GOOS=linux GOARCH=amd64 go build -o pedroceli-linux-amd64 cmd/cli.go

# Build for both
build-all: build-mac build-linux

# Test on current platform
test:
	go test ./...

# Install locally
install:
	go build -o pedroceli cmd/cli.go
	sudo mv pedroceli /usr/local/bin/

clean:
	rm -f pedroceli pedroceli-*
```

#### Usage

```bash
# Development on Mac
make build
./pedroceli build --description "Test"

# Build for Ubuntu (Spark)
make build-linux

# Copy to Spark
scp pedroceli-linux-amd64 miriah@dgx-spark-01:~/bin/pedroceli

# Run on Spark
ssh miriah@dgx-spark-01
~/bin/pedroceli build --description "Test"
```

### Dependency Checker Updates

```go
// pkg/init/checker.go

func (dc *DependencyChecker) checkPlatformSpecific() []CheckResult {
    results := []CheckResult{}
    
    os := platform.Current()
    
    switch os {
    case platform.macOS:
        // Mac-specific checks
        results = append(results, dc.checkXcode())
        
    case platform.Linux:
        // Linux-specific checks
        results = append(results, dc.checkBuildEssentials())
        
        // Check for Ubuntu specifically
        if dc.isUbuntu() {
            results = append(results, dc.checkUbuntuVersion())
        }
    }
    
    return results
}

func (dc *DependencyChecker) isUbuntu() bool {
    content, err := os.ReadFile("/etc/os-release")
    if err != nil {
        return false
    }
    
    return strings.Contains(string(content), "Ubuntu")
}

func (dc *DependencyChecker) checkUbuntuVersion() CheckResult {
    content, _ := os.ReadFile("/etc/os-release")
    
    // Parse VERSION_ID
    for _, line := range strings.Split(string(content), "\n") {
        if strings.HasPrefix(line, "VERSION_ID=") {
            version := strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
            
            // We target Ubuntu 24.04
            if version >= "24.04" {
                return CheckResult{
                    Name:     "Ubuntu Version",
                    Required: false,
                    Found:    true,
                    Version:  version,
                }
            }
            
            return CheckResult{
                Name:     "Ubuntu Version",
                Required: false,
                Found:    true,
                Version:  version,
                Error:    "Ubuntu 24.04+ recommended",
            }
        }
    }
    
    return CheckResult{
        Name:     "Ubuntu Version",
        Required: false,
        Found:    false,
    }
}
```

### Configuration

```json
// .pedroceli.json
{
  "platform": {
    "os": "auto",
    "shell": "/bin/sh"
  },
  "tools": {
    "allowed_bash_commands": [
      "git",
      "gh",
      "go",
      "cat",
      "ls",
      "head",
      "tail",
      "wc"
    ],
    "forbidden_commands": [
      "sed",
      "grep",
      "find",
      "xargs",
      "rm",
      "mv"
    ]
  }
}
```

### Testing Strategy

#### Test Matrix

```bash
# On Mac (development)
make test
make build
./pedroceli build --description "Test Mac"

# Build Linux binary on Mac
make build-linux

# Test on Ubuntu (Spark)
scp pedroceli-linux-amd64 spark:~/pedroceli
ssh spark
~/pedroceli build --description "Test Ubuntu"
```

#### CI/CD (Future)

```yaml
# .github/workflows/test.yml
name: Test
on: [push, pull_request]

jobs:
  test-mac:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - run: make test
      - run: make build-mac
      
  test-linux:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - run: make test
      - run: make build-linux
```

### Key Principles

1. **Use Go stdlib** - Not shell commands with different flags
2. **Test both platforms** - Mac for dev, Ubuntu for production
3. **Cross-compile** - Build Linux binary on Mac
4. **Detect OS** - Check platform and adjust behavior
5. **No sed/grep** - Use Go strings/regexp instead
6. **Document differences** - Clear about what works where

### Development Workflow

```
┌─────────────┐
│   Mac       │  Development
│  (ARM64)    │  - Write code
│             │  - Test locally
│             │  - Build: make build
└─────────────┘
       │
       │ make build-linux
       │ scp to Spark
       ▼
┌─────────────┐
│   Spark     │  Production
│ Ubuntu      │  - Run jobs
│ (x86_64)    │  - llama.cpp inference
│             │  - Full GPU access
└─────────────┘
```

## Implementation Phases

### Phase 1: MCP Server Core with llama.cpp (Weeks 1-2)

**Goal**: Build the MCP server with llama.cpp backend and all agent tools

#### 1.1 Basic Infrastructure
- [ ] Project setup with Go modules
- [ ] Config file parsing (`.pedroceli.json`)
- [ ] **OS detection and platform utilities**
- [ ] **Dependency checker with platform-specific checks**
- [ ] **File-based context manager (temp files, not in-memory)**
- [ ] llama.cpp client implementation
- [ ] Job state management (save/load to disk)
- [ ] Basic logging and error handling
- [ ] **Debug mode support (keeps temp files)**
- [ ] **Makefile for Mac/Linux builds**

#### 1.2 Tool System (Cross-Platform)
- [ ] Tool interface definition
- [ ] **Bash tool (safe commands only, no sed/grep/find)**
- [ ] **Git tool (same on Mac/Linux)**
- [ ] **File tool (pure Go, no sed - strings.ReplaceAll)**
- [ ] **Test tool (run tests, parse results)**

#### 1.3 MCP Server Implementation
- [ ] MCP protocol handler (stdio transport)
- [ ] Tool registration system
- [ ] Request/response handling
- [ ] Tool execution orchestration

#### 1.4 Agent: Background Builder
```
Tool: build_feature
Input: description, issue (optional), criteria (optional)
Process:
  1. Generate plan via llama.cpp one-shot
  2. Execute tools (read files, write code, run tests)
  3. Commit changes to branch
  4. Push draft PR
Output: job_id, status
```

#### 1.5 Agent: Debugger
```
Tool: debug_issue
Input: symptoms, logs (optional), files (optional)
Process:
  1. Analyze issue via llama.cpp one-shot
  2. Identify root cause
  3. Apply fix
  4. Run tests to verify
  5. Commit fix
Output: job_id, status
```

#### 1.6 Agent: PR Reviewer
```
Tool: review_pr
Input: branch, pr_number (optional)
Process:
  1. Get git diff
  2. Analyze with llama.cpp (blind review)
  3. Check: quality, bugs, tests, performance, security
  4. Generate review comments
  5. Post to GitHub (optional)
Output: review_text, issues_found
```

#### 1.7 Agent: Issue Triager
```
Tool: triage_issue
Input: description, error_logs (optional)
Process:
  1. Analyze symptoms
  2. Run diagnostic commands
  3. Check logs and state
  4. Generate triage report
  5. Suggest fix approach
Output: diagnosis, recommended_action
```

#### 1.8 Job Management Tools
```
Tool: get_job_status
Input: job_id
Output: status, progress, results

Tool: list_jobs
Input: (none)
Output: array of jobs

Tool: cancel_job
Input: job_id
Output: success/failure
```

**Deliverable**: Working MCP server that can be called via stdio

```bash
# Test MCP server directly
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"build_feature","arguments":{"description":"Add rate limiting"}}}' | pedroceli mcp-server
```

---

### Phase 2: CLI Client (Week 3)

**Goal**: Build CLI that wraps MCP server calls for easy command-line usage

#### 2.1 CLI Commands

```bash
# Build feature
pedroceli build --description "Add rate limiting" --issue GH-123

# Debug issue
pedroceli debug --symptoms "Bot crashes on startup" --logs error.log

# Review PR
pedroceli review --branch feature/rate-limiting

# Triage issue
pedroceli triage --description "Memory leak in Discord handler"

# Check status
pedroceli status [job-id]

# List jobs
pedroceli list

# Cancel job
pedroceli cancel <job-id>
```

#### 2.2 Implementation

```go
// cmd/cli.go
package main

import (
    "github.com/spf13/cobra"
    "pedroceli/pkg/mcp"
)

func main() {
    rootCmd := &cobra.Command{Use: "pedroceli"}
    
    // Build command
    buildCmd := &cobra.Command{
        Use:   "build",
        Short: "Build a new feature",
        RunE: func(cmd *cobra.Command, args []string) error {
            description, _ := cmd.Flags().GetString("description")
            issue, _ := cmd.Flags().GetString("issue")
            
            // Start MCP server as subprocess
            client := mcp.NewClient("pedroceli", []string{"mcp-server"})
            
            // Call build_feature tool
            result := client.CallTool("build_feature", map[string]interface{}{
                "description": description,
                "issue": issue,
            })
            
            fmt.Printf("Job started: %s\n", result["job_id"])
            return nil
        },
    }
    buildCmd.Flags().StringP("description", "d", "", "Feature description")
    buildCmd.Flags().StringP("issue", "i", "", "GitHub issue number")
    
    // Debug command
    debugCmd := &cobra.Command{
        Use:   "debug",
        Short: "Debug and fix an issue",
        RunE: func(cmd *cobra.Command, args []string) error {
            symptoms, _ := cmd.Flags().GetString("symptoms")
            logs, _ := cmd.Flags().GetString("logs")
            
            client := mcp.NewClient("pedroceli", []string{"mcp-server"})
            
            result := client.CallTool("debug_issue", map[string]interface{}{
                "symptoms": symptoms,
                "logs": logs,
            })
            
            fmt.Printf("Debug job started: %s\n", result["job_id"])
            return nil
        },
    }
    debugCmd.Flags().StringP("symptoms", "s", "", "Issue symptoms")
    debugCmd.Flags().StringP("logs", "l", "", "Path to log file")
    
    // Review command
    reviewCmd := &cobra.Command{
        Use:   "review",
        Short: "Review a PR",
        RunE: func(cmd *cobra.Command, args []string) error {
            branch, _ := cmd.Flags().GetString("branch")
            
            client := mcp.NewClient("pedroceli", []string{"mcp-server"})
            
            result := client.CallTool("review_pr", map[string]interface{}{
                "branch": branch,
            })
            
            fmt.Printf("Review:\n%s\n", result["review_text"])
            return nil
        },
    }
    reviewCmd.Flags().StringP("branch", "b", "", "Branch name")
    
    // Triage command
    triageCmd := &cobra.Command{
        Use:   "triage",
        Short: "Triage an issue",
        RunE: func(cmd *cobra.Command, args []string) error {
            description, _ := cmd.Flags().GetString("description")
            
            client := mcp.NewClient("pedroceli", []string{"mcp-server"})
            
            result := client.CallTool("triage_issue", map[string]interface{}{
                "description": description,
            })
            
            fmt.Printf("Diagnosis:\n%s\n", result["diagnosis"])
            fmt.Printf("\nRecommended action:\n%s\n", result["recommended_action"])
            return nil
        },
    }
    triageCmd.Flags().StringP("description", "d", "", "Issue description")
    
    // Status command
    statusCmd := &cobra.Command{
        Use:   "status [job-id]",
        Short: "Get job status",
        Args:  cobra.MaximumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            client := mcp.NewClient("pedroceli", []string{"mcp-server"})
            
            if len(args) == 0 {
                // List all jobs
                result := client.CallTool("list_jobs", map[string]interface{}{})
                // Display job list
            } else {
                // Get specific job
                result := client.CallTool("get_job_status", map[string]interface{}{
                    "job_id": args[0],
                })
                // Display job status
            }
            return nil
        },
    }
    
    rootCmd.AddCommand(buildCmd, debugCmd, reviewCmd, triageCmd, statusCmd)
    rootCmd.Execute()
}
```

**Deliverable**: Full CLI that works locally

```bash
# Example usage
cd ~/pedro-bot
pedroceli build --description "Add webhook validation" --issue GH-123
# Output: Job started: job-1699401234
# Monitor: pedroceli status job-1699401234
```

---

### Phase 3: Ollama Backend (Week 4)

**Goal**: Add Ollama as secondary backend for easier setup

#### 3.1 Ollama Client Implementation

Same interface as llama.cpp, different implementation:

```go
// pkg/llm/ollama.go
type OllamaClient struct {
    ollamaPath  string
    modelName   string
    temperature float64
    numCtx      int
}

func (o *OllamaClient) Infer(ctx context.Context, req InferenceRequest) (*InferenceResponse, error) {
    // Build ollama command
    cmd := exec.CommandContext(ctx, o.ollamaPath, "run", o.modelName)
    cmd.Stdin = strings.NewReader(req.UserPrompt)
    
    output, err := cmd.CombinedOutput()
    // Parse and return
}
```

#### 3.2 Config Switch

```json
// .pedroceli.json
{
  "model": {
    "type": "ollama",  // or "llamacpp"
    "model_name": "qwen2.5-coder:32b"
  }
}
```

#### 3.3 Backend Factory

```go
func NewInferenceBackend(config *Config) InferenceBackend {
    switch config.Model.Type {
    case "llamacpp":
        return NewLlamaCppClient(config)
    case "ollama":
        return NewOllamaClient(config)
    }
}
```

**Deliverable**: Can switch between llama.cpp and Ollama via config

---

### Phase 4: Web Client with Speech-to-Text (Weeks 5-6)

**Goal**: Voice-driven web interface on Tailnet

#### 4.1 Web Server Setup

```go
// cmd/web.go
func main() {
    // Initialize Whisper for STT
    whisper := stt.NewWhisperClient("/usr/local/bin/whisper-cpp")
    
    // Connect to MCP server
    mcpClient := mcp.NewClient("pedroceli", []string{"mcp-server"})
    
    // Setup routes
    http.HandleFunc("/", serveIndex)
    http.HandleFunc("/api/transcribe", transcribeHandler(whisper))
    http.HandleFunc("/api/build", buildHandler(mcpClient))
    http.HandleFunc("/api/debug", debugHandler(mcpClient))
    http.HandleFunc("/api/review", reviewHandler(mcpClient))
    http.HandleFunc("/api/jobs", jobsHandler(mcpClient))
    
    http.ListenAndServe(":8080", nil)
}
```

#### 4.2 Voice UI

```html
<!-- web/static/index.html -->
<div class="container">
    <h1>🎤 Pedroceli Voice</h1>
    
    <div class="agent-selector">
        <button class="agent-btn" data-agent="build">🏗️ Build</button>
        <button class="agent-btn" data-agent="debug">🐛 Debug</button>
        <button class="agent-btn" data-agent="review">📝 Review</button>
        <button class="agent-btn" data-agent="triage">🔍 Triage</button>
    </div>
    
    <button id="recordBtn">🎤 Tap to Record</button>
    
    <textarea id="transcription"></textarea>
    
    <button id="startJobBtn">Start Job</button>
    
    <div id="jobs"></div>
</div>
```

```javascript
// web/static/app.js
let selectedAgent = 'build';
let mediaRecorder;

document.querySelectorAll('.agent-btn').forEach(btn => {
    btn.addEventListener('click', () => {
        selectedAgent = btn.dataset.agent;
    });
});

recordBtn.addEventListener('click', async () => {
    if (!recording) {
        // Start recording
        const stream = await navigator.mediaDevices.getUserMedia({audio: true});
        mediaRecorder = new MediaRecorder(stream);
        // ... handle recording
    } else {
        // Stop and transcribe
        mediaRecorder.stop();
        const blob = new Blob(audioChunks, {type: 'audio/webm'});
        
        const formData = new FormData();
        formData.append('audio', blob);
        
        const response = await fetch('/api/transcribe', {
            method: 'POST',
            body: formData
        });
        
        const data = await response.json();
        document.getElementById('transcription').value = data.text;
    }
});

startJobBtn.addEventListener('click', async () => {
    const description = document.getElementById('transcription').value;
    
    const response = await fetch(`/api/${selectedAgent}`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({description})
    });
    
    const result = await response.json();
    alert(`Job started: ${result.job_id}`);
    loadJobs();
});
```

#### 4.3 Whisper.cpp Integration

```go
// pkg/stt/whisper.go
func (w *WhisperClient) Transcribe(audioFile string) (string, error) {
    // Convert to wav
    wavFile := convertToWav(audioFile)
    
    // Run whisper
    cmd := exec.Command(w.whisperPath,
        "-m", w.modelPath,
        "-f", wavFile,
        "--output-txt")
    
    cmd.Run()
    
    // Read output
    transcript, _ := os.ReadFile(wavFile + ".txt")
    return string(transcript), nil
}
```

#### 4.4 Deploy on Tailnet

```bash
# On DGX Spark
cd pedroceli
go build -o pedroceli-web cmd/web.go

# Setup Tailscale
sudo tailscale serve https / http://localhost:8080

# Access from phone
# https://dgx-spark.tailnet-name.ts.net
```

**Deliverable**: Voice-driven web UI accessible from phone via Tailnet

---

## Configuration File

### .pedroceli.json

```json
{
  "model": {
    "type": "llamacpp",
    "model_path": "/models/qwen2.5-coder-32b-instruct.gguf",
    "llamacpp_path": "/usr/local/bin/llama-cli",
    "context_size": 32768,
    "n_gpu_layers": -1,
    "temperature": 0.2,
    "threads": 32
  },
  "execution": {
    "run_on_spark": true,
    "spark_ssh": "miriah@dgx-spark-01"
  },
  "git": {
    "always_draft_pr": true,
    "branch_prefix": "pedroceli/",
    "remote": "origin"
  },
  "tools": {
    "allowed_bash_commands": [
      "go", "git", "grep", "sed", "cat", "ls", "find"
    ],
    "forbidden_commands": [
      "rm", "mv", "dd", "sudo"
    ]
  },
  "project": {
    "name": "Pedro Bot",
    "workdir": "/home/miriah/pedro-bot",
    "tech_stack": ["Go", "Docker", "PostgreSQL"]
  },
  "limits": {
    "max_task_duration_minutes": 30,
    "max_inference_runs": 20
  }
}
```

## MCP Tool Definitions

### Tool: build_feature

```json
{
  "name": "build_feature",
  "description": "Build a new feature autonomously in the background",
  "inputSchema": {
    "type": "object",
    "properties": {
      "description": {
        "type": "string",
        "description": "Natural language description of the feature to build"
      },
      "issue": {
        "type": "string",
        "description": "GitHub issue number (optional)"
      },
      "criteria": {
        "type": "array",
        "items": {"type": "string"},
        "description": "Acceptance criteria (optional)"
      }
    },
    "required": ["description"]
  }
}
```

### Tool: debug_issue

```json
{
  "name": "debug_issue",
  "description": "Debug and fix an issue",
  "inputSchema": {
    "type": "object",
    "properties": {
      "symptoms": {
        "type": "string",
        "description": "Description of the problem"
      },
      "logs": {
        "type": "string",
        "description": "Error logs or stack traces (optional)"
      },
      "files": {
        "type": "array",
        "items": {"type": "string"},
        "description": "Specific files to investigate (optional)"
      }
    },
    "required": ["symptoms"]
  }
}
```

### Tool: review_pr

```json
{
  "name": "review_pr",
  "description": "Perform code review on a PR (blind review - unaware it's AI-generated)",
  "inputSchema": {
    "type": "object",
    "properties": {
      "branch": {
        "type": "string",
        "description": "Branch name to review"
      },
      "pr_number": {
        "type": "string",
        "description": "GitHub PR number (optional)"
      }
    },
    "required": ["branch"]
  }
}
```

### Tool: triage_issue

```json
{
  "name": "triage_issue",
  "description": "Diagnose and triage an issue without fixing it",
  "inputSchema": {
    "type": "object",
    "properties": {
      "description": {
        "type": "string",
        "description": "Issue description"
      },
      "error_logs": {
        "type": "string",
        "description": "Error logs (optional)"
      }
    },
    "required": ["description"]
  }
}
```

## Testing Strategy

### Unit Tests
- Tool execution
- MCP protocol handling
- llama.cpp client
- Job management

### Integration Tests
- CLI commands end-to-end
- MCP server with mock llama.cpp
- Git operations
- File operations

### Manual Testing
- Build a real feature
- Debug a real bug
- Review a real PR
- Voice interface on phone

## Success Criteria

**Phase 1 (MCP Server):**
- ✅ MCP server responds to stdio requests
- ✅ All 4 agents work (build, debug, review, triage)
- ✅ llama.cpp backend executes one-shot inference
- ✅ Jobs persist to disk

**Phase 2 (CLI):**
- ✅ All CLI commands work
- ✅ CLI spawns MCP server correctly
- ✅ Can build feature end-to-end
- ✅ Can debug issue end-to-end

**Phase 3 (Ollama):**
- ✅ Ollama backend works
- ✅ Can switch between backends via config
- ✅ Both backends produce similar results

**Phase 4 (Web):**
- ✅ Voice recording works on phone
- ✅ Whisper transcribes accurately
- ✅ Can start jobs from web UI
- ✅ Accessible via Tailnet

## Timeline

- **Week 1-2**: Phase 1 (MCP Server + llama.cpp)
- **Week 3**: Phase 2 (CLI Client)
- **Week 4**: Phase 3 (Ollama Backend)
- **Week 5-6**: Phase 4 (Web Client + STT)

**Total**: ~6 weeks to complete system

## Getting Started

```bash
# 1. Clone/create repo
mkdir pedroceli && cd pedroceli
go mod init pedroceli

# 2. Create structure
mkdir -p cmd pkg/{mcp,agents,llm,tools,jobs,config}

# 3. Start with Phase 1
# Implement MCP server and llama.cpp backend

# 4. Test locally
pedroceli mcp-server

# 5. Build CLI wrapper
go build -o pedroceli cmd/cli.go

# 6. Use it!
pedroceli build --description "Add rate limiting"
```

---

**Let's build the future of autonomous coding! 🚀**
