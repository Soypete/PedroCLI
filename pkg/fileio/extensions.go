package fileio

import (
	"path/filepath"
	"strings"
	"sync"
)

// LanguageInfo contains information about a programming language
type LanguageInfo struct {
	Name          string   // Display name (e.g., "Go", "Python")
	ID            string   // Identifier (e.g., "go", "python")
	Extensions    []string // File extensions (e.g., [".go", ".GO"])
	LSPServers    []string // Known LSP server commands (e.g., ["gopls", "pylsp"])
	CommentSingle string   // Single-line comment prefix (e.g., "//")
	CommentMultiS string   // Multi-line comment start (e.g., "/*")
	CommentMultiE string   // Multi-line comment end (e.g., "*/")
	IndentStyle   string   // "tabs" or "spaces"
	IndentSize    int      // Number of spaces per indent
	FilePatterns  []string // Additional file patterns (e.g., ["Makefile", "Dockerfile"])
}

// ExtensionRegistry manages file extension to language mappings
type ExtensionRegistry struct {
	mu        sync.RWMutex
	languages map[string]*LanguageInfo // Extension -> Language mapping
	patterns  map[string]*LanguageInfo // Pattern -> Language mapping
	byID      map[string]*LanguageInfo // Language ID -> Language mapping
}

// NewExtensionRegistry creates a new registry with default language mappings
func NewExtensionRegistry() *ExtensionRegistry {
	reg := &ExtensionRegistry{
		languages: make(map[string]*LanguageInfo),
		patterns:  make(map[string]*LanguageInfo),
		byID:      make(map[string]*LanguageInfo),
	}
	reg.registerDefaults()
	return reg
}

