// Package testutil provides mock implementations for workflow integration testing.
package testutil

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/storage"
	"github.com/soypete/pedrocli/pkg/tools"
)

// MockLLMBackend is a programmable LLM backend for testing.
// It can be configured to return specific responses, errors,
// or simulate different scenarios like tool calls.
type MockLLMBackend struct {
	mu sync.Mutex

	// Responses is a queue of responses to return for each Infer call.
	// Each call pops the first response from the queue.
	Responses []*llm.InferenceResponse

	// Errors is a queue of errors to return (parallel to Responses).
	// If an error is non-nil, it's returned instead of the response.
	Errors []error

	// InferCalls records all calls made to Infer for verification.
	InferCalls []InferCall

	// ContextWindow is the mock context window size.
	ContextWindow int

	// TokenizeFunc allows custom tokenization logic.
	TokenizeFunc func(ctx context.Context, text string) ([]int, error)

	// CurrentCall tracks the current call index.
	CurrentCall int
}

// InferCall records a single call to the Infer method.
type InferCall struct {
	Request   *llm.InferenceRequest
	Timestamp time.Time
}

// NewMockLLMBackend creates a new mock backend with default settings.
func NewMockLLMBackend() *MockLLMBackend {
	return &MockLLMBackend{
		Responses:     make([]*llm.InferenceResponse, 0),
		Errors:        make([]error, 0),
		InferCalls:    make([]InferCall, 0),
		ContextWindow: 32768, // Default 32K context
	}
}

// AddResponse adds a response to the queue.
func (m *MockLLMBackend) AddResponse(resp *llm.InferenceResponse) *MockLLMBackend {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Responses = append(m.Responses, resp)
	m.Errors = append(m.Errors, nil)
	return m
}

// AddError adds an error to the queue.
func (m *MockLLMBackend) AddError(err error) *MockLLMBackend {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Responses = append(m.Responses, nil)
	m.Errors = append(m.Errors, err)
	return m
}

// AddToolCallResponse creates a response with tool calls.
func (m *MockLLMBackend) AddToolCallResponse(toolName string, args map[string]interface{}) *MockLLMBackend {
	return m.AddResponse(&llm.InferenceResponse{
		Text: fmt.Sprintf("I need to call the %s tool.", toolName),
		ToolCalls: []llm.ToolCall{
			{Name: toolName, Args: args},
		},
		TokensUsed: 100,
	})
}

// AddCompletionResponse creates a response that signals task completion.
func (m *MockLLMBackend) AddCompletionResponse(message string) *MockLLMBackend {
	return m.AddResponse(&llm.InferenceResponse{
		Text:       message + "\n\nTASK_COMPLETE",
		ToolCalls:  nil,
		TokensUsed: 50,
	})
}

// AddPhaseCompletionResponse creates a response that signals phase completion.
func (m *MockLLMBackend) AddPhaseCompletionResponse(message string) *MockLLMBackend {
	return m.AddResponse(&llm.InferenceResponse{
		Text:       message + "\n\nPHASE_COMPLETE",
		ToolCalls:  nil,
		TokensUsed: 50,
	})
}

// Infer implements llm.Backend.
func (m *MockLLMBackend) Infer(ctx context.Context, req *llm.InferenceRequest) (*llm.InferenceResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the call
	m.InferCalls = append(m.InferCalls, InferCall{
		Request:   req,
		Timestamp: time.Now(),
	})

	// Check if we have responses queued
	if m.CurrentCall >= len(m.Responses) {
		return nil, fmt.Errorf("mock: no more responses queued (call %d)", m.CurrentCall)
	}

	resp := m.Responses[m.CurrentCall]
	err := m.Errors[m.CurrentCall]
	m.CurrentCall++

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// GetContextWindow implements llm.Backend.
func (m *MockLLMBackend) GetContextWindow() int {
	return m.ContextWindow
}

// GetUsableContext implements llm.Backend.
func (m *MockLLMBackend) GetUsableContext() int {
	return int(float64(m.ContextWindow) * 0.75)
}

// Tokenize implements llm.Backend.
func (m *MockLLMBackend) Tokenize(ctx context.Context, text string) ([]int, error) {
	if m.TokenizeFunc != nil {
		return m.TokenizeFunc(ctx, text)
	}
	// Default: return simple token IDs based on length
	tokens := make([]int, len(text)/4)
	for i := range tokens {
		tokens[i] = i + 1
	}
	return tokens, nil
}

// Reset resets the mock for reuse.
func (m *MockLLMBackend) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Responses = make([]*llm.InferenceResponse, 0)
	m.Errors = make([]error, 0)
	m.InferCalls = make([]InferCall, 0)
	m.CurrentCall = 0
}

