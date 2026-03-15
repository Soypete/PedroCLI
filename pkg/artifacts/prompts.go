package artifacts

import "fmt"

// SummaryPrompts returns system and user prompts for generating a narrative summary.
func SummaryPrompts(text string) (system, user string) {
	system = `You are a study assistant. Write a 3-5 paragraph narrative summary of the provided source material. Ground every claim in the source; do not add information that is not present.`
	user = fmt.Sprintf("Summarize this source material:\n\n%s", text)
	return
}

// FAQPrompts returns system and user prompts for generating FAQ pairs.
func FAQPrompts(text string) (system, user string) {
	system = `You are a study assistant. Generate 8-12 FAQ pairs based solely on the provided source material. Return a JSON array of objects with "q" and "a" keys. Answers must cite only information from the source.`
	user = fmt.Sprintf("Generate FAQ pairs from this source material:\n\n%s", text)
	return
}

// StudyGuidePrompts returns system and user prompts for generating a structured study guide.
func StudyGuidePrompts(text string) (system, user string) {
	system = `You are a study assistant. Create a structured study guide from the provided source material. Return JSON with three keys:
- "key_concepts": array of {"term": string, "definition": string} (1-sentence definitions)
- "notable_quotes": array of {"quote": string, "context": string} (3 quotes with context)
- "comprehension_qa": array of {"question": string, "answer": string} (5 Q&A pairs)`
	user = fmt.Sprintf("Create a study guide from this source material:\n\n%s", text)
	return
}

// TimelinePrompts returns system and user prompts for extracting a timeline.
func TimelinePrompts(text string) (system, user string) {
	system = `You are a study assistant. Extract all explicitly dated events from the provided source material. Return a JSON array of objects with "date", "event", and "significance" keys. Only include dates that are explicitly stated in the source.`
	user = fmt.Sprintf("Extract a timeline from this source material:\n\n%s", text)
	return
}

// DigestPrompts returns system and user prompts for cross-item synthesis.
func DigestPrompts(feedTitle string, summaries []string) (system, user string) {
	system = `You are a study assistant. Synthesize the provided summaries from a single feed into a digest. Return JSON with three keys:
- "major_themes": array of strings identifying common themes across items
- "key_developments": array of strings noting significant developments
- "recurring_entities": array of strings listing people, orgs, or concepts that appear repeatedly`

	combined := ""
	for i, s := range summaries {
		combined += fmt.Sprintf("--- Item %d ---\n%s\n\n", i+1, s)
	}
	user = fmt.Sprintf("Create a digest for feed %q from these summaries:\n\n%s", feedTitle, combined)
	return
}