// registerDefaults registers built-in language definitions
func (r *ExtensionRegistry) registerDefaults() {
	defaultLanguages := []*LanguageInfo{
		{
			Name:          "Go",
			ID:            "go",
			Extensions:    []string{".go"},
			LSPServers:    []string{"gopls"},
			CommentSingle: "//",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "tabs",
			IndentSize:    4,
			FilePatterns:  []string{"go.mod", "go.sum"},
		},
		{
			Name:          "Python",
			ID:            "python",
			Extensions:    []string{".py", ".pyw", ".pyi"},
			LSPServers:    []string{"pylsp", "pyright", "python-language-server"},
			CommentSingle: "#",
			CommentMultiS: `"""`,
			CommentMultiE: `"""`,
			IndentStyle:   "spaces",
			IndentSize:    4,
			FilePatterns:  []string{"requirements.txt", "setup.py", "pyproject.toml", "Pipfile"},
		},
		{
			Name:          "JavaScript",
			ID:            "javascript",
			Extensions:    []string{".js", ".mjs", ".cjs"},
			LSPServers:    []string{"typescript-language-server", "eslint"},
			CommentSingle: "//",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    2,
			FilePatterns:  []string{"package.json", ".eslintrc.js", "webpack.config.js"},
		},
		{
			Name:          "TypeScript",
			ID:            "typescript",
			Extensions:    []string{".ts", ".mts", ".cts"},
			LSPServers:    []string{"typescript-language-server"},
			CommentSingle: "//",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    2,
			FilePatterns:  []string{"tsconfig.json"},
		},
		{
			Name:          "TypeScript React",
			ID:            "typescriptreact",
			Extensions:    []string{".tsx"},
			LSPServers:    []string{"typescript-language-server"},
			CommentSingle: "//",
			CommentMultiS: "{/*",
			CommentMultiE: "*/}",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:          "JavaScript React",
			ID:            "javascriptreact",
			Extensions:    []string{".jsx"},
			LSPServers:    []string{"typescript-language-server"},
			CommentSingle: "//",
			CommentMultiS: "{/*",
			CommentMultiE: "*/}",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:          "Rust",
			ID:            "rust",
			Extensions:    []string{".rs"},
			LSPServers:    []string{"rust-analyzer", "rls"},
			CommentSingle: "//",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    4,
			FilePatterns:  []string{"Cargo.toml", "Cargo.lock"},
		},
		{
			Name:          "Java",
			ID:            "java",
			Extensions:    []string{".java"},
			LSPServers:    []string{"jdtls", "java-language-server"},
			CommentSingle: "//",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    4,
			FilePatterns:  []string{"pom.xml", "build.gradle"},
		},
		{
			Name:          "C",
			ID:            "c",
			Extensions:    []string{".c", ".h"},
			LSPServers:    []string{"clangd", "ccls"},
			CommentSingle: "//",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    4,
		},
		{
			Name:          "C++",
			ID:            "cpp",
			Extensions:    []string{".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx", ".h++"},
			LSPServers:    []string{"clangd", "ccls"},
			CommentSingle: "//",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    4,
			FilePatterns:  []string{"CMakeLists.txt"},
		},
		{
			Name:          "C#",
			ID:            "csharp",
			Extensions:    []string{".cs"},
			LSPServers:    []string{"omnisharp", "csharp-ls"},
			CommentSingle: "//",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    4,
			FilePatterns:  []string{".csproj", ".sln"},
		},
		{
			Name:          "Ruby",
			ID:            "ruby",
			Extensions:    []string{".rb", ".rake"},
			LSPServers:    []string{"solargraph", "ruby-lsp"},
			CommentSingle: "#",
			CommentMultiS: "=begin",
			CommentMultiE: "=end",
			IndentStyle:   "spaces",
			IndentSize:    2,
			FilePatterns:  []string{"Gemfile", "Rakefile", ".ruby-version"},
		},
		{
			Name:          "PHP",
			ID:            "php",
			Extensions:    []string{".php", ".phtml", ".php3", ".php4", ".php5"},
			LSPServers:    []string{"intelephense", "phpactor"},
			CommentSingle: "//",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    4,
			FilePatterns:  []string{"composer.json"},
		},
		{
			Name:          "Swift",
			ID:            "swift",
			Extensions:    []string{".swift"},
			LSPServers:    []string{"sourcekit-lsp"},
			CommentSingle: "//",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    4,
			FilePatterns:  []string{"Package.swift"},
		},
		{
			Name:          "Kotlin",
			ID:            "kotlin",
			Extensions:    []string{".kt", ".kts"},
			LSPServers:    []string{"kotlin-language-server"},
			CommentSingle: "//",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    4,
		},
		{
			Name:          "Scala",
			ID:            "scala",
			Extensions:    []string{".scala", ".sc"},
			LSPServers:    []string{"metals"},
			CommentSingle: "//",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    2,
			FilePatterns:  []string{"build.sbt"},
		},
		{
			Name:          "Shell",
			ID:            "shellscript",
			Extensions:    []string{".sh", ".bash", ".zsh", ".fish"},
			LSPServers:    []string{"bash-language-server"},
			CommentSingle: "#",
			IndentStyle:   "spaces",
			IndentSize:    2,
			FilePatterns:  []string{".bashrc", ".zshrc", ".profile"},
		},
		{
			Name:          "PowerShell",
			ID:            "powershell",
			Extensions:    []string{".ps1", ".psm1", ".psd1"},
			LSPServers:    []string{"powershell-editor-services"},
			CommentSingle: "#",
			CommentMultiS: "<#",
			CommentMultiE: "#>",
			IndentStyle:   "spaces",
			IndentSize:    4,
		},
		{
			Name:          "SQL",
			ID:            "sql",
			Extensions:    []string{".sql"},
			LSPServers:    []string{"sqls", "sql-language-server"},
			CommentSingle: "--",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    4,
		},
		{
			Name:          "HTML",
			ID:            "html",
			Extensions:    []string{".html", ".htm", ".xhtml"},
			LSPServers:    []string{"vscode-html-languageservice", "html-languageserver"},
			CommentMultiS: "<!--",
			CommentMultiE: "-->",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:          "CSS",
			ID:            "css",
			Extensions:    []string{".css"},
			LSPServers:    []string{"vscode-css-languageservice", "css-languageserver"},
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:          "SCSS",
			ID:            "scss",
			Extensions:    []string{".scss"},
			LSPServers:    []string{"vscode-css-languageservice"},
			CommentSingle: "//",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:          "Less",
			ID:            "less",
			Extensions:    []string{".less"},
			LSPServers:    []string{"vscode-css-languageservice"},
			CommentSingle: "//",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:        "JSON",
			ID:          "json",
			Extensions:  []string{".json"},
			LSPServers:  []string{"vscode-json-languageservice"},
			IndentStyle: "spaces",
			IndentSize:  2,
		},
		{
			Name:          "JSONC",
			ID:            "jsonc",
			Extensions:    []string{".jsonc"},
			LSPServers:    []string{"vscode-json-languageservice"},
			CommentSingle: "//",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:          "YAML",
			ID:            "yaml",
			Extensions:    []string{".yaml", ".yml"},
			LSPServers:    []string{"yaml-language-server"},
			CommentSingle: "#",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:          "TOML",
			ID:            "toml",
			Extensions:    []string{".toml"},
			LSPServers:    []string{"taplo"},
			CommentSingle: "#",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:        "Markdown",
			ID:          "markdown",
			Extensions:  []string{".md", ".markdown", ".mdown"},
			LSPServers:  []string{"marksman", "markdown-oxide"},
			IndentStyle: "spaces",
			IndentSize:  2,
		},
		{
			Name:          "XML",
			ID:            "xml",
			Extensions:    []string{".xml", ".xsl", ".xslt", ".xsd"},
			LSPServers:    []string{"lemminx"},
			CommentMultiS: "<!--",
			CommentMultiE: "-->",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:          "Dockerfile",
			ID:            "dockerfile",
			Extensions:    []string{".dockerfile"},
			LSPServers:    []string{"docker-langserver", "dockerfile-language-server-nodejs"},
			CommentSingle: "#",
			IndentStyle:   "spaces",
			IndentSize:    4,
			FilePatterns:  []string{"Dockerfile", "Containerfile"},
		},
		{
			Name:          "Makefile",
			ID:            "makefile",
			Extensions:    []string{".mk", ".mak"},
			CommentSingle: "#",
			IndentStyle:   "tabs",
			IndentSize:    4,
			FilePatterns:  []string{"Makefile", "GNUmakefile", "makefile"},
		},
		{
			Name:          "Lua",
			ID:            "lua",
			Extensions:    []string{".lua"},
			LSPServers:    []string{"lua-language-server", "sumneko_lua"},
			CommentSingle: "--",
			CommentMultiS: "--[[",
			CommentMultiE: "]]",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:          "Perl",
			ID:            "perl",
			Extensions:    []string{".pl", ".pm", ".pod"},
			LSPServers:    []string{"perl-language-server"},
			CommentSingle: "#",
			CommentMultiS: "=pod",
			CommentMultiE: "=cut",
			IndentStyle:   "spaces",
			IndentSize:    4,
		},
		{
			Name:          "R",
			ID:            "r",
			Extensions:    []string{".r", ".R", ".Rmd"},
			LSPServers:    []string{"r-languageserver"},
			CommentSingle: "#",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:          "Elixir",
			ID:            "elixir",
			Extensions:    []string{".ex", ".exs"},
			LSPServers:    []string{"elixir-ls", "next-ls"},
			CommentSingle: "#",
			CommentMultiS: "@doc \"\"\"",
			CommentMultiE: "\"\"\"",
			IndentStyle:   "spaces",
			IndentSize:    2,
			FilePatterns:  []string{"mix.exs"},
		},
		{
			Name:          "Erlang",
			ID:            "erlang",
			Extensions:    []string{".erl", ".hrl"},
			LSPServers:    []string{"erlang_ls"},
			CommentSingle: "%",
			IndentStyle:   "spaces",
			IndentSize:    4,
		},
		{
			Name:          "Haskell",
			ID:            "haskell",
			Extensions:    []string{".hs", ".lhs"},
			LSPServers:    []string{"haskell-language-server", "hls"},
			CommentSingle: "--",
			CommentMultiS: "{-",
			CommentMultiE: "-}",
			IndentStyle:   "spaces",
			IndentSize:    2,
			FilePatterns:  []string{"stack.yaml", "cabal.project"},
		},
		{
			Name:          "Clojure",
			ID:            "clojure",
			Extensions:    []string{".clj", ".cljs", ".cljc", ".edn"},
			LSPServers:    []string{"clojure-lsp"},
			CommentSingle: ";",
			IndentStyle:   "spaces",
			IndentSize:    2,
			FilePatterns:  []string{"deps.edn", "project.clj"},
		},
		{
			Name:          "Dart",
			ID:            "dart",
			Extensions:    []string{".dart"},
			LSPServers:    []string{"dart-language-server"},
			CommentSingle: "//",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    2,
			FilePatterns:  []string{"pubspec.yaml"},
		},
		{
			Name:          "Vue",
			ID:            "vue",
			Extensions:    []string{".vue"},
			LSPServers:    []string{"volar", "vue-language-server"},
			CommentMultiS: "<!--",
			CommentMultiE: "-->",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:          "Svelte",
			ID:            "svelte",
			Extensions:    []string{".svelte"},
			LSPServers:    []string{"svelte-language-server"},
			CommentMultiS: "<!--",
			CommentMultiE: "-->",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:          "GraphQL",
			ID:            "graphql",
			Extensions:    []string{".graphql", ".gql"},
			LSPServers:    []string{"graphql-language-service-cli"},
			CommentSingle: "#",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:          "Terraform",
			ID:            "terraform",
			Extensions:    []string{".tf", ".tfvars"},
			LSPServers:    []string{"terraform-ls", "terraform-lsp"},
			CommentSingle: "#",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:          "Zig",
			ID:            "zig",
			Extensions:    []string{".zig"},
			LSPServers:    []string{"zls"},
			CommentSingle: "//",
			IndentStyle:   "spaces",
			IndentSize:    4,
		},
		{
			Name:          "Nim",
			ID:            "nim",
			Extensions:    []string{".nim", ".nims"},
			LSPServers:    []string{"nimlsp", "nimlangserver"},
			CommentSingle: "#",
			CommentMultiS: "#[",
			CommentMultiE: "]#",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:          "OCaml",
			ID:            "ocaml",
			Extensions:    []string{".ml", ".mli"},
			LSPServers:    []string{"ocamllsp", "ocaml-language-server"},
			CommentMultiS: "(*",
			CommentMultiE: "*)",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:          "F#",
			ID:            "fsharp",
			Extensions:    []string{".fs", ".fsi", ".fsx"},
			LSPServers:    []string{"fsautocomplete"},
			CommentSingle: "//",
			CommentMultiS: "(*",
			CommentMultiE: "*)",
			IndentStyle:   "spaces",
			IndentSize:    4,
		},
		{
			Name:          "Protocol Buffers",
			ID:            "proto",
			Extensions:    []string{".proto"},
			LSPServers:    []string{"bufls", "pbkit"},
			CommentSingle: "//",
			CommentMultiS: "/*",
			CommentMultiE: "*/",
			IndentStyle:   "spaces",
			IndentSize:    2,
		},
		{
			Name:        "Plain Text",
			ID:          "plaintext",
			Extensions:  []string{".txt", ".text"},
			IndentStyle: "spaces",
			IndentSize:  4,
		},
	}

	for _, lang := range defaultLanguages {
		r.RegisterLanguage(lang)
	}
}

