package logits

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ToolDefinition describes a tool that can be called by the LLM.
type ToolDefinition struct {
	// Name is the tool's identifier
	Name string `json:"name"`

	// Description describes what the tool does
	Description string `json:"description"`

	// Parameters is the JSON Schema for the tool's arguments
	Parameters *JSONSchema `json:"parameters"`

	// grammar is the pre-generated GBNF grammar for this tool's output
	grammar *GBNF
}

// NewToolDefinition creates a new tool definition.
func NewToolDefinition(name, description string, parameters *JSONSchema) *ToolDefinition {
	return &ToolDefinition{
		Name:        name,
		Description: description,
		Parameters:  parameters,
	}
}

// GenerateGrammar generates the GBNF grammar for this tool's output format.
func (t *ToolDefinition) GenerateGrammar() (*GBNF, error) {
	if t.grammar != nil {
		return t.grammar, nil
	}

	// Create schema for tool call output
	schema := ToolCallSchema(t.Name, t.Parameters)

	grammarStr, err := SchemaToGBNF(schema)
	if err != nil {
		return nil, fmt.Errorf("generate tool grammar: %w", err)
	}

	grammar, err := ParseGBNF(grammarStr)
	if err != nil {
		return nil, fmt.Errorf("parse tool grammar: %w", err)
	}

	t.grammar = grammar
	return grammar, nil
}

// ToolParseState tracks the parsing state of a tool call.
type ToolParseState int

const (
	// StateExpectingStart expects the opening brace
	StateExpectingStart ToolParseState = iota

	// StateExpectingToolName expects the "name" field
	StateExpectingToolName

	// StateExpectingNameValue expects the tool name value
	StateExpectingNameValue

	// StateExpectingArgs expects the "args" field
	StateExpectingArgs

	// StateExpectingArgsValue expects the args object
	StateExpectingArgsValue

	// StateParsingArgs is inside the args object
	StateParsingArgs

	// StateExpectingEnd expects the closing brace
	StateExpectingEnd

	// StateComplete parsing is complete
	StateComplete
)

// ToolCallFilter enforces tool call format at the logit level.
// It ensures output follows: {"name": "<tool_name>", "args": {...}}
type ToolCallFilter struct {
	StatefulFilter

	tools     map[string]*ToolDefinition
	tokenizer Tokenizer

	// State tracking
	parseState    ToolParseState
	currentTool   *ToolDefinition
	generatedText string

	// Grammar filter for current tool's args
	argsGrammarFilter *GrammarFilter

	// Multi-tool grammar for tool name selection
	multiToolGrammar *GBNF
}

// NewToolCallFilter creates a filter for the given tool definitions.
func NewToolCallFilter(tools []*ToolDefinition, tokenizer Tokenizer) (*ToolCallFilter, error) {
	toolMap := make(map[string]*ToolDefinition)
	for _, tool := range tools {
		toolMap[tool.Name] = tool
	}

	filter := &ToolCallFilter{
		StatefulFilter: StatefulFilter{enabled: true},
		tools:          toolMap,
		tokenizer:      tokenizer,
		parseState:     StateExpectingStart,
	}

	// Generate multi-tool grammar
	if err := filter.generateMultiToolGrammar(); err != nil {
		return nil, err
	}

	return filter, nil
}

// generateMultiToolGrammar creates a grammar that allows any registered tool.
func (f *ToolCallFilter) generateMultiToolGrammar() error {
	schemas := make(map[string]*JSONSchema)
	for name, tool := range f.tools {
		schemas[name] = tool.Parameters
	}

	schema := MultiToolCallSchema(schemas)
	grammarStr, err := SchemaToGBNF(schema)
	if err != nil {
		return fmt.Errorf("generate multi-tool grammar: %w", err)
	}

	grammar, err := ParseGBNF(grammarStr)
	if err != nil {
		return fmt.Errorf("parse multi-tool grammar: %w", err)
	}

	f.multiToolGrammar = grammar
	return nil
}

// Name returns the filter name.
func (f *ToolCallFilter) Name() string {
	return "tool_call"
}

// Description returns the filter description.
func (f *ToolCallFilter) Description() string {
	return "Enforces tool call format for structured outputs"
}

