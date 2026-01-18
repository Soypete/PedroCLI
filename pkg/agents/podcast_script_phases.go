package agents

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Script Generation Phases
// These implement the 6-phase workflow for generating podcast episode scripts

// phaseParseOutline extracts episode structure from the outline
func (a *UnifiedPodcastAgent) phaseParseOutline(ctx context.Context) error {
	fmt.Println("   📄 Parsing outline...")

	if a.outline == "" {
		// No outline provided, use episode metadata instead
		a.currentContent.Data["segments"] = []string{
			"Introduction",
			"Main Discussion",
			"Rapid-Fire Q&A",
			"Outro",
		}
		return nil
	}

	// Store outline in content data
	a.currentContent.Data["outline_parsed"] = true
	a.currentContent.Data["outline_length"] = len(a.outline)

	// TODO: Use LLM to parse outline structure
	// For now, just store the raw outline
	a.currentContent.Data["outline_raw"] = a.outline

	fmt.Printf("   📝 Parsed outline (%d characters)\n", len(a.outline))
	return nil
}

// phaseResearchTopics searches for relevant background information
func (a *UnifiedPodcastAgent) phaseResearchTopics(ctx context.Context) error {
	fmt.Println("   🔍 Researching topics...")

	// TODO: Implement research using web_search and rss_feed tools
	// For now, mark research as complete
	a.currentContent.Data["research_complete"] = true

	fmt.Println("   📚 Research complete")
	return nil
}

// phaseGenerateSegments creates intro, main segments, Q&A, and outro
func (a *UnifiedPodcastAgent) phaseGenerateSegments(ctx context.Context) error {
	fmt.Println("   ✍️  Generating script segments...")

	// TODO: Use LLM to generate each segment based on outline and research
	// For now, create placeholder segments
	segments := make(map[string]string)

	segments["intro"] = a.generateIntroSegment()
	segments["main"] = a.generateMainSegment()
	segments["qa"] = a.generateQASegment()
	segments["outro"] = a.generateOutroSegment()

	a.currentContent.Data["segments"] = segments

	fmt.Println("   ✅ Generated 4 script segments")
	return nil
}

// phaseAssembleScript combines segments into a cohesive script
func (a *UnifiedPodcastAgent) phaseAssembleScript(ctx context.Context) error {
	fmt.Println("   🔧 Assembling complete script...")

	segments, ok := a.currentContent.Data["segments"].(map[string]string)
	if !ok {
		return fmt.Errorf("no segments found to assemble")
	}

	// Assemble script in order
	var scriptBuilder strings.Builder

	// Header
	scriptBuilder.WriteString(fmt.Sprintf("# Episode %s: %s\n\n", a.episode, a.title))
	if a.guests != "" {
		scriptBuilder.WriteString(fmt.Sprintf("**Guests**: %s\n", a.guests))
	}
	scriptBuilder.WriteString(fmt.Sprintf("**Duration**: %d minutes\n\n", a.duration))
	scriptBuilder.WriteString("---\n\n")

	// Segments
	scriptBuilder.WriteString("## Introduction (5 min)\n\n")
	scriptBuilder.WriteString(segments["intro"])
	scriptBuilder.WriteString("\n\n")

	scriptBuilder.WriteString(fmt.Sprintf("## Main Discussion (%d min)\n\n", a.duration-20))
	scriptBuilder.WriteString(segments["main"])
	scriptBuilder.WriteString("\n\n")

	scriptBuilder.WriteString("## Rapid-Fire Q&A (10 min)\n\n")
	scriptBuilder.WriteString(segments["qa"])
	scriptBuilder.WriteString("\n\n")

	scriptBuilder.WriteString("## Outro (5 min)\n\n")
	scriptBuilder.WriteString(segments["outro"])
	scriptBuilder.WriteString("\n")

	a.script = scriptBuilder.String()
	a.currentContent.Data["script"] = a.script

	fmt.Printf("   📝 Assembled script (%d characters)\n", len(a.script))
	return nil
}