// RegisterLanguage registers a language with its extensions and patterns
func (r *ExtensionRegistry) RegisterLanguage(lang *LanguageInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Register by ID
	r.byID[lang.ID] = lang

	// Register extensions (case-insensitive)
	for _, ext := range lang.Extensions {
		r.languages[strings.ToLower(ext)] = lang
	}

	// Register file patterns
	for _, pattern := range lang.FilePatterns {
		r.patterns[strings.ToLower(pattern)] = lang
	}
}

// GetLanguage returns the language ID for a file extension
func (r *ExtensionRegistry) GetLanguage(ext string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if lang, ok := r.languages[strings.ToLower(ext)]; ok {
		return lang.ID
	}
	return "unknown"
}

// GetLanguageInfo returns full language info for an extension
func (r *ExtensionRegistry) GetLanguageInfo(ext string) *LanguageInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.languages[strings.ToLower(ext)]
}

// GetLanguageByID returns language info by language ID
func (r *ExtensionRegistry) GetLanguageByID(id string) *LanguageInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.byID[id]
}

// GetLanguageForPath determines the language for a file path
// It checks both extension and filename patterns
func (r *ExtensionRegistry) GetLanguageForPath(path string) *LanguageInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check extension first
	ext := strings.ToLower(filepath.Ext(path))
	if lang, ok := r.languages[ext]; ok {
		return lang
	}

	// Check filename patterns
	filename := strings.ToLower(filepath.Base(path))
	if lang, ok := r.patterns[filename]; ok {
		return lang
	}

	return nil
}

