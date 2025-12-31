package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/soypete/pedrocli/pkg/logits"
)

// LogitTool provides logit control capabilities for structured generation.
// It interfaces with llama-server or other backends that support grammar
// and sampling parameter control.
type LogitTool struct {
	backend   logits.LlamaBackend
	tokenizer logits.Tokenizer
}

// NewLogitTool creates a new logit control tool.
func NewLogitTool(backend logits.LlamaBackend) *LogitTool {
	var tokenizer logits.Tokenizer
	if backend != nil {
		tokenizer = backend.GetTokenizer()
	}
	return &LogitTool{
		backend:   backend,
		tokenizer: tokenizer,
	}
}

// Name returns the tool name.
func (t *LogitTool) Name() string {
	return "logit"
}

// Description returns the tool description.
func (t *LogitTool) Description() string {
	return `Logit control tool for structured generation with grammar constraints.

Actions:
- generate: Generate text with logit control
- generate_structured: Generate JSON matching a schema
- generate_tool_call: Generate a tool call with guaranteed format
- test_config: Test a logit configuration
- list_presets: List available generation presets
- analyze_vocabulary: Analyze tokenizer vocabulary`
}

// Execute executes the logit tool with the given arguments.
func (t *LogitTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return &Result{
			Success: false,
			Error:   "missing required parameter: action",
		}, nil
	}

	switch action {
	case "generate":
		return t.generate(ctx, args)
	case "generate_structured":
		return t.generateStructured(ctx, args)
	case "generate_tool_call":
		return t.generateToolCall(ctx, args)
	case "test_config":
		return t.testConfig(ctx, args)
	case "list_presets":
		return t.listPresets(ctx, args)
	case "analyze_vocabulary":
		return t.analyzeVocabulary(ctx, args)
	default:
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s", action),
		}, nil
	}
}

// generate performs generation with logit control.
func (t *LogitTool) generate(ctx context.Context, args map[string]interface{}) (*Result, error) {
	if t.backend == nil {
		return &Result{
			Success: false,
			Error:   "logit backend not configured",
		}, nil
	}

	prompt, _ := args["prompt"].(string)
	if prompt == "" {
		return &Result{
			Success: false,
			Error:   "missing required parameter: prompt",
		}, nil
	}

	req := &logits.GenerateRequest{
		Prompt: prompt,
	}

	// Apply preset if specified
	if presetName, ok := args["preset"].(string); ok && presetName != "" {
		preset := logits.GetPreset(presetName)
		if preset != nil {
			req.SamplerConfig = preset.Config.Clone()
			req.Grammar = preset.Grammar
		}
	}

	// Apply system prompt
	if systemPrompt, ok := args["system_prompt"].(string); ok {
		req.SystemPrompt = systemPrompt
	}

	// Apply grammar
	if grammar, ok := args["grammar"].(string); ok {
		req.Grammar = grammar
	}

	// Apply temperature override
	if temp, ok := args["temperature"].(float64); ok {
		if req.SamplerConfig == nil {
			req.SamplerConfig = logits.DefaultSamplerConfig()
		}
		req.SamplerConfig.Temperature = float32(temp)
	}

	// Apply max_tokens override
	if maxTokens, ok := args["max_tokens"].(float64); ok {
		if req.SamplerConfig == nil {
			req.SamplerConfig = logits.DefaultSamplerConfig()
		}
		req.SamplerConfig.MaxTokens = int(maxTokens)
	}

	resp, err := t.backend.Generate(ctx, req)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("generation failed: %v", err),
		}, nil
	}

	output := map[string]interface{}{
		"text":        resp.Text,
		"token_count": resp.TokenCount,
		"stop_reason": resp.StopReason,
	}

	outputJSON, _ := json.MarshalIndent(output, "", "  ")

	return &Result{
		Success: true,
		Output:  string(outputJSON),
	}, nil
}

