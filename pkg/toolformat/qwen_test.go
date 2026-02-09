package toolformat

import (
	"strings"
	"testing"
)

func TestQwenFormatter_Name(t *testing.T) {
	f := NewQwenFormatter()
	if f.Name() != "qwen" {
		t.Errorf("Expected name 'qwen', got '%s'", f.Name())
	}
}

func TestQwenFormatter_FormatToolsPrompt(t *testing.T) {
	f := NewQwenFormatter()

	schema := NewParameterSchema()
	schema.AddProperty("action", StringProperty("Action to perform"), true)
	schema.AddProperty("path", StringProperty("File path"), true)

	tools := []ToolDefinition{
		{
			Name:        "file",
			Description: "Read or write files",
			Parameters:  schema,
		},
	}

	prompt := f.FormatToolsPrompt(tools)

	// Check for key sections
	if !strings.Contains(prompt, "# Tools") {
		t.Error("Expected '# Tools' header")
	}
	if !strings.Contains(prompt, "<tools>") {
		t.Error("Expected opening <tools> tag")
	}
	if !strings.Contains(prompt, "</tools>") {
		t.Error("Expected closing </tools> tag")
	}
	// JSON is compacted, so check for "name":"file" (no space)
	if !strings.Contains(prompt, `"name":"file"`) {
		t.Error("Expected tool name 'file' in JSON")
	}
	if !strings.Contains(prompt, "Read or write files") {
		t.Error("Expected tool description")
	}
	if !strings.Contains(prompt, "<tool_call>") {
		t.Error("Expected <tool_call> example tag")
	}
	if !strings.Contains(prompt, "</tool_call>") {
		t.Error("Expected </tool_call> example tag")
	}
	if !strings.Contains(prompt, "\"arguments\"") {
		t.Error("Expected 'arguments' field in example")
	}
}

func TestQwenFormatter_FormatToolsPrompt_MultipleTools(t *testing.T) {
	f := NewQwenFormatter()

	schema1 := NewParameterSchema()
	schema1.AddProperty("action", StringProperty("Action type"), true)

	schema2 := NewParameterSchema()
	schema2.AddProperty("pattern", StringProperty("Search pattern"), true)

	tools := []ToolDefinition{
		{
			Name:        "file",
			Description: "File operations",
			Parameters:  schema1,
		},
		{
			Name:        "search",
			Description: "Search code",
			Parameters:  schema2,
		},
	}

	prompt := f.FormatToolsPrompt(tools)

	// Check both tools are included (JSON is compacted, no spaces)
	if !strings.Contains(prompt, `"name":"file"`) {
		t.Error("Expected 'file' tool in prompt")
	}
	if !strings.Contains(prompt, `"name":"search"`) {
		t.Error("Expected 'search' tool in prompt")
	}
	if !strings.Contains(prompt, "File operations") {
		t.Error("Expected file tool description")
	}
	if !strings.Contains(prompt, "Search code") {
		t.Error("Expected search tool description")
	}
}

func TestQwenFormatter_FormatToolsAPI(t *testing.T) {
	f := NewQwenFormatter()

	schema := NewParameterSchema()
	schema.AddProperty("action", StringProperty("Action to perform"), true)
	schema.AddProperty("path", StringProperty("File path"), false)

	tools := []ToolDefinition{
		{
			Name:        "file",
			Description: "File operations",
			Parameters:  schema,
		},
	}

	result := f.FormatToolsAPI(tools)

	// Check that result is a slice
	toolDefs, ok := result.([]map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be []map[string]interface{}")
	}

	if len(toolDefs) != 1 {
		t.Fatalf("Expected 1 tool definition, got %d", len(toolDefs))
	}

	// Check OpenAI-compatible format
	def := toolDefs[0]
	if def["type"] != "function" {
		t.Errorf("Expected type 'function', got '%v'", def["type"])
	}

	function, ok := def["function"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected function field to be map")
	}

	if function["name"] != "file" {
		t.Errorf("Expected name 'file', got '%v'", function["name"])
	}
	if function["description"] != "File operations" {
		t.Errorf("Expected description 'File operations', got '%v'", function["description"])
	}

	params, ok := function["parameters"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected parameters field to be map")
	}

	if params["type"] != "object" {
		t.Errorf("Expected parameters type 'object', got '%v'", params["type"])
	}

	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected properties field to be map")
	}

	if _, hasAction := properties["action"]; !hasAction {
		t.Error("Expected 'action' property in parameters")
	}

	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("Expected required field to be []string")
	}

	if len(required) != 1 || required[0] != "action" {
		t.Errorf("Expected required = [\"action\"], got %v", required)
	}
}

