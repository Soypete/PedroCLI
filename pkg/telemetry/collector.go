package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type TelemetryCollector interface {
	Record(event TelemetryEvent)
	Summary(jobID string) (*TelemetrySummary, error)
	Events(jobID string, eventType EventType) ([]TelemetryEvent, error)
	Close() error
}

type InMemoryCollector struct {
	mu     sync.RWMutex
	events map[string][]TelemetryEvent
	jobIDs []string
}

func NewInMemoryCollector() *InMemoryCollector {
	return &InMemoryCollector{
		events: make(map[string][]TelemetryEvent),
	}
}

func (c *InMemoryCollector) Record(event TelemetryEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.events[event.JobID] = append(c.events[event.JobID], event)

	found := false
	for _, id := range c.jobIDs {
		if id == event.JobID {
			found = true
			break
		}
	}
	if !found {
		c.jobIDs = append(c.jobIDs, event.JobID)
	}
}

func (c *InMemoryCollector) Summary(jobID string) (*TelemetrySummary, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	events, ok := c.events[jobID]
	if !ok {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	s := &TelemetrySummary{
		JobID: jobID,
	}

	var totalLLMLatency time.Duration

	for _, e := range events {
		switch e.EventType {
		case EventInference:
			if data, ok := e.Data["prompt_tokens"]; ok {
				if pt, ok := data.(map[string]any); ok {
					if v, ok := pt["prompt_tokens"]; ok {
						if n, ok := toInt(v); ok {
							s.PromptTokens += n
							s.TotalTokens += n
						}
					}
					if v, ok := pt["completion_tokens"]; ok {
						if n, ok := toInt(v); ok {
							s.CompletionTokens += n
							s.TotalTokens += n
						}
					}
				}
			}
			if latency, ok := e.Data["llm_latency"]; ok {
				if d, ok := toDuration(latency); ok {
					totalLLMLatency += d
				}
			}
			s.Rounds++

		case EventToolCall:
			s.ToolCalls++
			if success, ok := e.Data["success"].(bool); ok && !success {
				s.ToolFailures++
			}

		case EventPhaseComplete:
			s.Phases++

		case EventJobComplete:
			if cost, ok := e.Data["estimated_cost"].(float64); ok {
				s.EstimatedCost = cost
			}
		}
	}

	s.LLMLatency = totalLLMLatency
	s.TotalDuration = calculateTotalDuration(events)

	return s, nil
}

func (c *InMemoryCollector) Events(jobID string, eventType EventType) ([]TelemetryEvent, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	events, ok := c.events[jobID]
	if !ok {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	if eventType == "" {
		return events, nil
	}

	var filtered []TelemetryEvent
	for _, e := range events {
		if e.EventType == eventType {
			filtered = append(filtered, e)
		}
	}

	return filtered, nil
}

func (c *InMemoryCollector) Close() error {
	return nil
}

type FileTelemetryCollector struct {
	jobDir   string
	file     *os.File
	mu       sync.Mutex
	events   []TelemetryEvent
	inMemory *InMemoryCollector
}

func NewFileTelemetryCollector(jobDir string) (*FileTelemetryCollector, error) {
	telemetryPath := filepath.Join(jobDir, "telemetry.jsonl")

	f, err := os.OpenFile(telemetryPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open telemetry file: %w", err)
	}

	c := &FileTelemetryCollector{
		jobDir:   jobDir,
		file:     f,
		inMemory: NewInMemoryCollector(),
	}

	return c, nil
}

func (c *FileTelemetryCollector) Record(event TelemetryEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	event.Timestamp = time.Now().UTC()

	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	c.file.Write(append(data, '\n'))
	c.inMemory.Record(event)
	c.events = append(c.events, event)
}

func (c *FileTelemetryCollector) Summary(jobID string) (*TelemetrySummary, error) {
	return c.inMemory.Summary(jobID)
}

func (c *FileTelemetryCollector) Events(jobID string, eventType EventType) ([]TelemetryEvent, error) {
	return c.inMemory.Events(jobID, eventType)
}

func (c *FileTelemetryCollector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.file != nil {
		return c.file.Close()
	}
	return nil
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	}
	return 0, false
}

func toDuration(v any) (time.Duration, bool) {
	switch d := v.(type) {
	case time.Duration:
		return d, true
	case float64:
		return time.Duration(d), true
	case int:
		return time.Duration(d), true
	}
	return 0, false
}

func calculateTotalDuration(events []TelemetryEvent) time.Duration {
	if len(events) < 2 {
		return 0
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	first := events[0].Timestamp
	last := events[len(events)-1].Timestamp
	return last.Sub(first)
}
