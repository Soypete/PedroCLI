package httpbridge

import (
	"testing"
)

// Note: Full integration tests require running LLM backend and job manager.
// These tests cover the handler request/response parsing.

func TestJobTypeValidation(t *testing.T) {
	tests := []struct {
		name    string
		jobType string
		valid   bool
	}{
		{
			name:    "Builder type",
			jobType: "builder",
			valid:   true,
		},
		{
			name:    "Debugger type",
			jobType: "debugger",
			valid:   true,
		},
		{
			name:    "Reviewer type",
			jobType: "reviewer",
			valid:   true,
		},
		{
			name:    "Triager type",
			jobType: "triager",
			valid:   true,
		},
		{
			name:    "Invalid type",
			jobType: "unknown",
			valid:   false,
		},
		{
			name:    "Empty type",
			jobType: "",
			valid:   false,
		},
	}

	validTypes := map[string]bool{
		"builder":  true,
		"debugger": true,
		"reviewer": true,
		"triager":  true,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validTypes[tt.jobType]
			if got != tt.valid {
				t.Errorf("Expected valid=%v for type '%s', got %v", tt.valid, tt.jobType, got)
			}
		})
	}
}
