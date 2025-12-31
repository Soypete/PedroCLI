package logits

import (
	"testing"
)

func TestDefaultSamplerConfig(t *testing.T) {
	cfg := DefaultSamplerConfig()

	if cfg.Temperature != 0.7 {
		t.Errorf("expected default temperature 0.7, got %f", cfg.Temperature)
	}
	if cfg.TopK != 40 {
		t.Errorf("expected default top_k 40, got %d", cfg.TopK)
	}
	if cfg.TopP != 0.95 {
		t.Errorf("expected default top_p 0.95, got %f", cfg.TopP)
	}
	if cfg.MaxTokens != 2048 {
		t.Errorf("expected default max_tokens 2048, got %d", cfg.MaxTokens)
	}
}

func TestSamplerConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *SamplerConfig
		wantErr bool
	}{
		{
			name:    "valid default",
			config:  DefaultSamplerConfig(),
			wantErr: false,
		},
		{
			name: "negative temperature",
			config: &SamplerConfig{
				Temperature: -1.0,
			},
			wantErr: true,
		},
		{
			name: "negative top_k",
			config: &SamplerConfig{
				TopK: -1,
			},
			wantErr: true,
		},
		{
			name: "top_p out of range",
			config: &SamplerConfig{
				TopP: 1.5,
			},
			wantErr: true,
		},
		{
			name: "invalid mirostat",
			config: &SamplerConfig{
				Mirostat: 3,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestSamplerConfigClone(t *testing.T) {
	cfg := DefaultSamplerConfig()
	cfg.LogitBias = map[int]float32{1: 5.0, 2: -3.0}
	cfg.StopSequences = []string{"STOP", "END"}
	cfg.SamplerOrder = []string{"temp", "top_k"}

	clone := cfg.Clone()

	// Verify values are copied
	if clone.Temperature != cfg.Temperature {
		t.Error("temperature not cloned")
	}
	if clone.TopK != cfg.TopK {
		t.Error("top_k not cloned")
	}

	// Verify deep copy of maps
	clone.LogitBias[1] = 10.0
	if cfg.LogitBias[1] == 10.0 {
		t.Error("logit_bias not deep copied")
	}

	// Verify deep copy of slices
	clone.StopSequences[0] = "MODIFIED"
	if cfg.StopSequences[0] == "MODIFIED" {
		t.Error("stop_sequences not deep copied")
	}
}

func TestSamplerConfigMergeLogitBias(t *testing.T) {
	cfg := &SamplerConfig{}
	cfg.MergeLogitBias(map[int]float32{1: 5.0})

	if cfg.LogitBias[1] != 5.0 {
		t.Errorf("expected bias 5.0 for token 1, got %f", cfg.LogitBias[1])
	}

	// Merge again - should add
	cfg.MergeLogitBias(map[int]float32{1: 3.0, 2: 2.0})

	if cfg.LogitBias[1] != 8.0 {
		t.Errorf("expected merged bias 8.0 for token 1, got %f", cfg.LogitBias[1])
	}
	if cfg.LogitBias[2] != 2.0 {
		t.Errorf("expected bias 2.0 for token 2, got %f", cfg.LogitBias[2])
	}
}

func TestSamplerConfigToLlamaServerParams(t *testing.T) {
	cfg := &SamplerConfig{
		Temperature:       0.5,
		TopK:              50,
		TopP:              0.9,
		MinP:              0.05,
		RepetitionPenalty: 1.1,
		RepetitionWindow:  64,
		MaxTokens:         1024,
		Mirostat:          1,
		MirostatTau:       5.0,
		MirostatEta:       0.1,
		Seed:              42,
		StopSequences:     []string{"STOP"},
		LogitBias:         map[int]float32{1: 5.0},
		SamplerOrder:      []string{"temp", "top_k"},
	}

	params := cfg.ToLlamaServerParams()

	if params["temperature"] != float32(0.5) {
		t.Errorf("unexpected temperature: %v", params["temperature"])
	}
	if params["top_k"] != 50 {
		t.Errorf("unexpected top_k: %v", params["top_k"])
	}
	if params["mirostat"] != 1 {
		t.Errorf("unexpected mirostat: %v", params["mirostat"])
	}
	if params["seed"] != int64(42) {
		t.Errorf("unexpected seed: %v", params["seed"])
	}
	if params["stop"] == nil {
		t.Error("expected stop sequences")
	}
	if params["logit_bias"] == nil {
		t.Error("expected logit_bias")
	}
	if params["samplers"] == nil {
		t.Error("expected samplers order")
	}
}

func TestGetPreset(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"tool_call", true},
		{"json_strict", true},
		{"creative", true},
		{"deterministic", true},
		{"nonexistent", false},
	}

	for _, tc := range tests {
		preset := GetPreset(tc.name)
		if (preset != nil) != tc.expected {
			t.Errorf("GetPreset(%q) = %v, expected exists = %v", tc.name, preset, tc.expected)
		}
	}
}

func TestPresetClone(t *testing.T) {
	preset := GetPreset("tool_call")
	if preset == nil {
		t.Fatal("expected tool_call preset")
	}

	clone := preset.Clone()

	// Modifying clone should not affect registry
	clone.Description = "modified"

	original := GetPreset("tool_call")
	if original.Description == "modified" {
		t.Error("modifying clone should not affect original preset")
	}
}

func TestRegisterPreset(t *testing.T) {
	preset := &GenerationPreset{
		Name:        "test_preset",
		Description: "Test preset",
		Config:      DefaultSamplerConfig(),
	}

	RegisterPreset(preset)

	got := GetPreset("test_preset")
	if got == nil {
		t.Fatal("expected registered preset")
	}
	if got.Description != "Test preset" {
		t.Errorf("unexpected description: %q", got.Description)
	}

	// Cleanup
	delete(Presets, "test_preset")
}

func TestListPresets(t *testing.T) {
	names := ListPresets()

	if len(names) < 5 {
		t.Errorf("expected at least 5 presets, got %d", len(names))
	}

	// Check for expected presets
	found := make(map[string]bool)
	for _, name := range names {
		found[name] = true
	}

	expected := []string{"tool_call", "json_strict", "creative", "deterministic"}
	for _, name := range expected {
		if !found[name] {
			t.Errorf("expected preset %q not found", name)
		}
	}
}

func TestPresetBuilder(t *testing.T) {
	preset := NewPresetBuilder("test").
		Description("Test preset").
		Temperature(0.5).
		TopK(30).
		TopP(0.8).
		MinP(0.1).
		RepetitionPenalty(1.2).
		MaxTokens(512).
		Grammar("root ::= \"test\"").
		SafetyPreset("standard").
		AddFilter("safety", map[string]interface{}{"categories": []string{"code_injection"}}).
		Build()

	if preset.Name != "test" {
		t.Errorf("unexpected name: %q", preset.Name)
	}
	if preset.Description != "Test preset" {
		t.Errorf("unexpected description: %q", preset.Description)
	}
	if preset.Config.Temperature != 0.5 {
		t.Errorf("unexpected temperature: %f", preset.Config.Temperature)
	}
	if preset.Config.TopK != 30 {
		t.Errorf("unexpected top_k: %d", preset.Config.TopK)
	}
	if preset.Grammar != "root ::= \"test\"" {
		t.Errorf("unexpected grammar: %q", preset.Grammar)
	}
	if preset.SafetyPreset != "standard" {
		t.Errorf("unexpected safety preset: %q", preset.SafetyPreset)
	}
	if len(preset.FilterConfigs) != 1 {
		t.Errorf("expected 1 filter config, got %d", len(preset.FilterConfigs))
	}
}

func TestPresetBuilderBuildAndRegister(t *testing.T) {
	preset := NewPresetBuilder("builder_test").
		Description("Builder test").
		BuildAndRegister()

	got := GetPreset("builder_test")
	if got == nil {
		t.Fatal("expected registered preset")
	}
	// GetPreset returns a clone, so check by name instead
	if got.Name != preset.Name {
		t.Error("expected preset with same name")
	}
	if got.Description != preset.Description {
		t.Error("expected preset with same description")
	}

	// Cleanup
	delete(Presets, "builder_test")
}

func TestPredefinedConfigs(t *testing.T) {
	configs := []*SamplerConfig{
		StructuredOutputConfig,
		DeterministicConfig,
		CreativeConfig,
		CodeGenerationConfig,
		ChatConfig,
	}

	for i, cfg := range configs {
		err := cfg.Validate()
		if err != nil {
			t.Errorf("predefined config %d validation failed: %v", i, err)
		}
	}
}

func TestStructuredOutputConfig(t *testing.T) {
	if StructuredOutputConfig.Temperature > 0.2 {
		t.Error("structured output should have low temperature")
	}
}

func TestDeterministicConfig(t *testing.T) {
	if DeterministicConfig.Temperature != 0 {
		t.Error("deterministic config should have temperature 0")
	}
	if DeterministicConfig.TopK != 1 {
		t.Error("deterministic config should have top_k 1")
	}
}

func TestCreativeConfig(t *testing.T) {
	if CreativeConfig.Temperature < 0.7 {
		t.Error("creative config should have higher temperature")
	}
}
