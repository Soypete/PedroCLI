package fileio

import (
	"testing"
)

func TestExtensionRegistry(t *testing.T) {
	reg := NewExtensionRegistry()

	// Test Go language
	lang := reg.GetLanguage(".go")
	if lang != "go" {
		t.Errorf("expected 'go' for .go, got %q", lang)
	}

	// Test Python
	lang = reg.GetLanguage(".py")
	if lang != "python" {
		t.Errorf("expected 'python' for .py, got %q", lang)
	}

	// Test TypeScript
	lang = reg.GetLanguage(".ts")
	if lang != "typescript" {
		t.Errorf("expected 'typescript' for .ts, got %q", lang)
	}

	// Test unknown extension
	lang = reg.GetLanguage(".xyz")
	if lang != "unknown" {
		t.Errorf("expected 'unknown' for .xyz, got %q", lang)
	}
}

func TestGetLanguageInfo(t *testing.T) {
	reg := NewExtensionRegistry()

	info := reg.GetLanguageInfo(".go")
	if info == nil {
		t.Fatal("expected language info for .go")
	}

	if info.Name != "Go" {
		t.Errorf("expected name 'Go', got %q", info.Name)
	}

	if info.IndentStyle != "tabs" {
		t.Errorf("expected indent style 'tabs', got %q", info.IndentStyle)
	}

	if len(info.LSPServers) == 0 || info.LSPServers[0] != "gopls" {
		t.Errorf("expected LSP server 'gopls', got %v", info.LSPServers)
	}
}

func TestGetLanguageByID(t *testing.T) {
	reg := NewExtensionRegistry()

	info := reg.GetLanguageByID("python")
	if info == nil {
		t.Fatal("expected language info for 'python'")
	}

	if info.CommentSingle != "#" {
		t.Errorf("expected comment single '#', got %q", info.CommentSingle)
	}
}

func TestGetLanguageForPath(t *testing.T) {
	reg := NewExtensionRegistry()

	tests := []struct {
		path     string
		expected string
	}{
		{"/path/to/main.go", "go"},
		{"/path/to/script.py", "python"},
		{"/path/to/app.tsx", "typescriptreact"},
		{"/path/to/Makefile", "makefile"},
		{"/path/to/Dockerfile", "dockerfile"},
		// Note: extension matching takes priority over filename patterns
		// So .json files match "json" language, not the filename pattern
		{"/path/to/package.json", "json"},
		{"/path/to/unknown.xyz", ""},
	}

	for _, tt := range tests {
		info := reg.GetLanguageForPath(tt.path)
		if tt.expected == "" {
			if info != nil {
				t.Errorf("GetLanguageForPath(%q) = %v, want nil", tt.path, info)
			}
		} else {
			if info == nil {
				t.Errorf("GetLanguageForPath(%q) = nil, want %q", tt.path, tt.expected)
			} else if info.ID != tt.expected {
				t.Errorf("GetLanguageForPath(%q) = %q, want %q", tt.path, info.ID, tt.expected)
			}
		}
	}
}

func TestCaseInsensitiveExtensions(t *testing.T) {
	reg := NewExtensionRegistry()

	// Extensions should be case-insensitive
	lang1 := reg.GetLanguage(".GO")
	lang2 := reg.GetLanguage(".go")
	lang3 := reg.GetLanguage(".Go")

	if lang1 != "go" || lang2 != "go" || lang3 != "go" {
		t.Errorf("extension matching should be case-insensitive: %q, %q, %q", lang1, lang2, lang3)
	}
}

func TestGetLSPServers(t *testing.T) {
	reg := NewExtensionRegistry()

	servers := reg.GetLSPServers("go")
	if len(servers) == 0 {
		t.Error("expected at least one LSP server for Go")
	}
	if servers[0] != "gopls" {
		t.Errorf("expected 'gopls', got %q", servers[0])
	}

	servers = reg.GetLSPServers("unknown")
	if len(servers) != 0 {
		t.Error("expected no LSP servers for unknown language")
	}
}

func TestGetCommentStyle(t *testing.T) {
	reg := NewExtensionRegistry()

	tests := []struct {
		langID     string
		single     string
		multiStart string
		multiEnd   string
	}{
		{"go", "//", "/*", "*/"},
		{"python", "#", `"""`, `"""`},
		{"html", "", "<!--", "-->"},
		{"sql", "--", "/*", "*/"},
	}

	for _, tt := range tests {
		single, multiStart, multiEnd := reg.GetCommentStyle(tt.langID)
		if single != tt.single {
			t.Errorf("GetCommentStyle(%q) single = %q, want %q", tt.langID, single, tt.single)
		}
		if multiStart != tt.multiStart {
			t.Errorf("GetCommentStyle(%q) multiStart = %q, want %q", tt.langID, multiStart, tt.multiStart)
		}
		if multiEnd != tt.multiEnd {
			t.Errorf("GetCommentStyle(%q) multiEnd = %q, want %q", tt.langID, multiEnd, tt.multiEnd)
		}
	}
}