// phaseReviewScript checks grammar, coherence, and timing
func (a *UnifiedPodcastAgent) phaseReviewScript(ctx context.Context) error {
	fmt.Println("   🔍 Reviewing script...")

	if a.script == "" {
		return fmt.Errorf("no script to review")
	}

	// TODO: Use LLM to review and suggest edits
	// For now, just mark as reviewed
	a.currentContent.Data["reviewed"] = true
	a.currentContent.Data["review_timestamp"] = fmt.Sprintf("%v", ctx.Value("timestamp"))

	fmt.Println("   ✅ Script review complete")
	return nil
}

// phasePublishScript saves script to Notion Scripts database
func (a *UnifiedPodcastAgent) phasePublishScript(ctx context.Context) error {
	fmt.Println("   📤 Publishing script...")

	if a.script == "" {
		return fmt.Errorf("no script to publish")
	}

	// Publish to Notion Scripts database if enabled
	if a.config.Podcast.Notion.Enabled {
		notionTool, ok := a.tools["notion"]
		if !ok {
			fmt.Println("   ⚠️  Notion tool not available, saving to storage only")
		} else {
			fmt.Println("   📝 Creating Notion page in Scripts database...")

			// Prepare properties for Notion page
			properties := map[string]interface{}{
				"Episode #": a.episode,                    // Title property
				"Title":     a.title,                      // Rich text
				"Guests":    a.guests,                     // Rich text
				"Status":    "Draft",                      // Status/Select property
				"Duration":  float64(a.duration),          // Number property
			}

			// Create page in Scripts database
			result, err := notionTool.Execute(ctx, map[string]interface{}{
				"action":        "create_page",
				"database_name": "scripts",
				"properties":    properties,
				"content":       a.script,
			})

			if err != nil {
				fmt.Printf("   ⚠️  Failed to publish to Notion: %v\n", err)
			} else if !result.Success {
				fmt.Printf("   ⚠️  Notion publishing error: %s\n", result.Error)
			} else {
				fmt.Println("   ✅ Script published to Notion Scripts database")
				a.currentContent.Data["notion_page_created"] = true
				a.currentContent.Data["notion_output"] = result.Output
			}
		}
	} else {
		fmt.Println("   ℹ️  Notion integration disabled, saving to storage only")
	}

	// Mark as published in content store
	a.currentContent.Data["published"] = true
	a.currentContent.Data["publish_timestamp"] = fmt.Sprintf("%v", time.Now().UTC())

	fmt.Println("   ✅ Script saved to storage")
	return nil
}

// Helper functions for generating placeholder segments

func (a *UnifiedPodcastAgent) generateIntroSegment() string {
	var intro strings.Builder

	intro.WriteString(fmt.Sprintf("Welcome to SoypeteTech, episode %s!\n\n", a.episode))

	if a.guests != "" {
		intro.WriteString(fmt.Sprintf("Today we're joined by %s.\n\n", a.guests))
	}

	intro.WriteString(fmt.Sprintf("In this episode, we'll be diving into: %s\n\n", a.title))
	intro.WriteString("Let's get started!\n")

	return intro.String()
}

func (a *UnifiedPodcastAgent) generateMainSegment() string {
	// TODO: Parse outline and generate discussion points
	return "[Main discussion content based on outline and research]\n\n" +
		"Topics to cover:\n" +
		"- Key concepts and background\n" +
		"- Real-world applications\n" +
		"- Best practices and recommendations\n"
}

func (a *UnifiedPodcastAgent) generateQASegment() string {
	return "[Rapid-fire Q&A section]\n\n" +
		"Quick questions:\n" +
		"1. What's your top tip for...\n" +
		"2. Biggest mistake to avoid...\n" +
		"3. What's next in this space...\n"
}

func (a *UnifiedPodcastAgent) generateOutroSegment() string {
	var outro strings.Builder

	outro.WriteString("Thanks for listening to this episode!\n\n")

	if a.guests != "" {
		outro.WriteString(fmt.Sprintf("Big thanks to our guests %s for joining us today.\n\n", a.guests))
	}

	outro.WriteString("Find us at:\n")
	outro.WriteString("- Discord: https://discord.gg/soypete\n")
	outro.WriteString("- YouTube: https://youtube.com/@soypete\n")
	outro.WriteString("- Twitter: https://twitter.com/soypete\n\n")

	outro.WriteString("See you next time!\n")

	return outro.String()
}
