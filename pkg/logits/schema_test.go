package logits

import (
	"strings"
	"testing"
)

func TestParseJSONSchemaSimple(t *testing.T) {
	schemaStr := `{"type": "object", "properties": {"name": {"type": "string"}}}`

	schema, err := ParseJSONSchemaString(schemaStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if schema.Type != "object" {
		t.Errorf("expected type 'object', got %q", schema.Type)
	}

	if len(schema.Properties) != 1 {
		t.Errorf("expected 1 property, got %d", len(schema.Properties))
	}

	nameProp := schema.Properties["name"]
	if nameProp == nil {
		t.Fatal("expected 'name' property")
	}

	if nameProp.Type != "string" {
		t.Errorf("expected name type 'string', got %q", nameProp.Type)
	}
}

func TestParseJSONSchemaWithRequired(t *testing.T) {
	schemaStr := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"}
		},
		"required": ["name"]
	}`

	schema, err := ParseJSONSchemaString(schemaStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schema.Required) != 1 {
		t.Errorf("expected 1 required field, got %d", len(schema.Required))
	}

	if schema.Required[0] != "name" {
		t.Errorf("expected required field 'name', got %q", schema.Required[0])
	}
}

func TestSchemaToGBNFObject(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
		Required: []string{"name"},
	}

	grammar, err := SchemaToGBNF(schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that grammar contains expected elements
	if !strings.Contains(grammar, "root ::=") {
		t.Error("expected grammar to contain root rule")
	}

	// The property name appears with escaping in the grammar
	if !strings.Contains(grammar, "name") {
		t.Error("expected grammar to reference name property")
	}

	// Verify it's valid GBNF
	_, err = ParseGBNF(grammar)
	if err != nil {
		t.Errorf("generated grammar is not valid GBNF: %v", err)
	}
}

func TestSchemaToGBNFArray(t *testing.T) {
	schema := &JSONSchema{
		Type: "array",
		Items: &JSONSchema{
			Type: "string",
		},
	}

	grammar, err := SchemaToGBNF(schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(grammar, "[") {
		t.Error("expected grammar to contain array brackets")
	}

	// Verify it's valid GBNF
	_, err = ParseGBNF(grammar)
	if err != nil {
		t.Errorf("generated grammar is not valid GBNF: %v", err)
	}
}

func TestSchemaToGBNFEnum(t *testing.T) {
	schema := &JSONSchema{
		Type: "string",
		Enum: []interface{}{"red", "green", "blue"},
	}

	grammar, err := SchemaToGBNF(schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Enum values appear in the grammar (possibly with escaping)
	if !strings.Contains(grammar, "red") {
		t.Error("expected grammar to contain 'red'")
	}
	if !strings.Contains(grammar, "green") {
		t.Error("expected grammar to contain 'green'")
	}
	if !strings.Contains(grammar, "blue") {
		t.Error("expected grammar to contain 'blue'")
	}
}

func TestSchemaToGBNFConst(t *testing.T) {
	schema := &JSONSchema{
		Type:  "string",
		Const: "fixed_value",
	}

	grammar, err := SchemaToGBNF(schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(grammar, "fixed_value") {
		t.Error("expected grammar to contain const value")
	}
}

func TestSchemaToGBNFOneOf(t *testing.T) {
	schema := &JSONSchema{
		OneOf: []*JSONSchema{
			{Type: "string"},
			{Type: "integer"},
		},
	}

	grammar, err := SchemaToGBNF(schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(grammar, "|") {
		t.Error("expected grammar to contain alternation")
	}
}

func TestSchemaToGBNFNested(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"user": {
				Type: "object",
				Properties: map[string]*JSONSchema{
					"name":  {Type: "string"},
					"email": {Type: "string"},
				},
			},
		},
	}

	grammar, err := SchemaToGBNF(schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's valid GBNF
	_, err = ParseGBNF(grammar)
	if err != nil {
		t.Errorf("generated grammar is not valid GBNF: %v", err)
	}
}

// TODO(issue): Update test for native API tool calling - schema format has changed
func TestToolCallSchema(t *testing.T) {
	t.Skip("TODO: Update for native API tool calling - old GBNF schema format")
	// argsSchema := &JSONSchema{
	// 	Type: "object",
	// 	Properties: map[string]*JSONSchema{
	// 		"path": {Type: "string"},
	// 	},
	// 	Required: []string{"path"},
	// }

	// schema := ToolCallSchema("read_file", argsSchema)

	// if schema.Type != "object" {
	// 	t.Errorf("expected type 'object', got %q", schema.Type)
	// }

	// nameProp := schema.Properties["name"]
	// if nameProp == nil || nameProp.Const != "read_file" {
	// 	t.Error("expected name property with const 'read_file'")
	// }

	// argsProp := schema.Properties["args"]
	// if argsProp == nil || argsProp.Type != "object" {
	// 	t.Error("expected args property of type object")
	// }
}

func TestMultiToolCallSchema(t *testing.T) {
	tools := map[string]*JSONSchema{
		"read_file": {
			Type: "object",
			Properties: map[string]*JSONSchema{
				"path": {Type: "string"},
			},
		},
		"write_file": {
			Type: "object",
			Properties: map[string]*JSONSchema{
				"path":    {Type: "string"},
				"content": {Type: "string"},
			},
		},
	}

	schema := MultiToolCallSchema(tools)

	if len(schema.OneOf) != 2 {
		t.Errorf("expected 2 oneOf options, got %d", len(schema.OneOf))
	}
}

func TestNewJSONSchemaFilter(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"name": {Type: "string"},
		},
	}

	// Create a simple tokenizer for testing
	vocab := []string{"a", "b", "c", "{", "}", ":", "\"", "name", " "}
	tokenizer := NewVocabTokenizer(vocab)

	filter, err := NewJSONSchemaFilter(schema, tokenizer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if filter.Name() != "json_schema" {
		t.Errorf("expected name 'json_schema', got %q", filter.Name())
	}

	if filter.Schema() != schema {
		t.Error("expected filter to return original schema")
	}
}

func TestEscapeGBNF(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{`hello"world`, `hello\"world`},
		{"hello\nworld", `hello\nworld`},
		{"hello\tworld", `hello\tworld`},
		{`hello\world`, `hello\\world`},
	}

	for _, tc := range tests {
		got := escapeGBNF(tc.input)
		if got != tc.expected {
			t.Errorf("escapeGBNF(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}
