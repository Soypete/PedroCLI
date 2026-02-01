package toolformat

import (
	"strings"
	"testing"
)

func TestGLM4Formatter_Name(t *testing.T) {
	f := NewGLM4Formatter()
	if f.Name() != "glm4" {
		t.Errorf("Expected name 'glm4', got '%s'", f.Name())
	}
}

func TestGLM4Formatter_FormatToolsPrompt(t *testing.T) {
	f := NewGLM4Formatter()

	schema := NewParameterSchema()
	schema.AddProperty("file", StringProperty("The file to read"), true)
	schema.AddProperty("line_start", NumberProperty("Starting line number"), false)

	tools := []ToolDefinition{
		{
			Name:        "file",
			Description: "Read or write files",
			Parameters:  schema,
		},
	}

	prompt := f.FormatToolsPrompt(tools)

	// Check for key sections
	if !strings.Contains(prompt, "# Available Tools") {
		t.Error("Expected '# Available Tools' header")
	}
	if !strings.Contains(prompt, "## file") {
		t.Error("Expected tool name 'file'")
	}
	if !strings.Contains(prompt, "Read or write files") {
		t.Error("Expected tool description")
	}
	if !strings.Contains(prompt, "file (string) (required)") {
		t.Error("Expected required parameter 'file'")
	}
	if !strings.Contains(prompt, "line_start (number)") {
		t.Error("Expected optional parameter 'line_start'")
	}
	if !strings.Contains(prompt, "# Tool Call Format") {
		t.Error("Expected tool call format section")
	}
}

func TestGLM4Formatter_FormatToolsAPI(t *testing.T) {
	f := NewGLM4Formatter()

	schema := NewParameterSchema()
	schema.AddProperty("pattern", StringProperty("Search pattern"), true)

	tools := []ToolDefinition{
		{
			Name:        "search",
			Description: "Search code",
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

	if function["name"] != "search" {
		t.Errorf("Expected name 'search', got '%v'", function["name"])
	}
	if function["description"] != "Search code" {
		t.Errorf("Expected description 'Search code', got '%v'", function["description"])
	}

	params, ok := function["parameters"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected parameters field to be map")
	}

	if params["type"] != "object" {
		t.Errorf("Expected parameters type 'object', got '%v'", params["type"])
	}
}

func TestGLM4Formatter_ParseToolCalls_SingleJSON(t *testing.T) {
	f := NewGLM4Formatter()

	response := `{"name": "file", "args": {"file": "test.go", "action": "read"}}`

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

	if calls[0].Args["file"] != "test.go" {
		t.Errorf("Expected file 'test.go', got '%v'", calls[0].Args["file"])
	}
}

func TestGLM4Formatter_ParseToolCalls_CodeBlock(t *testing.T) {
	f := NewGLM4Formatter()

	response := "I'll search for the function:\n```json\n{\"name\": \"search\", \"args\": {\"pattern\": \"func main\"}}\n```"

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

func TestGLM4Formatter_ParseToolCalls_MultipleToolCalls(t *testing.T) {
	f := NewGLM4Formatter()

	response := `{"name": "file", "args": {"file": "main.go", "action": "read"}}
{"name": "search", "args": {"pattern": "func main"}}`

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

func TestGLM4Formatter_FormatToolResult_Success(t *testing.T) {
	f := NewGLM4Formatter()

	call := ToolCall{
		Name: "file",
		Args: map[string]interface{}{"file": "test.go"},
	}

	result := &ToolResult{
		Success:       true,
		Output:        "File contents here",
		ModifiedFiles: []string{"test.go"},
	}

	formatted := f.FormatToolResult(call, result)

	if !strings.Contains(formatted, "Tool: file") {
		t.Error("Expected tool name in result")
	}
	if !strings.Contains(formatted, "Status: Success") {
		t.Error("Expected success status")
	}
	if !strings.Contains(formatted, "File contents here") {
		t.Error("Expected output in result")
	}
	if !strings.Contains(formatted, "Modified files: test.go") {
		t.Error("Expected modified files in result")
	}
}

func TestGLM4Formatter_FormatToolResult_Failed(t *testing.T) {
	f := NewGLM4Formatter()

	call := ToolCall{
		Name: "search",
		Args: map[string]interface{}{"pattern": "missing"},
	}

	result := &ToolResult{
		Success: false,
		Error:   "Pattern not found",
	}

	formatted := f.FormatToolResult(call, result)

	if !strings.Contains(formatted, "Tool: search") {
		t.Error("Expected tool name in result")
	}
	if !strings.Contains(formatted, "Status: Failed") {
		t.Error("Expected failed status")
	}
	if !strings.Contains(formatted, "Error: Pattern not found") {
		t.Error("Expected error message in result")
	}
}

func TestDetectModelFamily_GLM4(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		expected  ModelFamily
	}{
		{"GLM-4 with dash", "glm-4-9b-chat", ModelFamilyGLM4},
		{"GLM4 without dash", "glm4-flash", ModelFamilyGLM4},
		{"ChatGLM", "chatglm3-6b", ModelFamilyGLM4},
		{"GLM-4 uppercase", "GLM-4-7B", ModelFamilyGLM4},
		{"Not GLM-4", "llama-3-8b", ModelFamilyLlama3},
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

func TestGetFormatter_GLM4(t *testing.T) {
	formatter := GetFormatter(ModelFamilyGLM4)
	if formatter.Name() != "glm4" {
		t.Errorf("Expected GLM4 formatter, got '%s'", formatter.Name())
	}
}