// generateStructured generates JSON matching a schema.
func (t *LogitTool) generateStructured(ctx context.Context, args map[string]interface{}) (*Result, error) {
	if t.backend == nil {
		return &Result{
			Success: false,
			Error:   "logit backend not configured",
		}, nil
	}

	prompt, _ := args["prompt"].(string)
	if prompt == "" {
		return &Result{
			Success: false,
			Error:   "missing required parameter: prompt",
		}, nil
	}

	// Get JSON schema
	schemaArg, ok := args["json_schema"]
	if !ok {
		return &Result{
			Success: false,
			Error:   "missing required parameter: json_schema",
		}, nil
	}

	var schema *logits.JSONSchema
	switch s := schemaArg.(type) {
	case string:
		var err error
		schema, err = logits.ParseJSONSchemaString(s)
		if err != nil {
			return &Result{
				Success: false,
				Error:   fmt.Sprintf("invalid JSON schema: %v", err),
			}, nil
		}
	case map[string]interface{}:
		schemaBytes, _ := json.Marshal(s)
		var err error
		schema, err = logits.ParseJSONSchema(schemaBytes)
		if err != nil {
			return &Result{
				Success: false,
				Error:   fmt.Sprintf("invalid JSON schema: %v", err),
			}, nil
		}
	default:
		return &Result{
			Success: false,
			Error:   "json_schema must be a string or object",
		}, nil
	}

	// Convert schema to grammar
	grammar, err := logits.SchemaToGBNF(schema)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to convert schema: %v", err),
		}, nil
	}

	req := &logits.GenerateRequest{
		Prompt:        prompt,
		SamplerConfig: logits.StructuredOutputConfig.Clone(),
		Grammar:       grammar,
		JSONSchema:    schema,
	}

	if systemPrompt, ok := args["system_prompt"].(string); ok {
		req.SystemPrompt = systemPrompt
	}

	resp, err := t.backend.Generate(ctx, req)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("generation failed: %v", err),
		}, nil
	}

	// Validate JSON
	var js interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Text)), &js); err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("output is not valid JSON: %v", err),
			Output:  resp.Text,
		}, nil
	}

	return &Result{
		Success: true,
		Output:  resp.Text,
	}, nil
}

// generateToolCall generates a tool call with guaranteed format.
func (t *LogitTool) generateToolCall(ctx context.Context, args map[string]interface{}) (*Result, error) {
	if t.backend == nil {
		return &Result{
			Success: false,
			Error:   "logit backend not configured",
		}, nil
	}

	prompt, _ := args["prompt"].(string)
	if prompt == "" {
		return &Result{
			Success: false,
			Error:   "missing required parameter: prompt",
		}, nil
	}

	// Get available tools
	toolsArg, ok := args["available_tools"]
	if !ok {
		return &Result{
			Success: false,
			Error:   "missing required parameter: available_tools",
		}, nil
	}

	var tools []*logits.ToolDefinition

	switch tt := toolsArg.(type) {
	case []interface{}:
		for _, toolArg := range tt {
			toolMap, ok := toolArg.(map[string]interface{})
			if !ok {
				continue
			}

			name, _ := toolMap["name"].(string)
			desc, _ := toolMap["description"].(string)

			var params *logits.JSONSchema
			if paramsArg, ok := toolMap["parameters"]; ok {
				paramsBytes, _ := json.Marshal(paramsArg)
				params, _ = logits.ParseJSONSchema(paramsBytes)
			}

			tools = append(tools, &logits.ToolDefinition{
				Name:        name,
				Description: desc,
				Parameters:  params,
			})
		}
	default:
		return &Result{
			Success: false,
			Error:   "available_tools must be an array",
		}, nil
	}

	if len(tools) == 0 {
		return &Result{
			Success: false,
			Error:   "no valid tools provided",
		}, nil
	}

	// Build multi-tool schema
	schemas := make(map[string]*logits.JSONSchema)
	for _, tool := range tools {
		schemas[tool.Name] = tool.Parameters
	}
	schema := logits.MultiToolCallSchema(schemas)

	grammar, err := logits.SchemaToGBNF(schema)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to build tool grammar: %v", err),
		}, nil
	}

	req := &logits.GenerateRequest{
		Prompt:        prompt,
		SamplerConfig: logits.DeterministicConfig.Clone(),
		Grammar:       grammar,
	}

	resp, err := t.backend.Generate(ctx, req)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("generation failed: %v", err),
		}, nil
	}

	// Parse the tool call
	var toolCall logits.ParsedToolCall
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Text)), &toolCall); err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to parse tool call: %v", err),
			Output:  resp.Text,
		}, nil
	}

	output := map[string]interface{}{
		"tool_call": toolCall,
		"raw_text":  resp.Text,
	}
	outputJSON, _ := json.MarshalIndent(output, "", "  ")

	return &Result{
		Success: true,
		Output:  string(outputJSON),
	}, nil
}

