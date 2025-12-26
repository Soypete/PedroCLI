package prompts

// Default podcast system prompt with TODO placeholders
const defaultPodcastSystemPrompt = `You are an AI assistant helping to produce the podcast "{{.PodcastName}}".

## Podcast Overview
{{.PodcastDescription}}

## Format
{{.PodcastFormat}}

## Team
{{.CohostList}}

## Resources
- Scripts Database: {{.NotionScriptsDB}}
- Articles Database: {{.NotionArticlesDB}}
- News Database: {{.NotionNewsDB}}
- Guests Database: {{.NotionGuestsDB}}
- Recording Calendar: {{.CalendarID}}
- Recording Platform: {{.RecordingPlatform}}

## Your Capabilities
You have access to the following tools:
- notion: Query and update Notion databases for content management
- calendar: View and create calendar events for recording sessions
- file: Read and write local files for scripts and notes

## Guidelines
1. Always maintain the podcast's tone and style
2. Ensure content is accurate and well-researched
3. Coordinate with the team calendar for scheduling
4. Keep scripts organized in the appropriate Notion database
5. Flag any potential issues or concerns for human review
`

// Default prompts for each podcast job type
var defaultPodcastPrompts = map[string]string{
	"create_podcast_script": `Create a podcast script for "{{.PodcastName}}".

## Task
Generate a structured episode script based on the provided topic and notes.

## Script Structure
1. **Cold Open** (30 seconds)
   - Hook the audience with an interesting fact or question

2. **Introduction** (1-2 minutes)
   - Welcome listeners
   - Introduce hosts
   - Preview today's topic

3. **Main Content** (15-25 minutes)
   - Present the topic with clear segments
   - Include discussion points for hosts
   - Add transitions between segments

4. **Guest Segment** (if applicable) (10-15 minutes)
   - Introduction of guest
   - Interview questions
   - Discussion points

5. **Wrap-up** (2-3 minutes)
   - Summary of key points
   - Call to action
   - Teaser for next episode
   - Sign-off

## Formatting Guidelines
- Use clear headers for each segment
- Include timing estimates
- Mark host speaking parts (e.g., [HOST1], [HOST2])
- Include notes for tone/delivery where helpful
- Add placeholder notes for ad reads if needed

## Output
Save the script to the Notion Scripts database with:
- Title: Episode title
- Status: Draft
- Date: Recording date if known
- Notes: Any additional context

Use the notion tool to create the page in database: {{.NotionScriptsDB}}
`,

	"add_notion_link": `Add a link to the podcast content database.

## Task
Add the provided article/news link to the appropriate Notion database for review.

## Process
1. Analyze the provided URL to determine content type:
   - News article → News Review database ({{.NotionNewsDB}})
   - Technical article/blog → Articles Review database ({{.NotionArticlesDB}})
   - Potential episode topic → Potential Articles database

2. Extract key information:
   - Title
   - Source/Author
   - Publication date
   - Brief summary (1-2 sentences)
   - Relevant tags/topics

3. Create a new page in the appropriate database with:
   - Title: Article title
   - URL: Original link
   - Summary: Brief description
   - Status: "To Review"
   - Added Date: Today
   - Tags: Relevant topics

Use the notion tool to add to the appropriate database.
`,

	"add_guest": `Add a new guest to the podcast guest database.

## Task
Add guest information to the Notion Guests database.

## Required Information
Collect or use the following guest details:
- Name
- Title/Role
- Organization/Company
- Bio (2-3 sentences)
- Contact email
- Social media handles
- Topics of expertise
- Potential episode topics
- Availability notes
- Recording preferences

## Process
1. Create a new page in the Guests database ({{.NotionGuestsDB}})
2. Fill in all available information
3. Set status to "Potential" or "Confirmed" based on context
4. Add any scheduling notes

Use the notion tool to create the guest entry.
`,

	"create_episode_outline": `Create an episode outline for "{{.PodcastName}}".

## Task
Generate a high-level outline for a podcast episode without full script details.

## Outline Structure
1. **Episode Concept**
   - Working title
   - One-sentence summary
   - Target audience

2. **Key Points** (3-5 main topics)
   - Topic 1: Brief description
   - Topic 2: Brief description
   - ...

3. **Research Needed**
   - List of facts to verify
   - Sources to check
   - Experts to potentially quote

4. **Discussion Questions**
   - Open-ended questions for host discussion
   - Potential audience engagement questions

5. **Resources**
   - Related articles from our database
   - Previous episodes that connect
   - External references

## Output
Save the outline to Notion with:
- Status: "Outline"
- Type: "Episode Outline"

Use the notion tool to query relevant articles from {{.NotionArticlesDB}} and {{.NotionNewsDB}} for research.
`,

	"review_news_summary": `Summarize recent news items for podcast prep.

## Task
Review and summarize news items from the News Review database for upcoming episode preparation.

## Process
1. Query the News Review database ({{.NotionNewsDB}}) for items with status "To Review"

2. For each item, create a summary including:
   - Headline
   - Key points (2-3 bullets)
   - Why it matters for our audience
   - Potential discussion angles
   - Related topics from previous episodes

3. Group summaries by topic/theme

4. Prioritize items by:
   - Relevance to podcast themes
   - Timeliness
   - Audience interest potential

5. Create an output document with:
   - Executive summary (top 3-5 stories)
   - Detailed summaries organized by topic
   - Recommended stories for next episode
   - Stories to watch for future episodes

## Output
Update the status of reviewed items in Notion to "Reviewed" and add your summary notes.

Use the notion tool to query and update the database: {{.NotionNewsDB}}
`,

	"schedule_recording": `Schedule a podcast recording session.

## Task
Create a calendar event for a podcast recording session.

## Required Information
- Episode topic/title
- Preferred date and time
- Duration (typically 60-90 minutes)
- Participants (hosts, guests if any)
- Recording platform: {{.RecordingPlatform}}

## Process
1. Check the calendar ({{.CalendarID}}) for availability
2. Create a calendar event with:
   - Title: "[Recording] Episode Title"
   - Duration: As specified
   - Location: Recording platform link
   - Description: Episode outline, talking points, guest info if applicable
   - Attendees: All participants

3. Update the corresponding Notion script/outline entry with:
   - Recording date
   - Calendar event link

Use the calendar tool to check availability and create the event.
`,
}
