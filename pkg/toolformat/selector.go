package toolformat

import (
	"regexp"
	"strings"
)

// GetFormatter returns the appropriate formatter for a model name.
// The model name is matched against known patterns to select the best formatter.
func GetFormatter(modelName string) ToolFormatter {
	modelLower := strings.ToLower(modelName)

	switch {
	// Qwen models (most commonly used in PedroCLI)
	case strings.Contains(modelLower, "qwen"):
		return &QwenFormatter{Version: detectQwenVersion(modelName)}

	// Llama 3.x models
	case strings.Contains(modelLower, "llama-3") ||
		strings.Contains(modelLower, "llama3") ||
		strings.Contains(modelLower, "llama-4") ||
		strings.Contains(modelLower, "llama4"):
		return &LlamaFormatter{Version: detectLlamaVersion(modelName)}

	// Mistral/Mixtral models
	case strings.Contains(modelLower, "mistral") ||
		strings.Contains(modelLower, "mixtral"):
		return &MistralFormatter{}

	// Default: generic JSON formatter
	default:
		return &GenericFormatter{}
	}
}

// detectQwenVersion extracts the Qwen version from model name
func detectQwenVersion(modelName string) string {
	modelLower := strings.ToLower(modelName)

	if strings.Contains(modelLower, "2.5-coder") || strings.Contains(modelLower, "2.5:coder") {
		return "2.5-coder"
	}
	if strings.Contains(modelLower, "2.5") {
		return "2.5"
	}
	if strings.Contains(modelLower, "2") {
		return "2"
	}
	return "unknown"
}

// detectLlamaVersion extracts the Llama version from model name
func detectLlamaVersion(modelName string) string {
	modelLower := strings.ToLower(modelName)

	// Try to extract version number
	re := regexp.MustCompile(`llama[-_]?(\d+(?:\.\d+)?)`)
	matches := re.FindStringSubmatch(modelLower)
	if len(matches) > 1 {
		return matches[1]
	}

	if strings.Contains(modelLower, "3.3") {
		return "3.3"
	}
	if strings.Contains(modelLower, "3.2") {
		return "3.2"
	}
	if strings.Contains(modelLower, "3.1") {
		return "3.1"
	}
	if strings.Contains(modelLower, "3") {
		return "3"
	}
	return "unknown"
}

// FormatterInfo provides information about a formatter
type FormatterInfo struct {
	Name        string
	Description string
	Models      []string
}

// ListFormatters returns information about all available formatters
func ListFormatters() []FormatterInfo {
	return []FormatterInfo{
		{
			Name:        "generic",
			Description: "Default JSON formatter for unknown models",
			Models:      []string{"*"},
		},
		{
			Name:        "qwen",
			Description: "Qwen 2.5 with <tool_call> XML tags",
			Models:      []string{"qwen2.5-coder", "qwen2.5", "qwen2"},
		},
		{
			Name:        "llama",
			Description: "Llama 3.x with <|python_tag|> format",
			Models:      []string{"llama-3.3", "llama-3.2", "llama-3.1", "llama-3"},
		},
		{
			Name:        "mistral",
			Description: "Mistral/Mixtral with [TOOL_CALLS] format",
			Models:      []string{"mistral", "mixtral", "mistral-nemo"},
		},
	}
}
