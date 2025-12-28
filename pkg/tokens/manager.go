package tokens

import (
	"context"
	"fmt"
	"time"
)

const (
	// DefaultRefreshBuffer is the time before expiry when we refresh tokens
	DefaultRefreshBuffer = 5 * time.Minute
)

// Manager provides high-level token management with automatic refresh
type Manager struct {
	store    Store
	handlers map[string]RefreshHandler
	buffer   time.Duration
}

// NewManager creates a new token manager
func NewManager(store Store) *Manager {
	return &Manager{
		store:    store,
		handlers: make(map[string]RefreshHandler),
		buffer:   DefaultRefreshBuffer,
	}
}

// RegisterRefreshHandler registers a provider-specific refresh handler
func (m *Manager) RegisterRefreshHandler(handler RefreshHandler) {
	m.handlers[handler.Provider()] = handler
}

// SetRefreshBuffer sets the time before expiry when tokens should be refreshed
func (m *Manager) SetRefreshBuffer(buffer time.Duration) {
	m.buffer = buffer
}

// GetToken retrieves a token, automatically refreshing if needed
func (m *Manager) GetToken(ctx context.Context, provider, service string) (*OAuthToken, error) {
	token, err := m.store.GetToken(ctx, provider, service)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	// Check if token needs refresh
	if token.NeedsRefresh(m.buffer) {
		refreshed, err := m.refresh(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}
		return refreshed, nil
	}

	return token, nil
}

// SaveToken saves or updates a token
func (m *Manager) SaveToken(ctx context.Context, token *OAuthToken) error {
	return m.store.SaveToken(ctx, token)
}

// DeleteToken removes a token
func (m *Manager) DeleteToken(ctx context.Context, provider, service string) error {
	return m.store.DeleteToken(ctx, provider, service)
}

// ListTokens lists all tokens for a provider
func (m *Manager) ListTokens(ctx context.Context, provider string, service *string) ([]*OAuthToken, error) {
	return m.store.ListTokens(ctx, provider, service)
}

// RefreshIfNeeded manually checks and refreshes a token if needed
func (m *Manager) RefreshIfNeeded(ctx context.Context, token *OAuthToken) (*OAuthToken, error) {
	if token.NeedsRefresh(m.buffer) {
		return m.refresh(ctx, token)
	}
	return token, nil
}

// refresh refreshes a token using the appropriate handler
func (m *Manager) refresh(ctx context.Context, token *OAuthToken) (*OAuthToken, error) {
	handler, ok := m.handlers[token.Provider]
	if !ok {
		return nil, fmt.Errorf("no refresh handler registered for provider: %s", token.Provider)
	}

	refreshed, err := handler.Refresh(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("refresh failed for provider %s: %w", token.Provider, err)
	}

	// Save refreshed token
	now := time.Now()
	refreshed.LastRefreshed = &now
	if err := m.store.SaveToken(ctx, refreshed); err != nil {
		return nil, fmt.Errorf("failed to save refreshed token: %w", err)
	}

	return refreshed, nil
}

// CreateAPIKeyToken creates a non-expiring API key token (e.g., for Notion)
func CreateAPIKeyToken(provider, service, apiKey string) *OAuthToken {
	return &OAuthToken{
		Provider:     provider,
		Service:      service,
		AccessToken:  apiKey,
		RefreshToken: "",       // No refresh token for API keys
		TokenType:    "Bearer", // Standard for API keys
		Scope:        "",       // No OAuth scopes
		ExpiresAt:    nil,      // Never expires
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// CreateOAuth2Token creates an OAuth2 token with expiry
func CreateOAuth2Token(provider, service, accessToken, refreshToken, scope string, expiresIn time.Duration) *OAuthToken {
	expiresAt := time.Now().Add(expiresIn)
	return &OAuthToken{
		Provider:     provider,
		Service:      service,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		Scope:        scope,
		ExpiresAt:    &expiresAt,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}
