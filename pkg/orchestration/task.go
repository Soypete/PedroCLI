package orchestration

import (
	"context"
	"math/rand"
	"time"
)

type TaskEnvelope struct {
	ID           string            `json:"id"`
	Agent        string            `json:"agent"`
	Goal         string            `json:"goal"`
	Mode         Mode              `json:"mode"`
	Context      TaskContext       `json:"context"`
	ToolsAllowed []string          `json:"tools_allowed,omitempty"`
	MaxSteps     int               `json:"max_steps,omitempty"`
	ReturnSchema map[string]string `json:"return_schema,omitempty"`
}

type TaskContext struct {
	Workspace  string                 `json:"workspace"`
	WorkingDir string                 `json:"working_dir"`
	Files      []string               `json:"files,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type TaskResult struct {
	ID         string                 `json:"id"`
	Success    bool                   `json:"success"`
	Output     string                 `json:"output"`
	Parsed     map[string]interface{} `json:"parsed,omitempty"`
	Error      string                 `json:"error,omitempty"`
	RoundsUsed int                    `json:"rounds_used"`
	Finished   bool                   `json:"finished"`
	StartedAt  time.Time              `json:"started_at"`
	FinishedAt *time.Time             `json:"finished_at,omitempty"`
}

type TaskEngine interface {
	ExecuteTask(ctx context.Context, envelope *TaskEnvelope) (*TaskResult, error)
	ValidateEnvelope(envelope *TaskEnvelope) error
}

func NewTaskEnvelope(agent, goal string, mode Mode, workspace string) *TaskEnvelope {
	return &TaskEnvelope{
		ID:    GenerateTaskID(),
		Agent: agent,
		Goal:  goal,
		Mode:  mode,
		Context: TaskContext{
			Workspace:  workspace,
			WorkingDir: workspace,
			Metadata:   make(map[string]interface{}),
		},
		MaxSteps: 20,
	}
}

func (e *TaskEnvelope) SetToolsAllowed(tools []string) {
	e.ToolsAllowed = tools
}

func (e *TaskEnvelope) SetReturnSchema(schema map[string]string) {
	e.ReturnSchema = schema
}

func (e *TaskEnvelope) AddFile(file string) {
	e.Context.Files = append(e.Context.Files, file)
}

func (e *TaskEnvelope) SetMetadata(key string, value interface{}) {
	if e.Context.Metadata == nil {
		e.Context.Metadata = make(map[string]interface{})
	}
	e.Context.Metadata[key] = value
}

func (e *TaskEnvelope) Validate() error {
	if e.Agent == "" {
		return ErrInvalidTaskEnvelope("agent is required")
	}
	if e.Goal == "" {
		return ErrInvalidTaskEnvelope("goal is required")
	}
	if e.Context.Workspace == "" {
		return ErrInvalidTaskEnvelope("workspace is required")
	}
	return nil
}

func (r *TaskResult) SetParsed(key string, value interface{}) {
	if r.Parsed == nil {
		r.Parsed = make(map[string]interface{})
	}
	r.Parsed[key] = value
}

func (r *TaskResult) MarkComplete(success bool, output string) {
	r.Finished = true
	r.Success = success
	r.Output = output
	now := time.Now()
	r.FinishedAt = &now
}

type TaskEnvelopeValidationError struct {
	Message string
}

func ErrInvalidTaskEnvelope(msg string) *TaskEnvelopeValidationError {
	return &TaskEnvelopeValidationError{Message: msg}
}

func (e *TaskEnvelopeValidationError) Error() string {
	return "invalid task envelope: " + e.Message
}

func GenerateTaskID() string {
	return time.Now().Format("20060102-150405") + "-" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
