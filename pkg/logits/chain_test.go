package logits

import (
	"math"
	"testing"
)

func TestFilterChainAdd(t *testing.T) {
	chain := NewFilterChain()

	filter1 := NewTokenBanFilter("filter1", "first", []int{1})
	filter2 := NewTokenBanFilter("filter2", "second", []int{2})

	chain.Add(filter1)
	chain.Add(filter2)

	if chain.Len() != 2 {
		t.Errorf("expected chain length 2, got %d", chain.Len())
	}
}

func TestFilterChainApply(t *testing.T) {
	chain := NewFilterChain()

	// Add filters that ban tokens 1 and 2
	chain.Add(NewTokenBanFilter("filter1", "first", []int{1}))
	chain.Add(NewTokenBanFilter("filter2", "second", []int{2}))

	logits := make([]float32, 10)
	for i := range logits {
		logits[i] = 1.0
	}

	ctx := NewGenerationContext("test")
	result := chain.Apply(logits, ctx)

	// Both tokens should be banned
	if !math.IsInf(float64(result[1]), -1) {
		t.Errorf("expected token 1 to be -inf")
	}
	if !math.IsInf(float64(result[2]), -1) {
		t.Errorf("expected token 2 to be -inf")
	}
	if result[0] != 1.0 {
		t.Errorf("expected token 0 to be unchanged")
	}
}

func TestFilterChainApplyWithResult(t *testing.T) {
	chain := NewFilterChain()
	chain.Add(NewTokenBanFilter("filter1", "first", []int{1, 2, 3}))

	logits := make([]float32, 10)
	for i := range logits {
		logits[i] = 1.0
	}

	ctx := NewGenerationContext("test")
	result := chain.ApplyWithResult(logits, ctx)

	if result.BannedTokenCount != 3 {
		t.Errorf("expected 3 banned tokens, got %d", result.BannedTokenCount)
	}
	if len(result.ActiveFilters) != 1 {
		t.Errorf("expected 1 active filter, got %d", len(result.ActiveFilters))
	}
	if result.ActiveFilters[0] != "filter1" {
		t.Errorf("expected filter name 'filter1', got %q", result.ActiveFilters[0])
	}
}

func TestFilterChainRemove(t *testing.T) {
	chain := NewFilterChain()
	chain.Add(NewTokenBanFilter("filter1", "first", []int{1}))
	chain.Add(NewTokenBanFilter("filter2", "second", []int{2}))

	if !chain.Remove("filter1") {
		t.Error("expected Remove to return true")
	}

	if chain.Len() != 1 {
		t.Errorf("expected chain length 1 after remove, got %d", chain.Len())
	}

	// Removing non-existent filter should return false
	if chain.Remove("nonexistent") {
		t.Error("expected Remove to return false for nonexistent filter")
	}
}

func TestFilterChainGet(t *testing.T) {
	chain := NewFilterChain()
	filter1 := NewTokenBanFilter("filter1", "first", []int{1})
	chain.Add(filter1)

	got := chain.Get("filter1")
	if got != filter1 {
		t.Error("expected to get the same filter instance")
	}

	got = chain.Get("nonexistent")
	if got != nil {
		t.Error("expected nil for nonexistent filter")
	}
}

func TestFilterChainEnableDisable(t *testing.T) {
	chain := NewFilterChain()
	filter1 := NewTokenBanFilter("filter1", "first", []int{1})
	chain.Add(filter1)

	// Disable specific filter
	chain.Disable("filter1")

	logits := make([]float32, 10)
	for i := range logits {
		logits[i] = 1.0
	}

	ctx := NewGenerationContext("test")
	result := chain.Apply(logits, ctx)

	// Token should not be banned when filter is disabled
	if math.IsInf(float64(result[1]), -1) {
		t.Error("expected token 1 to not be banned when filter disabled")
	}

	// Re-enable
	chain.Enable("filter1")
	for i := range logits {
		logits[i] = 1.0
	}
	result = chain.Apply(logits, ctx)

	if !math.IsInf(float64(result[1]), -1) {
		t.Error("expected token 1 to be banned when filter enabled")
	}
}

