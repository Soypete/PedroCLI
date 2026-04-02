package orchestration

import (
	"context"
	"time"
)

type SubagentStatus string

const (
	SubagentRunning   SubagentStatus = "running"
	SubagentCompleted SubagentStatus = "completed"
	SubagentFailed    SubagentStatus = "failed"
	SubagentCancelled SubagentStatus = "cancelled"
)

type SubagentHandle struct {
	ID       string
	TaskID   string
	Agent    string
	ParentID string
	Status   SubagentStatus
}

type SubagentConfig struct {
	AgentType  string
	Tools      []string
	MaxRounds  int
	WorkingDir string
}

var DefaultSubagentConfigs = map[string]SubagentConfig{
	"explorer": {
		AgentType: "explorer",
		Tools:     []string{"search", "navigate", "file", "lsp"},
		MaxRounds: 10,
	},
	"implementer": {
		AgentType: "implementer",
		Tools:     []string{"file", "code_edit", "bash"},
		MaxRounds: 20,
	},
	"tester": {
		AgentType: "tester",
		Tools:     []string{"test", "bash", "file", "code_edit"},
		MaxRounds: 15,
	},
	"reviewer": {
		AgentType: "reviewer",
		Tools:     []string{"search", "file", "git", "test"},
		MaxRounds: 10,
	},
	"doc-writer": {
		AgentType: "doc-writer",
		Tools:     []string{"file", "search", "navigate"},
		MaxRounds: 10,
	},
}

type SubagentManager interface {
	Spawn(ctx context.Context, task TaskEnvelope) (SubagentHandle, error)
	SpawnAll(ctx context.Context, tasks []TaskEnvelope, parallel bool) ([]SubagentHandle, error)
	Wait(ctx context.Context, handle SubagentHandle) (*TaskResult, error)
	WaitAll(ctx context.Context, handles []SubagentHandle) ([]*TaskResult, error)
	Cancel(handle SubagentHandle) error
	List(ctx context.Context) ([]SubagentHandle, error)
}

type SubagentSpawner interface {
	SpawnSubagent(ctx context.Context, envelope *TaskEnvelope) (*TaskResult, error)
}

type SubagentExecutor struct {
	TaskEngine
	SubagentManager
}

func NewSubagentHandle(id, taskID, agent, parentID string) SubagentHandle {
	return SubagentHandle{
		ID:       id,
		TaskID:   taskID,
		Agent:    agent,
		ParentID: parentID,
		Status:   SubagentRunning,
	}
}

func (h SubagentHandle) IsRunning() bool {
	return h.Status == SubagentRunning
}

func (h *SubagentHandle) MarkCompleted() {
	h.Status = SubagentCompleted
}

func (h *SubagentHandle) MarkFailed() {
	h.Status = SubagentFailed
}

func (h *SubagentHandle) MarkCancelled() {
	h.Status = SubagentCancelled
}

type SubagentResult struct {
	Handle     SubagentHandle
	TaskResult *TaskResult
	Duration   time.Duration
	Error      error
}

func GetSubagentConfig(agentType string) SubagentConfig {
	if cfg, ok := DefaultSubagentConfigs[agentType]; ok {
		return cfg
	}
	return SubagentConfig{
		AgentType: agentType,
		Tools:     []string{},
		MaxRounds: 20,
	}
}

func GetSubagentTypes() []string {
	return []string{
		"explorer",
		"implementer",
		"tester",
		"reviewer",
		"doc-writer",
	}
}