// GetLSPServers returns the list of known LSP servers for a language
func (r *ExtensionRegistry) GetLSPServers(languageID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if lang, ok := r.byID[languageID]; ok {
		return lang.LSPServers
	}
	return nil
}

// GetCommentStyle returns the comment style for a language
func (r *ExtensionRegistry) GetCommentStyle(languageID string) (single, multiStart, multiEnd string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if lang, ok := r.byID[languageID]; ok {
		return lang.CommentSingle, lang.CommentMultiS, lang.CommentMultiE
	}
	return "", "", ""
}

// GetIndentStyle returns the indent style for a language
func (r *ExtensionRegistry) GetIndentStyle(languageID string) (style string, size int) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if lang, ok := r.byID[languageID]; ok {
		return lang.IndentStyle, lang.IndentSize
	}
	return "spaces", 4
}

// ListLanguages returns all registered language IDs
func (r *ExtensionRegistry) ListLanguages() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.byID))
	for id := range r.byID {
		ids = append(ids, id)
	}
	return ids
}

// ListExtensions returns all registered extensions for a language
func (r *ExtensionRegistry) ListExtensions(languageID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if lang, ok := r.byID[languageID]; ok {
		return lang.Extensions
	}
	return nil
}

// SupportsLSP returns true if the language has known LSP servers
func (r *ExtensionRegistry) SupportsLSP(languageID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if lang, ok := r.byID[languageID]; ok {
		return len(lang.LSPServers) > 0
	}
	return false
}
