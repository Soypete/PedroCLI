package context

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/llm"
)

// Manager handles file-based context storage for jobs
type Manager struct {
	jobID     string
	jobDir    string
	counter   int
	debugMode bool
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
func NewManager(jobID string, debugMode bool) (*Manager, error) {
	timestamp := time.Now().Format("20060102-150405")
	jobDir := filepath.Join("/tmp/pedroceli-jobs", fmt.Sprintf("%s-%s", jobID, timestamp))

	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create job directory: %w", err)
	}

	return &Manager{
		jobID:     jobID,
		jobDir:    jobDir,
		counter:   0,
		debugMode: debugMode,
	}, nil
}

// GetJobDir returns the job directory path
func (m *Manager) GetJobDir() string {
	return m.jobDir
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

	// Keep recent files (last 3 inference rounds = 6 files: 3 prompts + 3 responses)
	keepRecent := 3
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
		summary := m.summarizeHistory(oldFiles)
		summaryTokens := llm.EstimateTokens(summary)

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
	var summary strings.Builder

	for _, file := range files {
		// Extract key facts from tool calls
		toolCallsFile := strings.Replace(file, "-prompt.txt", "-tool-calls.json", 1)
		if data, err := os.ReadFile(toolCallsFile); err == nil {
			var calls []ToolCall
			if json.Unmarshal(data, &calls) == nil {
				summary.WriteString(fmt.Sprintf("Step %s: %d tool calls\n",
					filepath.Base(file), len(calls)))

				// List files modified
				toolResultsFile := strings.Replace(file, "-prompt.txt", "-tool-results.json", 1)
				if resultData, err := os.ReadFile(toolResultsFile); err == nil {
					var results []ToolResult
					if json.Unmarshal(resultData, &results) == nil {
						for _, result := range results {
							if len(result.ModifiedFiles) > 0 {
								for _, modFile := range result.ModifiedFiles {
									summary.WriteString(fmt.Sprintf("  - Modified: %s\n", modFile))
								}
							}
						}
					}
				}
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
