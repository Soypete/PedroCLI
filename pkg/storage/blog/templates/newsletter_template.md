---
# Newsletter Addendum Template
# This template is used to generate the newsletter section at the end of blog posts
---

## 📬 Newsletter Highlights

{{if .FeaturedVideo}}
### 🎥 Featured Video

{{.FeaturedVideo.Title}}

{{if .FeaturedVideo.EmbedCode}}
{{.FeaturedVideo.EmbedCode}}
{{else}}
[Watch on YouTube]({{.FeaturedVideo.URL}})
{{end}}

{{if .FeaturedVideo.Description}}
{{.FeaturedVideo.Description}}
{{end}}

{{end}}

{{if .UpcomingEvents}}
### 📅 Upcoming Events

{{range .UpcomingEvents}}
**{{.Title}}** - {{formatDate .EventDate "January 2, 2006"}}
{{if .Description}}
{{.Description}}
{{end}}
{{if .URL}}
[Learn more and register]({{.URL}})
{{end}}

{{end}}
{{end}}

{{if .MeetupHighlights}}
### 🤝 Meetup Highlights

{{range .MeetupHighlights}}
**{{.Title}}**{{if .EventDate}} - {{formatDate .EventDate "January 2, 2006"}}{{end}}
{{if .Description}}
{{.Description}}
{{end}}
{{if .URL}}
[Join us]({{.URL}})
{{end}}

{{end}}
{{end}}

{{if .CommunitySpotlight}}
### ✨ Community Spotlight

{{.CommunitySpotlight.Title}}

{{.CommunitySpotlight.Description}}

{{if .CommunitySpotlight.URL}}
[Check it out]({{.CommunitySpotlight.URL}})
{{end}}

{{end}}

{{if .Reading}}
### 📚 What I'm Reading/Watching

{{range .Reading}}
- [{{.Title}}]({{.URL}}){{if .Description}} - {{.Description}}{{end}}
{{end}}
{{end}}

{{if .Sponsor}}
### 💼 Sponsor

{{.Sponsor.Title}}

{{.Sponsor.Description}}

[Learn more]({{.Sponsor.URL}})

{{end}}

---

## Stay Connected

- 📧 **Subscribe** to this newsletter to get posts like this in your inbox
- 💬 **Join the discussion** on [Discord](https://discord.gg/soypete-tech) or [Twitter](https://twitter.com/soypete)
- 🔄 **Share** this post with someone who might find it useful
- ⭐ **Star** the projects mentioned if you find them helpful

Thanks for reading! 🙏
