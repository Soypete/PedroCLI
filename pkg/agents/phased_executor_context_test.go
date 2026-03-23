package agents

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/soypete/pedrocli/pkg/agents/testutil"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContextManagerLogging verifies prompts/responses/tools are logged to context manager files
func TestContextManagerLogging(t *testing.T) {
	// Create temp context manager
	jobID := "test-context-" + strconv.FormatInt(time.Now().Unix(), 10)
	contextMgr, err := llmcontext.NewManager(jobID, true, 16384)
	require.NoError(t, err)
	defer os.RemoveAll(contextMgr.GetJobDir())

	// Create mock LLM and tools
	mockLLM := testutil.NewMockLLMBackend()
	mockLLM.AddPhaseCompletionResponse("Task complete")

	mockTool1 := testutil.NewMockTool("file_read", "Read file").WithSuccess("file contents")
	mockTool2 := testutil.NewMockTool("code_edit", "Edit code").WithSuccess("edited")

	// Create agent with minimal config
	cfg := &config.Config{
		Model: config.ModelConfig{
			Type:        "mock",
			ModelName:   "test-model",
			ContextSize: 16384,
		},
		Limits: config.LimitsConfig{
			MaxInferenceRuns: 5,
		},
	}

	agent := NewBaseAgent("test-agent", "test description", cfg, mockLLM, nil)
	agent.RegisterTool(mockTool1)
	agent.RegisterTool(mockTool2)

	// Create phased executor (no job manager, only context manager)
	phase := Phase{
		Name:         "test",
		Description:  "Test phase",
		SystemPrompt: "You are a test assistant",
		MaxRounds:    2,
	}

	executor := &phaseInferenceExecutor{
		agent:        agent,
		contextMgr:   contextMgr,
		phase:        phase,
		maxRounds:    2,
		currentRound: 0,
		jobID:        jobID,
		result: &PhaseResult{
			PhaseName: phase.Name,
			StartedAt: time.Now(),
			Data:      make(map[string]interface{}),
		},
	}

	ctx := context.Background()

	// Simulate prompt/response logging
	err = contextMgr.SavePrompt("test user prompt")
	require.NoError(t, err)

	err = contextMgr.SaveResponse("test assistant response")
	require.NoError(t, err)

	// Simulate tool execution
	calls := []llm.ToolCall{
		{Name: "file_read", Args: map[string]interface{}{"path": "test.go"}},
		{Name: "code_edit", Args: map[string]interface{}{"file": "test.go", "operation": "insert"}},
	}

	// Execute tools (should save tool calls and results)
	results, err := executor.executeTools(ctx, calls)
	require.NoError(t, err)
	assert.Equal(t, 2, len(results))

	// Verify files exist
	files, err := os.ReadDir(contextMgr.GetJobDir())
	require.NoError(t, err)
	assert.True(t, len(files) > 0, "Expected context files to be created")

	// Verify specific file types exist
	var promptFiles, responseFiles, toolCallFiles, toolResultFiles []string
	for _, file := range files {
		name := file.Name()
		if filepath.Ext(name) == ".txt" {
			if strings.Contains(name, "prompt") {
				promptFiles = append(promptFiles, name)
			} else if strings.Contains(name, "response") {
				responseFiles = append(responseFiles, name)
			}
		} else if filepath.Ext(name) == ".json" {
			if strings.Contains(name, "tool-calls") {
				toolCallFiles = append(toolCallFiles, name)
			} else if strings.Contains(name, "tool-results") {
				toolResultFiles = append(toolResultFiles, name)
			}
		}
	}

	assert.Equal(t, 1, len(promptFiles), "Expected 1 prompt file")
	assert.Equal(t, 1, len(responseFiles), "Expected 1 response file")
	assert.Equal(t, 1, len(toolCallFiles), "Expected 1 tool-calls file")
	assert.Equal(t, 1, len(toolResultFiles), "Expected 1 tool-results file")

	// Verify tool calls content
	if len(toolCallFiles) > 0 {
		data, err := os.ReadFile(filepath.Join(contextMgr.GetJobDir(), toolCallFiles[0]))
		require.NoError(t, err)

		var savedCalls []llmcontext.ToolCall
		err = json.Unmarshal(data, &savedCalls)
		require.NoError(t, err)
		assert.Equal(t, 2, len(savedCalls))
		assert.Equal(t, "file_read", savedCalls[0].Name)
		assert.Equal(t, "code_edit", savedCalls[1].Name)
	}

	// Verify tool results content
	if len(toolResultFiles) > 0 {
		data, err := os.ReadFile(filepath.Join(contextMgr.GetJobDir(), toolResultFiles[0]))
		require.NoError(t, err)

		var savedResults []llmcontext.ToolResult
		err = json.Unmarshal(data, &savedResults)
		require.NoError(t, err)
		assert.Equal(t, 2, len(savedResults))
		assert.True(t, savedResults[0].Success)
		assert.True(t, savedResults[1].Success)
		assert.Equal(t, "file_read", savedResults[0].Name)
		assert.Equal(t, "code_edit", savedResults[1].Name)
	}

	// Verify mock tools were actually called
	assert.Equal(t, 1, mockTool1.GetCallCount(), "file_read should be called once")
	assert.Equal(t, 1, mockTool2.GetCallCount(), "code_edit should be called once")
}

