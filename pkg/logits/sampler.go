package logits

import (
	"encoding/json"
	"fmt"
)

// SamplerConfig configures the token sampling process.
// This is passed to the LLM backend to control generation behavior.
type SamplerConfig struct {
	// Temperature controls randomness (0.0 = deterministic, 1.0+ = more random)
	Temperature float32 `json:"temperature"`

	// TopK limits sampling to the K most likely tokens (0 = disabled)
	TopK int `json:"top_k"`

	// TopP (nucleus sampling) considers tokens until cumulative probability >= P
	TopP float32 `json:"top_p"`

	// MinP ignores tokens with probability < P * max_probability
	MinP float32 `json:"min_p"`

	// TypicalP enables locally typical sampling (0.0 = disabled)
	TypicalP float32 `json:"typical_p"`

	// RepetitionPenalty penalizes repeated tokens (1.0 = no penalty)
	RepetitionPenalty float32 `json:"repetition_penalty"`

	// RepetitionWindow is how many recent tokens to check for repetition
	RepetitionWindow int `json:"repetition_window"`

	// FrequencyPenalty reduces probability based on token frequency (0.0 = disabled)
	FrequencyPenalty float32 `json:"frequency_penalty"`

	// PresencePenalty reduces probability if token appeared at all (0.0 = disabled)
	PresencePenalty float32 `json:"presence_penalty"`

	// LogitBias applies per-token biases (token ID -> bias value)
	LogitBias map[int]float32 `json:"logit_bias,omitempty"`

	// SamplerOrder defines the order of sampling operations
	// Default: ["temperature", "top_k", "top_p", "min_p"]
	SamplerOrder []string `json:"sampler_order,omitempty"`

	// Mirostat enables Mirostat sampling (0 = disabled, 1 or 2 for versions)
	Mirostat int `json:"mirostat"`

	// MirostatTau is the target entropy for Mirostat
	MirostatTau float32 `json:"mirostat_tau"`

	// MirostatEta is the learning rate for Mirostat
	MirostatEta float32 `json:"mirostat_eta"`

	// Seed for reproducible generation (-1 = random)
	Seed int64 `json:"seed"`

	// MaxTokens is the maximum number of tokens to generate
	MaxTokens int `json:"max_tokens"`

	// StopSequences are strings that stop generation when produced
	StopSequences []string `json:"stop,omitempty"`
}

// DefaultSamplerConfig returns a sensible default configuration.
func DefaultSamplerConfig() *SamplerConfig {
	return &SamplerConfig{
		Temperature:       0.7,
		TopK:              40,
		TopP:              0.95,
		MinP:              0.05,
		RepetitionPenalty: 1.1,
		RepetitionWindow:  64,
		MaxTokens:         2048,
		Seed:              -1,
		SamplerOrder:      []string{"temperature", "top_k", "top_p", "min_p"},
	}
}

// Validate checks if the config values are valid.
func (c *SamplerConfig) Validate() error {
	if c.Temperature < 0 {
		return fmt.Errorf("temperature must be >= 0")
	}
	if c.TopK < 0 {
		return fmt.Errorf("top_k must be >= 0")
	}
	if c.TopP < 0 || c.TopP > 1 {
		return fmt.Errorf("top_p must be between 0 and 1")
	}
	if c.MinP < 0 || c.MinP > 1 {
		return fmt.Errorf("min_p must be between 0 and 1")
	}
	if c.RepetitionPenalty < 0 {
		return fmt.Errorf("repetition_penalty must be >= 0")
	}
	if c.Mirostat < 0 || c.Mirostat > 2 {
		return fmt.Errorf("mirostat must be 0, 1, or 2")
	}
	return nil
}

// Clone creates a copy of the config.
func (c *SamplerConfig) Clone() *SamplerConfig {
	clone := *c

	// Deep copy maps
	if c.LogitBias != nil {
		clone.LogitBias = make(map[int]float32, len(c.LogitBias))
		for k, v := range c.LogitBias {
			clone.LogitBias[k] = v
		}
	}

	// Deep copy slices
	if c.SamplerOrder != nil {
		clone.SamplerOrder = make([]string, len(c.SamplerOrder))
		copy(clone.SamplerOrder, c.SamplerOrder)
	}

	if c.StopSequences != nil {
		clone.StopSequences = make([]string, len(c.StopSequences))
		copy(clone.StopSequences, c.StopSequences)
	}

	return &clone
}

// MergeLogitBias adds biases from a filter to the config.
func (c *SamplerConfig) MergeLogitBias(biases map[int]float32) {
	if c.LogitBias == nil {
		c.LogitBias = make(map[int]float32)
	}
	for tokenID, bias := range biases {
		c.LogitBias[tokenID] += bias
	}
}

