package blog

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"
	"time"

	"github.com/google/uuid"
)

//go:embed templates/newsletter_template.md
var newsletterTemplate string

// NewsletterData holds data for rendering the newsletter addendum
type NewsletterData struct {
	FeaturedVideo      *NewsletterAsset   `json:"featured_video,omitempty"`
	UpcomingEvents     []*NewsletterAsset `json:"upcoming_events,omitempty"`
	MeetupHighlights   []*NewsletterAsset `json:"meetup_highlights,omitempty"`
	CommunitySpotlight *NewsletterAsset   `json:"community_spotlight,omitempty"`
	Reading            []*NewsletterAsset `json:"reading,omitempty"`
	Sponsor            *NewsletterAsset   `json:"sponsor,omitempty"`
}

// NewsletterBuilder helps build newsletter addendums
type NewsletterBuilder struct {
	store *NewsletterStore
	tmpl  *template.Template
}

// NewNewsletterBuilder creates a new newsletter builder
func NewNewsletterBuilder(store *NewsletterStore) (*NewsletterBuilder, error) {
	// Parse template with custom functions
	funcMap := template.FuncMap{
		"formatDate": func(t *time.Time, layout string) string {
			if t == nil {
				return ""
			}
			return t.Format(layout)
		},
	}

	tmpl, err := template.New("newsletter").Funcs(funcMap).Parse(newsletterTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse newsletter template: %w", err)
	}

	return &NewsletterBuilder{
		store: store,
		tmpl:  tmpl,
	}, nil
}

// BuildForPost builds a newsletter addendum for a blog post
func (b *NewsletterBuilder) BuildForPost(postID uuid.UUID) (string, error) {
	// Get unused assets
	data := &NewsletterData{}

	// Get featured video (most recent unused video)
	videoType := AssetVideo
	videos, err := b.store.List(AssetFilters{
		AssetType:  &videoType,
		OnlyUnused: true,
		Limit:      1,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get videos: %w", err)
	}
	if len(videos) > 0 {
		data.FeaturedVideo = videos[0]
	}

	// Get upcoming events
	upcomingEvents, err := b.store.GetUpcomingEvents()
	if err != nil {
		return "", fmt.Errorf("failed to get upcoming events: %w", err)
	}
	data.UpcomingEvents = upcomingEvents

	// Get meetup highlights (unused meetups)
	meetupType := AssetMeetup
	meetups, err := b.store.List(AssetFilters{
		AssetType:  &meetupType,
		OnlyUnused: true,
		Limit:      3,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get meetups: %w", err)
	}
	data.MeetupHighlights = meetups

	// Get reading/watching (unused links and reading)
	readingType := AssetReading
	reading, err := b.store.List(AssetFilters{
		AssetType:  &readingType,
		OnlyUnused: true,
		Limit:      5,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get reading: %w", err)
	}
	data.Reading = reading

	// Render template
	var buf bytes.Buffer
	if err := b.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render newsletter template: %w", err)
	}

	return buf.String(), nil
}

// BuildWithCustomData builds a newsletter addendum with custom data
func (b *NewsletterBuilder) BuildWithCustomData(data *NewsletterData) (string, error) {
	var buf bytes.Buffer
	if err := b.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render newsletter template: %w", err)
	}

	return buf.String(), nil
}

// MarkAssetsAsUsed marks all assets in the newsletter data as used for a post
func (b *NewsletterBuilder) MarkAssetsAsUsed(postID uuid.UUID, data *NewsletterData) error {
	var assetIDs []uuid.UUID

	if data.FeaturedVideo != nil {
		assetIDs = append(assetIDs, data.FeaturedVideo.ID)
	}

	for _, event := range data.UpcomingEvents {
		assetIDs = append(assetIDs, event.ID)
	}

	for _, meetup := range data.MeetupHighlights {
		assetIDs = append(assetIDs, meetup.ID)
	}

	if data.CommunitySpotlight != nil {
		assetIDs = append(assetIDs, data.CommunitySpotlight.ID)
	}

	for _, reading := range data.Reading {
		assetIDs = append(assetIDs, reading.ID)
	}

	if data.Sponsor != nil {
		assetIDs = append(assetIDs, data.Sponsor.ID)
	}

	// Mark all assets as used
	for _, assetID := range assetIDs {
		if err := b.store.MarkAsUsed(assetID, postID); err != nil {
			return fmt.Errorf("failed to mark asset %s as used: %w", assetID, err)
		}
	}

	return nil
}