// GetCallCount returns the number of Infer calls made.
func (m *MockLLMBackend) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.InferCalls)
}

// MockJobManager is a mock implementation of jobs.JobManager for testing.
type MockJobManager struct {
	mu sync.Mutex

	// Jobs stores all created jobs.
	Jobs map[string]*jobs.Job

	// Conversations stores conversation entries per job.
	Conversations map[string][]storage.ConversationEntry

	// Phases stores current phase per job.
	Phases map[string]string

	// PhaseResults stores phase results per job.
	PhaseResults map[string]map[string]interface{}

	// CreateError if set, causes Create to return this error.
	CreateError error

	// UpdateError if set, causes Update to return this error.
	UpdateError error
}

// NewMockJobManager creates a new mock job manager.
func NewMockJobManager() *MockJobManager {
	return &MockJobManager{
		Jobs:          make(map[string]*jobs.Job),
		Conversations: make(map[string][]storage.ConversationEntry),
		Phases:        make(map[string]string),
		PhaseResults:  make(map[string]map[string]interface{}),
	}
}

// Create implements jobs.JobManager.
func (m *MockJobManager) Create(ctx context.Context, jobType, description string, input map[string]interface{}) (*jobs.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.CreateError != nil {
		return nil, m.CreateError
	}

	job := &jobs.Job{
		ID:          fmt.Sprintf("test-job-%d", len(m.Jobs)+1),
		Type:        jobType,
		Description: description,
		Input:       input,
		Status:      jobs.StatusPending,
		CreatedAt:   time.Now(),
	}
	m.Jobs[job.ID] = job
	m.Conversations[job.ID] = make([]storage.ConversationEntry, 0)
	return job, nil
}

// Get implements jobs.JobManager.
func (m *MockJobManager) Get(ctx context.Context, id string) (*jobs.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.Jobs[id]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	return job, nil
}

// List implements jobs.JobManager.
func (m *MockJobManager) List(ctx context.Context) ([]*jobs.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]*jobs.Job, 0, len(m.Jobs))
	for _, job := range m.Jobs {
		result = append(result, job)
	}
	return result, nil
}

// Update implements jobs.JobManager.
func (m *MockJobManager) Update(ctx context.Context, id string, status jobs.Status, output map[string]interface{}, err error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.UpdateError != nil {
		return m.UpdateError
	}

	job, ok := m.Jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}

	job.Status = status
	job.Output = output
	if err != nil {
		job.Error = err.Error()
	}
	if status == jobs.StatusRunning {
		now := time.Now()
		job.StartedAt = &now
	}
	if status == jobs.StatusCompleted || status == jobs.StatusFailed || status == jobs.StatusCancelled {
		now := time.Now()
		job.CompletedAt = &now
	}

	return nil
}

// Cancel implements jobs.JobManager.
func (m *MockJobManager) Cancel(ctx context.Context, id string) error {
	return m.Update(ctx, id, jobs.StatusCancelled, nil, nil)
}

// CleanupOldJobs implements jobs.JobManager.
func (m *MockJobManager) CleanupOldJobs(ctx context.Context, olderThan time.Duration) error {
	return nil // No-op for tests
}

// SetWorkDir implements jobs.JobManager.
func (m *MockJobManager) SetWorkDir(ctx context.Context, id string, workDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.Jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}
	job.WorkDir = workDir
	return nil
}

// SetContextDir implements jobs.JobManager.
func (m *MockJobManager) SetContextDir(ctx context.Context, id string, contextDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.Jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}
	job.ContextDir = contextDir
	return nil
}

// AppendConversation implements jobs.JobManager.
func (m *MockJobManager) AppendConversation(ctx context.Context, id string, entry storage.ConversationEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.Conversations[id]; !ok {
		m.Conversations[id] = make([]storage.ConversationEntry, 0)
	}
	m.Conversations[id] = append(m.Conversations[id], entry)
	return nil
}