// ToJSON serializes the config to JSON.
func (c *SamplerConfig) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// ToLlamaServerParams converts to llama-server compatible parameters.
func (c *SamplerConfig) ToLlamaServerParams() map[string]interface{} {
	params := map[string]interface{}{
		"temperature":       c.Temperature,
		"top_k":             c.TopK,
		"top_p":             c.TopP,
		"min_p":             c.MinP,
		"repeat_penalty":    c.RepetitionPenalty,
		"repeat_last_n":     c.RepetitionWindow,
		"frequency_penalty": c.FrequencyPenalty,
		"presence_penalty":  c.PresencePenalty,
		"n_predict":         c.MaxTokens,
	}

	if c.Mirostat > 0 {
		params["mirostat"] = c.Mirostat
		params["mirostat_tau"] = c.MirostatTau
		params["mirostat_eta"] = c.MirostatEta
	}

	if c.Seed >= 0 {
		params["seed"] = c.Seed
	}

	if len(c.StopSequences) > 0 {
		params["stop"] = c.StopSequences
	}

	if len(c.LogitBias) > 0 {
		// Convert to format expected by llama-server
		biasArray := make([][]interface{}, 0, len(c.LogitBias))
		for tokenID, bias := range c.LogitBias {
			biasArray = append(biasArray, []interface{}{tokenID, bias})
		}
		params["logit_bias"] = biasArray
	}

	if len(c.SamplerOrder) > 0 {
		params["samplers"] = c.SamplerOrder
	}

	return params
}

// GenerationPreset combines sampler config with filters and metadata.
type GenerationPreset struct {
	// Name is the preset identifier
	Name string `json:"name"`

	// Description describes the preset's purpose
	Description string `json:"description"`

	// Config is the sampler configuration
	Config *SamplerConfig `json:"config"`

	// FilterConfigs describes filters to apply
	// Actual filter instances are created at runtime
	FilterConfigs []FilterConfig `json:"filters,omitempty"`

	// Grammar is an optional GBNF grammar to enforce
	Grammar string `json:"grammar,omitempty"`

	// JSONSchema is an optional JSON schema to enforce
	JSONSchema string `json:"json_schema,omitempty"`

	// SafetyPreset is the safety preset to apply
	SafetyPreset string `json:"safety_preset,omitempty"`
}

