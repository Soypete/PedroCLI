package telemetry

import (
	"encoding/json"
	"fmt"
	"time"
)

type EventType string

const (
	EventInference     EventType = "inference"
	EventToolCall      EventType = "tool_call"
	EventPhaseComplete EventType = "phase_complete"
	EventJobComplete   EventType = "job_complete"
	EventError         EventType = "error"
)

type TelemetryEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	JobID     string                 `json:"job_id"`
	AgentID   string                 `json:"agent_id"`
	Phase     string                 `json:"phase,omitempty"`
	Round     int                    `json:"round,omitempty"`
	EventType EventType              `json:"event_type"`
	Data      map[string]interface{} `json:"data"`
}

func (e TelemetryEvent) String() string {
	return fmt.Sprintf("[%s] %s %s round=%d", e.Timestamp.Format(time.RFC3339), e.JobID, e.EventType, e.Round)
}

func (e TelemetryEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Timestamp time.Time `json:"timestamp"`
		JobID     string    `json:"job_id"`
		AgentID   string    `json:"agent_id"`
		Phase     string    `json:"phase,omitempty"`
		Round     int       `json:"round,omitempty"`
		EventType string    `json:"event_type"`
		Data      any       `json:"data"`
	}{
		Timestamp: e.Timestamp,
		JobID:     e.JobID,
		AgentID:   e.AgentID,
		Phase:     e.Phase,
		Round:     e.Round,
		EventType: string(e.EventType),
		Data:      e.Data,
	})
}

type InferenceData struct {
	PromptTokens     int           `json:"prompt_tokens"`
	CompletionTokens int           `json:"completion_tokens"`
	TotalTokens      int           `json:"total_tokens"`
	LLMLatency       time.Duration `json:"llm_latency"`
	Model            string        `json:"model"`
	Success          bool          `json:"success"`
}

type ToolCallData struct {
	ToolName string        `json:"tool_name"`
	Args     any           `json:"args,omitempty"`
	Success  bool          `json:"success"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
}

type PhaseData struct {
	PhaseName   string        `json:"phase_name"`
	Duration    time.Duration `json:"duration"`
	Rounds      int           `json:"rounds"`
	TotalTokens int           `json:"total_tokens"`
	Success     bool          `json:"success"`
	Error       string        `json:"error,omitempty"`
}

type JobData struct {
	Status         string        `json:"status"`
	Duration       time.Duration `json:"duration"`
	TotalRounds    int           `json:"total_rounds"`
	TotalPhases    int           `json:"total_phases"`
	TotalTokens    int           `json:"total_tokens"`
	TotalToolCalls int           `json:"total_tool_calls"`
	EstimatedCost  float64       `json:"estimated_cost_usd"`
}

type TelemetrySummary struct {
	JobID            string        `json:"job_id"`
	TotalTokens      int           `json:"total_tokens"`
	PromptTokens     int           `json:"prompt_tokens"`
	CompletionTokens int           `json:"completion_tokens"`
	TotalDuration    time.Duration `json:"total_duration"`
	LLMLatency       time.Duration `json:"llm_latency"`
	ToolCalls        int           `json:"tool_calls"`
	ToolFailures     int           `json:"tool_failures"`
	Rounds           int           `json:"rounds"`
	Phases           int           `json:"phases"`
	EstimatedCost    float64       `json:"estimated_cost_usd"`
}

func (s TelemetrySummary) String() string {
	return fmt.Sprintf("Job %s: %d tokens (prompt=%d, completion=%d), %d tool calls, %d rounds, %d phases, ~$%.4f",
		s.JobID, s.TotalTokens, s.PromptTokens, s.CompletionTokens,
		s.ToolCalls, s.Rounds, s.Phases, s.EstimatedCost)
}

var tokenPricing = map[string]float64{
	"qwen2.5-coder:32b": 0.0008,
	"qwen2.5-coder:14b": 0.0004,
	"qwen2.5-coder:7b":  0.0002,
	"llama3.1:70b":      0.0009,
	"llama3.1:8b":       0.0002,
	"mistral:7b":        0.00024,
	"default":           0.001,
}

func EstimateCost(model string, promptTokens, completionTokens int) float64 {
	price, ok := tokenPricing[model]
	if !ok {
		price = tokenPricing["default"]
	}
	return float64(promptTokens+completionTokens) * price / 1000.0
}