func TestGetIndentStyle(t *testing.T) {
	reg := NewExtensionRegistry()

	tests := []struct {
		langID string
		style  string
		size   int
	}{
		{"go", "tabs", 4},
		{"python", "spaces", 4},
		{"javascript", "spaces", 2},
		{"ruby", "spaces", 2},
	}

	for _, tt := range tests {
		style, size := reg.GetIndentStyle(tt.langID)
		if style != tt.style {
			t.Errorf("GetIndentStyle(%q) style = %q, want %q", tt.langID, style, tt.style)
		}
		if size != tt.size {
			t.Errorf("GetIndentStyle(%q) size = %d, want %d", tt.langID, size, tt.size)
		}
	}
}

func TestListLanguages(t *testing.T) {
	reg := NewExtensionRegistry()

	languages := reg.ListLanguages()
	if len(languages) == 0 {
		t.Fatal("expected at least some languages")
	}

	// Check some expected languages are present
	found := make(map[string]bool)
	for _, lang := range languages {
		found[lang] = true
	}

	expectedLangs := []string{"go", "python", "javascript", "typescript", "rust"}
	for _, lang := range expectedLangs {
		if !found[lang] {
			t.Errorf("expected language %q not found", lang)
		}
	}
}

func TestListExtensions(t *testing.T) {
	reg := NewExtensionRegistry()

	exts := reg.ListExtensions("go")
	if len(exts) == 0 {
		t.Fatal("expected at least one extension for Go")
	}

	found := false
	for _, ext := range exts {
		if ext == ".go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected .go extension for Go language")
	}
}

func TestSupportsLSP(t *testing.T) {
	reg := NewExtensionRegistry()

	if !reg.SupportsLSP("go") {
		t.Error("Go should support LSP")
	}

	if !reg.SupportsLSP("python") {
		t.Error("Python should support LSP")
	}

	if reg.SupportsLSP("plaintext") {
		t.Error("Plain text should not support LSP")
	}

	if reg.SupportsLSP("nonexistent") {
		t.Error("Non-existent language should not support LSP")
	}
}

func TestRegisterLanguage(t *testing.T) {
	reg := NewExtensionRegistry()

	customLang := &LanguageInfo{
		Name:          "CustomLang",
		ID:            "customlang",
		Extensions:    []string{".custom", ".cst"},
		LSPServers:    []string{"customlsp"},
		CommentSingle: "//",
		IndentStyle:   "spaces",
		IndentSize:    3,
	}

	reg.RegisterLanguage(customLang)

	// Test registration worked
	lang := reg.GetLanguage(".custom")
	if lang != "customlang" {
		t.Errorf("expected 'customlang' for .custom, got %q", lang)
	}

	info := reg.GetLanguageByID("customlang")
	if info == nil {
		t.Fatal("expected language info for customlang")
	}

	if info.IndentSize != 3 {
		t.Errorf("expected indent size 3, got %d", info.IndentSize)
	}
}

func TestMultipleExtensions(t *testing.T) {
	reg := NewExtensionRegistry()

	// Python has multiple extensions
	pyExts := []string{".py", ".pyw", ".pyi"}
	for _, ext := range pyExts {
		lang := reg.GetLanguage(ext)
		if lang != "python" {
			t.Errorf("expected 'python' for %s, got %q", ext, lang)
		}
	}

	// C++ has multiple extensions
	cppExts := []string{".cpp", ".cc", ".cxx", ".hpp", ".hh"}
	for _, ext := range cppExts {
		lang := reg.GetLanguage(ext)
		if lang != "cpp" {
			t.Errorf("expected 'cpp' for %s, got %q", ext, lang)
		}
	}
}

func TestFilePatterns(t *testing.T) {
	reg := NewExtensionRegistry()

	// Note: File patterns only match when there's no extension match.
	// Files like package.json match "json" by extension, not "javascript" by pattern.
	// Patterns are primarily for files without extensions (Makefile, Dockerfile, .bashrc)
	tests := []struct {
		filename string
		expected string
	}{
		{"Makefile", "makefile"},     // No extension, matches by pattern
		{"GNUmakefile", "makefile"},  // No extension, matches by pattern
		{"Dockerfile", "dockerfile"}, // No extension, matches by pattern
		{".bashrc", "shellscript"},   // No extension (dotfile), matches by pattern
	}

	for _, tt := range tests {
		info := reg.GetLanguageForPath("/some/path/" + tt.filename)
		if info == nil {
			t.Errorf("GetLanguageForPath(%q) = nil, want language info", tt.filename)
			continue
		}
		if info.ID != tt.expected {
			t.Errorf("GetLanguageForPath(%q) = %q, want %q", tt.filename, info.ID, tt.expected)
		}
	}
}
