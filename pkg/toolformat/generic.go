package toolformat

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/soypete/pedrocli/pkg/tools"
)

// GenericFormatter is the default formatter that uses simple JSON format.
// This works with most models that understand JSON tool calls.
type GenericFormatter struct{}

// Name returns the formatter name
func (f *GenericFormatter) Name() string {
	return "generic"
}

// FormatToolsForPrompt generates tool descriptions using a simple format
func (f *GenericFormatter) FormatToolsForPrompt(registry *tools.ToolRegistry) string {
	toolList := registry.List()
	if len(toolList) == 0 {
		return "No tools available."
	}

	// Sort by name for consistent output
	sort.Slice(toolList, func(i, j int) bool {
		return toolList[i].Name() < toolList[j].Name()
	})

	var sb strings.Builder
	for _, tool := range toolList {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", tool.Name(), tool.Description()))
	}

	sb.WriteString(`
Call tools using JSON format:
{"tool": "tool_name", "args": {"param": "value"}}
`)
	return sb.String()
}

// ParseToolCalls parses tool calls from the LLM response.
// Supports multiple JSON formats: arrays, single objects, code blocks, and inline JSON.
func (f *GenericFormatter) ParseToolCalls(text string) ([]ToolCall, error) {
	var calls []ToolCall

	// Strategy 1: Try parsing entire response as JSON array
	calls = f.tryParseArray(text)
	if len(calls) > 0 {
		return calls, nil
	}

	// Strategy 2: Try parsing as single JSON object
	calls = f.tryParseSingle(text)
	if len(calls) > 0 {
		return calls, nil
	}

	// Strategy 3: Extract from markdown code blocks
	calls = f.extractFromCodeBlocks(text)
	if len(calls) > 0 {
		return calls, nil
	}

	// Strategy 4: Line-by-line JSON parsing
	calls = f.extractFromLines(text)
	return calls, nil
}

// FormatToolResult formats a tool result for the feedback prompt
func (f *GenericFormatter) FormatToolResult(call ToolCall, result *tools.Result) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Tool: %s\n", call.Name))

	if result.Success {
		sb.WriteString("Status: Success\n")
		if result.Output != "" {
			output := result.Output
			if len(output) > 1000 {
				output = output[:1000] + "\n... (truncated)"
			}
			sb.WriteString(fmt.Sprintf("Output:\n%s\n", output))
		}
		if len(result.ModifiedFiles) > 0 {
			sb.WriteString(fmt.Sprintf("Modified files: %v\n", result.ModifiedFiles))
		}
	} else {
		sb.WriteString("Status: Failed\n")
		sb.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
	}

	return sb.String()
}

// SupportsNativeToolUse returns false - generic format uses prompt-based tools
func (f *GenericFormatter) SupportsNativeToolUse() bool {
	return false
}

// tryParseArray attempts to parse the response as a JSON array of tool calls
func (f *GenericFormatter) tryParseArray(text string) []ToolCall {
	var rawCalls []json.RawMessage
	if err := json.Unmarshal([]byte(text), &rawCalls); err != nil {
		return nil
	}

	var calls []ToolCall
	for _, raw := range rawCalls {
		call := f.parseRawToolCall(raw)
		if call != nil {
			calls = append(calls, *call)
		}
	}
	return calls
}

// tryParseSingle attempts to parse the response as a single tool call
func (f *GenericFormatter) tryParseSingle(text string) []ToolCall {
	call := f.parseRawToolCall([]byte(text))
	if call != nil {
		return []ToolCall{*call}
	}
	return nil
}

// parseRawToolCall parses a raw JSON into a ToolCall
// Supports both "tool"/"args" and "name"/"arguments" formats
func (f *GenericFormatter) parseRawToolCall(raw []byte) *ToolCall {
	// Try standard format: {"tool": "name", "args": {...}}
	var standard struct {
		Tool string                 `json:"tool"`
		Args map[string]interface{} `json:"args"`
	}
	if err := json.Unmarshal(raw, &standard); err == nil && standard.Tool != "" {
		return &ToolCall{Name: standard.Tool, Args: standard.Args}
	}

	// Try alternative format: {"name": "name", "arguments": {...}}
	var alt struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &alt); err == nil && alt.Name != "" {
		return &ToolCall{Name: alt.Name, Args: alt.Arguments}
	}

	return nil
}

// extractFromCodeBlocks extracts tool calls from markdown code blocks
func (f *GenericFormatter) extractFromCodeBlocks(text string) []ToolCall {
	var calls []ToolCall

	// Match ```json...``` or ```...``` blocks
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n(.*?)\\n```")
	matches := re.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		content := strings.TrimSpace(match[1])

		// Try as array first
		arrayCalls := f.tryParseArray(content)
		if len(arrayCalls) > 0 {
			calls = append(calls, arrayCalls...)
			continue
		}

		// Try as single object
		singleCalls := f.tryParseSingle(content)
		calls = append(calls, singleCalls...)
	}

	return calls
}

// extractFromLines extracts tool calls from individual JSON lines
func (f *GenericFormatter) extractFromLines(text string) []ToolCall {
	var calls []ToolCall
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Must start with { to be JSON
		if !strings.HasPrefix(line, "{") {
			continue
		}

		call := f.parseRawToolCall([]byte(line))
		if call != nil {
			calls = append(calls, *call)
		}
	}

	return calls
}
