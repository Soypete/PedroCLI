package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/lib/pq"
	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/database"
	"github.com/soypete/pedrocli/pkg/llm"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	transcriptionFile := os.Args[1]

	// Read transcription
	transcription, err := os.ReadFile(transcriptionFile)
	if err != nil {
		log.Fatalf("Failed to read transcription: %v", err)
	}

	// Load config
	cfg, err := config.LoadDefault()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Setup LLM backend
	var backend llm.Backend

	switch cfg.Model.Type {
	case "ollama":
		backend = llm.NewOllamaClient(cfg)
	case "llamacpp":
		backend = llm.NewServerClient(llm.ServerClientConfig{
			BaseURL:     cfg.Model.ServerURL,
			ModelName:   cfg.Model.ModelName,
			ContextSize: cfg.Model.ContextSize,
			EnableTools: true,
		})
	default:
		log.Fatalf("Unknown model type: %s", cfg.Model.Type)
	}

	// Setup database (optional - can be nil for testing)
	var db *sql.DB
	if cfg.Blog.Enabled && cfg.Database.Database != "" {
		dbCfg := &database.Config{
			Host:     cfg.Database.Host,
			Port:     cfg.Database.Port,
			User:     cfg.Database.User,
			Password: cfg.Database.Password,
			Database: cfg.Database.Database,
			SSLMode:  cfg.Database.SSLMode,
		}

		dbWrapper, err := database.New(dbCfg)
		if err != nil {
			log.Printf("Warning: Database connection failed: %v", err)
			log.Println("Continuing without database (versions won't be saved)")
		} else {
			db = dbWrapper.DB
			defer db.Close()
		}
	}

	// Extract title from first line if available
	title := "Untitled Blog Post"
	lines := string(transcription)
	if len(lines) > 0 {
		firstLine := lines[:min(len(lines), 100)]
		if len(firstLine) > 10 {
			title = firstLine[:min(len(firstLine), 60)] + "..."
		}
	}

	// Create agent
	agent := agents.NewBlogContentAgent(agents.BlogContentAgentConfig{
		Backend:       backend,
		DB:            db,
		WorkingDir:    cfg.Project.Workdir,
		MaxIterations: 10,
		Transcription: string(transcription),
		Title:         title,
	})

	// Execute workflow
	fmt.Println("\n🚀 Starting BlogContentAgent workflow...")

	if err := agent.Execute(context.Background()); err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	// Print results
	post := agent.GetCurrentPost()
	socialPosts := agent.GetSocialPosts()

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📝 FINAL BLOG POST")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	fmt.Println(post.FinalContent)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📱 SOCIAL MEDIA POSTS")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	for platform, post := range socialPosts {
		fmt.Printf("**%s:**\n%s\n\n", platform, post)
	}

	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("✏️ EDITOR FEEDBACK")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	fmt.Println(post.EditorOutput)

	if db != nil {
		fmt.Printf("\n💾 Saved to database with ID: %s\n", post.ID)
		fmt.Println("📚 Version history available in blog_post_versions table")
	}

	fmt.Println("\n✅ Workflow complete!")
}

func printUsage() {
	fmt.Print(`Test BlogContentAgent 7-Phase Workflow

Usage: go run cmd/test-blog-agent/main.go <transcription-file>

Example:
  go run cmd/test-blog-agent/main.go test/fixtures/sample_transcription.txt

The agent will:
1. Transcribe (load your file)
2. Research (web search + scraping)
3. Outline (generate structure)
4. Generate Sections (with TLDR)
5. Assemble (combine + social posts)
6. Editor Review (grammar/coherence)
7. Publish (save to database)

Progress will be shown with a tree view like Claude Code.`)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
