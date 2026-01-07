package agents

import (
	"testing"
)

func TestPhaseStructure(t *testing.T) {
	// Test that Phase struct can be created with all fields
	phase := Phase{
		Name:         "test_phase",
		Description:  "Test phase description",
		SystemPrompt: "Test system prompt",
		Tools:        []string{"tool1", "tool2"},
		MaxRounds:    5,
		ExpectsJSON:  true,
		Validator: func(result *PhaseResult) error {
			return nil
		},
	}

	if phase.Name != "test_phase" {
		t.Errorf("expected Name to be 'test_phase', got %s", phase.Name)
	}
	if phase.MaxRounds != 5 {
		t.Errorf("expected MaxRounds to be 5, got %d", phase.MaxRounds)
	}
	if len(phase.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(phase.Tools))
	}
}

func TestPhaseResult(t *testing.T) {
	result := &PhaseResult{
		PhaseName:  "analyze",
		Success:    true,
		Output:     "Analysis complete",
		RoundsUsed: 3,
	}

	if !result.Success {
		t.Error("expected Success to be true")
	}
	if result.RoundsUsed != 3 {
		t.Errorf("expected RoundsUsed to be 3, got %d", result.RoundsUsed)
	}
}

func TestExtractJSONData(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantKey string
		wantErr bool
	}{
		{
			name:    "valid JSON object",
			input:   `Some text before {"key": "value"} some text after`,
			wantKey: "key",
			wantErr: false,
		},
		{
			name:    "JSON with code block",
			input:   "```json\n{\"key\": \"value\"}\n```",
			wantKey: "key",
			wantErr: false,
		},
		{
			name:    "no JSON",
			input:   "No JSON here",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := extractJSONData(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractJSONData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && data[tt.wantKey] == nil {
				t.Errorf("expected key %s in result", tt.wantKey)
			}
		})
	}
}

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is a ..."},
		{"exact", 5, "exact"},
	}

	for _, tt := range tests {
		got := truncateOutput(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateOutput(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}
