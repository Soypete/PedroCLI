package webscrape

import (
	"regexp"
	"strings"
)

// ContentExtractor extracts content from HTML
type ContentExtractor interface {
	Extract(html string, url string) (*ExtractedContent, error)
	CanHandle(url string) bool
}

// ReadabilityExtractor extracts clean text using a readability-like algorithm
type ReadabilityExtractor struct{}

// NewReadabilityExtractor creates a new readability extractor
func NewReadabilityExtractor() *ReadabilityExtractor {
	return &ReadabilityExtractor{}
}

// CanHandle returns true - the readability extractor can handle any URL
func (e *ReadabilityExtractor) CanHandle(url string) bool {
	return true
}

// Extract extracts clean content from HTML
func (e *ReadabilityExtractor) Extract(html string, url string) (*ExtractedContent, error) {
	content := &ExtractedContent{
		Metadata: make(map[string]string),
	}

	// Extract title
	content.Title = extractTitle(html)

	// Extract code blocks first (before stripping HTML)
	content.CodeBlocks = extractCodeBlocks(html)

	// Extract and clean main text
	content.MainText = extractCleanText(html)

	// Extract metadata
	content.Metadata["description"] = extractMetaDescription(html)

	return content, nil
}

// CodeExtractor specializes in extracting code from pages
type CodeExtractor struct{}

// NewCodeExtractor creates a new code extractor
func NewCodeExtractor() *CodeExtractor {
	return &CodeExtractor{}
}

// CanHandle returns true for technical sites
func (e *CodeExtractor) CanHandle(url string) bool {
	techDomains := []string{
		"github.com", "gitlab.com", "stackoverflow.com",
		"gist.github.com", "pastebin.com", "codepen.io",
	}
	for _, domain := range techDomains {
		if strings.Contains(url, domain) {
			return true
		}
	}
	return false
}

// Extract extracts code-focused content from HTML
func (e *CodeExtractor) Extract(html string, url string) (*ExtractedContent, error) {
	content := &ExtractedContent{
		Metadata: make(map[string]string),
	}

	content.Title = extractTitle(html)
	content.CodeBlocks = extractCodeBlocks(html)
	content.MainText = extractCleanText(html)

	return content, nil
}

// Helper functions for HTML parsing (simple regex-based approach)
// For production, consider using golang.org/x/net/html

func extractTitle(html string) string {
	// Try to find <title> tag
	titleRe := regexp.MustCompile(`(?is)<title[^>]*>([^<]+)</title>`)
	if matches := titleRe.FindStringSubmatch(html); len(matches) > 1 {
		return cleanHTMLText(matches[1])
	}

	// Try to find <h1> tag
	h1Re := regexp.MustCompile(`(?is)<h1[^>]*>([^<]+)</h1>`)
	if matches := h1Re.FindStringSubmatch(html); len(matches) > 1 {
		return cleanHTMLText(matches[1])
	}

	return ""
}

func extractMetaDescription(html string) string {
	// Meta description
	descRe := regexp.MustCompile(`(?is)<meta[^>]+name=["']description["'][^>]+content=["']([^"']+)["']`)
	if matches := descRe.FindStringSubmatch(html); len(matches) > 1 {
		return cleanHTMLText(matches[1])
	}

	// Try reverse order
	descRe2 := regexp.MustCompile(`(?is)<meta[^>]+content=["']([^"']+)["'][^>]+name=["']description["']`)
	if matches := descRe2.FindStringSubmatch(html); len(matches) > 1 {
		return cleanHTMLText(matches[1])
	}

	return ""
}

