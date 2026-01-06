package llmcontext

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/storage"
)

// Manager handles file-based context storage for jobs
type Manager struct {
	jobID               string
	jobDir              string
	counter             int
	debugMode           bool
	lastPromptTokens    int                         // Tokens in last prompt sent (system + user)
	contextLimit        int                         // Total context window size
	compactionThreshold float64                     // Trigger compaction at this % (default 0.75)
	modelName           string                      // Model name for tokenization tracking
	statsStore          storage.CompactionStatsStore // Optional stats recording
}

// ToolCall represents a tool call in the context
type ToolCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// ToolResult represents a tool execution result
type ToolResult struct {
	Name          string   `json:"name"`
	Success       bool     `json:"success"`
	Output        string   `json:"output"`
	Error         string   `json:"error,omitempty"`
	ModifiedFiles []string `json:"modified_files,omitempty"`
}

// NewManager creates a new context manager for a job
func NewManager(jobID string, debugMode bool, contextLimit int) (*Manager, error) {
	timestamp := time.Now().Format("20060102-150405")
	jobDir := filepath.Join("/tmp/pedroceli-jobs", fmt.Sprintf("%s-%s", jobID, timestamp))

	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create job directory: %w", err)
	}

	return &Manager{
		jobID:               jobID,
		jobDir:              jobDir,
		counter:             0,
		debugMode:           debugMode,
		lastPromptTokens:    0,
		contextLimit:        contextLimit,
		compactionThreshold: 0.75, // 75% threshold
	}, nil
}

// GetJobDir returns the job directory path
func (m *Manager) GetJobDir() string {
	return m.jobDir
}

// GetJobID returns the job ID
func (m *Manager) GetJobID() string {
	return m.jobID
}

// SetModelName sets the model name for tokenization tracking
func (m *Manager) SetModelName(modelName string) {
	m.modelName = modelName
}

// SetStatsStore sets the compaction statistics store (optional)
func (m *Manager) SetStatsStore(store storage.CompactionStatsStore) {
	m.statsStore = store
}

// SavePrompt saves a prompt to the context
func (m *Manager) SavePrompt(prompt string) error {
	m.counter++
	filename := fmt.Sprintf("%03d-prompt.txt", m.counter)
	return os.WriteFile(filepath.Join(m.jobDir, filename), []byte(prompt), 0644)
}

// SaveResponse saves a response to the context
func (m *Manager) SaveResponse(response string) error {
	m.counter++
	filename := fmt.Sprintf("%03d-response.txt", m.counter)
	return os.WriteFile(filepath.Join(m.jobDir, filename), []byte(response), 0644)
}

// SaveToolCalls saves tool calls to the context
func (m *Manager) SaveToolCalls(calls []ToolCall) error {
	m.counter++
	filename := fmt.Sprintf("%03d-tool-calls.json", m.counter)
	data, err := json.MarshalIndent(calls, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.jobDir, filename), data, 0644)
}

// SaveToolResults saves tool results to the context
func (m *Manager) SaveToolResults(results []ToolResult) error {
	m.counter++
	filename := fmt.Sprintf("%03d-tool-results.json", m.counter)
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.jobDir, filename), data, 0644)
}

// GetHistory returns all history as a concatenated string
func (m *Manager) GetHistory() (string, error) {
	files, err := filepath.Glob(filepath.Join(m.jobDir, "*.txt"))
	if err != nil {
		return "", err
	}
	sort.Strings(files)

	var history strings.Builder
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		history.WriteString(fmt.Sprintf("\n=== %s ===\n", filepath.Base(file)))
		history.Write(content)
		history.WriteString("\n")
	}

	return history.String(), nil
}

