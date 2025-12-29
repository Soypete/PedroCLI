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
Generate a structured episode script based on the provided topic and notes using the episode template.

## Episode Template Structure

Use this format for all episode scripts:

# 🧾 Episode Details
- Episode # / Title: [Episode number and title]
- Hosts: [Host names]
- Guests: [Guest names if any]
- Recording Date: [Date if known]
- Publish Date: [Date if known]
- Status: Idea / Recording / Editing / Released
- Notes: [Key points and context]

# ⏱ Segment Outline

**00:00 – 01:00  Intro**
- Quick host intros
- One-line show summary
- Optional: mention tagline or mission
- Notes / timestamps:

**01:00 – 10:00  News Segment**
- Each host brings one or two articles or updates
- Key discussion points:
  • Why does this update matter?
  • What changed this week?
  • Quick takes or concerns
- Paste links here:
- Summary notes / reactions:

**10:00 – 12:00  Sponsor / Community Shout-out**
- Placeholder for sponsor or community mention
- Read text / insert mid-roll marker:

**12:00 – 35:00  Main Conversation**
- Central topic / guest theme: [Topic]
- Discussion prompts:
  • Background and how you got started
  • What's your current setup?
  • What tools, stacks, or workflows are you using?
  • Why does this topic matter to you?
  • What are the current challenges or limits?
- Follow-ups / quotes / time markers:

**35:00 – 42:00  Reflection / Future Plans**
- What stood out today?
- What topics or guests are coming next?
- Any new tools to explore?
- Key takeaways / clips to pull:

**42:00 – 45:00  Outro**
- Recap key points / favorite moment
- Mention where to find the show
- Tease next episode topic or guest
- Sign-off line
- Outro music / notes / timestamp:

## Instructions
1. Fill in the episode details section with the provided topic and notes
2. Expand the Main Conversation section based on the specific topic
3. Add relevant discussion points and prompts
4. Keep other sections as template structure for hosts to fill in

## Output
First, create a local file with the script content using the template above.
Then, save to the Notion "Episode Planner" database with these exact properties:
- Episode # (title property): Episode number and title (e.g., "S01E01 - Topic Name")
- Title / Working Topic: Episode topic/title
- Status 🎛: "Draft"
- Recording Date: Recording date if known
- Notes: Link to the script file or key context

Use the file tool to create the script, then the notion tool to create the database entry.

Example:
{"tool": "file", "args": {"action": "write", "path": "episode_S01E01.md", "content": "..."}}
{"tool": "notion", "args": {"action": "create_page", "database_id": "...", "properties": {"Episode #": "S01E01 - Topic", "Title / Working Topic": "Topic here", "Status 🎛": "Draft"}}}
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
   - Status 🎛: "To Review"
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
- Status 🎛: "Outline"
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
