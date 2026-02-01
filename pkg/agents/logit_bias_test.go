package agents

import (
	"testing"
)

// MockTokenIDProvider provides mock token IDs for testing
type MockTokenIDProvider struct {
	tokenMap map[string][]int
}

func NewMockTokenIDProvider() *MockTokenIDProvider {
	// Use simple sequential IDs for testing
	return &MockTokenIDProvider{
		tokenMap: map[string][]int{
			"```":       {1000},
			"json":      {1001},
			"Tool":      {1002},
			"Result":    {1003},
			"Output":    {1004},
			"Let's":     {1005},
			"should":    {1006},
			"would":     {1007},
			"will":      {1008},
			"expected":  {1009},
			"✓":         {1010},
			"✗":         {1011},
			"PASS":      {1012},
			"FAIL":      {1013},
			"returned":  {1014},
			"received":  {1015},
			"actual":    {1016},
			"shows":     {1017},
			"indicates": {1018},
			"The":       {1019},
			"failed":    {1020},
			"succeeded": {1021},
		},
	}
}

func (p *MockTokenIDProvider) GetTokenIDs(phrases []string) (map[string][]int, error) {
	result := make(map[string][]int)
	for _, phrase := range phrases {
		if ids, ok := p.tokenMap[phrase]; ok {
			result[phrase] = ids
		}
	}
	return result, nil
}

func (p *MockTokenIDProvider) GetSingleTokenID(phrase string) (int, error) {
	ids, err := p.GetTokenIDs([]string{phrase})
	if err != nil || len(ids[phrase]) == 0 {
		return 0, err
	}
	return ids[phrase][0], nil
}

func TestGetAntiHallucinationBias(t *testing.T) {
	provider := NewMockTokenIDProvider()
	bias := GetAntiHallucinationBias(provider)

	// Check that bias is applied for known patterns
	tests := []struct {
		phrase   string
		tokenID  int
		expected float32
	}{
		{"```", 1000, -50.0},
		{"json", 1001, -30.0},
		{"Tool", 1002, -40.0},
		{"Result", 1003, -40.0},
		{"Output", 1004, -40.0},
		{"Let's", 1005, -20.0},
		{"should", 1006, -20.0},
		{"returned", 1014, 10.0}, // Positive bias
		{"received", 1015, 10.0}, // Positive bias
		{"actual", 1016, 10.0},   // Positive bias
	}

	for _, tt := range tests {
		t.Run(tt.phrase, func(t *testing.T) {
			biasValue, ok := bias[tt.tokenID]
			if !ok {
				t.Errorf("Expected bias for token ID %d (phrase: %s)", tt.tokenID, tt.phrase)
				return
			}
			if biasValue != tt.expected {
				t.Errorf("Expected bias value %v for %s, got %v", tt.expected, tt.phrase, biasValue)
			}
		})
	}
}

func TestGetAntiHallucinationBias_EmptyProvider(t *testing.T) {
	// Test with null provider (returns empty map)
	provider := NewNullTokenIDProvider()
	bias := GetAntiHallucinationBias(provider)

	if len(bias) != 0 {
		t.Errorf("Expected empty bias map with null provider, got %d entries", len(bias))
	}
}

func TestGetToolResultValidationBias(t *testing.T) {
	provider := NewMockTokenIDProvider()
	bias := GetToolResultValidationBias(provider)

	// Check that stronger penalties are applied
	tests := []struct {
		phrase   string
		tokenID  int
		expected float32
		reason   string
	}{
		{"```", 1000, -80.0, "should have stronger penalty"},
		{"should", 1006, -50.0, "should have stronger penalty"},
		{"would", 1007, -50.0, "should have stronger penalty"},
		{"will", 1008, -50.0, "should have stronger penalty"},
		{"The", 1019, 20.0, "should encourage factual reporting"},
		{"returned", 1014, 20.0, "should boost further"},
		{"failed", 1020, 20.0, "should encourage factual reporting"},
		{"succeeded", 1021, 20.0, "should encourage factual reporting"},
	}

	for _, tt := range tests {
		t.Run(tt.phrase, func(t *testing.T) {
			biasValue, ok := bias[tt.tokenID]
			if !ok {
				t.Errorf("Expected bias for token ID %d (phrase: %s)", tt.tokenID, tt.phrase)
				return
			}
			if biasValue != tt.expected {
				t.Errorf("Expected bias value %v for %s (%s), got %v",
					tt.expected, tt.phrase, tt.reason, biasValue)
			}
		})
	}
}

