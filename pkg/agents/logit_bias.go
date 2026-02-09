package agents

// BiasPattern represents a phrase to bias and its bias value
type BiasPattern struct {
	Phrase string
	Bias   float32
}

// GetAntiHallucinationBias returns logit bias to prevent tool result fabrication
// Uses dynamic token ID lookup instead of hardcoded values
func GetAntiHallucinationBias(provider TokenIDProvider) map[int]float32 {
	bias := make(map[int]float32)

	// Define bias patterns (phrase -> bias value)
	patterns := []BiasPattern{
		// Prevent starting fabricated JSON blocks
		{"```", -50.0},  // markdown code fence - heavily penalize
		{"json", -30.0}, // json keyword

		// Prevent writing fake tool results
		{"Tool", -40.0},
		{"Result", -40.0},
		{"Output", -40.0},

		// Prevent narrative mode phrases
		{"Let's", -20.0},
		{"should", -20.0},
		{"would", -20.0},
		{"will", -20.0},
		{"expected", -20.0},

		// Prevent fake success indicators
		{"✓", -30.0},
		{"✗", -30.0},
		{"PASS", -30.0},
		{"FAIL", -30.0},

		// Encourage reading actual results (positive bias)
		{"returned", 10.0},
		{"received", 10.0},
		{"actual", 10.0},
		{"shows", 10.0},
		{"indicates", 10.0},
	}

	// Get dynamic token IDs for all phrases
	phrases := make([]string, len(patterns))
	for i, p := range patterns {
		phrases[i] = p.Phrase
	}

	tokenIDs, err := provider.GetTokenIDs(phrases)
	if err != nil {
		// If tokenization fails, return empty bias
		// Empty bias is safer than wrong bias
		return bias
	}

	// Apply bias values to token IDs
	for _, pattern := range patterns {
		if ids, ok := tokenIDs[pattern.Phrase]; ok {
			for _, tokenID := range ids {
				bias[tokenID] = pattern.Bias
			}
		}
	}

	return bias
}

// GetToolResultValidationBias returns bias specifically for after tool execution
// This is applied when the agent should be reading tool results, not fabricating
func GetToolResultValidationBias(provider TokenIDProvider) map[int]float32 {
	// Start with base anti-hallucination bias
	bias := GetAntiHallucinationBias(provider)

	// Extra penalties/boosts for tool result validation
	extraPatterns := []BiasPattern{
		// Even stronger penalties for code blocks after tool execution
		{"```", -80.0}, // Override base -50.0 with -80.0

		// Stronger penalties for modal language
		{"should", -50.0}, // Override base -20.0
		{"would", -50.0},  // Override base -20.0
		{"will", -50.0},   // Override base -20.0

		// Encourage factual reporting
		{"The", 20.0},
		{"returned", 20.0}, // Boost further from base 10.0
		{"failed", 20.0},
		{"succeeded", 20.0},
	}

	// Get token IDs for extra patterns
	phrases := make([]string, len(extraPatterns))
	for i, p := range extraPatterns {
		phrases[i] = p.Phrase
	}

	tokenIDs, err := provider.GetTokenIDs(phrases)
	if err != nil {
		// If tokenization fails, return base bias
		return bias
	}

	// Apply extra bias values (overrides base values)
	for _, pattern := range extraPatterns {
		if ids, ok := tokenIDs[pattern.Phrase]; ok {
			for _, tokenID := range ids {
				bias[tokenID] = pattern.Bias
			}
		}
	}

	return bias
}

// GetMultiActionToolBias returns logit bias to encourage "action" parameter
// This helps LLMs correctly call multi-action tools with the required "action" parameter
func GetMultiActionToolBias(provider TokenIDProvider) map[int]float32 {
	bias := make(map[int]float32)

	patterns := []BiasPattern{
		// Strongly encourage "action" key in JSON
		{"\"action\"", 50.0},
		{"\"args\"", 20.0},

		// Penalize common mistakes
		{"\"type\"", -100.0},        // Wrong parameter name
		{"\"action_type\"", -100.0}, // Wrong parameter name
		{"\"command\"", -100.0},     // Wrong parameter name
	}

	// Get token IDs for patterns
	phrases := make([]string, len(patterns))
	for i, p := range patterns {
		phrases[i] = p.Phrase
	}

	tokenIDs, err := provider.GetTokenIDs(phrases)
	if err != nil {
		// If tokenization fails, return empty bias
		return bias
	}

	// Apply bias values to token IDs
	for _, pattern := range patterns {
		if ids, ok := tokenIDs[pattern.Phrase]; ok {
			for _, tokenID := range ids {
				bias[tokenID] = pattern.Bias
			}
		}
	}

	return bias
}

// Note on Dynamic Token IDs:
// This implementation uses dynamic token ID lookup via the LLM backend's /tokenize endpoint.
// Benefits:
// - Always uses the correct token IDs for the active model
// - No need to maintain hardcoded token maps for each model
// - Gracefully degrades to empty bias if tokenization fails (safer than wrong bias)
//
// Performance:
// - Results are cached per phrase, so tokenization only happens once per phrase
// - Typical overhead: <1 second at startup for ~20-30 unique phrases
//
// Fallback:
// - If backend doesn't support tokenization, StaticTokenIDProvider can be used
// - See pkg/agents/token_ids.go for static token maps
//
// Alternative approach: Use grammar constraints to force structured output
// See pkg/llm/interface.go Grammar field for GBNF grammar support
