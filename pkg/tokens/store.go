package tokens

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// TokenStore implements Store using PostgreSQL
type TokenStore struct {
	db *sql.DB
}

// NewTokenStore creates a new PostgreSQL token store
// db should already have migrations applied (oauth_tokens table created)
func NewTokenStore(db *sql.DB) *TokenStore {
	return &TokenStore{db: db}
}

// GetToken retrieves a token by provider and service
func (s *TokenStore) GetToken(ctx context.Context, provider, service string) (*OAuthToken, error) {
	query := `
		SELECT id, provider, service, access_token, refresh_token, token_type, scope,
		       expires_at, last_refreshed, created_at, updated_at
		FROM oauth_tokens
		WHERE provider = $1 AND service = $2
	`

	row := s.db.QueryRowContext(ctx, query, provider, service)
	return s.scanToken(row)
}

// SaveToken saves or updates a token (upsert)
func (s *TokenStore) SaveToken(ctx context.Context, token *OAuthToken) error {
	if token.ID == "" {
		token.ID = uuid.New().String()
	}

	now := time.Now()
	if token.CreatedAt.IsZero() {
		token.CreatedAt = now
	}
	token.UpdatedAt = now

	query := `
		INSERT INTO oauth_tokens (id, provider, service, access_token, refresh_token, token_type, scope, expires_at, last_refreshed, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (provider, service) DO UPDATE SET
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			token_type = EXCLUDED.token_type,
			scope = EXCLUDED.scope,
			expires_at = EXCLUDED.expires_at,
			last_refreshed = EXCLUDED.last_refreshed,
			updated_at = EXCLUDED.updated_at
	`

	_, err := s.db.ExecContext(ctx, query,
		token.ID,
		token.Provider,
		token.Service,
		token.AccessToken,
		nullString(token.RefreshToken),
		token.TokenType,
		nullString(token.Scope),
		nullTime(token.ExpiresAt),
		nullTime(token.LastRefreshed),
		token.CreatedAt,
		token.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	return nil
}

// DeleteToken removes a token
func (s *TokenStore) DeleteToken(ctx context.Context, provider, service string) error {
	query := `DELETE FROM oauth_tokens WHERE provider = $1 AND service = $2`

	result, err := s.db.ExecContext(ctx, query, provider, service)
	if err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("token not found")
	}

	return nil
}

// ListTokens lists all tokens for a provider (optional service filter)
func (s *TokenStore) ListTokens(ctx context.Context, provider string, service *string) ([]*OAuthToken, error) {
	var query string
	var args []interface{}

	if service != nil {
		query = `
			SELECT id, provider, service, access_token, refresh_token, token_type, scope,
			       expires_at, last_refreshed, created_at, updated_at
			FROM oauth_tokens
			WHERE provider = $1 AND service = $2
			ORDER BY created_at DESC
		`
		args = []interface{}{provider, *service}
	} else {
		query = `
			SELECT id, provider, service, access_token, refresh_token, token_type, scope,
			       expires_at, last_refreshed, created_at, updated_at
			FROM oauth_tokens
			WHERE provider = $1
			ORDER BY created_at DESC
		`
		args = []interface{}{provider}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*OAuthToken
	for rows.Next() {
		token, err := s.scanTokenRow(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tokens: %w", err)
	}

	return tokens, nil
}

// scanToken scans a single row into an OAuthToken
func (s *TokenStore) scanToken(row *sql.Row) (*OAuthToken, error) {
	var token OAuthToken
	var refreshToken sql.NullString
	var scope sql.NullString
	var expiresAt sql.NullTime
	var lastRefreshed sql.NullTime

	err := row.Scan(
		&token.ID,
		&token.Provider,
		&token.Service,
		&token.AccessToken,
		&refreshToken,
		&token.TokenType,
		&scope,
		&expiresAt,
		&lastRefreshed,
		&token.CreatedAt,
		&token.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("token not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan token: %w", err)
	}

	// Handle nullable fields
	if refreshToken.Valid {
		token.RefreshToken = refreshToken.String
	}
	if scope.Valid {
		token.Scope = scope.String
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		token.ExpiresAt = &t
	}
	if lastRefreshed.Valid {
		t := lastRefreshed.Time
		token.LastRefreshed = &t
	}

	return &token, nil
}

// scanTokenRow scans a row from sql.Rows into an OAuthToken
func (s *TokenStore) scanTokenRow(rows *sql.Rows) (*OAuthToken, error) {
	var token OAuthToken
	var refreshToken sql.NullString
	var scope sql.NullString
	var expiresAt sql.NullTime
	var lastRefreshed sql.NullTime

	err := rows.Scan(
		&token.ID,
		&token.Provider,
		&token.Service,
		&token.AccessToken,
		&refreshToken,
		&token.TokenType,
		&scope,
		&expiresAt,
		&lastRefreshed,
		&token.CreatedAt,
		&token.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan token row: %w", err)
	}

	// Handle nullable fields
	if refreshToken.Valid {
		token.RefreshToken = refreshToken.String
	}
	if scope.Valid {
		token.Scope = scope.String
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		token.ExpiresAt = &t
	}
	if lastRefreshed.Valid {
		t := lastRefreshed.Time
		token.LastRefreshed = &t
	}

	return &token, nil
}

// Helper functions for nullable values

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: *t, Valid: true}
}
