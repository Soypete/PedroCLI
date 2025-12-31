package webscrape

import (
	"testing"
)

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "extracts title tag",
			html:     `<html><head><title>Test Title</title></head><body></body></html>`,
			expected: "Test Title",
		},
		{
			name:     "extracts h1 when no title",
			html:     `<html><body><h1>Heading One</h1></body></html>`,
			expected: "Heading One",
		},
		{
			name:     "handles empty html",
			html:     "",
			expected: "",
		},
		{
			name:     "decodes HTML entities in title",
			html:     `<title>Test &amp; Title</title>`,
			expected: "Test & Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTitle(tt.html)
			if result != tt.expected {
				t.Errorf("extractTitle() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractCodeBlocks(t *testing.T) {
	tests := []struct {
		name          string
		html          string
		expectedCount int
		checkFirst    *CodeBlock
	}{
		{
			name:          "extracts pre>code blocks",
			html:          `<pre><code class="language-go">func main() {}</code></pre>`,
			expectedCount: 1,
			checkFirst: &CodeBlock{
				Language: "go",
				Code:     "func main() {}",
			},
		},
		{
			name:          "extracts standalone pre blocks",
			html:          `<pre>echo "hello"</pre>`,
			expectedCount: 1,
			checkFirst: &CodeBlock{
				Language: "",
				Code:     `echo "hello"`,
			},
		},
		{
			name:          "handles multiple code blocks",
			html:          `<pre><code>code1</code></pre><pre><code>code2</code></pre>`,
			expectedCount: 2,
		},
		{
			name:          "handles empty html",
			html:          "",
			expectedCount: 0,
		},
		{
			name:          "extracts fenced markdown code blocks",
			html:          "```python\nprint('hello')\n```",
			expectedCount: 1,
			checkFirst: &CodeBlock{
				Language: "python",
				Code:     "print('hello')",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCodeBlocks(tt.html)
			if len(result) != tt.expectedCount {
				t.Errorf("extractCodeBlocks() returned %d blocks, want %d", len(result), tt.expectedCount)
			}
			if tt.checkFirst != nil && len(result) > 0 {
				if result[0].Language != tt.checkFirst.Language {
					t.Errorf("first block Language = %q, want %q", result[0].Language, tt.checkFirst.Language)
				}
				if result[0].Code != tt.checkFirst.Code {
					t.Errorf("first block Code = %q, want %q", result[0].Code, tt.checkFirst.Code)
				}
			}
		})
	}
}

func TestExtractCleanText(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		contains []string
		excludes []string
	}{
		{
			name:     "removes script tags",
			html:     `<p>Hello</p><script>alert('bad')</script><p>World</p>`,
			contains: []string{"Hello", "World"},
			excludes: []string{"script", "alert"},
		},
		{
			name:     "removes style tags",
			html:     `<p>Hello</p><style>.foo { color: red; }</style><p>World</p>`,
			contains: []string{"Hello", "World"},
			excludes: []string{"style", "color", ".foo"},
		},
		{
			name:     "converts paragraphs to newlines",
			html:     `<p>Para 1</p><p>Para 2</p>`,
			contains: []string{"Para 1", "Para 2"},
		},
		{
			name:     "decodes HTML entities",
			html:     `<p>Test &amp; entities &lt;here&gt;</p>`,
			contains: []string{"Test & entities <here>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCleanText(tt.html)
			for _, c := range tt.contains {
				if !containsString(result, c) {
					t.Errorf("extractCleanText() should contain %q, got %q", c, result)
				}
			}
			for _, e := range tt.excludes {
				if containsString(result, e) {
					t.Errorf("extractCleanText() should not contain %q, got %q", e, result)
				}
			}
		})
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "detects Go",
			code:     "package main\n\nfunc main() {}",
			expected: "go",
		},
		{
			name:     "detects Python def",
			code:     "def hello():\n    print('hello')",
			expected: "python",
		},
		{
			name:     "detects Python class",
			code:     "class Foo:\n    pass",
			expected: "python",
		},
		{
			name:     "detects JavaScript",
			code:     "function hello() { return 'world'; }",
			expected: "javascript",
		},
		{
			name:     "detects TypeScript",
			code:     "interface Foo { bar: string; }",
			expected: "typescript",
		},
		{
			name:     "detects JSON",
			code:     `{"key": "value"}`,
			expected: "json",
		},
		{
			name:     "detects SQL",
			code:     "SELECT * FROM users WHERE id = 1",
			expected: "sql",
		},
		{
			name:     "detects Bash",
			code:     "#!/bin/bash\necho hello",
			expected: "bash",
		},
		{
			name:     "returns empty for unknown",
			code:     "random text here",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectLanguage(tt.code)
			if result != tt.expected {
				t.Errorf("DetectLanguage() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestReadabilityExtractor(t *testing.T) {
	extractor := NewReadabilityExtractor()

	t.Run("CanHandle returns true for any URL", func(t *testing.T) {
		if !extractor.CanHandle("https://example.com") {
			t.Error("CanHandle should return true for any URL")
		}
	})

	t.Run("Extract returns content", func(t *testing.T) {
		html := `
			<html>
			<head><title>Test Page</title></head>
			<body>
				<h1>Main Content</h1>
				<p>This is the body text.</p>
				<pre><code class="language-go">func main() {}</code></pre>
			</body>
			</html>
		`
		content, err := extractor.Extract(html, "https://example.com")
		if err != nil {
			t.Fatalf("Extract() error = %v", err)
		}
		if content.Title != "Test Page" {
			t.Errorf("Title = %q, want %q", content.Title, "Test Page")
		}
		if len(content.CodeBlocks) != 1 {
			t.Errorf("CodeBlocks count = %d, want 1", len(content.CodeBlocks))
		}
		if !containsString(content.MainText, "Main Content") {
			t.Error("MainText should contain 'Main Content'")
		}
	})
}

func TestCodeExtractor(t *testing.T) {
	extractor := NewCodeExtractor()

	t.Run("CanHandle returns true for tech sites", func(t *testing.T) {
		techURLs := []string{
			"https://github.com/foo/bar",
			"https://gitlab.com/foo/bar",
			"https://stackoverflow.com/questions/123",
			"https://gist.github.com/foo/123",
		}
		for _, url := range techURLs {
			if !extractor.CanHandle(url) {
				t.Errorf("CanHandle(%q) should return true", url)
			}
		}
	})

	t.Run("CanHandle returns false for non-tech sites", func(t *testing.T) {
		if extractor.CanHandle("https://example.com") {
			t.Error("CanHandle(example.com) should return false")
		}
	})
}

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsSubstr(s, substr)))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