func TestGetToolResultValidationBias_OverridesBase(t *testing.T) {
	provider := NewMockTokenIDProvider()
	baseBias := GetAntiHallucinationBias(provider)
	validationBias := GetToolResultValidationBias(provider)

	// Check that validation bias overrides base bias values
	// "```" should be -80.0 in validation vs -50.0 in base
	if baseBias[1000] != -50.0 {
		t.Errorf("Expected base bias for ``` to be -50.0, got %v", baseBias[1000])
	}
	if validationBias[1000] != -80.0 {
		t.Errorf("Expected validation bias for ``` to be -80.0, got %v", validationBias[1000])
	}

	// "should" should be -50.0 in validation vs -20.0 in base
	if baseBias[1006] != -20.0 {
		t.Errorf("Expected base bias for should to be -20.0, got %v", baseBias[1006])
	}
	if validationBias[1006] != -50.0 {
		t.Errorf("Expected validation bias for should to be -50.0, got %v", validationBias[1006])
	}
}

func TestBiasPattern(t *testing.T) {
	// Test BiasPattern struct
	pattern := BiasPattern{
		Phrase: "test",
		Bias:   -10.0,
	}

	if pattern.Phrase != "test" {
		t.Errorf("Expected phrase 'test', got '%s'", pattern.Phrase)
	}
	if pattern.Bias != -10.0 {
		t.Errorf("Expected bias -10.0, got %v", pattern.Bias)
	}
}

func TestStaticTokenIDProvider(t *testing.T) {
	// Test static provider with Llama 3 tokens
	provider := NewStaticTokenIDProvider("llama3")

	phrases := []string{"```", "json", "Tool"}
	ids, err := provider.GetTokenIDs(phrases)
	if err != nil {
		t.Fatalf("GetTokenIDs failed: %v", err)
	}

	// Check that we got IDs for known phrases
	if len(ids["```"]) == 0 {
		t.Error("Expected token IDs for ``` in llama3 map")
	}
	if len(ids["json"]) == 0 {
		t.Error("Expected token IDs for json in llama3 map")
	}

	// Check single token ID
	id, err := provider.GetSingleTokenID("```")
	if err != nil {
		t.Fatalf("GetSingleTokenID failed: %v", err)
	}
	if id == 0 {
		t.Error("Expected non-zero token ID for ```")
	}
}

func TestStaticTokenIDProvider_UnknownModel(t *testing.T) {
	// Test static provider with unknown model (should return empty map)
	provider := NewStaticTokenIDProvider("unknown-model")

	phrases := []string{"```", "json"}
	ids, err := provider.GetTokenIDs(phrases)
	if err != nil {
		t.Fatalf("GetTokenIDs failed: %v", err)
	}

	// Should return empty results for unknown model
	if len(ids) != 0 {
		t.Errorf("Expected empty results for unknown model, got %d entries", len(ids))
	}
}

func TestNullTokenIDProvider(t *testing.T) {
	provider := NewNullTokenIDProvider()

	// Should always return empty
	phrases := []string{"```", "json", "test"}
	ids, err := provider.GetTokenIDs(phrases)
	if err != nil {
		t.Fatalf("GetTokenIDs failed: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("Expected empty map from null provider, got %d entries", len(ids))
	}

	// GetSingleTokenID should return error
	_, err = provider.GetSingleTokenID("test")
	if err == nil {
		t.Error("Expected error from GetSingleTokenID on null provider")
	}
}
