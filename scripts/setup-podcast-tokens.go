package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/database"
	"github.com/soypete/pedrocli/pkg/tokens"
)

func main() {
	// Parse command-line flags
	provider := flag.String("provider", "", "Token provider (notion, google)")
	service := flag.String("service", "", "Service name (database, calendar)")
	apiKey := flag.String("api-key", "", "API key or access token (required for Notion)")
	accessToken := flag.String("access-token", "", "OAuth access token (for Google)")
	refreshToken := flag.String("refresh-token", "", "OAuth refresh token (for Google)")
	expiresIn := flag.Int("expires-in", 0, "Token expiration in seconds (for OAuth tokens)")
	scope := flag.String("scope", "", "OAuth scope (for Google)")

	flag.Parse()

	// Validate required fields
	if *provider == "" || *service == "" {
		fmt.Fprintln(os.Stderr, "Error: -provider and -service are required")
		flag.Usage()
		os.Exit(1)
	}

	if *provider != "notion" && *provider != "google" {
		fmt.Fprintln(os.Stderr, "Error: provider must be 'notion' or 'google'")
		os.Exit(1)
	}

	// For Notion, require api-key
	if *provider == "notion" && *apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: -api-key is required for Notion")
		os.Exit(1)
	}

	// For Google, require access-token
	if *provider == "google" && *accessToken == "" {
		fmt.Fprintln(os.Stderr, "Error: -access-token is required for Google")
		os.Exit(1)
	}

	// Load configuration to get database settings
	cfg, err := config.LoadDefault()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Create database config from environment or config file
	dbCfg := database.DefaultConfig()
	if cfg.Database.Host != "" {
		dbCfg.Host = cfg.Database.Host
		dbCfg.Port = cfg.Database.Port
		dbCfg.Database = cfg.Database.Database
		dbCfg.User = cfg.Database.User
		dbCfg.Password = cfg.Database.Password
		dbCfg.SSLMode = cfg.Database.SSLMode
	}

	// Open PostgreSQL database
	db, err := database.New(dbCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Run migrations
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error running migrations: %v\n", err)
		os.Exit(1)
	}

	// Create token store
	tokenStore := tokens.NewTokenStore(db.DB)

	// Build token object based on provider type
	var token *tokens.OAuthToken
	now := time.Now()
	if *provider == "notion" {
		// Notion API keys don't expire
		token = &tokens.OAuthToken{
			Provider:      "notion",
			Service:       *service,
			AccessToken:   *apiKey,
			TokenType:     "Bearer",
			ExpiresAt:     nil, // API keys don't expire
			LastRefreshed: &now,
		}
	} else if *provider == "google" {
		// Google OAuth tokens
		var expiresAt *time.Time
		if *expiresIn > 0 {
			expires := time.Now().Add(time.Duration(*expiresIn) * time.Second)
			expiresAt = &expires
		}

		token = &tokens.OAuthToken{
			Provider:      "google",
			Service:       *service,
			AccessToken:   *accessToken,
			RefreshToken:  *refreshToken,
			TokenType:     "Bearer",
			Scope:         *scope,
			ExpiresAt:     expiresAt,
			LastRefreshed: &now,
		}
	}

	// Save token to database
	if err := tokenStore.SaveToken(ctx, token); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving token: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Token saved successfully for %s/%s\n", *provider, *service)
	fmt.Printf("  Provider: %s\n", token.Provider)
	fmt.Printf("  Service: %s\n", token.Service)
	if token.ExpiresAt != nil {
		fmt.Printf("  Expires: %s\n", token.ExpiresAt.Format(time.RFC3339))
	} else {
		fmt.Printf("  Expires: Never\n")
	}
}