func extractCodeBlocks(html string) []CodeBlock {
	var blocks []CodeBlock

	// Helper to extract language from tag attributes
	langRe := regexp.MustCompile(`class=["'][^"']*(?:language-|lang-)([a-zA-Z0-9]+)`)

	// Extract <pre><code> blocks - capture the code tag with its attributes
	preCodeRe := regexp.MustCompile(`(?is)<pre[^>]*>\s*<code([^>]*)>(.*?)</code>\s*</pre>`)
	matches := preCodeRe.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		attrs := match[1]
		code := match[2]
		lang := ""
		if langMatch := langRe.FindStringSubmatch(attrs); len(langMatch) > 1 {
			lang = langMatch[1]
		}
		blocks = append(blocks, CodeBlock{
			Language: lang,
			Code:     cleanCodeBlock(code),
		})
	}

	// Extract standalone <pre> blocks - capture the pre tag with its attributes
	preRe := regexp.MustCompile(`(?is)<pre([^>]*)>(.*?)</pre>`)
	preMatches := preRe.FindAllStringSubmatch(html, -1)
	for _, match := range preMatches {
		attrs := match[1]
		code := match[2]
		// Skip if this looks like it was already captured as pre>code
		if strings.Contains(code, "<code") {
			continue
		}
		lang := ""
		if langMatch := langRe.FindStringSubmatch(attrs); len(langMatch) > 1 {
			lang = langMatch[1]
		}
		blocks = append(blocks, CodeBlock{
			Language: lang,
			Code:     cleanCodeBlock(code),
		})
	}

	// Extract fenced code blocks from markdown content
	fencedRe := regexp.MustCompile("(?m)```([a-zA-Z0-9]*)\n([^`]+)```")
	fencedMatches := fencedRe.FindAllStringSubmatch(html, -1)
	for _, match := range fencedMatches {
		lang := ""
		code := match[2]
		if len(match) > 1 && match[1] != "" {
			lang = match[1]
		}
		blocks = append(blocks, CodeBlock{
			Language: lang,
			Code:     strings.TrimSpace(code),
		})
	}

	// Deduplicate blocks
	seen := make(map[string]bool)
	var unique []CodeBlock
	for _, block := range blocks {
		key := block.Language + ":" + block.Code
		if !seen[key] && len(strings.TrimSpace(block.Code)) > 0 {
			seen[key] = true
			unique = append(unique, block)
		}
	}

	return unique
}

func extractCleanText(html string) string {
	// Remove script and style tags
	scriptRe := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = scriptRe.ReplaceAllString(html, "")

	styleRe := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = styleRe.ReplaceAllString(html, "")

	// Remove HTML comments
	commentRe := regexp.MustCompile(`(?s)<!--.*?-->`)
	html = commentRe.ReplaceAllString(html, "")

	// Remove navigation, header, footer, aside (each tag handled separately since Go RE2 doesn't support backreferences)
	for _, tag := range []string{"nav", "header", "footer", "aside"} {
		tagRe := regexp.MustCompile(`(?is)<` + tag + `[^>]*>.*?</` + tag + `>`)
		html = tagRe.ReplaceAllString(html, "")
	}

	// Convert block elements to newlines
	blockTags := []string{"div", "p", "br", "li", "h1", "h2", "h3", "h4", "h5", "h6", "tr", "td", "th"}
	for _, tag := range blockTags {
		re := regexp.MustCompile(`(?i)<` + tag + `[^>]*>`)
		html = re.ReplaceAllString(html, "\n")
		re = regexp.MustCompile(`(?i)</` + tag + `>`)
		html = re.ReplaceAllString(html, "\n")
	}

	// Remove remaining HTML tags
	tagRe := regexp.MustCompile(`<[^>]+>`)
	html = tagRe.ReplaceAllString(html, "")

	// Decode HTML entities
	html = decodeHTMLEntities(html)

	// Normalize whitespace
	html = normalizeWhitespace(html)

	return strings.TrimSpace(html)
}

func cleanHTMLText(text string) string {
	// Remove HTML tags
	tagRe := regexp.MustCompile(`<[^>]+>`)
	text = tagRe.ReplaceAllString(text, "")

	// Decode entities
	text = decodeHTMLEntities(text)

	// Normalize whitespace
	return strings.TrimSpace(normalizeWhitespace(text))
}

func cleanCodeBlock(code string) string {
	// Remove HTML tags from code
	tagRe := regexp.MustCompile(`<[^>]+>`)
	code = tagRe.ReplaceAllString(code, "")

	// Decode HTML entities
	code = decodeHTMLEntities(code)

	return strings.TrimSpace(code)
}

