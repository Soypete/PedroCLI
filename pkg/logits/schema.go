package logits

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// JSONSchema represents a JSON Schema for structured output.
type JSONSchema struct {
	Type        string                 `json:"type,omitempty"`
	Properties  map[string]*JSONSchema `json:"properties,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Items       *JSONSchema            `json:"items,omitempty"`
	Enum        []interface{}          `json:"enum,omitempty"`
	Const       interface{}            `json:"const,omitempty"`
	MinLength   *int                   `json:"minLength,omitempty"`
	MaxLength   *int                   `json:"maxLength,omitempty"`
	Minimum     *float64               `json:"minimum,omitempty"`
	Maximum     *float64               `json:"maximum,omitempty"`
	Pattern     string                 `json:"pattern,omitempty"`
	Description string                 `json:"description,omitempty"`
	Default     interface{}            `json:"default,omitempty"`
	OneOf       []*JSONSchema          `json:"oneOf,omitempty"`
	AnyOf       []*JSONSchema          `json:"anyOf,omitempty"`
	AllOf       []*JSONSchema          `json:"allOf,omitempty"`
	Ref         string                 `json:"$ref,omitempty"`
	Defs        map[string]*JSONSchema `json:"$defs,omitempty"`

	// Additional fields for compatibility
	AdditionalProperties interface{} `json:"additionalProperties,omitempty"`
	MinItems             *int        `json:"minItems,omitempty"`
	MaxItems             *int        `json:"maxItems,omitempty"`
}

// ParseJSONSchema parses a JSON schema from bytes.
func ParseJSONSchema(data []byte) (*JSONSchema, error) {
	var schema JSONSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("parse JSON schema: %w", err)
	}
	return &schema, nil
}

// ParseJSONSchemaString parses a JSON schema from a string.
func ParseJSONSchemaString(s string) (*JSONSchema, error) {
	return ParseJSONSchema([]byte(s))
}

// SchemaToGBNF converts a JSON Schema to a GBNF grammar string.
func SchemaToGBNF(schema *JSONSchema) (string, error) {
	converter := &schemaConverter{
		rules:     make(map[string]string),
		ruleCount: 0,
		defs:      schema.Defs,
	}

	rootRule, err := converter.convert(schema, "root")
	if err != nil {
		return "", fmt.Errorf("convert schema: %w", err)
	}

	// Build grammar string
	var sb strings.Builder

	// Write root rule first
	sb.WriteString(fmt.Sprintf("root ::= %s\n", rootRule))

	// Write other rules in sorted order for determinism
	ruleNames := make([]string, 0, len(converter.rules))
	for name := range converter.rules {
		if name != "root" {
			ruleNames = append(ruleNames, name)
		}
	}
	sort.Strings(ruleNames)

	for _, name := range ruleNames {
		sb.WriteString(fmt.Sprintf("%s ::= %s\n", name, converter.rules[name]))
	}

	// Add common rules if needed
	if converter.needsWS {
		sb.WriteString("ws ::= [ \\t\\n\\r]*\n")
	}
	if converter.needsString {
		sb.WriteString("string ::= \"\\\"\" ([^\"\\\\] | \"\\\\\" [\"\\\\/bfnrt] | \"\\\\u\" [0-9a-fA-F] [0-9a-fA-F] [0-9a-fA-F] [0-9a-fA-F])* \"\\\"\"\n")
	}
	if converter.needsNumber {
		sb.WriteString("number ::= \"-\"? ([0-9] | [1-9] [0-9]*) (\".\" [0-9]+)? ([eE] [+-]? [0-9]+)?\n")
	}
	if converter.needsInteger {
		sb.WriteString("integer ::= \"-\"? ([0-9] | [1-9] [0-9]*)\n")
	}
	if converter.needsBoolean {
		sb.WriteString("boolean ::= \"true\" | \"false\"\n")
	}
	if converter.needsNull {
		sb.WriteString("null ::= \"null\"\n")
	}

	return sb.String(), nil
}

type schemaConverter struct {
	rules       map[string]string
	ruleCount   int
	defs        map[string]*JSONSchema
	needsWS     bool
	needsString bool
	needsNumber bool
	needsInteger bool
	needsBoolean bool
	needsNull   bool
}

func (c *schemaConverter) newRuleName(prefix string) string {
	c.ruleCount++
	return fmt.Sprintf("%s_%d", prefix, c.ruleCount)
}

func (c *schemaConverter) convert(schema *JSONSchema, ruleName string) (string, error) {
	if schema == nil {
		return "null", nil
	}

	// Handle $ref
	if schema.Ref != "" {
		return c.handleRef(schema.Ref, ruleName)
	}

	// Handle const
	if schema.Const != nil {
		return c.convertConst(schema.Const)
	}

	// Handle enum
	if len(schema.Enum) > 0 {
		return c.convertEnum(schema.Enum)
	}

	// Handle oneOf/anyOf
	if len(schema.OneOf) > 0 {
		return c.convertOneOf(schema.OneOf, ruleName)
	}
	if len(schema.AnyOf) > 0 {
		return c.convertAnyOf(schema.AnyOf, ruleName)
	}

	// Handle by type
	switch schema.Type {
	case "object":
		return c.convertObject(schema, ruleName)
	case "array":
		return c.convertArray(schema, ruleName)
	case "string":
		return c.convertString(schema)
	case "number":
		return c.convertNumber(schema)
	case "integer":
		return c.convertInteger(schema)
	case "boolean":
		c.needsBoolean = true
		return "boolean", nil
	case "null":
		c.needsNull = true
		return "null", nil
	default:
		// No type specified, allow any JSON value
		return c.convertAnyValue()
	}
}

func (c *schemaConverter) handleRef(ref string, ruleName string) (string, error) {
	// Handle local refs like #/$defs/MyType
	if strings.HasPrefix(ref, "#/$defs/") {
		defName := strings.TrimPrefix(ref, "#/$defs/")
		if c.defs != nil {
			if defSchema, ok := c.defs[defName]; ok {
				// Create a rule for this definition if not exists
				defRuleName := "def_" + defName
				if _, exists := c.rules[defRuleName]; !exists {
					defRule, err := c.convert(defSchema, defRuleName)
					if err != nil {
						return "", err
					}
					c.rules[defRuleName] = defRule
				}
				return defRuleName, nil
			}
		}
		return "", fmt.Errorf("undefined $ref: %s", ref)
	}
	return "", fmt.Errorf("unsupported $ref format: %s", ref)
}

func (c *schemaConverter) convertConst(val interface{}) (string, error) {
	switch v := val.(type) {
	case string:
		return fmt.Sprintf(`"\"%s\""`, escapeGBNF(v)), nil
	case float64:
		return fmt.Sprintf(`"%v"`, v), nil
	case bool:
		if v {
			return `"true"`, nil
		}
		return `"false"`, nil
	case nil:
		return `"null"`, nil
	default:
		// For complex types, serialize to JSON
		data, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(`"%s"`, escapeGBNF(string(data))), nil
	}
}

func (c *schemaConverter) convertEnum(values []interface{}) (string, error) {
	var parts []string
	for _, v := range values {
		part, err := c.convertConst(v)
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " | "), nil
}

func (c *schemaConverter) convertOneOf(schemas []*JSONSchema, ruleName string) (string, error) {
	var parts []string
	for i, schema := range schemas {
		subRuleName := fmt.Sprintf("%s_opt%d", ruleName, i)
		rule, err := c.convert(schema, subRuleName)
		if err != nil {
			return "", err
		}
		parts = append(parts, rule)
	}
	return strings.Join(parts, " | "), nil
}

func (c *schemaConverter) convertAnyOf(schemas []*JSONSchema, ruleName string) (string, error) {
	// AnyOf is similar to OneOf for generation purposes
	return c.convertOneOf(schemas, ruleName)
}

func (c *schemaConverter) convertObject(schema *JSONSchema, ruleName string) (string, error) {
	c.needsWS = true

	if len(schema.Properties) == 0 {
		// Empty object or any object
		return `"{" ws "}"`, nil
	}

	// Build property list
	// Sort properties for deterministic output
	propNames := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		propNames = append(propNames, name)
	}
	sort.Strings(propNames)

	// Create required set
	requiredSet := make(map[string]bool)
	for _, name := range schema.Required {
		requiredSet[name] = true
	}

	var parts []string
	parts = append(parts, `"{"`)
	parts = append(parts, "ws")

	first := true
	for _, propName := range propNames {
		propSchema := schema.Properties[propName]
		isRequired := requiredSet[propName]

		// Create rule for property value
		valueRuleName := c.newRuleName(ruleName + "_" + propName)
		valueRule, err := c.convert(propSchema, valueRuleName)
		if err != nil {
			return "", err
		}

		// Build property rule
		propRule := fmt.Sprintf(`"\"%s\"" ws ":" ws %s`, escapeGBNF(propName), valueRule)

		if !first {
			propRule = fmt.Sprintf(`"," ws %s`, propRule)
		}

		if !isRequired {
			propRule = fmt.Sprintf("(%s)?", propRule)
		}

		parts = append(parts, propRule)
		first = false
	}

	parts = append(parts, "ws")
	parts = append(parts, `"}"`)

	return strings.Join(parts, " "), nil
}

func (c *schemaConverter) convertArray(schema *JSONSchema, ruleName string) (string, error) {
	c.needsWS = true

	// Get item schema
	var itemRule string
	if schema.Items != nil {
		itemRuleName := c.newRuleName(ruleName + "_item")
		var err error
		itemRule, err = c.convert(schema.Items, itemRuleName)
		if err != nil {
			return "", err
		}
	} else {
		// Any value allowed
		itemRule, _ = c.convertAnyValue()
	}

	// Build array rule with repetition
	minItems := 0
	if schema.MinItems != nil {
		minItems = *schema.MinItems
	}

	if minItems == 0 {
		// Optional items
		return fmt.Sprintf(`"[" ws (%s (ws "," ws %s)*)? ws "]"`, itemRule, itemRule), nil
	}

	// At least minItems required
	var itemParts []string
	for i := 0; i < minItems; i++ {
		if i == 0 {
			itemParts = append(itemParts, itemRule)
		} else {
			itemParts = append(itemParts, fmt.Sprintf(`ws "," ws %s`, itemRule))
		}
	}
	// Additional optional items
	itemParts = append(itemParts, fmt.Sprintf(`(ws "," ws %s)*`, itemRule))

	return fmt.Sprintf(`"[" ws %s ws "]"`, strings.Join(itemParts, " ")), nil
}

func (c *schemaConverter) convertString(schema *JSONSchema) (string, error) {
	// If pattern specified, try to convert to character class
	// This is limited - full regex support would be complex
	if schema.Pattern != "" {
		// For simple patterns, we might be able to convert
		// For now, fall back to generic string
		c.needsString = true
		return "string", nil
	}

	// If enum, handle that
	if len(schema.Enum) > 0 {
		return c.convertEnum(schema.Enum)
	}

	c.needsString = true
	return "string", nil
}

func (c *schemaConverter) convertNumber(schema *JSONSchema) (string, error) {
	c.needsNumber = true
	return "number", nil
}

func (c *schemaConverter) convertInteger(schema *JSONSchema) (string, error) {
	c.needsInteger = true
	return "integer", nil
}

func (c *schemaConverter) convertAnyValue() (string, error) {
	c.needsString = true
	c.needsNumber = true
	c.needsBoolean = true
	c.needsNull = true
	c.needsWS = true

	// Create a recursive value rule if not exists
	if _, exists := c.rules["value"]; !exists {
		c.rules["value"] = `object | array | string | number | boolean | null`
		c.rules["object"] = `"{" ws (string ws ":" ws value (ws "," ws string ws ":" ws value)*)? ws "}"`
		c.rules["array"] = `"[" ws (value (ws "," ws value)*)? ws "]"`
	}

	return "value", nil
}

func escapeGBNF(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	s = strings.ReplaceAll(s, "\r", "\\r")
	return s
}

// JSONSchemaFilter wraps a grammar filter with JSON Schema support.
type JSONSchemaFilter struct {
	*GrammarFilter
	schema *JSONSchema
}

// NewJSONSchemaFilter creates a filter from a JSON Schema.
func NewJSONSchemaFilter(schema *JSONSchema, tokenizer Tokenizer) (*JSONSchemaFilter, error) {
	grammarStr, err := SchemaToGBNF(schema)
	if err != nil {
		return nil, fmt.Errorf("convert schema to grammar: %w", err)
	}

	grammar, err := ParseGBNF(grammarStr)
	if err != nil {
		return nil, fmt.Errorf("parse generated grammar: %w", err)
	}

	return &JSONSchemaFilter{
		GrammarFilter: NewGrammarFilter(grammar, tokenizer),
		schema:        schema,
	}, nil
}

// NewJSONSchemaFilterFromString creates a filter from a JSON Schema string.
func NewJSONSchemaFilterFromString(schemaStr string, tokenizer Tokenizer) (*JSONSchemaFilter, error) {
	schema, err := ParseJSONSchemaString(schemaStr)
	if err != nil {
		return nil, err
	}
	return NewJSONSchemaFilter(schema, tokenizer)
}

// Name returns the filter name.
func (f *JSONSchemaFilter) Name() string {
	return "json_schema"
}

// Description returns the filter description.
func (f *JSONSchemaFilter) Description() string {
	return "Enforces JSON Schema constraints on generation"
}

// Schema returns the underlying JSON Schema.
func (f *JSONSchemaFilter) Schema() *JSONSchema {
	return f.schema
}

// ToolCallSchema creates a JSON Schema for tool call output.
// The schema matches: {"name": "<tool_name>", "args": {...}}
func ToolCallSchema(toolName string, argsSchema *JSONSchema) *JSONSchema {
	return &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"name": {
				Type:  "string",
				Const: toolName,
			},
			"args": argsSchema,
		},
		Required: []string{"name", "args"},
	}
}

// MultiToolCallSchema creates a schema that allows any of the given tools.
func MultiToolCallSchema(tools map[string]*JSONSchema) *JSONSchema {
	var oneOf []*JSONSchema
	for name, argsSchema := range tools {
		oneOf = append(oneOf, ToolCallSchema(name, argsSchema))
	}
	return &JSONSchema{
		OneOf: oneOf,
	}
}
