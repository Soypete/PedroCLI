package httpbridge

import (
	"testing"
)

func TestConvertToSSH(t *testing.T) {
	wm := NewWorkspaceManager("/tmp/test")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "GitHub HTTPS with .git",
			input:    "https://github.com/user/repo.git",
			expected: "git@github.com:user/repo.git",
		},
		{
			name:     "GitHub HTTPS without .git",
			input:    "https://github.com/user/repo",
			expected: "git@github.com:user/repo.git",
		},
		{
			name:     "GitLab HTTPS",
			input:    "https://gitlab.com/user/repo.git",
			expected: "git@gitlab.com:user/repo.git",
		},
		{
			name:     "Bitbucket HTTPS",
			input:    "https://bitbucket.org/user/repo.git",
			expected: "git@bitbucket.org:user/repo.git",
		},
		{
			name:     "Already SSH format",
			input:    "git@github.com:user/repo.git",
			expected: "git@github.com:user/repo.git",
		},
		{
			name:     "Local path",
			input:    "/path/to/local/repo",
			expected: "/path/to/local/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wm.ConvertToSSH(tt.input)
			if result != tt.expected {
				t.Errorf("ConvertToSSH(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
