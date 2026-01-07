package tokens

import (
	"testing"
	"time"
)

// NOTE: Database integration tests have been removed.
// Only utility function tests remain that don't require a database instance.

func TestOAuthToken_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		token    *OAuthToken
		buffer   time.Duration
		expected bool
	}{
		{
			name: "api key never expires",
			token: &OAuthToken{
				ExpiresAt: nil,
			},
			buffer:   5 * time.Minute,
			expected: false,
		},
		{
			name: "token expired",
			token: &OAuthToken{
				ExpiresAt: &[]time.Time{time.Now().Add(-1 * time.Hour)}[0],
			},
			buffer:   5 * time.Minute,
			expected: true,
		},
		{
			name: "token valid",
			token: &OAuthToken{
				ExpiresAt: &[]time.Time{time.Now().Add(1 * time.Hour)}[0],
			},
			buffer:   5 * time.Minute,
			expected: false,
		},
		{
			name: "token expires within buffer",
			token: &OAuthToken{
				ExpiresAt: &[]time.Time{time.Now().Add(3 * time.Minute)}[0],
			},
			buffer:   5 * time.Minute,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.token.IsExpired(tt.buffer)
			if result != tt.expected {
				t.Errorf("IsExpired() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestOAuthToken_NeedsRefresh(t *testing.T) {
	tests := []struct {
		name     string
		token    *OAuthToken
		buffer   time.Duration
		expected bool
	}{
		{
			name: "api key with no refresh token",
			token: &OAuthToken{
				RefreshToken: "",
				ExpiresAt:    nil,
			},
			buffer:   5 * time.Minute,
			expected: false,
		},
		{
			name: "valid token with refresh",
			token: &OAuthToken{
				RefreshToken: "refresh-token",
				ExpiresAt:    &[]time.Time{time.Now().Add(1 * time.Hour)}[0],
			},
			buffer:   5 * time.Minute,
			expected: false,
		},
		{
			name: "expired token with refresh",
			token: &OAuthToken{
				RefreshToken: "refresh-token",
				ExpiresAt:    &[]time.Time{time.Now().Add(-1 * time.Hour)}[0],
			},
			buffer:   5 * time.Minute,
			expected: true,
		},
		{
			name: "token expiring soon with refresh",
			token: &OAuthToken{
				RefreshToken: "refresh-token",
				ExpiresAt:    &[]time.Time{time.Now().Add(3 * time.Minute)}[0],
			},
			buffer:   5 * time.Minute,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.token.NeedsRefresh(tt.buffer)
			if result != tt.expected {
				t.Errorf("NeedsRefresh() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
