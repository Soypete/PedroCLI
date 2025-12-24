package httpbridge

import (
	"testing"
)

// Note: Full integration tests require a running MCP server.
// These tests cover the helper functions and data parsing.

// TestExtractJobID tests job ID extraction from MCP responses
func TestExtractJobID(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
		wantErr  bool
	}{
		{
			name:     "Valid job ID",
			text:     "Job job-1234567890 started and running in background.",
			expected: "job-1234567890",
			wantErr:  false,
		},
		{
			name:     "No job ID",
			text:     "No job started",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "Multiple job IDs (first match)",
			text:     "Job job-123 and job-456",
			expected: "job-123",
			wantErr:  false,
		},
		{
			name:     "Job ID in middle of text",
			text:     "Successfully started Job job-9999 for the builder agent",
			expected: "job-9999",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractJobID(tt.text)

			if tt.wantErr && err == nil {
				t.Error("Expected error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if got != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, got)
			}
		})
	}
}

// TODO: Add integration tests that spawn a test MCP server in future phases