func decodeHTMLEntities(text string) string {
	entities := map[string]string{
		"&amp;":    "&",
		"&lt;":     "<",
		"&gt;":     ">",
		"&quot;":   `"`,
		"&#39;":    "'",
		"&apos;":   "'",
		"&nbsp;":   " ",
		"&copy;":   "(c)",
		"&reg;":    "(R)",
		"&trade;":  "(TM)",
		"&ndash;":  "-",
		"&mdash;":  "--",
		"&lsquo;":  "'",
		"&rsquo;":  "'",
		"&ldquo;":  `"`,
		"&rdquo;":  `"`,
		"&bull;":   "*",
		"&hellip;": "...",
	}

	for entity, replacement := range entities {
		text = strings.ReplaceAll(text, entity, replacement)
	}

	// Handle numeric entities
	numericRe := regexp.MustCompile(`&#(\d+);`)
	text = numericRe.ReplaceAllStringFunc(text, func(s string) string {
		matches := numericRe.FindStringSubmatch(s)
		if len(matches) > 1 {
			var code int
			if _, err := parseIntFromString(matches[1]); err == nil {
				code, _ = parseIntFromString(matches[1])
				if code > 0 && code < 128 {
					return string(rune(code))
				}
			}
		}
		return s
	})

	return text
}

func normalizeWhitespace(text string) string {
	// Replace multiple newlines with double newline
	multiNewline := regexp.MustCompile(`\n{3,}`)
	text = multiNewline.ReplaceAllString(text, "\n\n")

	// Replace multiple spaces with single space
	multiSpace := regexp.MustCompile(`[ \t]+`)
	text = multiSpace.ReplaceAllString(text, " ")

	// Trim lines
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	text = strings.Join(lines, "\n")

	return text
}

func parseIntFromString(s string) (int, error) {
	var result int
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		result = result*10 + int(c-'0')
	}
	return result, nil
}

// ExtractLinks extracts all links from HTML
func ExtractLinks(html string) []Link {
	var links []Link

	linkRe := regexp.MustCompile(`(?is)<a[^>]+href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	matches := linkRe.FindAllStringSubmatch(html, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			url := match[1]
			text := cleanHTMLText(match[2])

			// Skip empty or fragment-only links
			if url == "" || strings.HasPrefix(url, "#") {
				continue
			}

			// Determine link type
			linkType := ""
			lowerURL := strings.ToLower(url)
			if strings.Contains(lowerURL, "github.com") || strings.Contains(lowerURL, "gitlab.com") {
				linkType = "source"
			} else if strings.Contains(lowerURL, "docs") || strings.Contains(lowerURL, "documentation") {
				linkType = "documentation"
			}

			links = append(links, Link{
				Text: text,
				URL:  url,
				Type: linkType,
			})
		}
	}

	return links
}

// DetectLanguage attempts to detect the programming language of a code block
func DetectLanguage(code string) string {
	// Simple heuristic-based detection
	code = strings.TrimSpace(code)

	// Go
	if strings.Contains(code, "func ") && strings.Contains(code, "package ") {
		return "go"
	}
	if strings.HasPrefix(code, "package ") {
		return "go"
	}

	// Python
	if strings.HasPrefix(code, "def ") || strings.HasPrefix(code, "class ") {
		return "python"
	}
	if strings.Contains(code, "import ") && strings.Contains(code, ":") {
		return "python"
	}

	// TypeScript (check first as it's more specific)
	if strings.Contains(code, ": ") && (strings.Contains(code, "interface ") || strings.Contains(code, "type ")) {
		return "typescript"
	}

	// JavaScript
	if strings.Contains(code, "function ") || strings.Contains(code, "const ") || strings.Contains(code, "let ") {
		return "javascript"
	}

	// Rust
	if strings.Contains(code, "fn ") && strings.Contains(code, "let ") {
		return "rust"
	}

	// Java
	if strings.Contains(code, "public class ") || strings.Contains(code, "private ") {
		return "java"
	}

	// HTML
	if strings.Contains(code, "<!DOCTYPE") || strings.Contains(code, "<html") {
		return "html"
	}

	// CSS
	if strings.Contains(code, "{") && (strings.Contains(code, "color:") || strings.Contains(code, "margin:") || strings.Contains(code, "padding:")) {
		return "css"
	}

	// Shell/Bash
	if strings.HasPrefix(code, "#!/bin/") || strings.HasPrefix(code, "$ ") {
		return "bash"
	}

	// JSON
	if (strings.HasPrefix(code, "{") && strings.HasSuffix(code, "}")) ||
		(strings.HasPrefix(code, "[") && strings.HasSuffix(code, "]")) {
		return "json"
	}

	// YAML
	if strings.Contains(code, "---") && strings.Contains(code, ":") {
		return "yaml"
	}

	// SQL
	if strings.Contains(strings.ToUpper(code), "SELECT ") || strings.Contains(strings.ToUpper(code), "CREATE TABLE") {
		return "sql"
	}

	return ""
}
