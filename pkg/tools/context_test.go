package tools

import (
	"context"
	"testing"
)

func TestContextTool_Compact(t *testing.T) {
	ct := NewContextTool()

	result, err := ct.Execute(context.Background(), map[string]interface{}{
		"action":  "compact",
		"key":     "test_key",
		"content": "This is a very long content that should be summarized",
		"summary": "Summarized content",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	// Verify it was stored
	summary, ok := ct.GetSummary("test_key")
	if !ok {
		t.Fatal("expected summary to be stored")
	}
	if summary != "Summarized content" {
		t.Errorf("expected 'Summarized content', got %s", summary)
	}
}

func TestContextTool_Recall(t *testing.T) {
	ct := NewContextTool()

	// Store a summary first
	ct.SetSummary("test_key", "Test summary value")

	result, err := ct.Execute(context.Background(), map[string]interface{}{
		"action": "recall",
		"key":    "test_key",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Output != "Test summary value" {
		t.Errorf("expected 'Test summary value', got %s", result.Output)
	}
}

func TestContextTool_RecallMissing(t *testing.T) {
	ct := NewContextTool()

	result, err := ct.Execute(context.Background(), map[string]interface{}{
		"action": "recall",
		"key":    "nonexistent",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for nonexistent key")
	}
}

func TestContextTool_Checkpoint(t *testing.T) {
	ct := NewContextTool()

	result, err := ct.Execute(context.Background(), map[string]interface{}{
		"action":      "checkpoint",
		"name":        "pre_impl",
		"description": "Ready for implementation",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	// Verify checkpoint was stored
	summary, ok := ct.GetSummary("checkpoint:pre_impl")
	if !ok {
		t.Fatal("expected checkpoint to be stored")
	}
	if summary != "Ready for implementation" {
		t.Errorf("expected 'Ready for implementation', got %s", summary)
	}
}

func TestContextTool_List(t *testing.T) {
	ct := NewContextTool()

	// Add some summaries
	ct.SetSummary("summary1", "First summary")
	ct.SetSummary("checkpoint:cp1", "First checkpoint")

	result, err := ct.Execute(context.Background(), map[string]interface{}{
		"action": "list",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Data["count"].(int) != 2 {
		t.Errorf("expected count of 2, got %v", result.Data["count"])
	}
}

func TestContextTool_Metadata(t *testing.T) {
	ct := NewContextTool()
	meta := ct.Metadata()

	if meta == nil {
		t.Fatal("expected metadata to be non-nil")
	}
	if meta.Category != CategoryUtility {
		t.Errorf("expected category Utility, got %v", meta.Category)
	}
}