// Apply constrains logits based on current parse state.
func (f *ToolCallFilter) Apply(logits []float32, ctx *GenerationContext) []float32 {
	if !f.enabled {
		return logits
	}

	switch f.parseState {
	case StateExpectingStart:
		// Only allow '{' token
		return f.maskToLiteral(logits, "{")

	case StateExpectingToolName:
		// Only allow '"name"' tokens
		return f.maskToLiteral(logits, `"name"`)

	case StateExpectingNameValue:
		// Only allow valid tool names
		return f.maskToToolNames(logits)

	case StateExpectingArgs:
		// Only allow '"args"' tokens
		return f.maskToLiteral(logits, `"args"`)

	case StateParsingArgs:
		// Delegate to tool's args grammar filter if available
		if f.argsGrammarFilter != nil {
			return f.argsGrammarFilter.Apply(logits, ctx)
		}
		// Otherwise allow any JSON value
		return logits

	case StateExpectingEnd:
		// Only allow '}' token
		return f.maskToLiteral(logits, "}")

	case StateComplete:
		// Only allow EOS
		return f.maskToEOS(logits)
	}

	return logits
}

// maskToLiteral allows only tokens that could produce the given literal.
func (f *ToolCallFilter) maskToLiteral(logits []float32, literal string) []float32 {
	// Find what part of the literal we've already matched
	remaining := literal
	if len(f.generatedText) > 0 {
		// Check if generated text is a prefix of literal
		for i := 0; i < len(f.generatedText) && i < len(literal); i++ {
			if f.generatedText[i] == literal[i] {
				remaining = literal[i+1:]
			} else {
				break
			}
		}
	}

	if len(remaining) == 0 {
		// Already matched, allow next state
		return logits
	}

	vocab := f.tokenizer.GetVocabulary()

	for tokenID := range logits {
		if tokenID < len(vocab) {
			tokenStr := vocab[tokenID]
			// Allow if token is a prefix of remaining or remaining is prefix of token
			if !strings.HasPrefix(remaining, tokenStr) && !strings.HasPrefix(tokenStr, remaining) {
				logits[tokenID] = NegativeInfinity
			}
		}
	}

	return logits
}

// maskToToolNames allows only tokens that form valid tool names.
func (f *ToolCallFilter) maskToToolNames(logits []float32) []float32 {
	vocab := f.tokenizer.GetVocabulary()

	// Build set of valid tool name prefixes
	validPrefixes := make(map[string]bool)
	for name := range f.tools {
		quotedName := fmt.Sprintf(`"%s"`, name)
		// Add all prefixes
		for i := 1; i <= len(quotedName); i++ {
			validPrefixes[quotedName[:i]] = true
		}
	}

	// Current prefix being built
	currentPrefix := ""
	if len(f.generatedText) > 0 {
		// Extract the part after the last colon (the value we're building)
		idx := strings.LastIndex(f.generatedText, ":")
		if idx >= 0 {
			currentPrefix = strings.TrimSpace(f.generatedText[idx+1:])
		}
	}

	for tokenID := range logits {
		if tokenID < len(vocab) {
			tokenStr := vocab[tokenID]
			testStr := currentPrefix + tokenStr

			// Allow if testStr is a prefix of any valid tool name
			isValid := false
			for name := range f.tools {
				quotedName := fmt.Sprintf(`"%s"`, name)
				if strings.HasPrefix(quotedName, testStr) || strings.HasPrefix(testStr, quotedName) {
					isValid = true
					break
				}
			}

			if !isValid {
				logits[tokenID] = NegativeInfinity
			}
		}
	}

	return logits
}

// maskToEOS only allows the EOS token.
func (f *ToolCallFilter) maskToEOS(logits []float32) []float32 {
	eosID := f.tokenizer.EOSToken()

	for i := range logits {
		if i != eosID {
			logits[i] = NegativeInfinity
		}
	}

	return logits
}

// OnTokenGenerated updates parse state based on generated tokens.
func (f *ToolCallFilter) OnTokenGenerated(tokenID int, tokenText string, ctx *GenerationContext) {
	if !f.enabled {
		return
	}

	f.generatedText += tokenText

	// Update parse state based on generated text
	f.updateParseState()

	// Forward to args grammar filter if in args parsing state
	if f.parseState == StateParsingArgs && f.argsGrammarFilter != nil {
		f.argsGrammarFilter.OnTokenGenerated(tokenID, tokenText, ctx)
	}
}

