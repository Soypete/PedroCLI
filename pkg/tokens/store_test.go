package tokens

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Create oauth_tokens table
	schema := `
		CREATE TABLE oauth_tokens (
			id TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			service TEXT NOT NULL,
			access_token TEXT NOT NULL,
			refresh_token TEXT,
			token_type TEXT DEFAULT 'Bearer',
			scope TEXT,
			expires_at DATETIME,
			last_refreshed DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(provider, service)
		);
	`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		t.Fatalf("failed to create schema: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

func TestSQLiteTokenStore_SaveAndGetToken(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSQLiteTokenStore(db)
	ctx := context.Background()

	// Create test token
	expiresAt := time.Now().Add(1 * time.Hour)
	token := &OAuthToken{
		Provider:     "google",
		Service:      "calendar",
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Scope:        "https://www.googleapis.com/auth/calendar",
		ExpiresAt:    &expiresAt,
	}

	// Save token
	err := store.SaveToken(ctx, token)
	if err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}

	if token.ID == "" {
		t.Error("expected ID to be generated")
	}

	// Retrieve token
	retrieved, err := store.GetToken(ctx, "google", "calendar")
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}

	// Verify fields
	if retrieved.Provider != "google" {
		t.Errorf("expected provider 'google', got '%s'", retrieved.Provider)
	}
	if retrieved.Service != "calendar" {
		t.Errorf("expected service 'calendar', got '%s'", retrieved.Service)
	}
	if retrieved.AccessToken != "test-access-token" {
		t.Errorf("expected access token 'test-access-token', got '%s'", retrieved.AccessToken)
	}
	if retrieved.RefreshToken != "test-refresh-token" {
		t.Errorf("expected refresh token 'test-refresh-token', got '%s'", retrieved.RefreshToken)
	}
	if retrieved.ExpiresAt == nil {
		t.Error("expected expires_at to be set")
	}
}

func TestSQLiteTokenStore_SaveAPIKey(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSQLiteTokenStore(db)
	ctx := context.Background()

	// Create API key token (no expiry)
	token := CreateAPIKeyToken("notion", "database", "secret_notion_key_123")

	// Save token
	err := store.SaveToken(ctx, token)
	if err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}

	// Retrieve token
	retrieved, err := store.GetToken(ctx, "notion", "database")
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}

	// Verify it's an API key (no expiry, no refresh token)
	if retrieved.ExpiresAt != nil {
		t.Error("expected expires_at to be nil for API key")
	}
	if retrieved.RefreshToken != "" {
		t.Error("expected refresh_token to be empty for API key")
	}
	if retrieved.AccessToken != "secret_notion_key_123" {
		t.Errorf("expected access token 'secret_notion_key_123', got '%s'", retrieved.AccessToken)
	}
}

func TestSQLiteTokenStore_Upsert(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSQLiteTokenStore(db)
	ctx := context.Background()

	// Save initial token
	token := CreateAPIKeyToken("notion", "database", "old-token")
	err := store.SaveToken(ctx, token)
	if err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}

	// Update token (same provider/service)
	updated := CreateAPIKeyToken("notion", "database", "new-token")
	err = store.SaveToken(ctx, updated)
	if err != nil {
		t.Fatalf("SaveToken (update) failed: %v", err)
	}

	// Retrieve - should have new token
	retrieved, err := store.GetToken(ctx, "notion", "database")
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}

	if retrieved.AccessToken != "new-token" {
		t.Errorf("expected token to be updated to 'new-token', got '%s'", retrieved.AccessToken)
	}
}

func TestSQLiteTokenStore_DeleteToken(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSQLiteTokenStore(db)
	ctx := context.Background()

	// Save token
	token := CreateAPIKeyToken("notion", "database", "test-token")
	err := store.SaveToken(ctx, token)
	if err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}

	// Delete token
	err = store.DeleteToken(ctx, "notion", "database")
	if err != nil {
		t.Fatalf("DeleteToken failed: %v", err)
	}

	// Try to retrieve - should fail
	_, err = store.GetToken(ctx, "notion", "database")
	if err == nil {
		t.Error("expected GetToken to fail after deletion")
	}
}

func TestSQLiteTokenStore_ListTokens(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSQLiteTokenStore(db)
	ctx := context.Background()

	// Save multiple tokens
	tokens := []*OAuthToken{
		CreateAPIKeyToken("google", "calendar", "token1"),
		CreateAPIKeyToken("google", "drive", "token2"),
		CreateAPIKeyToken("notion", "database", "token3"),
	}

	for _, tok := range tokens {
		if err := store.SaveToken(ctx, tok); err != nil {
			t.Fatalf("SaveToken failed: %v", err)
		}
	}

	// List all Google tokens
	googleTokens, err := store.ListTokens(ctx, "google", nil)
	if err != nil {
		t.Fatalf("ListTokens failed: %v", err)
	}

	if len(googleTokens) != 2 {
		t.Errorf("expected 2 Google tokens, got %d", len(googleTokens))
	}

	// List specific service
	service := "calendar"
	calendarTokens, err := store.ListTokens(ctx, "google", &service)
	if err != nil {
		t.Fatalf("ListTokens with service filter failed: %v", err)
	}

	if len(calendarTokens) != 1 {
		t.Errorf("expected 1 calendar token, got %d", len(calendarTokens))
	}
	if calendarTokens[0].Service != "calendar" {
		t.Errorf("expected service 'calendar', got '%s'", calendarTokens[0].Service)
	}
}

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
