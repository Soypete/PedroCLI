package handlers

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/webscrape"
)

// StackOverflowHandler handles Stack Overflow-specific fetching operations
type StackOverflowHandler struct {
	apiKey      string
	httpClient  *http.Client
	rateLimiter *webscrape.RateLimiter
	extractor   *webscrape.CodeExtractor
}

// StackOverflowConfig configures the Stack Overflow handler
type StackOverflowConfig struct {
	APIKey  string // Optional, for higher rate limits
	Timeout time.Duration
}

// NewStackOverflowHandler creates a new Stack Overflow handler
func NewStackOverflowHandler(cfg *StackOverflowConfig) *StackOverflowHandler {
	if cfg == nil {
		cfg = &StackOverflowConfig{}
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	h := &StackOverflowHandler{
		apiKey: cfg.APIKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		rateLimiter: webscrape.NewRateLimiter(),
		extractor:   webscrape.NewCodeExtractor(),
	}

	// Stack Exchange API is generous with rate limits
	h.rateLimiter.SetLimit("api.stackexchange.com", 30)

	return h
}

// FetchQuestion fetches a Stack Overflow question by ID
func (s *StackOverflowHandler) FetchQuestion(ctx context.Context, questionID int) (*webscrape.SOQuestion, error) {
	apiURL := fmt.Sprintf("https://api.stackexchange.com/2.3/questions/%d?order=desc&sort=activity&site=stackoverflow&filter=withbody",
		questionID)

	if s.apiKey != "" {
		apiURL += "&key=" + url.QueryEscape(s.apiKey)
	}

	var response struct {
		Items []struct {
			QuestionID       int      `json:"question_id"`
			Title            string   `json:"title"`
			Body             string   `json:"body"`
			Tags             []string `json:"tags"`
			Score            int      `json:"score"`
			AcceptedAnswerID *int     `json:"accepted_answer_id"`
			AnswerCount      int      `json:"answer_count"`
			Link             string   `json:"link"`
			CreationDate     int64    `json:"creation_date"`
		} `json:"items"`
		HasMore        bool `json:"has_more"`
		QuotaMax       int  `json:"quota_max"`
		QuotaRemaining int  `json:"quota_remaining"`
	}

	if err := s.doAPIRequest(ctx, apiURL, &response); err != nil {
		return nil, err
	}

	if len(response.Items) == 0 {
		return nil, &webscrape.ScrapeError{
			Type:       webscrape.ErrNotFound,
			Message:    fmt.Sprintf("question %d not found", questionID),
			Suggestion: "Verify the question ID is correct",
		}
	}

	item := response.Items[0]

	// Extract code blocks from body
	extracted, _ := s.extractor.Extract(item.Body, item.Link)
	var codeBlocks []webscrape.CodeBlock
	if extracted != nil {
		codeBlocks = extracted.CodeBlocks
	}

	question := &webscrape.SOQuestion{
		ID:           item.QuestionID,
		Title:        item.Title,
		Body:         item.Body,
		BodyMarkdown: stripHTMLToMarkdown(item.Body),
		Tags:         item.Tags,
		Score:        item.Score,
		AcceptedID:   item.AcceptedAnswerID,
		URL:          item.Link,
		CreatedAt:    time.Unix(item.CreationDate, 0),
	}

	// Set code blocks on first answer if any (will be replaced when fetching answers)
	if len(codeBlocks) > 0 {
		question.Answers = []webscrape.SOAnswer{{
			CodeBlocks: codeBlocks,
		}}
	}

	return question, nil
}

// FetchAnswers fetches answers for a Stack Overflow question
func (s *StackOverflowHandler) FetchAnswers(ctx context.Context, questionID int, maxAnswers int) ([]webscrape.SOAnswer, error) {
	if maxAnswers == 0 {
		maxAnswers = 5
	}

	apiURL := fmt.Sprintf("https://api.stackexchange.com/2.3/questions/%d/answers?order=desc&sort=votes&site=stackoverflow&filter=withbody&pagesize=%d",
		questionID, maxAnswers)

	if s.apiKey != "" {
		apiURL += "&key=" + url.QueryEscape(s.apiKey)
	}

	var response struct {
		Items []struct {
			AnswerID     int    `json:"answer_id"`
			QuestionID   int    `json:"question_id"`
			Body         string `json:"body"`
			Score        int    `json:"score"`
			IsAccepted   bool   `json:"is_accepted"`
			Link         string `json:"link"`
			CreationDate int64  `json:"creation_date"`
		} `json:"items"`
		HasMore        bool `json:"has_more"`
		QuotaMax       int  `json:"quota_max"`
		QuotaRemaining int  `json:"quota_remaining"`
	}

	if err := s.doAPIRequest(ctx, apiURL, &response); err != nil {
		return nil, err
	}

	var answers []webscrape.SOAnswer
	for _, item := range response.Items {
		// Extract code blocks from answer body
		extracted, _ := s.extractor.Extract(item.Body, "")
		var codeBlocks []webscrape.CodeBlock
		if extracted != nil {
			codeBlocks = extracted.CodeBlocks
		}

		// Build answer URL
		answerURL := fmt.Sprintf("https://stackoverflow.com/a/%d", item.AnswerID)

		answers = append(answers, webscrape.SOAnswer{
			ID:           item.AnswerID,
			Body:         item.Body,
			BodyMarkdown: stripHTMLToMarkdown(item.Body),
			CodeBlocks:   codeBlocks,
			Score:        item.Score,
			IsAccepted:   item.IsAccepted,
			URL:          answerURL,
		})
	}

	return answers, nil
}

// FetchQuestionWithAnswers fetches a question with its answers
func (s *StackOverflowHandler) FetchQuestionWithAnswers(ctx context.Context, questionID int, maxAnswers int) (*webscrape.SOQuestion, error) {
	question, err := s.FetchQuestion(ctx, questionID)
	if err != nil {
		return nil, err
	}

	answers, err := s.FetchAnswers(ctx, questionID, maxAnswers)
	if err != nil {
		// Return question without answers if fetch fails
		return question, nil
	}

	question.Answers = answers
	return question, nil
}

// SearchQuestions searches Stack Overflow for questions
func (s *StackOverflowHandler) SearchQuestions(ctx context.Context, query string, tags []string, sort string, maxResults int) ([]webscrape.SOSearchResult, error) {
	if sort == "" {
		sort = "relevance"
	}
	if maxResults == 0 {
		maxResults = 10
	}

	// Build search URL
	params := url.Values{}
	params.Set("order", "desc")
	params.Set("sort", sort)
	params.Set("site", "stackoverflow")
	params.Set("pagesize", strconv.Itoa(maxResults))
	params.Set("intitle", query)

	if len(tags) > 0 {
		params.Set("tagged", strings.Join(tags, ";"))
	}

	if s.apiKey != "" {
		params.Set("key", s.apiKey)
	}

	apiURL := "https://api.stackexchange.com/2.3/search/advanced?" + params.Encode()

	var response struct {
		Items []struct {
			QuestionID  int      `json:"question_id"`
			Title       string   `json:"title"`
			Tags        []string `json:"tags"`
			Score       int      `json:"score"`
			AnswerCount int      `json:"answer_count"`
			IsAnswered  bool     `json:"is_answered"`
			Link        string   `json:"link"`
		} `json:"items"`
		HasMore        bool `json:"has_more"`
		QuotaMax       int  `json:"quota_max"`
		QuotaRemaining int  `json:"quota_remaining"`
	}

	if err := s.doAPIRequest(ctx, apiURL, &response); err != nil {
		return nil, err
	}

	var results []webscrape.SOSearchResult
	for _, item := range response.Items {
		results = append(results, webscrape.SOSearchResult{
			ID:        item.QuestionID,
			Title:     item.Title,
			Tags:      item.Tags,
			Score:     item.Score,
			Answers:   item.AnswerCount,
			URL:       item.Link,
			IsAnswerd: item.IsAnswered,
		})
	}

	return results, nil
}

// doAPIRequest performs an API request and decodes the JSON response
func (s *StackOverflowHandler) doAPIRequest(ctx context.Context, apiURL string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return &webscrape.ScrapeError{
			Type:    webscrape.ErrNetwork,
			Message: fmt.Sprintf("failed to create request: %v", err),
		}
	}

	// Stack Exchange API returns gzip-compressed responses
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("User-Agent", "PedroCLI/1.0")

	if err := s.rateLimiter.Wait(ctx, "api.stackexchange.com"); err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return &webscrape.ScrapeError{
			Type:      webscrape.ErrNetwork,
			Message:   fmt.Sprintf("request failed: %v", err),
			Retryable: true,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return s.handleHTTPError(resp.StatusCode)
	}

	// Handle gzip compression
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return &webscrape.ScrapeError{
				Type:    webscrape.ErrParseFailure,
				Message: fmt.Sprintf("failed to decompress response: %v", err),
			}
		}
		defer gzReader.Close()
		reader = gzReader
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return &webscrape.ScrapeError{
			Type:    webscrape.ErrNetwork,
			Message: fmt.Sprintf("failed to read response: %v", err),
		}
	}

	if err := json.Unmarshal(body, result); err != nil {
		return &webscrape.ScrapeError{
			Type:    webscrape.ErrParseFailure,
			Message: fmt.Sprintf("failed to parse response: %v", err),
		}
	}

	return nil
}

