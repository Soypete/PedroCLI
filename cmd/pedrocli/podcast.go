package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/storage/content"
)

func podcastCommand(cfg *config.Config, args []string) {
	if len(args) == 0 {
		printPodcastUsage()
		os.Exit(1)
	}

	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "script":
		podcastScriptCmd(cfg, subargs)
	case "news":
		podcastNewsCmd(cfg, subargs)
	case "schedule":
		podcastScheduleCmd(cfg, subargs)
	case "prep":
		podcastPrepCmd(cfg, subargs)
	case "help", "-h", "--help":
		printPodcastUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown podcast subcommand: %s\n\n", subcommand)
		printPodcastUsage()
		os.Exit(1)
	}
}

func printPodcastUsage() {
	fmt.Println(`Usage: pedrocli podcast <subcommand> [flags]

Podcast episode preparation workflows for SoypeteTech podcast.

Available subcommands:
  script   - Generate podcast episode script from outline
  news     - Review and summarize news items for episode prep
  schedule - Create Cal.com booking link for podcast recording
  prep     - Full episode prep workflow (script + news + schedule)

Examples:
  # Generate script from outline
  pedrocli podcast script -outline outline.md -episode "S01E03"

  # Review AI news for episode prep
  pedrocli podcast news -focus "model selection" -max 10

  # Create booking link
  pedrocli podcast schedule -episode "S01E03" -title "How to Choose a Model"

  # Full prep workflow
  pedrocli podcast prep -outline outline.md -episode "S01E03"

Use 'pedrocli podcast <subcommand> -h' for more information about a subcommand.`)
}

