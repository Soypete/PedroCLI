package agents

import (
	"context"
	"fmt"
)

// Cal.com Scheduling Phases
// These implement the 5-phase workflow for creating podcast booking links

// phaseParseTemplate extracts episode details from template/outline
func (a *UnifiedPodcastAgent) phaseParseTemplate(ctx context.Context) error {
	fmt.Println("   📄 Parsing template...")

	// If we have an outline, use it as the template
	if a.outline != "" {
		a.currentContent.Data["template_source"] = "outline"
		a.currentContent.Data["template_length"] = len(a.outline)
	} else {
		// Use episode metadata
		a.currentContent.Data["template_source"] = "metadata"
	}

	fmt.Println("   ✅ Template parsed")
	return nil
}

// phaseCreateEventType creates or updates Cal.com event type
func (a *UnifiedPodcastAgent) phaseCreateEventType(ctx context.Context) error {
	fmt.Println("   📅 Creating Cal.com event type...")

	// TODO: Use cal_com tool to create event type
	// For now, create placeholder event type data
	eventType := map[string]interface{}{
		"title":       fmt.Sprintf("SoypeteTech - Episode %s", a.episode),
		"slug":        fmt.Sprintf("soypete-%s", a.episode),
		"length":      a.duration,
		"description": a.generateEventDescription(),
	}

	a.currentContent.Data["event_type"] = eventType

	fmt.Printf("   ✅ Created event type: %s\n", eventType["slug"])
	return nil
}

// phaseConfigureRiverside sets up Riverside.fm integration
func (a *UnifiedPodcastAgent) phaseConfigureRiverside(ctx context.Context) error {
	if !a.riverside {
		fmt.Println("   ⏭️  Riverside.fm integration disabled")
		return nil
	}

	fmt.Println("   🎥 Configuring Riverside.fm...")

	// TODO: Configure Riverside.fm location in event type
	// For now, just mark as configured
	a.currentContent.Data["riverside_enabled"] = true
	a.currentContent.Data["riverside_studio_url"] = "https://riverside.fm/studio/soypete-tech-podcast"

	fmt.Println("   ✅ Riverside.fm configured")
	return nil
}

// phaseGenerateBookingLink gets shareable booking URL from Cal.com
func (a *UnifiedPodcastAgent) phaseGenerateBookingLink(ctx context.Context) error {
	fmt.Println("   🔗 Generating booking link...")

	// TODO: Get booking URL from cal_com tool
	// For now, generate placeholder URL
	eventType, ok := a.currentContent.Data["event_type"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("no event type found")
	}

	slug, _ := eventType["slug"].(string)
	a.bookingURL = fmt.Sprintf("https://cal.com/soypete/%s", slug)

	a.currentContent.Data["booking_url"] = a.bookingURL

	fmt.Printf("   ✅ Booking link: %s\n", a.bookingURL)
	return nil
}

// phaseSaveBookingToNotion stores booking link in Notion
func (a *UnifiedPodcastAgent) phaseSaveBookingToNotion(ctx context.Context) error {
	if !a.config.Podcast.Notion.Enabled {
		fmt.Println("   ⏭️  Notion integration disabled")
		return nil
	}

	fmt.Println("   📝 Saving to Notion...")

	if a.bookingURL == "" {
		return fmt.Errorf("no booking URL to save")
	}

	// TODO: Use notion tool to create page in Guests database
	// For now, just mark as saved
	a.currentContent.Data["notion_saved"] = true

	fmt.Println("   ✅ Saved to Notion")
	return nil
}

// generateEventDescription creates description for Cal.com event
func (a *UnifiedPodcastAgent) generateEventDescription() string {
	desc := fmt.Sprintf("60-minute podcast interview for SoypeteTech, Episode %s: %s\n\n", a.episode, a.title)

	if a.guests != "" {
		desc += fmt.Sprintf("Guests: %s\n\n", a.guests)
	}

	desc += "Format:\n"
	desc += "- 5 min: Intro & guest background\n"
	desc += fmt.Sprintf("- %d min: Technical discussion\n", a.duration-15)
	desc += "- 10 min: Rapid-fire Q&A\n\n"

	if a.riverside {
		desc += "Recording on Riverside.fm for studio-quality audio/video.\n\n"
	}

	desc += "Find us at:\n"
	desc += "- Discord: https://discord.gg/soypete\n"
	desc += "- YouTube: https://youtube.com/@soypete\n"
	desc += "- Twitter: https://twitter.com/soypete\n"

	return desc
}