// TestContextManagerLoggingWithNilContextMgr verifies graceful handling when context manager is nil
func TestContextManagerLoggingWithNilContextMgr(t *testing.T) {
	mockLLM := testutil.NewMockLLMBackend()
	mockLLM.AddPhaseCompletionResponse("Done")

	mockTool := testutil.NewMockTool("test_tool", "Test tool").WithSuccess("ok")

	cfg := &config.Config{
		Model: config.ModelConfig{
			Type:        "mock",
			ModelName:   "test-model",
			ContextSize: 16384,
		},
		Limits: config.LimitsConfig{
			MaxInferenceRuns: 5,
		},
	}

	agent := NewBaseAgent("test-agent", "test description", cfg, mockLLM, nil)
	agent.RegisterTool(mockTool)

	executor := &phaseInferenceExecutor{
		agent:        agent,
		contextMgr:   nil, // nil context manager
		phase:        Phase{Name: "test", SystemPrompt: "test"},
		maxRounds:    2,
		currentRound: 0,
		jobID:        "test-job",
		result:       &PhaseResult{},
	}

	ctx := context.Background()

	calls := []llm.ToolCall{
		{Name: "test_tool", Args: map[string]interface{}{"arg": "value"}},
	}

	// Should not panic with nil context manager
	results, err := executor.executeTools(ctx, calls)
	require.NoError(t, err)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, 1, mockTool.GetCallCount())
}

// TestContextManagerLoggingFileSequence verifies files are numbered sequentially
func TestContextManagerLoggingFileSequence(t *testing.T) {
	jobID := "test-seq-" + strconv.FormatInt(time.Now().Unix(), 10)
	contextMgr, err := llmcontext.NewManager(jobID, true, 16384)
	require.NoError(t, err)
	defer os.RemoveAll(contextMgr.GetJobDir())

	// Save sequence: prompt -> response -> tool-calls -> tool-results
	err = contextMgr.SavePrompt("prompt 1")
	require.NoError(t, err)

	err = contextMgr.SaveResponse("response 1")
	require.NoError(t, err)

	err = contextMgr.SaveToolCalls([]llmcontext.ToolCall{
		{Name: "tool1", Args: map[string]interface{}{"arg": "val"}},
	})
	require.NoError(t, err)

	err = contextMgr.SaveToolResults([]llmcontext.ToolResult{
		{Name: "tool1", Success: true, Output: "done"},
	})
	require.NoError(t, err)

	// Check file sequence
	files, err := os.ReadDir(contextMgr.GetJobDir())
	require.NoError(t, err)

	fileNames := make([]string, len(files))
	for i, f := range files {
		fileNames[i] = f.Name()
	}

	// Should have files numbered 001, 002, 003, 004
	assert.Contains(t, fileNames, "001-prompt.txt")
	assert.Contains(t, fileNames, "002-response.txt")
	assert.Contains(t, fileNames, "003-tool-calls.json")
	assert.Contains(t, fileNames, "004-tool-results.json")
}
