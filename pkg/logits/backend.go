package logits

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LlamaBackend defines the interface for LLM backends that support logit control.
// This extends the basic inference capability with grammar and sampling control.
type LlamaBackend interface {
	// Generate performs generation with logit control.
	Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)

	// GenerateStream performs streaming generation with per-token callback.
	GenerateStream(ctx context.Context, req *GenerateRequest, onToken func(token string) bool) error

	// GetTokenizer returns the tokenizer for this backend.
	// May return nil if tokenizer is not available.
	GetTokenizer() Tokenizer

	// SupportsGrammar returns true if the backend supports GBNF grammar.
	SupportsGrammar() bool

	// SupportsLogitBias returns true if the backend supports logit bias.
	SupportsLogitBias() bool

	// Health checks if the backend is available.
	Health(ctx context.Context) error
}

// GenerateRequest is the request for generation with logit control.
type GenerateRequest struct {
	// Prompt is the input prompt
	Prompt string `json:"prompt"`

	// SystemPrompt is an optional system prompt
	SystemPrompt string `json:"system_prompt,omitempty"`

	// SamplerConfig controls sampling behavior
	SamplerConfig *SamplerConfig `json:"sampler_config,omitempty"`

	// Grammar is an optional GBNF grammar to enforce
	Grammar string `json:"grammar,omitempty"`

	// JSONSchema is an optional JSON schema to enforce (converted to grammar)
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`

	// Filters are logit filters to apply (client-side filtering)
	Filters *FilterChain `json:"-"`

	// Stream enables streaming mode
	Stream bool `json:"stream,omitempty"`
}

// GenerateResponse is the response from generation.
type GenerateResponse struct {
	// Text is the generated text
	Text string `json:"content"`

	// TokenCount is the number of tokens generated
	TokenCount int `json:"tokens_predicted,omitempty"`

	// PromptTokenCount is the number of tokens in the prompt
	PromptTokenCount int `json:"tokens_evaluated,omitempty"`

	// StopReason is why generation stopped
	StopReason string `json:"stop_reason,omitempty"`

	// TimingInfo contains timing information
	TimingInfo *TimingInfo `json:"timings,omitempty"`
}

// TimingInfo contains generation timing information.
type TimingInfo struct {
	PromptMs     float64 `json:"prompt_ms"`
	PredictedMs  float64 `json:"predicted_ms"`
	TokensPerSec float64 `json:"tokens_per_second"`
}

// LlamaHTTPBackend implements LlamaBackend using llama-server's HTTP API.
type LlamaHTTPBackend struct {
	baseURL    string
	client     *http.Client
	tokenizer  Tokenizer
	vocabPath  string
}

// NewLlamaHTTPBackend creates a new HTTP backend for llama-server.
func NewLlamaHTTPBackend(baseURL string) *LlamaHTTPBackend {
	return &LlamaHTTPBackend{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client: &http.Client{
			Timeout: 5 * time.Minute, // Long timeout for generation
		},
	}
}

// SetTokenizer sets a custom tokenizer.
func (b *LlamaHTTPBackend) SetTokenizer(tokenizer Tokenizer) {
	b.tokenizer = tokenizer
}

// LoadVocabulary loads vocabulary from a file for tokenization.
func (b *LlamaHTTPBackend) LoadVocabulary(vocabPath string) error {
	var err error
	if strings.HasSuffix(vocabPath, ".json") {
		b.tokenizer, err = LoadVocabFromJSON(vocabPath)
	} else {
		b.tokenizer, err = LoadVocabFromFile(vocabPath)
	}
	if err != nil {
		return fmt.Errorf("load vocabulary: %w", err)
	}
	b.vocabPath = vocabPath
	return nil
}

// Generate performs generation with the given request.
func (b *LlamaHTTPBackend) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	// Build request body
	body := b.buildRequestBody(req)

	// Make HTTP request
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/completion", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var genResp GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &genResp, nil
}

// GenerateStream performs streaming generation.
func (b *LlamaHTTPBackend) GenerateStream(ctx context.Context, req *GenerateRequest, onToken func(token string) bool) error {
	// Enable streaming
	req.Stream = true
	body := b.buildRequestBody(req)

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/completion", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error %d: %s", resp.StatusCode, string(body))
	}

	// Read streaming response
	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk struct {
			Content string `json:"content"`
			Stop    bool   `json:"stop"`
		}
		if err := decoder.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("decode chunk: %w", err)
		}

		if chunk.Content != "" {
			if !onToken(chunk.Content) {
				return nil // Callback requested stop
			}
		}

		if chunk.Stop {
			break
		}
	}

	return nil
}

// buildRequestBody builds the request body for llama-server.
func (b *LlamaHTTPBackend) buildRequestBody(req *GenerateRequest) map[string]interface{} {
	body := map[string]interface{}{
		"prompt": req.Prompt,
		"stream": req.Stream,
	}

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		// For chat-style models, prepend system prompt
		body["prompt"] = fmt.Sprintf("System: %s\n\nUser: %s\n\nAssistant: ",
			req.SystemPrompt, req.Prompt)
	}

	// Add sampler config
	if req.SamplerConfig != nil {
		params := req.SamplerConfig.ToLlamaServerParams()
		for k, v := range params {
			body[k] = v
		}
	}

	// Add grammar if provided
	if req.Grammar != "" {
		body["grammar"] = req.Grammar
	}

	// Convert JSON schema to grammar if provided
	if req.JSONSchema != nil && req.Grammar == "" {
		grammarStr, err := SchemaToGBNF(req.JSONSchema)
		if err == nil {
			body["grammar"] = grammarStr
		}
	}

	// Merge logit biases from filters
	if req.Filters != nil && req.SamplerConfig != nil {
		for _, filter := range req.Filters.Filters() {
			if sf, ok := filter.(*SafetyFilter); ok && sf.Enabled() {
				biases := sf.GetLogitBiases()
				if len(biases) > 0 {
					if req.SamplerConfig.LogitBias == nil {
						req.SamplerConfig.LogitBias = make(map[int]float32)
					}
					for tokenID, bias := range biases {
						req.SamplerConfig.LogitBias[tokenID] += bias
					}
					// Re-add to body
					params := req.SamplerConfig.ToLlamaServerParams()
					if lb, ok := params["logit_bias"]; ok {
						body["logit_bias"] = lb
					}
				}
			}
		}
	}

	return body
}

// GetTokenizer returns the tokenizer.
func (b *LlamaHTTPBackend) GetTokenizer() Tokenizer {
	return b.tokenizer
}

// SupportsGrammar returns true.
func (b *LlamaHTTPBackend) SupportsGrammar() bool {
	return true
}

// SupportsLogitBias returns true.
func (b *LlamaHTTPBackend) SupportsLogitBias() bool {
	return true
}

// Health checks if the server is available.
func (b *LlamaHTTPBackend) Health(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", b.baseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

// GenerateWithPreset generates using a named preset.
func (b *LlamaHTTPBackend) GenerateWithPreset(ctx context.Context, prompt string, presetName string) (*GenerateResponse, error) {
	preset := GetPreset(presetName)
	if preset == nil {
		return nil, fmt.Errorf("unknown preset: %s", presetName)
	}

	req := &GenerateRequest{
		Prompt:        prompt,
		SamplerConfig: preset.Config,
		Grammar:       preset.Grammar,
	}

	return b.Generate(ctx, req)
}

// GenerateStructured generates with a JSON schema constraint.
func (b *LlamaHTTPBackend) GenerateStructured(ctx context.Context, prompt string, schema *JSONSchema) (*GenerateResponse, error) {
	grammarStr, err := SchemaToGBNF(schema)
	if err != nil {
		return nil, fmt.Errorf("convert schema: %w", err)
	}

	req := &GenerateRequest{
		Prompt:        prompt,
		SamplerConfig: StructuredOutputConfig.Clone(),
		Grammar:       grammarStr,
	}

	return b.Generate(ctx, req)
}

// GenerateToolCall generates a tool call with the given tools.
func (b *LlamaHTTPBackend) GenerateToolCall(ctx context.Context, prompt string, tools []*ToolDefinition) (*ParsedToolCall, error) {
	// Build multi-tool schema
	schemas := make(map[string]*JSONSchema)
	for _, tool := range tools {
		schemas[tool.Name] = tool.Parameters
	}
	schema := MultiToolCallSchema(schemas)

	grammarStr, err := SchemaToGBNF(schema)
	if err != nil {
		return nil, fmt.Errorf("convert tool schema: %w", err)
	}

	req := &GenerateRequest{
		Prompt:        prompt,
		SamplerConfig: DeterministicConfig.Clone(),
		Grammar:       grammarStr,
	}

	resp, err := b.Generate(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse the tool call
	var toolCall ParsedToolCall
	if err := json.Unmarshal([]byte(resp.Text), &toolCall); err != nil {
		return nil, fmt.Errorf("parse tool call: %w", err)
	}

	return &toolCall, nil
}

// BackendConfig holds configuration for creating backends.
type BackendConfig struct {
	// Type is the backend type ("http", "command")
	Type string `json:"type"`

	// URL is the base URL for HTTP backends
	URL string `json:"url,omitempty"`

	// VocabPath is the path to vocabulary file
	VocabPath string `json:"vocab_path,omitempty"`

	// DefaultPreset is the default generation preset
	DefaultPreset string `json:"default_preset,omitempty"`

	// Timeout is the request timeout
	Timeout time.Duration `json:"timeout,omitempty"`
}

// NewBackend creates a backend from configuration.
func NewBackend(cfg *BackendConfig) (LlamaBackend, error) {
	switch cfg.Type {
	case "http", "llama-server":
		backend := NewLlamaHTTPBackend(cfg.URL)
		if cfg.VocabPath != "" {
			if err := backend.LoadVocabulary(cfg.VocabPath); err != nil {
				return nil, err
			}
		}
		if cfg.Timeout > 0 {
			backend.client.Timeout = cfg.Timeout
		}
		return backend, nil
	default:
		return nil, fmt.Errorf("unknown backend type: %s", cfg.Type)
	}
}