// handleHTTPError converts HTTP status codes to ScrapeError
func (s *StackOverflowHandler) handleHTTPError(statusCode int) *webscrape.ScrapeError {
	switch statusCode {
	case 400:
		return &webscrape.ScrapeError{
			Type:       webscrape.ErrInvalidURL,
			Message:    "bad request to Stack Exchange API",
			StatusCode: statusCode,
			Suggestion: "Check your query parameters",
		}
	case 401, 403:
		return &webscrape.ScrapeError{
			Type:       webscrape.ErrAccessDenied,
			Message:    "access denied to Stack Exchange API",
			StatusCode: statusCode,
			Suggestion: "Check your API key",
		}
	case 404:
		return &webscrape.ScrapeError{
			Type:       webscrape.ErrNotFound,
			Message:    "resource not found on Stack Overflow",
			StatusCode: statusCode,
			Suggestion: "Verify the question ID is correct",
		}
	case 429:
		return &webscrape.ScrapeError{
			Type:       webscrape.ErrRateLimited,
			Message:    "Stack Exchange API rate limit exceeded",
			StatusCode: statusCode,
			Retryable:  true,
			Suggestion: "Wait before making more requests or use an API key",
		}
	case 502, 503:
		return &webscrape.ScrapeError{
			Type:       webscrape.ErrNetwork,
			Message:    "Stack Exchange API temporarily unavailable",
			StatusCode: statusCode,
			Retryable:  true,
			Suggestion: "Try again in a few seconds",
		}
	default:
		return &webscrape.ScrapeError{
			Type:       webscrape.ErrNetwork,
			Message:    fmt.Sprintf("Stack Exchange API error: %d", statusCode),
			StatusCode: statusCode,
			Retryable:  statusCode >= 500,
		}
	}
}

