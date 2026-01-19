package agents

// GetAntiHallucinationBias returns logit bias to prevent tool result fabrication
// This uses approximate token IDs for common LLMs (Llama 3.x, Qwen 2.5)
// Token IDs are model-specific, so this may need tuning per model
func GetAntiHallucinationBias() map[int]float32 {
	bias := make(map[int]float32)

	// Bias against narrative/fabrication patterns
	// These are APPROXIMATE token IDs - may vary by model

	// Prevent starting fabricated JSON blocks
	// "```json" - heavily penalize
	bias[13249] = -50.0 // ``` (markdown code fence)
	bias[2285] = -30.0  // json

	// Prevent writing fake tool results
	bias[7575] = -40.0 // "Tool"
	bias[2122] = -40.0 // "Result"
	bias[5207] = -40.0 // "Output"

	// Prevent narrative mode phrases
	bias[5562] = -20.0 // "Let's"
	bias[8005] = -20.0 // "should"
	bias[1053] = -20.0 // "would"
	bias[3685] = -20.0 // "will"
	bias[3685] = -20.0 // "expected"

	// Prevent fake success indicators
	bias[2375] = -30.0  // "✓" or "✅"
	bias[2377] = -30.0  // "✗" or "❌"
	bias[12950] = -30.0 // "PASS"
	bias[8755] = -30.0  // "FAIL"

	// Encourage reading actual results (positive bias)
	bias[5263] = 10.0  // "returned"
	bias[22217] = 10.0 // "received"
	bias[37373] = 10.0 // "actual"
	bias[2427] = 10.0  // "shows"
	bias[5039] = 10.0  // "indicates"

	return bias
}

// GetToolResultValidationBias returns bias specifically for after tool execution
// This is applied when the agent should be reading tool results, not fabricating
func GetToolResultValidationBias() map[int]float32 {
	bias := GetAntiHallucinationBias()

	// Extra penalties after tool calls
	// We REALLY don't want fabrication here

	// Heavily penalize starting any code blocks
	bias[13249] = -80.0 // ``` (even stronger)

	// Penalize modal language (should/would/will)
	bias[8005] = -50.0 // "should"
	bias[1053] = -50.0 // "would"
	bias[3685] = -50.0 // "will"

	// Encourage factual reporting
	bias[791] = 20.0   // "The"
	bias[2122] = 20.0  // "returned" (boost further)
	bias[6052] = 20.0  // "failed"
	bias[23130] = 20.0 // "succeeded"

	return bias
}

// Note on Token IDs:
// These token IDs are approximations based on Llama 3.x vocabulary.
// For production use, you should:
// 1. Get actual token IDs from the model's tokenizer
// 2. Test and tune bias values per model
// 3. Consider using string-based bias if your LLM API supports it
//
// Alternative approach: Use grammar constraints to force structured output
// See pkg/llm/interface.go Grammar field for GBNF grammar support