// GetHistoryWithinBudget returns history that fits within the token budget
func (m *Manager) GetHistoryWithinBudget(budget int) (string, error) {
	files, err := filepath.Glob(filepath.Join(m.jobDir, "*-prompt.txt"))
	if err != nil {
		return "", err
	}
	sort.Strings(files)

	// Determine how many recent files to keep based on whether we need compaction
	keepRecent := 3 // Default

	// If we exceeded threshold in last round, be more aggressive
	if m.ShouldCompact() {
		keepRecent = 2 // Keep only last 2 rounds when compacting
	}

	var recentFiles []string

	if len(files) > keepRecent {
		recentFiles = files[len(files)-keepRecent:]
	} else {
		recentFiles = files
	}

	// Estimate tokens for recent files
	var selected []string
	totalTokens := 0

	for _, file := range recentFiles {
		// Include both prompt and response
		promptContent, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		tokens := llm.EstimateTokens(string(promptContent))

		// Also include corresponding response
		responseFile := strings.Replace(file, "-prompt.txt", "-response.txt", 1)
		if responseContent, err := os.ReadFile(responseFile); err == nil {
			tokens += llm.EstimateTokens(string(responseContent))
		}

		if totalTokens+tokens > budget {
			break
		}

		selected = append(selected, file)
		totalTokens += tokens
	}

	// If we have room and older files exist, summarize them
	if totalTokens < budget && len(files) > len(recentFiles) {
		oldFiles := files[:len(files)-len(recentFiles)]

		// Calculate tokens before compaction (all old files)
		tokensBefore := 0
		for _, file := range oldFiles {
			if content, err := os.ReadFile(file); err == nil {
				tokensBefore += llm.EstimateTokens(string(content))
			}
			responseFile := strings.Replace(file, "-prompt.txt", "-response.txt", 1)
			if responseContent, err := os.ReadFile(responseFile); err == nil {
				tokensBefore += llm.EstimateTokens(string(responseContent))
			}
		}

		// Measure compaction time
		compactionStart := time.Now()
		summary := m.summarizeHistory(oldFiles)
		compactionDuration := time.Since(compactionStart)

		summaryTokens := llm.EstimateTokens(summary)

		// Record compaction stats if store is available
		if m.statsStore != nil && len(oldFiles) > 0 {
			stats := &storage.CompactionStats{
				JobID:            m.jobID,
				InferenceRound:   len(files), // Current round number
				ModelName:        m.modelName,
				ContextLimit:     m.contextLimit,
				TokensBefore:     tokensBefore,
				TokensAfter:      summaryTokens,
				RoundsCompacted:  len(oldFiles),
				RoundsKept:       len(recentFiles),
				CompactionTimeMs: int(compactionDuration.Milliseconds()),
				ThresholdHit:     m.ShouldCompact(),
				CreatedAt:        time.Now(),
			}

			// Record stats (non-blocking, log errors but don't fail)
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := m.statsStore.RecordCompaction(ctx, stats); err != nil {
					// Log error but don't fail the operation
					if m.debugMode {
						fmt.Fprintf(os.Stderr, "Warning: failed to record compaction stats: %v\n", err)
					}
				}
			}()
		}

		if totalTokens+summaryTokens <= budget {
			var result strings.Builder
			result.WriteString("=== Previous Work Summary ===\n")
			result.WriteString(summary)
			result.WriteString("\n\n=== Recent Context ===\n")

			for _, file := range selected {
				content, _ := os.ReadFile(file)
				result.Write(content)
				result.WriteString("\n")

				// Include corresponding response
				responseFile := strings.Replace(file, "-prompt.txt", "-response.txt", 1)
				if responseContent, err := os.ReadFile(responseFile); err == nil {
					result.Write(responseContent)
					result.WriteString("\n")
				}
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

		// Include corresponding response
		responseFile := strings.Replace(file, "-prompt.txt", "-response.txt", 1)
		if responseContent, err := os.ReadFile(responseFile); err == nil {
			result.Write(responseContent)
			result.WriteString("\n")
		}
	}

	return result.String(), nil
}

// summarizeHistory creates a summary of old inference rounds
func (m *Manager) summarizeHistory(files []string) string {
	if len(files) == 0 {
		return ""
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Summarizing %d earlier inference rounds:\n\n", len(files)))

	for _, file := range files {
		roundNum := filepath.Base(file)

		// Extract the file number from the prompt file
		baseNum := strings.TrimSuffix(roundNum, "-prompt.txt")
		var promptNum int
		fmt.Sscanf(baseNum, "%d", &promptNum)

		// Try to find tool calls file (should be a few numbers after the prompt)
		toolCallsExist := false
		var calls []ToolCall

		for offset := 1; offset <= 5; offset++ {
			toolCallsFile := filepath.Join(m.jobDir, fmt.Sprintf("%03d-tool-calls.json", promptNum+offset))
			if data, err := os.ReadFile(toolCallsFile); err == nil {
				if json.Unmarshal(data, &calls) == nil && len(calls) > 0 {
					toolCallsExist = true
					summary.WriteString(fmt.Sprintf("Round %s: %d tool call(s)\n", roundNum, len(calls)))

					// Summarize tool calls with key arguments
					for _, call := range calls {
						summary.WriteString(fmt.Sprintf("  - %s", call.Name))

						// Add key args if available
						if file, ok := call.Args["file"].(string); ok {
							summary.WriteString(fmt.Sprintf(" (file: %s)", file))
						}
						if pattern, ok := call.Args["pattern"].(string); ok {
							summary.WriteString(fmt.Sprintf(" (pattern: %s)", pattern))
						}
						if path, ok := call.Args["path"].(string); ok {
							summary.WriteString(fmt.Sprintf(" (path: %s)", path))
						}
						summary.WriteString("\n")
					}

					// List files modified - look for tool-results.json nearby
					for resultOffset := offset; resultOffset <= offset+2; resultOffset++ {
						toolResultsFile := filepath.Join(m.jobDir, fmt.Sprintf("%03d-tool-results.json", promptNum+resultOffset))
						if resultData, err := os.ReadFile(toolResultsFile); err == nil {
							var results []ToolResult
							if json.Unmarshal(resultData, &results) == nil {
								for _, result := range results {
									if len(result.ModifiedFiles) > 0 {
										summary.WriteString(fmt.Sprintf("  Modified: %s\n", strings.Join(result.ModifiedFiles, ", ")))
									}
								}
							}
							break
						}
					}
					break
				}
			}
		}

		// If no tool calls, summarize the response instead
		if !toolCallsExist {
			// Extract the file number from the prompt file (e.g., "001" from "001-prompt.txt")
			baseNum := strings.TrimSuffix(roundNum, "-prompt.txt")

			// The response file should be the next number (e.g., "002-response.txt")
			// Try a few nearby numbers to find the response
			found := false
			for offset := 1; offset <= 3; offset++ {
				var num int
				fmt.Sscanf(baseNum, "%d", &num)
				nextNum := num + offset
				responseFile := filepath.Join(m.jobDir, fmt.Sprintf("%03d-response.txt", nextNum))

				if respData, err := os.ReadFile(responseFile); err == nil {
					respText := string(respData)

					// Extract first non-empty line or first 100 chars as summary
					lines := strings.Split(respText, "\n")
					firstLine := ""
					for _, line := range lines {
						trimmed := strings.TrimSpace(line)
						if trimmed != "" {
							firstLine = trimmed
							break
						}
					}

					if len(firstLine) > 100 {
						firstLine = firstLine[:100] + "..."
					}

					if firstLine != "" {
						summary.WriteString(fmt.Sprintf("Round %s: %s\n", roundNum, firstLine))
					} else {
						summary.WriteString(fmt.Sprintf("Round %s: [empty response]\n", roundNum))
					}
					found = true
					break
				}
			}

			if !found {
				// If we can't find response file, note it
				summary.WriteString(fmt.Sprintf("Round %s: [no details available]\n", roundNum))
			}
		}
	}

	return summary.String()
}

// CompactHistory compacts history by keeping recent and summarizing old
func (m *Manager) CompactHistory(keepRecentFiles int) (string, error) {
	files, err := filepath.Glob(filepath.Join(m.jobDir, "*-prompt.txt"))
	if err != nil {
		return "", err
	}
	sort.Strings(files)

	if len(files) <= keepRecentFiles {
		return m.GetHistory()
	}

	// Keep recent files as-is
	recentFiles := files[len(files)-keepRecentFiles:]

	// Summarize older files
	oldFiles := files[:len(files)-keepRecentFiles]
	summary := m.summarizeHistory(oldFiles)

	var result strings.Builder
	result.WriteString("=== Previous Work Summary ===\n")
	result.WriteString(summary)
	result.WriteString("\n\n=== Recent Context ===\n")

	for _, file := range recentFiles {
		content, _ := os.ReadFile(file)
		result.WriteString(fmt.Sprintf("\n=== %s ===\n", filepath.Base(file)))
		result.Write(content)
		result.WriteString("\n")
	}

	return result.String(), nil
}

// Cleanup removes the job directory
func (m *Manager) Cleanup() error {
	if m.debugMode {
		fmt.Printf("Debug mode: keeping temp files in %s\n", m.jobDir)
		return nil
	}

	return os.RemoveAll(m.jobDir)
}

// RecordPromptTokens tracks the token count of the last prompt
func (m *Manager) RecordPromptTokens(tokens int) {
	m.lastPromptTokens = tokens
}

// ShouldCompact returns true if we should compact history
func (m *Manager) ShouldCompact() bool {
	if m.contextLimit == 0 {
		return false // No limit configured
	}

	threshold := int(float64(m.contextLimit) * m.compactionThreshold)
	return m.lastPromptTokens >= threshold
}

// GetLastPromptTokens returns the token count of the last prompt
func (m *Manager) GetLastPromptTokens() int {
	return m.lastPromptTokens
}

// CompactionStats contains stats about context compaction
type CompactionStats struct {
	TotalRounds      int
	CompactedRounds  int
	RecentRounds     int
	LastPromptTokens int
	ContextLimit     int
	ThresholdTokens  int
	IsOverThreshold  bool
}

// GetCompactionStats returns stats about context compaction
func (m *Manager) GetCompactionStats() (*CompactionStats, error) {
	files, err := filepath.Glob(filepath.Join(m.jobDir, "*-prompt.txt"))
	if err != nil {
		return nil, err
	}

	keepRecent := 3
	if m.ShouldCompact() {
		keepRecent = 2
	}

	totalRounds := len(files)
	recentRounds := keepRecent
	if totalRounds < keepRecent {
		recentRounds = totalRounds
	}

	compactedRounds := totalRounds - recentRounds
	if compactedRounds < 0 {
		compactedRounds = 0
	}

	thresholdTokens := int(float64(m.contextLimit) * m.compactionThreshold)

	return &CompactionStats{
		TotalRounds:      totalRounds,
		CompactedRounds:  compactedRounds,
		RecentRounds:     recentRounds,
		LastPromptTokens: m.lastPromptTokens,
		ContextLimit:     m.contextLimit,
		ThresholdTokens:  thresholdTokens,
		IsOverThreshold:  m.ShouldCompact(),
	}, nil
}