// stripHTMLToMarkdown converts HTML to a simple markdown-like format
func stripHTMLToMarkdown(html string) string {
	// This is a simple conversion - for production, consider a proper HTML to markdown library
	result := html

	// Convert code blocks
	result = strings.ReplaceAll(result, "<pre><code>", "\n```\n")
	result = strings.ReplaceAll(result, "</code></pre>", "\n```\n")
	result = strings.ReplaceAll(result, "<pre>", "\n```\n")
	result = strings.ReplaceAll(result, "</pre>", "\n```\n")
	result = strings.ReplaceAll(result, "<code>", "`")
	result = strings.ReplaceAll(result, "</code>", "`")

	// Convert lists
	result = strings.ReplaceAll(result, "<li>", "- ")
	result = strings.ReplaceAll(result, "</li>", "\n")
	result = strings.ReplaceAll(result, "<ul>", "\n")
	result = strings.ReplaceAll(result, "</ul>", "\n")
	result = strings.ReplaceAll(result, "<ol>", "\n")
	result = strings.ReplaceAll(result, "</ol>", "\n")

	// Convert paragraphs and line breaks
	result = strings.ReplaceAll(result, "<p>", "\n\n")
	result = strings.ReplaceAll(result, "</p>", "")
	result = strings.ReplaceAll(result, "<br>", "\n")
	result = strings.ReplaceAll(result, "<br/>", "\n")
	result = strings.ReplaceAll(result, "<br />", "\n")

	// Convert headers
	result = strings.ReplaceAll(result, "<h1>", "\n# ")
	result = strings.ReplaceAll(result, "</h1>", "\n")
	result = strings.ReplaceAll(result, "<h2>", "\n## ")
	result = strings.ReplaceAll(result, "</h2>", "\n")
	result = strings.ReplaceAll(result, "<h3>", "\n### ")
	result = strings.ReplaceAll(result, "</h3>", "\n")

	// Convert bold and italic
	result = strings.ReplaceAll(result, "<strong>", "**")
	result = strings.ReplaceAll(result, "</strong>", "**")
	result = strings.ReplaceAll(result, "<b>", "**")
	result = strings.ReplaceAll(result, "</b>", "**")
	result = strings.ReplaceAll(result, "<em>", "*")
	result = strings.ReplaceAll(result, "</em>", "*")
	result = strings.ReplaceAll(result, "<i>", "*")
	result = strings.ReplaceAll(result, "</i>", "*")

	// Strip remaining HTML tags
	inTag := false
	var cleaned strings.Builder
	for _, r := range result {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			cleaned.WriteRune(r)
		}
	}

	// Decode HTML entities
	text := cleaned.String()
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", `"`)
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	return strings.TrimSpace(text)
}

// ParseStackOverflowURL parses a Stack Overflow URL to extract question/answer ID
func ParseStackOverflowURL(rawURL string) (questionID int, answerID int, err error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return 0, 0, err
	}

	if !strings.Contains(parsed.Host, "stackoverflow.com") {
		return 0, 0, fmt.Errorf("not a Stack Overflow URL")
	}

	path := strings.Trim(parsed.Path, "/")
	parts := strings.Split(path, "/")

	// Handle different URL formats:
	// /questions/123456/title
	// /questions/123456
	// /a/123456
	// /q/123456

	for i, part := range parts {
		switch part {
		case "questions", "q":
			if i+1 < len(parts) {
				id, err := strconv.Atoi(parts[i+1])
				if err == nil {
					questionID = id
				}
			}
		case "a":
			if i+1 < len(parts) {
				id, err := strconv.Atoi(parts[i+1])
				if err == nil {
					answerID = id
				}
			}
		}
	}

	// Check for answer hash in URL (e.g., #123456)
	if parsed.Fragment != "" {
		id, err := strconv.Atoi(parsed.Fragment)
		if err == nil {
			answerID = id
		}
	}

	if questionID == 0 && answerID == 0 {
		return 0, 0, fmt.Errorf("could not parse question or answer ID from URL")
	}

	return questionID, answerID, nil
}