func podcastScriptCmd(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("podcast script", flag.ExitOnError)
	outline := fs.String("outline", "", "Path to outline file (markdown)")
	topic := fs.String("topic", "", "Episode topic/title")
	episode := fs.String("episode", "", "Episode number (e.g., 'S01E03')")
	guests := fs.String("guests", "", "Guest names (comma-separated)")
	duration := fs.Int("duration", 60, "Target duration in minutes")
	output := fs.String("output", "", "Output file path (default: stdout)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: pedrocli podcast script [flags]

Generate a structured podcast episode script from an outline file.

The script will include:
- Introduction and guest background
- Main discussion sections based on outline
- Rapid-fire Q&A section
- Outro with links and CTAs

Examples:
  pedrocli podcast script -outline /tmp/podcast-outline.md -episode "S01E03"
  pedrocli podcast script -topic "How to Choose a Model" -guests "Matt,Chris" -duration 90
  pedrocli podcast script -outline outline.md -output script.md

Flags:
`)
		fs.PrintDefaults()
	}

	fs.Parse(args)

	// Validate flags
	if *outline == "" && *topic == "" {
		fmt.Fprintln(os.Stderr, "Error: either -outline or -topic must be specified")
		fs.Usage()
		os.Exit(1)
	}

	// Load outline if provided
	var outlineContent string
	if *outline != "" {
		data, err := os.ReadFile(*outline)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to read outline file: %v\n", err)
			os.Exit(1)
		}
		outlineContent = string(data)
	}

	fmt.Printf("⏳ Generating podcast script for episode %s...\n", *episode)
	fmt.Printf("📝 Outline: %d characters\n", len(outlineContent))
	if *topic != "" {
		fmt.Printf("🎙️  Topic: %s\n", *topic)
	}
	if *guests != "" {
		fmt.Printf("👥 Guests: %s\n", *guests)
	}
	fmt.Printf("⏱️  Duration: %d minutes\n", *duration)

	// Create LLM backend
	backend, err := llm.NewBackend(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create LLM backend: %v\n", err)
		os.Exit(1)
	}

	// Create file-based storage for CLI
	storeConfig := content.StoreConfig{
		FileBaseDir: podcastContentDir(),
	}
	contentStore, err := content.NewContentStore(storeConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create content store: %v\n", err)
		os.Exit(1)
	}
	versionStore, err := content.NewVersionStore(storeConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create version store: %v\n", err)
		os.Exit(1)
	}

	// Use topic as title if title not provided
	episodeTitle := *topic
	if episodeTitle == "" && outlineContent != "" {
		episodeTitle = "Untitled Episode"
	}

	// Create UnifiedPodcastAgent with script workflow
	agent := agents.NewUnifiedPodcastAgent(agents.UnifiedPodcastAgentConfig{
		Backend:      backend,
		ContentStore: contentStore,
		VersionStore: versionStore,
		Config:       cfg,
		Mode:         agents.ExecutionModeSync,
		WorkflowType: agents.WorkflowScript,
		Outline:      outlineContent,
		Episode:      *episode,
		Title:        episodeTitle,
		Guests:       *guests,
		Duration:     *duration,
	})

	// Execute agent workflow synchronously (CLI doesn't use job management)
	ctx := context.Background()
	if err := agent.ExecuteWorkflow(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "\nError: Script generation failed: %v\n", err)
		os.Exit(1)
	}

	// Get output
	result := agent.GetOutput()
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "\nError: Failed to get agent output")
		os.Exit(1)
	}

	script, _ := resultMap["script"].(string)
	if script == "" {
		fmt.Fprintln(os.Stderr, "\nError: No script generated")
		os.Exit(1)
	}

	// Output results
	if *output != "" {
		if err := os.WriteFile(*output, []byte(script), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "\nError: Failed to write output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\n✅ Script saved to: %s\n", *output)
	} else {
		fmt.Println("\n" + script)
	}
}

func podcastNewsCmd(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("podcast news", flag.ExitOnError)
	focus := fs.String("focus", "", "Focus topic for news filtering (required)")
	maxNews := fs.Int("max", 5, "Maximum news items to include")
	sources := fs.String("sources", "", "File containing RSS/news sources")
	output := fs.String("output", "", "Output file path (default: stdout)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: pedrocli podcast news [flags]

Review and summarize recent news items to prepare for podcast discussion.

This command fetches news from RSS feeds and other sources, filters by topic,
ranks by relevance, and generates a summary for episode prep.

Examples:
  pedrocli podcast news -focus "AI" -max 10
  pedrocli podcast news -sources rss-feeds.txt -focus "model selection"
  pedrocli podcast news -focus "kubernetes" -output news-summary.md

Flags:
`)
		fs.PrintDefaults()
	}

	fs.Parse(args)

	// Validate flags
	if *focus == "" {
		fmt.Fprintln(os.Stderr, "Error: -focus must be specified")
		fs.Usage()
		os.Exit(1)
	}

	// TODO: Create storage (file-based for CLI)
	// TODO: Create unified podcast agent with news workflow
	// TODO: Execute agent
	// TODO: Output results

	fmt.Printf("⏳ Reviewing news items...\n")
	fmt.Printf("🎯 Focus: %s\n", *focus)
	fmt.Printf("📊 Max items: %d\n", *maxNews)

	if *sources != "" {
		fmt.Printf("📰 Sources file: %s\n", *sources)
	}

	if *output != "" {
		fmt.Printf("💾 Output will be saved to: %s\n", *output)
	}

	// TODO: Implement actual agent execution
	fmt.Fprintln(os.Stderr, "\nError: podcast news review not yet implemented - coming in PR #3 (UnifiedPodcastAgent)")
	os.Exit(1)
}

