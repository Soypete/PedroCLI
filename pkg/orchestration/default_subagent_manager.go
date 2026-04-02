package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/tools"
)

type DefaultSubagentManager struct {
	config        *config.Config
	backend       llm.Backend
	jobManager    *jobs.Manager
	toolRegistry  *tools.ToolRegistry
	parentJobDir  string
	mu            sync.RWMutex
	activeHandles map[string]*SubagentHandle
	results       map[string]*TaskResult
	workspaceDir  string
}

func NewDefaultSubagentManager(cfg *config.Config, backend llm.Backend, jobMgr *jobs.Manager, toolRegistry *tools.ToolRegistry, parentJobDir, workspaceDir string) *DefaultSubagentManager {
	return &DefaultSubagentManager{
		config:        cfg,
		backend:       backend,
		jobManager:    jobMgr,
		toolRegistry:  toolRegistry,
		parentJobDir:  parentJobDir,
		workspaceDir:  workspaceDir,
		activeHandles: make(map[string]*SubagentHandle),
		results:       make(map[string]*TaskResult),
	}
}

func (m *DefaultSubagentManager) Spawn(ctx context.Context, task TaskEnvelope) (SubagentHandle, error) {
	subagentID := generateSubagentID()
	handle := NewSubagentHandle(subagentID, task.ID, task.Agent, "")

	subagentDir := m.getSubagentDir(subagentID)
	if err := os.MkdirAll(subagentDir, 0755); err != nil {
		return handle, fmt.Errorf("failed to create subagent directory: %w", err)
	}

	subagentTask := task
	subagentTask.ID = subagentID

	if err := m.writeTaskEnvelope(subagentDir, &subagentTask); err != nil {
		return handle, fmt.Errorf("failed to write task envelope: %w", err)
	}

	m.mu.Lock()
	m.activeHandles[subagentID] = &handle
	m.mu.Unlock()

	go func() {
		result := m.executeSubagent(ctx, subagentID, subagentDir, &subagentTask)

		m.mu.Lock()
		if h, ok := m.activeHandles[subagentID]; ok {
			if result.Error != "" {
				h.MarkFailed()
			} else if result.Finished {
				h.MarkCompleted()
			}
		}
		m.results[subagentID] = result
		m.mu.Unlock()
	}()

	return handle, nil
}

func (m *DefaultSubagentManager) SpawnAll(ctx context.Context, tasks []TaskEnvelope, parallel bool) ([]SubagentHandle, error) {
	if parallel {
		handles := make([]SubagentHandle, len(tasks))
		var wg sync.WaitGroup
		var errs []error
		var errsMu sync.Mutex

		for i, task := range tasks {
			wg.Add(1)
			go func(idx int, t TaskEnvelope) {
				defer wg.Done()
				handle, err := m.Spawn(ctx, t)
				handles[idx] = handle
				if err != nil {
					errsMu.Lock()
					errs = append(errs, err)
					errsMu.Unlock()
				}
			}(i, task)
		}
		wg.Wait()

		if len(errs) > 0 {
			return handles, fmt.Errorf("failed to spawn some subagents: %v", errs)
		}
		return handles, nil
	}

	handles := make([]SubagentHandle, 0, len(tasks))
	for _, task := range tasks {
		handle, err := m.Spawn(ctx, task)
		if err != nil {
			return handles, err
		}
		handles = append(handles, handle)

		result, err := m.Wait(ctx, handle)
		if err != nil {
			return handles, err
		}
		if !result.Success {
			break
		}
	}
	return handles, nil
}

func (m *DefaultSubagentManager) Wait(ctx context.Context, handle SubagentHandle) (*TaskResult, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("subagent %s timed out", handle.ID)
		case <-ticker.C:
			m.mu.RLock()
			result, ok := m.results[handle.ID]
			status := handle.Status
			if handle, exists := m.activeHandles[handle.ID]; exists {
				status = handle.Status
			}
			m.mu.RUnlock()

			if ok && status != SubagentRunning {
				return result, nil
			}
		}
	}
}

func (m *DefaultSubagentManager) WaitAll(ctx context.Context, handles []SubagentHandle) ([]*TaskResult, error) {
	results := make([]*TaskResult, len(handles))

	var wg sync.WaitGroup
	for i, handle := range handles {
		wg.Add(1)
		go func(idx int, h SubagentHandle) {
			defer wg.Done()
			result, err := m.Wait(ctx, h)
			results[idx] = result
			if err != nil {
				results[idx] = &TaskResult{
					ID:    h.TaskID,
					Error: err.Error(),
				}
			}
		}(i, handle)
	}
	wg.Wait()

	return results, nil
}