// GetConversation implements jobs.JobManager.
func (m *MockJobManager) GetConversation(ctx context.Context, id string) ([]storage.ConversationEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, ok := m.Conversations[id]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	return conv, nil
}

// SetWorkflowType implements jobs.JobManager.
func (m *MockJobManager) SetWorkflowType(ctx context.Context, id string, workflowType string) error {
	return nil // No-op for tests
}

// SetCurrentPhase implements jobs.JobManager.
func (m *MockJobManager) SetCurrentPhase(ctx context.Context, id string, phase string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Phases[id] = phase
	return nil
}

// SetPhaseResults implements jobs.JobManager.
func (m *MockJobManager) SetPhaseResults(ctx context.Context, id string, results map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PhaseResults[id] = results
	return nil
}

// SetPlan implements jobs.JobManager.
func (m *MockJobManager) SetPlan(ctx context.Context, id string, plan map[string]interface{}) error {
	return nil // No-op for tests
}

// GetPhase returns the current phase for a job.
func (m *MockJobManager) GetPhase(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Phases[id]
}

// GetConversationEntries returns all conversation entries for a job.
func (m *MockJobManager) GetConversationEntries(id string) []storage.ConversationEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Conversations[id]
}

// MockTool is a configurable mock tool for testing.
type MockTool struct {
	name        string
	description string

	// ExecuteFunc is called when Execute is invoked.
	ExecuteFunc func(ctx context.Context, args map[string]interface{}) (*tools.Result, error)

	// Calls records all calls to Execute.
	Calls []MockToolCall

	mu sync.Mutex
}

// MockToolCall records a single call to a mock tool.
type MockToolCall struct {
	Args      map[string]interface{}
	Timestamp time.Time
}

// NewMockTool creates a new mock tool.
func NewMockTool(name, description string) *MockTool {
	return &MockTool{
		name:        name,
		description: description,
		Calls:       make([]MockToolCall, 0),
	}
}

// WithExecute sets the execute function.
func (t *MockTool) WithExecute(fn func(ctx context.Context, args map[string]interface{}) (*tools.Result, error)) *MockTool {
	t.ExecuteFunc = fn
	return t
}

// WithSuccess configures the tool to always return success.
func (t *MockTool) WithSuccess(output string) *MockTool {
	t.ExecuteFunc = func(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
		return &tools.Result{
			Success: true,
			Output:  output,
		}, nil
	}
	return t
}

// WithFailure configures the tool to always return failure.
func (t *MockTool) WithFailure(errMsg string) *MockTool {
	t.ExecuteFunc = func(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
		return &tools.Result{
			Success: false,
			Error:   errMsg,
		}, nil
	}
	return t
}

// WithError configures the tool to always return an error.
func (t *MockTool) WithError(err error) *MockTool {
	t.ExecuteFunc = func(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
		return nil, err
	}
	return t
}

// Name implements tools.Tool.
func (t *MockTool) Name() string {
	return t.name
}

// Description implements tools.Tool.
func (t *MockTool) Description() string {
	return t.description
}

// Execute implements tools.Tool.
func (t *MockTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	t.mu.Lock()
	t.Calls = append(t.Calls, MockToolCall{
		Args:      args,
		Timestamp: time.Now(),
	})
	t.mu.Unlock()

	if t.ExecuteFunc != nil {
		return t.ExecuteFunc(ctx, args)
	}

	// Default: return success
	return &tools.Result{
		Success: true,
		Output:  fmt.Sprintf("Mock %s executed successfully", t.name),
	}, nil
}

// GetCallCount returns the number of times the tool was called.
func (t *MockTool) GetCallCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.Calls)
}

// GetLastCall returns the last call made to the tool.
func (t *MockTool) GetLastCall() *MockToolCall {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.Calls) == 0 {
		return nil
	}
	return &t.Calls[len(t.Calls)-1]
}

// Reset clears all recorded calls.
func (t *MockTool) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Calls = make([]MockToolCall, 0)
}

// Verify interface compliance
var _ llm.Backend = (*MockLLMBackend)(nil)
var _ jobs.JobManager = (*MockJobManager)(nil)
var _ tools.Tool = (*MockTool)(nil)
