package tokens

import (
	"context"
	"time"
)

// OAuthToken represents an OAuth token or API key stored in the database
type OAuthToken struct {
	ID            string     `json:"id"`
	Provider      string     `json:"provider"`       // "notion", "google", "github"
	Service       string     `json:"service"`        // "database", "calendar", etc.
	AccessToken   string     `json:"access_token"`   // The actual token (encrypted at rest)
	RefreshToken  string     `json:"refresh_token"`  // For OAuth2 refresh flow (NULL for API keys)
	TokenType     string     `json:"token_type"`     // "Bearer", "Basic", etc.
	Scope         string     `json:"scope"`          // Space-separated OAuth scopes
	ExpiresAt     *time.Time `json:"expires_at"`     // When token expires (NULL for non-expiring keys)
	LastRefreshed *time.Time `json:"last_refreshed"` // When we last refreshed
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// IsExpired checks if the token is expired or will expire within the buffer duration
func (t *OAuthToken) IsExpired(buffer time.Duration) bool {
	if t.ExpiresAt == nil {
		return false // Non-expiring token (e.g., API key)
	}
	return time.Now().Add(buffer).After(*t.ExpiresAt)
}

// NeedsRefresh checks if token needs refreshing (expired and has refresh token)
func (t *OAuthToken) NeedsRefresh(buffer time.Duration) bool {
	return t.RefreshToken != "" && t.IsExpired(buffer)
}

// Store defines the interface for token storage operations
type Store interface {
	// GetToken retrieves a token by provider and service
	GetToken(ctx context.Context, provider, service string) (*OAuthToken, error)

	// SaveToken saves or updates a token (upsert)
	SaveToken(ctx context.Context, token *OAuthToken) error

	// DeleteToken removes a token
	DeleteToken(ctx context.Context, provider, service string) error

	// ListTokens lists all tokens for a provider (optional service filter)
	ListTokens(ctx context.Context, provider string, service *string) ([]*OAuthToken, error)
}

// RefreshHandler defines the interface for provider-specific token refresh
type RefreshHandler interface {
	// Refresh refreshes an OAuth token using the refresh token
	Refresh(ctx context.Context, token *OAuthToken) (*OAuthToken, error)

	// Provider returns the provider name this handler supports
	Provider() string
}