// FilterConfig describes a filter to be created.
type FilterConfig struct {
	Type    string                 `json:"type"`
	Enabled bool                   `json:"enabled"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// Clone creates a copy of the preset.
func (p *GenerationPreset) Clone() *GenerationPreset {
	clone := *p
	clone.Config = p.Config.Clone()

	if p.FilterConfigs != nil {
		clone.FilterConfigs = make([]FilterConfig, len(p.FilterConfigs))
		copy(clone.FilterConfigs, p.FilterConfigs)
	}

	return &clone
}

// Predefined sampler configurations

// StructuredOutputConfig is optimized for structured output generation.
var StructuredOutputConfig = &SamplerConfig{
	Temperature:       0.1,
	TopK:              40,
	TopP:              0.9,
	MinP:              0.05,
	RepetitionPenalty: 1.0,
	MaxTokens:         2048,
	SamplerOrder:      []string{"top_k", "top_p", "temperature"},
}

// DeterministicConfig produces deterministic output.
var DeterministicConfig = &SamplerConfig{
	Temperature:       0.0,
	TopK:              1,
	TopP:              1.0,
	RepetitionPenalty: 1.0,
	MaxTokens:         2048,
}

// CreativeConfig allows more creative/varied output.
var CreativeConfig = &SamplerConfig{
	Temperature:       0.8,
	TopK:              100,
	TopP:              0.95,
	MinP:              0.02,
	RepetitionPenalty: 1.15,
	RepetitionWindow:  128,
	MaxTokens:         4096,
}

// CodeGenerationConfig is tuned for code generation.
var CodeGenerationConfig = &SamplerConfig{
	Temperature:       0.2,
	TopK:              50,
	TopP:              0.9,
	MinP:              0.05,
	RepetitionPenalty: 1.1,
	RepetitionWindow:  64,
	MaxTokens:         4096,
}

// ChatConfig is tuned for conversational responses.
var ChatConfig = &SamplerConfig{
	Temperature:       0.7,
	TopK:              40,
	TopP:              0.95,
	MinP:              0.05,
	RepetitionPenalty: 1.1,
	RepetitionWindow:  64,
	MaxTokens:         1024,
}

// Presets is a registry of predefined generation presets.
var Presets = map[string]*GenerationPreset{
	"tool_call": {
		Name:        "tool_call",
		Description: "Structured tool calls with guaranteed format",
		Config:      DeterministicConfig.Clone(),
		Grammar:     ToolCallGrammar,
		FilterConfigs: []FilterConfig{
			{Type: "tool_call", Enabled: true},
		},
	},

	"json_strict": {
		Name:        "json_strict",
		Description: "Strict JSON output matching a schema",
		Config:      StructuredOutputConfig.Clone(),
		Grammar:     JSONObjectGrammar,
		FilterConfigs: []FilterConfig{
			{Type: "json_object", Enabled: true},
		},
	},

	"json_loose": {
		Name:        "json_loose",
		Description: "JSON output with some flexibility",
		Config: func() *SamplerConfig {
			c := StructuredOutputConfig.Clone()
			c.Temperature = 0.3
			return c
		}(),
		Grammar: JSONGrammar,
	},

	"safe_chat": {
		Name:         "safe_chat",
		Description:  "Chat responses with safety filtering",
		Config:       ChatConfig.Clone(),
		SafetyPreset: "standard",
		FilterConfigs: []FilterConfig{
			{Type: "safety", Enabled: true, Options: map[string]interface{}{
				"categories": []string{"profanity", "violence"},
			}},
		},
	},

	"code_generation": {
		Name:         "code_generation",
		Description:  "Code generation with injection protection",
		Config:       CodeGenerationConfig.Clone(),
		SafetyPreset: "minimal",
		FilterConfigs: []FilterConfig{
			{Type: "safety", Enabled: true, Options: map[string]interface{}{
				"categories": []string{"code_injection"},
			}},
		},
	},

	"creative": {
		Name:        "creative",
		Description: "Creative text generation with minimal constraints",
		Config:      CreativeConfig.Clone(),
	},

	"deterministic": {
		Name:        "deterministic",
		Description: "Fully deterministic output",
		Config:      DeterministicConfig.Clone(),
	},
}

// GetPreset returns a preset by name, or nil if not found.
func GetPreset(name string) *GenerationPreset {
	if preset, ok := Presets[name]; ok {
		return preset.Clone()
	}
	return nil
}

// RegisterPreset adds a custom preset.
func RegisterPreset(preset *GenerationPreset) {
	Presets[preset.Name] = preset
}

// ListPresets returns all available preset names.
func ListPresets() []string {
	names := make([]string, 0, len(Presets))
	for name := range Presets {
		names = append(names, name)
	}
	return names
}

// PresetBuilder provides a fluent interface for creating presets.
type PresetBuilder struct {
	preset *GenerationPreset
}

// NewPresetBuilder starts building a new preset.
func NewPresetBuilder(name string) *PresetBuilder {
	return &PresetBuilder{
		preset: &GenerationPreset{
			Name:   name,
			Config: DefaultSamplerConfig(),
		},
	}
}

// Description sets the preset description.
func (b *PresetBuilder) Description(desc string) *PresetBuilder {
	b.preset.Description = desc
	return b
}

// Temperature sets the temperature.
func (b *PresetBuilder) Temperature(t float32) *PresetBuilder {
	b.preset.Config.Temperature = t
	return b
}

// TopK sets top-k.
func (b *PresetBuilder) TopK(k int) *PresetBuilder {
	b.preset.Config.TopK = k
	return b
}

// TopP sets top-p.
func (b *PresetBuilder) TopP(p float32) *PresetBuilder {
	b.preset.Config.TopP = p
	return b
}

// MinP sets min-p.
func (b *PresetBuilder) MinP(p float32) *PresetBuilder {
	b.preset.Config.MinP = p
	return b
}

// RepetitionPenalty sets repetition penalty.
func (b *PresetBuilder) RepetitionPenalty(p float32) *PresetBuilder {
	b.preset.Config.RepetitionPenalty = p
	return b
}

// MaxTokens sets max tokens.
func (b *PresetBuilder) MaxTokens(n int) *PresetBuilder {
	b.preset.Config.MaxTokens = n
	return b
}

// Grammar sets the GBNF grammar.
func (b *PresetBuilder) Grammar(grammar string) *PresetBuilder {
	b.preset.Grammar = grammar
	return b
}

// JSONSchema sets the JSON schema.
func (b *PresetBuilder) JSONSchema(schema string) *PresetBuilder {
	b.preset.JSONSchema = schema
	return b
}

// SafetyPreset sets the safety preset.
func (b *PresetBuilder) SafetyPreset(preset string) *PresetBuilder {
	b.preset.SafetyPreset = preset
	return b
}

// AddFilter adds a filter configuration.
func (b *PresetBuilder) AddFilter(filterType string, options map[string]interface{}) *PresetBuilder {
	b.preset.FilterConfigs = append(b.preset.FilterConfigs, FilterConfig{
		Type:    filterType,
		Enabled: true,
		Options: options,
	})
	return b
}

// Build returns the constructed preset.
func (b *PresetBuilder) Build() *GenerationPreset {
	return b.preset
}

// BuildAndRegister builds and registers the preset.
func (b *PresetBuilder) BuildAndRegister() *GenerationPreset {
	RegisterPreset(b.preset)
	return b.preset
}
