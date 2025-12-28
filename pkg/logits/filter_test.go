package logits

import (
	"math"
	"testing"
)

func TestNewGenerationContext(t *testing.T) {
	ctx := NewGenerationContext("test prompt")

	if ctx.Prompt != "test prompt" {
		t.Errorf("expected prompt 'test prompt', got %q", ctx.Prompt)
	}
	if len(ctx.GeneratedTokens) != 0 {
		t.Errorf("expected empty tokens, got %d", len(ctx.GeneratedTokens))
	}
	if ctx.GeneratedText != "" {
		t.Errorf("expected empty text, got %q", ctx.GeneratedText)
	}
	if ctx.CurrentPosition != 0 {
		t.Errorf("expected position 0, got %d", ctx.CurrentPosition)
	}
}

func TestGenerationContextAppendToken(t *testing.T) {
	ctx := NewGenerationContext("test")

	ctx.AppendToken(1, "hello")
	ctx.AppendToken(2, " world")

	if len(ctx.GeneratedTokens) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(ctx.GeneratedTokens))
	}
	if ctx.GeneratedText != "hello world" {
		t.Errorf("expected 'hello world', got %q", ctx.GeneratedText)
	}
	if ctx.CurrentPosition != 2 {
		t.Errorf("expected position 2, got %d", ctx.CurrentPosition)
	}
}

func TestGenerationContextReset(t *testing.T) {
	ctx := NewGenerationContext("test")
	ctx.AppendToken(1, "hello")
	ctx.Metadata["key"] = "value"

	ctx.Reset()

	if len(ctx.GeneratedTokens) != 0 {
		t.Errorf("expected empty tokens after reset")
	}
	if ctx.GeneratedText != "" {
		t.Errorf("expected empty text after reset")
	}
	if ctx.CurrentPosition != 0 {
		t.Errorf("expected position 0 after reset")
	}
	if len(ctx.Metadata) != 0 {
		t.Errorf("expected empty metadata after reset")
	}
}

func TestTokenBanFilter(t *testing.T) {
	filter := NewTokenBanFilter("test", "test filter", []int{1, 2, 3})

	if filter.Name() != "test" {
		t.Errorf("expected name 'test', got %q", filter.Name())
	}

	logits := make([]float32, 10)
	for i := range logits {
		logits[i] = 1.0
	}

	ctx := NewGenerationContext("test")
	result := filter.Apply(logits, ctx)

	// Check that banned tokens are -inf
	if !math.IsInf(float64(result[1]), -1) {
		t.Errorf("expected token 1 to be -inf, got %f", result[1])
	}
	if !math.IsInf(float64(result[2]), -1) {
		t.Errorf("expected token 2 to be -inf, got %f", result[2])
	}
	if !math.IsInf(float64(result[3]), -1) {
		t.Errorf("expected token 3 to be -inf, got %f", result[3])
	}

	// Check that other tokens are unchanged
	if result[0] != 1.0 {
		t.Errorf("expected token 0 to be 1.0, got %f", result[0])
	}
	if result[4] != 1.0 {
		t.Errorf("expected token 4 to be 1.0, got %f", result[4])
	}
}

func TestTokenBanFilterDisabled(t *testing.T) {
	filter := NewTokenBanFilter("test", "test filter", []int{1, 2, 3})
	filter.SetEnabled(false)

	logits := make([]float32, 10)
	for i := range logits {
		logits[i] = 1.0
	}

	ctx := NewGenerationContext("test")
	result := filter.Apply(logits, ctx)

	// All tokens should be unchanged when disabled
	for i := range result {
		if result[i] != 1.0 {
			t.Errorf("expected token %d to be 1.0 when disabled, got %f", i, result[i])
		}
	}
}

func TestLogitBiasFilter(t *testing.T) {
	biases := map[int]float32{
		1: 5.0,
		2: -3.0,
	}
	filter := NewLogitBiasFilter(biases)

	logits := make([]float32, 10)
	for i := range logits {
		logits[i] = 1.0
	}

	ctx := NewGenerationContext("test")
	result := filter.Apply(logits, ctx)

	if result[1] != 6.0 {
		t.Errorf("expected token 1 to be 6.0, got %f", result[1])
	}
	if result[2] != -2.0 {
		t.Errorf("expected token 2 to be -2.0, got %f", result[2])
	}
	if result[0] != 1.0 {
		t.Errorf("expected token 0 to be 1.0, got %f", result[0])
	}
}

func TestTokenBanFilterAddRemove(t *testing.T) {
	filter := NewTokenBanFilter("test", "test filter", []int{1})

	// Add a token
	filter.AddBannedToken(2)

	logits := make([]float32, 10)
	for i := range logits {
		logits[i] = 1.0
	}

	ctx := NewGenerationContext("test")
	result := filter.Apply(logits, ctx)

	if !math.IsInf(float64(result[2]), -1) {
		t.Errorf("expected newly banned token 2 to be -inf")
	}

	// Remove a token
	filter.RemoveBannedToken(1)

	for i := range logits {
		logits[i] = 1.0
	}
	result = filter.Apply(logits, ctx)

	if math.IsInf(float64(result[1]), -1) {
		t.Errorf("expected removed token 1 to not be -inf")
	}
}
