---
# Newsletter Addendum Template
# This template is used to generate the newsletter section at the end of blog posts
# Note: Emojis are intentionally excluded per Soypete Tech brand guidelines
---

## Newsletter Highlights

{{if .YouTubePlaceholder}}
### Featured Video

{{.YouTubePlaceholder}}

{{end}}

{{if .UpcomingEvents}}
### Upcoming Events

{{range .UpcomingEvents}}
**{{.Title}}** - {{if .Date}}{{.Date}}{{end}}
{{if .Description}}
{{.Description}}
{{end}}
{{if .URL}}
[Learn more and register]({{.URL}})
{{end}}

{{end}}
{{end}}

{{if .RecentPosts}}
### Recent Posts You Might Have Missed

{{range .RecentPosts}}
- [{{.Title}}]({{.Link}}){{if .Date}} ({{.Date}}){{end}}
{{end}}

{{end}}

{{if .CommunitySpotlight}}
### Community Spotlight

{{.CommunitySpotlight.Title}}

{{.CommunitySpotlight.Description}}

{{if .CommunitySpotlight.URL}}
[Check it out]({{.CommunitySpotlight.URL}})
{{end}}

{{end}}

{{if .Reading}}
### What I'm Reading/Watching

{{range .Reading}}
- [{{.Title}}]({{.URL}}){{if .Description}} - {{.Description}}{{end}}
{{end}}
{{end}}

---

## Stay Connected

{{if .SocialLinks}}
{{if .SocialLinks.Discord}}
- [Join our Discord]({{.SocialLinks.Discord}})
{{end}}
{{if .SocialLinks.YouTube}}
- [Subscribe on YouTube]({{.SocialLinks.YouTube}})
{{end}}
{{if .SocialLinks.Twitter}}
- [Follow on Twitter/X]({{.SocialLinks.Twitter}})
{{end}}
{{if .SocialLinks.Bluesky}}
- [Follow on Bluesky]({{.SocialLinks.Bluesky}})
{{end}}
{{if .SocialLinks.LinkedIn}}
- [Connect on LinkedIn]({{.SocialLinks.LinkedIn}})
{{end}}
{{if .SocialLinks.Newsletter}}
- [Subscribe to Newsletter]({{.SocialLinks.Newsletter}})
{{end}}
{{if .SocialLinks.LinkTree}}
- [All links]({{.SocialLinks.LinkTree}})
{{end}}
{{range .SocialLinks.CustomLinks}}
- [{{.Name}}]({{.URL}})
{{end}}
{{else}}
- Subscribe to this newsletter to get posts like this in your inbox
- Join the discussion on Discord or Twitter
- Share this post with someone who might find it useful
{{end}}

Thanks for reading!