// testConfig tests a logit configuration.
func (t *LogitTool) testConfig(ctx context.Context, args map[string]interface{}) (*Result, error) {
	if t.backend == nil {
		return &Result{
			Success: false,
			Error:   "logit backend not configured",
		}, nil
	}

	// Get test prompts
	promptsArg, ok := args["test_prompts"]
	if !ok {
		return &Result{
			Success: false,
			Error:   "missing required parameter: test_prompts",
		}, nil
	}

	var prompts []string
	switch p := promptsArg.(type) {
	case []interface{}:
		for _, item := range p {
			if s, ok := item.(string); ok {
				prompts = append(prompts, s)
			}
		}
	case string:
		prompts = []string{p}
	default:
		return &Result{
			Success: false,
			Error:   "test_prompts must be a string or array of strings",
		}, nil
	}

	if len(prompts) == 0 {
		return &Result{
			Success: false,
			Error:   "no test prompts provided",
		}, nil
	}

	// Get iterations
	iterations := 1
	if iter, ok := args["iterations"].(float64); ok {
		iterations = int(iter)
	}

	// Build test cases
	var testCases []*logits.LogitTestCase
	for i, prompt := range prompts {
		tc := &logits.LogitTestCase{
			Name:       fmt.Sprintf("test_%d", i+1),
			Prompt:     prompt,
			Iterations: iterations,
		}

		if presetName, ok := args["preset"].(string); ok {
			tc.PresetName = presetName
		}

		if grammar, ok := args["grammar"].(string); ok {
			tc.Grammar = grammar
		}

		if expectedFormat, ok := args["expected_format"].(string); ok {
			tc.ExpectedFormat = expectedFormat
		}

		if expectedJSON, ok := args["expected_json"].(bool); ok {
			tc.ExpectedJSON = expectedJSON
		}

		testCases = append(testCases, tc)
	}

	// Run tests
	harness := logits.NewLogitTestHarness(t.backend)
	harness.AddTestCases(testCases...)
	results := harness.RunTests(ctx)

	// Format results
	output := harness.PrintResults()

	// Determine overall success
	allPassed := true
	for _, result := range results {
		if result.Summary != nil && result.Summary.SuccessRate < 1.0 {
			allPassed = false
			break
		}
	}

	return &Result{
		Success: allPassed,
		Output:  output,
	}, nil
}

// listPresets lists available generation presets.
func (t *LogitTool) listPresets(ctx context.Context, args map[string]interface{}) (*Result, error) {
	presets := make([]map[string]interface{}, 0)

	for name, preset := range logits.Presets {
		presetInfo := map[string]interface{}{
			"name":        name,
			"description": preset.Description,
		}

		if preset.Config != nil {
			presetInfo["temperature"] = preset.Config.Temperature
			presetInfo["top_k"] = preset.Config.TopK
			presetInfo["top_p"] = preset.Config.TopP
		}

		if preset.Grammar != "" {
			presetInfo["has_grammar"] = true
		}

		if preset.SafetyPreset != "" {
			presetInfo["safety_preset"] = preset.SafetyPreset
		}

		presets = append(presets, presetInfo)
	}

	output, _ := json.MarshalIndent(presets, "", "  ")

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// analyzeVocabulary analyzes the tokenizer vocabulary.
func (t *LogitTool) analyzeVocabulary(ctx context.Context, args map[string]interface{}) (*Result, error) {
	if t.tokenizer == nil {
		return &Result{
			Success: false,
			Error:   "tokenizer not available",
		}, nil
	}

	result := map[string]interface{}{
		"vocab_size": t.tokenizer.VocabSize(),
		"eos_token":  t.tokenizer.EOSToken(),
		"bos_token":  t.tokenizer.BOSToken(),
	}

	// Search for specific terms if requested
	if searchTerms, ok := args["search_terms"].([]interface{}); ok {
		matches := make(map[string][]int)
		for _, term := range searchTerms {
			if termStr, ok := term.(string); ok {
				tokenIDs := t.tokenizer.(*logits.VocabTokenizer).FindTokensContaining(termStr)
				matches[termStr] = tokenIDs
			}
		}
		result["search_matches"] = matches
	}

	// Analyze by category if requested
	if category, ok := args["category"].(string); ok {
		// Find tokens related to the category
		var tokenIDs []int
		switch category {
		case "special":
			for i := 0; i < t.tokenizer.VocabSize(); i++ {
				if t.tokenizer.IsSpecialToken(i) {
					tokenIDs = append(tokenIDs, i)
				}
			}
		case "punctuation":
			vocab := t.tokenizer.GetVocabulary()
			for i, token := range vocab {
				if len(token) == 1 && strings.ContainsAny(token, ".,;:!?'\"()[]{}") {
					tokenIDs = append(tokenIDs, i)
				}
			}
		case "whitespace":
			vocab := t.tokenizer.GetVocabulary()
			for i, token := range vocab {
				if strings.TrimSpace(token) == "" && token != "" {
					tokenIDs = append(tokenIDs, i)
				}
			}
		}
		result["category_tokens"] = tokenIDs
	}

	output, _ := json.MarshalIndent(result, "", "  ")

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// SetBackend sets the backend for generation.
func (t *LogitTool) SetBackend(backend logits.LlamaBackend) {
	t.backend = backend
	if backend != nil {
		t.tokenizer = backend.GetTokenizer()
	}
}

// SetTokenizer sets a custom tokenizer.
func (t *LogitTool) SetTokenizer(tokenizer logits.Tokenizer) {
	t.tokenizer = tokenizer
}
