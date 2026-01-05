package toolformat

import (
	"encoding/json"
	"fmt"
)

// ToolCall represents a parsed tool call from an LLM response
type ToolCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// UnmarshalJSON implements custom unmarshaling to accept both "tool" and "name" fields
// This provides backwards compatibility with prompts that use "tool" instead of "name"
func (tc *ToolCall) UnmarshalJSON(data []byte) error {
	// Define an alias to prevent recursion
	type Alias ToolCall

	// Temporary struct that accepts both "tool" and "name"
	aux := &struct {
		Tool string `json:"tool"` // Accept "tool" as alternative to "name"
		*Alias
	}{
		Alias: (*Alias)(tc),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// If "tool" was provided but "name" wasn't, use "tool" value for Name
	if aux.Tool != "" && tc.Name == "" {
		tc.Name = aux.Tool
	}

	return nil
}

// ToolFormatter defines the interface for model-specific tool formatting
type ToolFormatter interface {
	// Name returns the formatter name (e.g., "llama3", "qwen", "claude")
	Name() string

	// FormatToolsPrompt generates the tool definitions portion of the system prompt
	// This is used for models that expect tools in the prompt text
	FormatToolsPrompt(tools []ToolDefinition) string

	// FormatToolsAPI generates tool definitions for API-based tool use
	// Returns nil if the model doesn't support native API tool use
	FormatToolsAPI(tools []ToolDefinition) interface{}

	// ParseToolCalls extracts tool calls from an LLM response
	ParseToolCalls(response string) ([]ToolCall, error)

	// FormatToolResult formats a tool result for feeding back to the LLM
	FormatToolResult(call ToolCall, result *ToolResult) string
}

// ModelFamily represents a family of LLM models with similar tool formats
type ModelFamily string

const (
	ModelFamilyLlama3  ModelFamily = "llama3"  // Llama 3.x models
	ModelFamilyQwen    ModelFamily = "qwen"    // Qwen 2.5 models
	ModelFamilyMistral ModelFamily = "mistral" // Mistral/Mixtral models
	ModelFamilyHermes  ModelFamily = "hermes"  // Hermes/Nous models
	ModelFamilyClaude  ModelFamily = "claude"  // Claude API
	ModelFamilyOpenAI  ModelFamily = "openai"  // OpenAI-compatible APIs
	ModelFamilyGeneric ModelFamily = "generic" // Generic JSON fallback
)

// GetFormatterForModel returns the appropriate formatter for a model name
func GetFormatterForModel(modelName string) ToolFormatter {
	family := DetectModelFamily(modelName)
	return GetFormatter(family)
}

// GetFormatter returns a formatter for the specified model family
func GetFormatter(family ModelFamily) ToolFormatter {
	switch family {
	case ModelFamilyLlama3:
		return NewLlama3Formatter()
	case ModelFamilyQwen:
		return NewQwenFormatter()
	case ModelFamilyMistral:
		return NewMistralFormatter()
	case ModelFamilyHermes:
		return NewHermesFormatter()
	case ModelFamilyClaude:
		return NewClaudeFormatter()
	case ModelFamilyOpenAI:
		return NewOpenAIFormatter()
	default:
		return NewGenericFormatter()
	}
}

// DetectModelFamily attempts to detect the model family from the model name
func DetectModelFamily(modelName string) ModelFamily {
	// Normalize to lowercase for matching
	name := normalizeModelName(modelName)

	// Check for specific model families
	switch {
	case containsAny(name, "llama3", "llama-3", "llama:3"):
		return ModelFamilyLlama3
	case containsAny(name, "qwen", "qwen2"):
		return ModelFamilyQwen
	case containsAny(name, "mistral", "mixtral"):
		return ModelFamilyMistral
	case containsAny(name, "hermes", "nous"):
		return ModelFamilyHermes
	case containsAny(name, "claude"):
		return ModelFamilyClaude
	case containsAny(name, "gpt-4", "gpt-3.5", "openai"):
		return ModelFamilyOpenAI
	default:
		return ModelFamilyGeneric
	}
}

// Helper to normalize model names
func normalizeModelName(name string) string {
	// Convert to lowercase
	result := make([]byte, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			c = c + 32 // lowercase
		}
		result[i] = c
	}
	return string(result)
}

// Helper to check if string contains any of the substrings
func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if containsString(s, substr) {
			return true
		}
	}
	return false
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// FormatToolCallJSON formats a tool call as JSON string
func FormatToolCallJSON(name string, args map[string]interface{}) string {
	call := ToolCall{Name: name, Args: args}
	data, _ := json.Marshal(call)
	return string(data)
}

// FormatError creates an error result
func FormatError(format string, args ...interface{}) *ToolResult {
	return &ToolResult{
		Success: false,
		Error:   fmt.Sprintf(format, args...),
	}
}