func (m *DefaultSubagentManager) Cancel(handle SubagentHandle) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if h, ok := m.activeHandles[handle.ID]; ok {
		h.MarkCancelled()
		m.activeHandles[handle.ID] = h
	}
	return nil
}

func (m *DefaultSubagentManager) List(ctx context.Context) ([]SubagentHandle, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	handles := make([]SubagentHandle, 0, len(m.activeHandles))
	for _, handle := range m.activeHandles {
		handles = append(handles, *handle)
	}
	return handles, nil
}

func (m *DefaultSubagentManager) getSubagentDir(subagentID string) string {
	if m.parentJobDir == "" {
		return filepath.Join("/tmp/pedrocli-jobs/subagents", subagentID)
	}
	return filepath.Join(m.parentJobDir, "subagents", subagentID)
}

func (m *DefaultSubagentManager) writeTaskEnvelope(dir string, task *TaskEnvelope) error {
	taskPath := filepath.Join(dir, "task.json")
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}
	return os.WriteFile(taskPath, data, 0644)
}

func (m *DefaultSubagentManager) executeSubagent(ctx context.Context, subagentID, subagentDir string, task *TaskEnvelope) *TaskResult {
	subagentConfig := GetSubagentConfig(task.Agent)

	var contextLimit int
	keepTempFiles := false
	if m.config != nil {
		if m.config.Model.ContextSize > 0 {
			contextLimit = m.config.Model.ContextSize
		}
		keepTempFiles = m.config.Debug.KeepTempFiles
	}
	if contextLimit == 0 {
		contextLimit = 8192
	}

	now := time.Now()
	result := &TaskResult{
		ID:        subagentID,
		StartedAt: now,
	}

	jobID := fmt.Sprintf("subagent-%s", subagentID)
	contextMgr, err := llmcontext.NewManager(jobID, keepTempFiles, contextLimit)
	if err != nil {
		result.MarkComplete(false, fmt.Sprintf("failed to create context manager: %v", err))
		return result
	}

	agentName := fmt.Sprintf("subagent-%s", task.Agent)
	agent := agents.NewBaseAgent(agentName, fmt.Sprintf("Subagent: %s", task.Agent), m.config, m.backend, m.jobManager)

	if m.toolRegistry != nil {
		agent.SetRegistry(m.toolRegistry)
	}

	for _, tool := range m.getToolsForSubagent(subagentConfig.Tools) {
		agent.RegisterTool(tool)
	}

	executor := agents.NewInferenceExecutor(agent, contextMgr)
	executor.SetMaxRounds(subagentConfig.MaxRounds)

	if task.ToolsAllowed != nil && len(task.ToolsAllowed) > 0 {
		executor.SetModeConstraints(task.ToolsAllowed, nil, true)
	}

	err = executor.Execute(ctx, task.Goal)
	if err != nil {
		if strings.Contains(err.Error(), "max inference rounds") {
			result.RoundsUsed = subagentConfig.MaxRounds
			result.Output = "Max rounds reached - task may be incomplete"
			result.Finished = true
			result.Success = false
		} else {
			result.MarkComplete(false, fmt.Sprintf("subagent execution failed: %v", err))
			return result
		}
	} else {
		result.RoundsUsed = subagentConfig.MaxRounds
		result.Output = "Task completed successfully"
		result.Finished = true
		result.Success = true
	}

	now = time.Now()
	result.FinishedAt = &now

	resultPath := filepath.Join(subagentDir, "result.json")
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		result.Error = fmt.Sprintf("failed to marshal result: %v", err)
	}
	if err := os.WriteFile(resultPath, data, 0644); err != nil {
		result.Error = fmt.Sprintf("failed to write result: %v", err)
	}

	return result
}

func (m *DefaultSubagentManager) getToolsForSubagent(allowedTools []string) map[string]tools.Tool {
	if m.toolRegistry == nil {
		return nil
	}

	tools := make(map[string]tools.Tool)
	allTools := m.toolRegistry.List()

	allowedSet := make(map[string]bool)
	for _, t := range allowedTools {
		allowedSet[strings.ToLower(t)] = true
	}

	for _, tool := range allTools {
		if len(allowedTools) == 0 || allowedSet[strings.ToLower(tool.Name())] {
			tools[tool.Name()] = tool
		}
	}

	return tools
}

func generateSubagentID() string {
	return fmt.Sprintf("sa-%s", time.Now().Format("20060102-150405-070000"))
}