func TestFilterChainDisableAll(t *testing.T) {
	chain := NewFilterChain()
	chain.Add(NewTokenBanFilter("filter1", "first", []int{1}))
	chain.Add(NewTokenBanFilter("filter2", "second", []int{2}))

	chain.DisableAll()

	active := chain.ActiveFilters()
	if len(active) != 0 {
		t.Errorf("expected 0 active filters after DisableAll, got %d", len(active))
	}
}

func TestFilterChainEnableAll(t *testing.T) {
	chain := NewFilterChain()
	filter1 := NewTokenBanFilter("filter1", "first", []int{1})
	filter2 := NewTokenBanFilter("filter2", "second", []int{2})
	filter1.SetEnabled(false)
	filter2.SetEnabled(false)
	chain.Add(filter1)
	chain.Add(filter2)

	chain.EnableAll()

	active := chain.ActiveFilters()
	if len(active) != 2 {
		t.Errorf("expected 2 active filters after EnableAll, got %d", len(active))
	}
}

func TestFilterChainClone(t *testing.T) {
	chain := NewFilterChain()
	chain.Add(NewTokenBanFilter("filter1", "first", []int{1}))

	clone := chain.Clone()

	if clone.Len() != chain.Len() {
		t.Errorf("expected clone to have same length")
	}

	// Modifying clone shouldn't affect original
	clone.Add(NewTokenBanFilter("filter2", "second", []int{2}))

	if chain.Len() != 1 {
		t.Error("expected original chain to be unmodified")
	}
}

func TestFilterChainClear(t *testing.T) {
	chain := NewFilterChain()
	chain.Add(NewTokenBanFilter("filter1", "first", []int{1}))
	chain.Add(NewTokenBanFilter("filter2", "second", []int{2}))

	chain.Clear()

	if chain.Len() != 0 {
		t.Errorf("expected chain length 0 after clear, got %d", chain.Len())
	}
}

func TestChainBuilder(t *testing.T) {
	chain := NewChainBuilder().
		WithBannedTokens("banned", []int{1, 2}).
		WithLogitBias(map[int]float32{3: 5.0}).
		Build()

	if chain.Len() != 2 {
		t.Errorf("expected chain length 2, got %d", chain.Len())
	}

	logits := make([]float32, 10)
	for i := range logits {
		logits[i] = 1.0
	}

	ctx := NewGenerationContext("test")
	result := chain.Apply(logits, ctx)

	if !math.IsInf(float64(result[1]), -1) {
		t.Error("expected token 1 to be banned")
	}
	if result[3] != 6.0 {
		t.Errorf("expected token 3 to have bias applied, got %f", result[3])
	}
}

func TestNewFilterChainWithFilters(t *testing.T) {
	filter1 := NewTokenBanFilter("filter1", "first", []int{1})
	filter2 := NewTokenBanFilter("filter2", "second", []int{2})

	chain := NewFilterChainWithFilters(filter1, filter2)

	if chain.Len() != 2 {
		t.Errorf("expected chain length 2, got %d", chain.Len())
	}
}

func TestFilterChainInsert(t *testing.T) {
	chain := NewFilterChain()
	filter1 := NewTokenBanFilter("filter1", "first", []int{1})
	filter3 := NewTokenBanFilter("filter3", "third", []int{3})
	chain.Add(filter1)
	chain.Add(filter3)

	filter2 := NewTokenBanFilter("filter2", "second", []int{2})
	err := chain.Insert(1, filter2)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	filters := chain.Filters()
	if filters[1].Name() != "filter2" {
		t.Errorf("expected filter2 at index 1, got %s", filters[1].Name())
	}

	// Test invalid index
	err = chain.Insert(100, filter2)
	if err == nil {
		t.Error("expected error for invalid index")
	}
}

func TestFilterChainReset(t *testing.T) {
	chain := NewFilterChain()

	// Create a filter that tracks state
	filter := NewTokenBanFilter("filter1", "first", []int{1})
	chain.Add(filter)

	// Reset should call Reset on all filters
	chain.Reset()

	// This is more of a smoke test since TokenBanFilter.Reset() is a no-op
	if chain.Len() != 1 {
		t.Error("chain should still have filters after reset")
	}
}