func podcastScheduleCmd(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("podcast schedule", flag.ExitOnError)
	template := fs.String("template", "", "Episode template/outline file")
	duration := fs.Int("duration", 60, "Episode duration in minutes")
	episode := fs.String("episode", "", "Episode number (e.g., 'S01E03')")
	title := fs.String("title", "", "Episode title")
	riverside := fs.Bool("riverside", true, "Include Riverside.fm integration")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: pedrocli podcast schedule [flags]

Create a Cal.com event type and booking link for podcast episode recording.

This command integrates with Cal.com to:
- Create or update an event type with episode details
- Configure Riverside.fm integration for recording
- Generate a shareable booking link for guests

Examples:
  pedrocli podcast schedule -template outline.md -duration 60
  pedrocli podcast schedule -episode "S01E03" -title "How to Choose a Model" -duration 90
  pedrocli podcast schedule -episode "S01E03" -riverside=false

Flags:
`)
		fs.PrintDefaults()
	}

	fs.Parse(args)

	// Validate flags
	if *episode == "" && *template == "" {
		fmt.Fprintln(os.Stderr, "Error: either -episode or -template must be specified")
		fs.Usage()
		os.Exit(1)
	}

	// Load template if provided
	var templateContent string
	if *template != "" {
		data, err := os.ReadFile(*template)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to read template file: %v\n", err)
			os.Exit(1)
		}
		templateContent = string(data)
	}

	// TODO: Create storage (file-based for CLI)
	// TODO: Create unified podcast agent with schedule workflow
	// TODO: Execute agent
	// TODO: Output booking URL

	fmt.Printf("⏳ Creating booking link...\n")
	fmt.Printf("🎙️  Episode: %s\n", *episode)
	if *title != "" {
		fmt.Printf("📝 Title: %s\n", *title)
	}
	fmt.Printf("⏱️  Duration: %d minutes\n", *duration)
	fmt.Printf("🎥 Riverside.fm: %t\n", *riverside)

	if *template != "" {
		fmt.Printf("📄 Template: %d characters\n", len(templateContent))
	}

	// TODO: Implement actual agent execution
	fmt.Fprintln(os.Stderr, "\nError: podcast scheduling not yet implemented - coming in PR #3 (UnifiedPodcastAgent)")
	os.Exit(1)
}

func podcastPrepCmd(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("podcast prep", flag.ExitOnError)
	outline := fs.String("outline", "", "Path to outline file (required)")
	episode := fs.String("episode", "", "Episode number (required)")
	focus := fs.String("focus", "AI", "Focus topic for news filtering")
	duration := fs.Int("duration", 60, "Episode duration in minutes")
	output := fs.String("output", "", "Output directory for prep package")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: pedrocli podcast prep [flags]

Run the complete podcast preparation workflow:
1. Generate episode script from outline
2. Review and summarize relevant news
3. Create Cal.com booking link

This is a convenience command that runs all three workflows in sequence
and outputs a combined prep package.

Examples:
  pedrocli podcast prep -outline outline.md -episode "S01E03"
  pedrocli podcast prep -outline outline.md -episode "S01E03" -focus "model selection" -duration 90
  pedrocli podcast prep -outline outline.md -episode "S01E03" -output prep-package/

Flags:
`)
		fs.PrintDefaults()
	}

	fs.Parse(args)

	// Validate flags
	if *outline == "" {
		fmt.Fprintln(os.Stderr, "Error: -outline must be specified")
		fs.Usage()
		os.Exit(1)
	}
	if *episode == "" {
		fmt.Fprintln(os.Stderr, "Error: -episode must be specified")
		fs.Usage()
		os.Exit(1)
	}

	// Load outline
	outlineData, err := os.ReadFile(*outline)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to read outline file: %v\n", err)
		os.Exit(1)
	}
	outlineContent := string(outlineData)

	// Create output directory if specified
	if *output != "" {
		if err := os.MkdirAll(*output, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to create output directory: %v\n", err)
			os.Exit(1)
		}
	}

	// TODO: Create storage (file-based for CLI)
	// TODO: Create unified podcast agent with full prep workflow
	// TODO: Execute all three workflows in sequence
	// TODO: Output combined results

	fmt.Printf("⏳ Running full episode prep for %s...\n", *episode)
	fmt.Printf("📝 Outline: %d characters\n", len(outlineContent))
	fmt.Printf("🎯 News focus: %s\n", *focus)
	fmt.Printf("⏱️  Duration: %d minutes\n", *duration)

	if *output != "" {
		fmt.Printf("💾 Output directory: %s\n", *output)
	}

	// TODO: Implement actual agent execution
	// TODO: Save results to:
	//   - script.md
	//   - news-summary.md
	//   - booking-link.txt

	fmt.Fprintln(os.Stderr, "\nError: full podcast prep not yet implemented - coming in PR #3 (UnifiedPodcastAgent)")
	os.Exit(1)
}

// podcastContentDir returns the base directory for podcast content storage
func podcastContentDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".pedrocli/content/podcast"
	}
	return filepath.Join(home, ".pedrocli", "content", "podcast")
}
