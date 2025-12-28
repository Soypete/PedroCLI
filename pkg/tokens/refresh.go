package tokens

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleRefreshHandler handles OAuth2 token refresh for Google services
type GoogleRefreshHandler struct {
	clientID     string
	clientSecret string
}

// NewGoogleRefreshHandler creates a new Google OAuth2 refresh handler
func NewGoogleRefreshHandler(clientID, clientSecret string) *GoogleRefreshHandler {
	return &GoogleRefreshHandler{
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// Provider returns the provider name
func (h *GoogleRefreshHandler) Provider() string {
	return "google"
}

// Refresh refreshes a Google OAuth2 token
func (h *GoogleRefreshHandler) Refresh(ctx context.Context, token *OAuthToken) (*OAuthToken, error) {
	if token.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	// Create OAuth2 config
	config := &oauth2.Config{
		ClientID:     h.clientID,
		ClientSecret: h.clientSecret,
		Endpoint:     google.Endpoint,
	}

	// Create token source from refresh token
	oldToken := &oauth2.Token{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
	}
	if token.ExpiresAt != nil {
		oldToken.Expiry = *token.ExpiresAt
	}

	// Use token source to get a fresh token
	tokenSource := config.TokenSource(ctx, oldToken)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	// Update token fields
	token.AccessToken = newToken.AccessToken
	if newToken.RefreshToken != "" {
		// Google sometimes returns a new refresh token
		token.RefreshToken = newToken.RefreshToken
	}
	if !newToken.Expiry.IsZero() {
		token.ExpiresAt = &newToken.Expiry
	}
	token.UpdatedAt = time.Now()

	return token, nil
}

// NotionRefreshHandler is a no-op handler for Notion (API keys don't expire)
type NotionRefreshHandler struct{}

// NewNotionRefreshHandler creates a new Notion refresh handler
func NewNotionRefreshHandler() *NotionRefreshHandler {
	return &NotionRefreshHandler{}
}

// Provider returns the provider name
func (h *NotionRefreshHandler) Provider() string {
	return "notion"
}

// Refresh is a no-op for Notion since API keys don't expire
func (h *NotionRefreshHandler) Refresh(ctx context.Context, token *OAuthToken) (*OAuthToken, error) {
	// Notion API keys don't expire, no refresh needed
	return token, nil
}
