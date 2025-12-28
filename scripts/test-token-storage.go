package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/soypete/pedrocli/pkg/database"
	"github.com/soypete/pedrocli/pkg/tokens"
)

func main() {
	fmt.Println("=== PedroCLI Token Storage Test ===")

	// Create temporary database
	dbPath := "/tmp/test-token-storage.db"
	os.Remove(dbPath) // Clean up from previous runs

	fmt.Printf("Creating test database at %s\n", dbPath)
	store, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}
	defer store.Close()

	// Get underlying DB connection for token store
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create token store and manager
	tokenStore := tokens.NewSQLiteTokenStore(db)
	tokenManager := tokens.NewManager(tokenStore)

	// Register refresh handlers
	tokenManager.RegisterRefreshHandler(tokens.NewGoogleRefreshHandler("client-id", "client-secret"))
	tokenManager.RegisterRefreshHandler(tokens.NewNotionRefreshHandler())

	ctx := context.Background()

	// Test 1: Save and retrieve Notion API key (non-expiring)
	fmt.Println("\n--- Test 1: Notion API Key (non-expiring) ---")
	notionToken := tokens.CreateAPIKeyToken("notion", "database", "secret_notion_api_key_12345")

	err = tokenManager.SaveToken(ctx, notionToken)
	if err != nil {
		log.Fatalf("Failed to save Notion token: %v", err)
	}
	fmt.Println("✓ Saved Notion API key")

	retrieved, err := tokenManager.GetToken(ctx, "notion", "database")
	if err != nil {
		log.Fatalf("Failed to get Notion token: %v", err)
	}
	accessPreview := retrieved.AccessToken
	if len(accessPreview) > 20 {
		accessPreview = accessPreview[:20] + "..."
	}
	fmt.Printf("✓ Retrieved Notion token: %s (expires: %v)\n",
		accessPreview, retrieved.ExpiresAt)

	// Test 2: Save OAuth2 token (Google Calendar - expiring)
	fmt.Println("\n--- Test 2: Google Calendar OAuth Token (expiring) ---")
	googleToken := tokens.CreateOAuth2Token(
		"google",
		"calendar",
		"ya29.a0AfH6SMBx...",
		"1//0gH-refresh-token",
		"https://www.googleapis.com/auth/calendar",
		1*time.Hour, // Expires in 1 hour
	)

	err = tokenManager.SaveToken(ctx, googleToken)
	if err != nil {
		log.Fatalf("Failed to save Google token: %v", err)
	}
	fmt.Printf("✓ Saved Google OAuth token (expires at: %s)\n",
		googleToken.ExpiresAt.Format(time.RFC3339))

	retrieved, err = tokenManager.GetToken(ctx, "google", "calendar")
	if err != nil {
		log.Fatalf("Failed to get Google token: %v", err)
	}
	accessPreview = retrieved.AccessToken
	if len(accessPreview) > 20 {
		accessPreview = accessPreview[:20] + "..."
	}
	fmt.Printf("✓ Retrieved Google token: %s (expires: %s)\n",
		accessPreview,
		retrieved.ExpiresAt.Format(time.RFC3339))

	// Test 3: Check expiry logic
	fmt.Println("\n--- Test 3: Expiry Detection ---")
	fmt.Printf("Notion token needs refresh: %v (expected: false)\n",
		notionToken.NeedsRefresh(5*time.Minute))
	fmt.Printf("Google token needs refresh: %v (expected: false, expires in 1h)\n",
		googleToken.NeedsRefresh(5*time.Minute))

	// Create an expiring-soon token
	expiringSoon := tokens.CreateOAuth2Token(
		"google",
		"drive",
		"ya29.expiring-soon",
		"1//refresh",
		"https://www.googleapis.com/auth/drive",
		3*time.Minute, // Expires in 3 minutes (within 5min buffer)
	)
	err = tokenManager.SaveToken(ctx, expiringSoon)
	if err != nil {
		log.Fatalf("Failed to save expiring token: %v", err)
	}
	fmt.Printf("Token expiring in 3min needs refresh: %v (expected: true)\n",
		expiringSoon.NeedsRefresh(5*time.Minute))

	// Test 4: List tokens
	fmt.Println("\n--- Test 4: List Tokens ---")
	allGoogleTokens, err := tokenManager.ListTokens(ctx, "google", nil)
	if err != nil {
		log.Fatalf("Failed to list Google tokens: %v", err)
	}
	fmt.Printf("✓ Found %d Google tokens:\n", len(allGoogleTokens))
	for _, tok := range allGoogleTokens {
		expires := "never"
		if tok.ExpiresAt != nil {
			expires = tok.ExpiresAt.Format(time.RFC3339)
		}
		fmt.Printf("  - %s/%s (expires: %s)\n", tok.Provider, tok.Service, expires)
	}

	// Test 5: Update token (upsert)
	fmt.Println("\n--- Test 5: Update Token (Upsert) ---")
	updatedNotion := tokens.CreateAPIKeyToken("notion", "database", "new_notion_key_67890")
	err = tokenManager.SaveToken(ctx, updatedNotion)
	if err != nil {
		log.Fatalf("Failed to update Notion token: %v", err)
	}
	fmt.Println("✓ Updated Notion token")

	retrieved, err = tokenManager.GetToken(ctx, "notion", "database")
	if err != nil {
		log.Fatalf("Failed to get updated token: %v", err)
	}
	if retrieved.AccessToken == "new_notion_key_67890" {
		fmt.Println("✓ Verified token was updated (not duplicated)")
	} else {
		log.Fatalf("Token update failed: expected new key, got %s", retrieved.AccessToken)
	}

	// Test 6: Delete token
	fmt.Println("\n--- Test 6: Delete Token ---")
	err = tokenManager.DeleteToken(ctx, "google", "drive")
	if err != nil {
		log.Fatalf("Failed to delete token: %v", err)
	}
	fmt.Println("✓ Deleted google/drive token")

	_, err = tokenManager.GetToken(ctx, "google", "drive")
	if err != nil {
		fmt.Println("✓ Verified token was deleted (not found)")
	} else {
		log.Fatalf("Token should have been deleted")
	}

	// Summary
	fmt.Println("\n=== Summary ===")
	fmt.Println("✓ All tests passed!")
	fmt.Printf("✓ Database created at: %s\n", dbPath)
	fmt.Println("✓ Token storage system working correctly")
	fmt.Println("\nYou can inspect the database with:")
	fmt.Printf("  sqlite3 %s \"SELECT provider, service, expires_at FROM oauth_tokens;\"\n", dbPath)
}