// updateParseState updates the state machine based on generated text.
func (f *ToolCallFilter) updateParseState() {
	text := strings.TrimSpace(f.generatedText)

	switch f.parseState {
	case StateExpectingStart:
		if strings.HasPrefix(text, "{") {
			f.parseState = StateExpectingToolName
		}

	case StateExpectingToolName:
		if strings.Contains(text, `"name"`) {
			f.parseState = StateExpectingNameValue
		}

	case StateExpectingNameValue:
		// Look for completed tool name
		for name := range f.tools {
			if strings.Contains(text, fmt.Sprintf(`"%s"`, name)) {
				f.currentTool = f.tools[name]
				f.parseState = StateExpectingArgs

				// Setup args grammar filter
				if f.currentTool.Parameters != nil {
					grammar, err := f.currentTool.GenerateGrammar()
					if err == nil && grammar != nil {
						// Create a filter for just the args portion
						// This is simplified - full implementation would extract args grammar
						f.argsGrammarFilter = NewGrammarFilter(grammar, f.tokenizer)
					}
				}
				break
			}
		}

	case StateExpectingArgs:
		if strings.Contains(text, `"args"`) {
			f.parseState = StateParsingArgs
		}

	case StateParsingArgs:
		// Check for args object completion
		// This is simplified - would need proper brace matching
		if f.isArgsComplete(text) {
			f.parseState = StateExpectingEnd
		}

	case StateExpectingEnd:
		if strings.HasSuffix(text, "}") {
			f.parseState = StateComplete
		}
	}
}

// isArgsComplete checks if the args object is complete.
// This is a simplified check - proper implementation would track brace depth.
func (f *ToolCallFilter) isArgsComplete(text string) bool {
	// Find "args": and check if the object is closed
	idx := strings.Index(text, `"args"`)
	if idx == -1 {
		return false
	}

	argsSection := text[idx:]

	// Find the opening brace of args
	braceIdx := strings.Index(argsSection, "{")
	if braceIdx == -1 {
		return false
	}

	// Count braces
	depth := 0
	for _, ch := range argsSection[braceIdx:] {
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return true
			}
		}
	}

	return false
}

// Reset resets the filter state for a new generation.
func (f *ToolCallFilter) Reset() {
	f.parseState = StateExpectingStart
	f.currentTool = nil
	f.generatedText = ""
	f.argsGrammarFilter = nil
}

// RegisterTool adds a new tool to the filter.
func (f *ToolCallFilter) RegisterTool(tool *ToolDefinition) error {
	f.tools[tool.Name] = tool
	return f.generateMultiToolGrammar()
}

// UnregisterTool removes a tool from the filter.
func (f *ToolCallFilter) UnregisterTool(name string) error {
	delete(f.tools, name)
	return f.generateMultiToolGrammar()
}

// Tools returns the registered tool definitions.
func (f *ToolCallFilter) Tools() []*ToolDefinition {
	tools := make([]*ToolDefinition, 0, len(f.tools))
	for _, tool := range f.tools {
		tools = append(tools, tool)
	}
	return tools
}

// CurrentTool returns the currently detected tool, if any.
func (f *ToolCallFilter) CurrentTool() *ToolDefinition {
	return f.currentTool
}

// ParseState returns the current parse state.
func (f *ToolCallFilter) ParseState() ToolParseState {
	return f.parseState
}

// GeneratedText returns the text generated so far.
func (f *ToolCallFilter) GeneratedText() string {
	return f.generatedText
}

// ParseToolCall attempts to parse the generated text as a tool call.
func (f *ToolCallFilter) ParseToolCall() (*ParsedToolCall, error) {
	if f.parseState != StateComplete {
		return nil, fmt.Errorf("tool call not complete")
	}

	var call ParsedToolCall
	if err := json.Unmarshal([]byte(f.generatedText), &call); err != nil {
		return nil, fmt.Errorf("parse tool call: %w", err)
	}

	return &call, nil
}

// ParsedToolCall represents a parsed tool call.
type ParsedToolCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// SimpleToolCallFilter is a simplified filter that just enforces JSON structure.
// Use this when you don't need per-tool grammar constraints.
type SimpleToolCallFilter struct {
	*GrammarFilter
}

// NewSimpleToolCallFilter creates a filter using the generic tool call grammar.
func NewSimpleToolCallFilter(tokenizer Tokenizer) (*SimpleToolCallFilter, error) {
	grammar, err := ParseGBNF(ToolCallGrammar)
	if err != nil {
		return nil, fmt.Errorf("parse tool call grammar: %w", err)
	}

	return &SimpleToolCallFilter{
		GrammarFilter: NewGrammarFilter(grammar, tokenizer),
	}, nil
}

// Name returns the filter name.
func (f *SimpleToolCallFilter) Name() string {
	return "simple_tool_call"
}

// Description returns the filter description.
func (f *SimpleToolCallFilter) Description() string {
	return "Enforces basic tool call JSON structure"
}