func TestQwenFormatter_ParseToolCalls_XMLFormat(t *testing.T) {
	f := NewQwenFormatter()

	response := `I'll read the file for you.
<tool_call>
{"name": "file", "arguments": {"action": "read", "path": "main.go"}}
</tool_call>`

	calls, err := f.ParseToolCalls(response)
	if err != nil {
		t.Fatalf("ParseToolCalls failed: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(calls))
	}

	if calls[0].Name != "file" {
		t.Errorf("Expected name 'file', got '%s'", calls[0].Name)
	}

	if calls[0].Args["action"] != "read" {
		t.Errorf("Expected action 'read', got '%v'", calls[0].Args["action"])
	}

	if calls[0].Args["path"] != "main.go" {
		t.Errorf("Expected path 'main.go', got '%v'", calls[0].Args["path"])
	}
}

func TestQwenFormatter_ParseToolCalls_MultipleXMLTags(t *testing.T) {
	f := NewQwenFormatter()

	response := `I'll perform two operations:
<tool_call>
{"name": "file", "arguments": {"action": "read", "path": "a.go"}}
</tool_call>

Then I'll search:
<tool_call>
{"name": "search", "arguments": {"action": "grep", "pattern": "func main"}}
</tool_call>`

	calls, err := f.ParseToolCalls(response)
	if err != nil {
		t.Fatalf("ParseToolCalls failed: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("Expected 2 calls, got %d", len(calls))
	}

	if calls[0].Name != "file" {
		t.Errorf("Expected first call name 'file', got '%s'", calls[0].Name)
	}
	if calls[1].Name != "search" {
		t.Errorf("Expected second call name 'search', got '%s'", calls[1].Name)
	}
}

func TestQwenFormatter_ParseToolCalls_AlternativeFormat(t *testing.T) {
	f := NewQwenFormatter()

	// Test alternative format with "tool" instead of "name"
	response := `<tool_call>
{"tool": "search", "args": {"action": "grep", "pattern": "test"}}
</tool_call>`

	calls, err := f.ParseToolCalls(response)
	if err != nil {
		t.Fatalf("ParseToolCalls failed: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(calls))
	}

	if calls[0].Name != "search" {
		t.Errorf("Expected name 'search', got '%s'", calls[0].Name)
	}
}

func TestQwenFormatter_ParseToolCalls_FallbackToGeneric(t *testing.T) {
	f := NewQwenFormatter()

	// No XML tags - should fall back to generic parsing
	response := `{"tool": "file", "args": {"action": "read", "path": "test.go"}}`

	calls, err := f.ParseToolCalls(response)
	if err != nil {
		t.Fatalf("ParseToolCalls failed: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("Expected 1 call (via fallback), got %d", len(calls))
	}

	if calls[0].Name != "file" {
		t.Errorf("Expected name 'file', got '%s'", calls[0].Name)
	}
}

func TestQwenFormatter_ParseToolCalls_InvalidJSON(t *testing.T) {
	f := NewQwenFormatter()

	// Invalid JSON inside XML tags - should skip
	response := `<tool_call>
{this is not valid json}
</tool_call>
<tool_call>
{"name": "file", "arguments": {"action": "read"}}
</tool_call>`

	calls, err := f.ParseToolCalls(response)
	if err != nil {
		t.Fatalf("ParseToolCalls failed: %v", err)
	}

	// Should only get the valid one
	if len(calls) != 1 {
		t.Fatalf("Expected 1 call (skipping invalid), got %d", len(calls))
	}

	if calls[0].Name != "file" {
		t.Errorf("Expected name 'file', got '%s'", calls[0].Name)
	}
}

func TestQwenFormatter_FormatToolResult_Success(t *testing.T) {
	f := NewQwenFormatter()

	call := ToolCall{
		Name: "file",
		Args: map[string]interface{}{
			"action": "read",
			"path":   "test.go",
		},
	}

	result := &ToolResult{
		Success:       true,
		Output:        "package main\n\nfunc main() {}",
		ModifiedFiles: []string{"test.go"},
	}

	formatted := f.FormatToolResult(call, result)

	// Check XML structure
	if !strings.Contains(formatted, "<tool_response>") {
		t.Error("Expected opening <tool_response> tag")
	}
	if !strings.Contains(formatted, "</tool_response>") {
		t.Error("Expected closing </tool_response> tag")
	}
	if !strings.Contains(formatted, "\"name\": \"file\"") {
		t.Error("Expected tool name in result")
	}
	if !strings.Contains(formatted, "\"content\":") {
		t.Error("Expected 'content' field for success")
	}
	if !strings.Contains(formatted, "package main") {
		t.Error("Expected output content in result")
	}
}

func TestQwenFormatter_FormatToolResult_Failed(t *testing.T) {
	f := NewQwenFormatter()

	call := ToolCall{
		Name: "search",
		Args: map[string]interface{}{
			"action":  "grep",
			"pattern": "missing",
		},
	}

	result := &ToolResult{
		Success: false,
		Error:   "Pattern not found in any files",
	}

	formatted := f.FormatToolResult(call, result)

	// Check XML structure
	if !strings.Contains(formatted, "<tool_response>") {
		t.Error("Expected opening <tool_response> tag")
	}
	if !strings.Contains(formatted, "</tool_response>") {
		t.Error("Expected closing </tool_response> tag")
	}
	if !strings.Contains(formatted, "\"name\": \"search\"") {
		t.Error("Expected tool name in result")
	}
	if !strings.Contains(formatted, "\"error\":") {
		t.Error("Expected 'error' field for failure")
	}
	if !strings.Contains(formatted, "Pattern not found") {
		t.Error("Expected error message in result")
	}
	if strings.Contains(formatted, "\"content\":") {
		t.Error("Should not have 'content' field for failure")
	}
}

func TestQwenFormatter_FormatToolResult_EmptyOutput(t *testing.T) {
	f := NewQwenFormatter()

	call := ToolCall{
		Name: "bash",
		Args: map[string]interface{}{
			"command": "echo",
		},
	}

	result := &ToolResult{
		Success: true,
		Output:  "",
	}

	formatted := f.FormatToolResult(call, result)

	// Should still have valid XML structure
	if !strings.Contains(formatted, "<tool_response>") {
		t.Error("Expected opening <tool_response> tag")
	}
	if !strings.Contains(formatted, "</tool_response>") {
		t.Error("Expected closing </tool_response> tag")
	}
	if !strings.Contains(formatted, "\"content\":") {
		t.Error("Expected 'content' field even for empty output")
	}
}

func TestDetectModelFamily_Qwen(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		expected  ModelFamily
	}{
		{"Qwen 2.5 Coder", "qwen2.5-coder:32b", ModelFamilyQwen},
		{"Qwen 3 Coder", "Qwen3-Coder-30B-A3B-Instruct", ModelFamilyQwen},
		{"Qwen lowercase", "qwen:7b", ModelFamilyQwen},
		{"QWen mixed case", "QWen2.5-Coder-32B", ModelFamilyQwen},
		{"Not Qwen", "llama-3-8b", ModelFamilyLlama3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectModelFamily(tt.modelName)
			if got != tt.expected {
				t.Errorf("DetectModelFamily(%q) = %v, want %v", tt.modelName, got, tt.expected)
			}
		})
	}
}

func TestGetFormatter_Qwen(t *testing.T) {
	formatter := GetFormatter(ModelFamilyQwen)
	if formatter.Name() != "qwen" {
		t.Errorf("Expected Qwen formatter, got '%s'", formatter.Name())
	}
}
