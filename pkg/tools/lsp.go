package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/logits"
	"github.com/soypete/pedrocli/pkg/lsp"
)

// LSPTool provides Language Server Protocol operations for code navigation and analysis.
type LSPTool struct {
	manager   *lsp.Manager
	config    *config.LSPConfig
	workspace string
}

// NewLSPTool creates a new LSP tool.
func NewLSPTool(cfg *config.Config, workspace string) *LSPTool {
	var lspConfig *config.LSPConfig
	if cfg != nil {
		lspConfig = &cfg.LSP
	}

	return &LSPTool{
		manager:   lsp.NewManager(lspConfig, workspace),
		config:    lspConfig,
		workspace: workspace,
	}
}

// Name returns the tool name.
func (t *LSPTool) Name() string {
	return "lsp"
}

// Description returns the tool description.
func (t *LSPTool) Description() string {
	return `Language Server Protocol tool for intelligent code navigation and analysis.

Operations:
- definition: Go to where a symbol (function, type, variable) is defined
  Args: file (string), line (int, 1-indexed), column (int, 1-indexed)
- references: Find all places where a symbol is used
  Args: file (string), line (int, 1-indexed), column (int, 1-indexed)
- hover: Get type information and documentation for a symbol
  Args: file (string), line (int, 1-indexed), column (int, 1-indexed)
- diagnostics: Get compiler errors and warnings for a file
  Args: file (string)
- symbols: List all symbols (functions, types, variables) in a file or workspace
  Args: file (string), scope (optional: "file" or "workspace", defaults to "file")

Use this tool to:
- Understand unfamiliar code by jumping to definitions
- Find all usages before refactoring
- Check for errors after making changes
- Navigate large codebases efficiently

The LSP tool is more accurate than grep/search for understanding symbol relationships.

Supported languages: Go (gopls), TypeScript/JavaScript, Python, Rust, C/C++, and more.

Examples:
{"tool": "lsp", "args": {"operation": "definition", "file": "pkg/server/handler.go", "line": 42, "column": 15}}
{"tool": "lsp", "args": {"operation": "references", "file": "main.go", "line": 10, "column": 5}}
{"tool": "lsp", "args": {"operation": "diagnostics", "file": "pkg/api/routes.go"}}
{"tool": "lsp", "args": {"operation": "symbols", "file": "internal/service.go"}}`
}

// Execute executes the LSP tool.
func (t *LSPTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	operation, ok := args["operation"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "missing 'operation' parameter (required: definition, references, hover, diagnostics, symbols)",
		}, nil
	}

	file, ok := args["file"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "missing 'file' parameter",
		}, nil
	}

	// Convert relative path to absolute
	if !filepath.IsAbs(file) {
		file = filepath.Join(t.workspace, file)
	}

	switch operation {
	case "definition":
		return t.definition(ctx, file, args)
	case "references":
		return t.references(ctx, file, args)
	case "hover":
		return t.hover(ctx, file, args)
	case "diagnostics":
		return t.diagnostics(ctx, file)
	case "symbols":
		return t.symbols(ctx, file, args)
	default:
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("unknown operation: %s (valid: definition, references, hover, diagnostics, symbols)", operation),
		}, nil
	}
}

// definition handles the definition operation.
func (t *LSPTool) definition(ctx context.Context, file string, args map[string]interface{}) (*Result, error) {
	line, col, err := t.extractPosition(args)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	locations, err := t.manager.Definition(ctx, file, line, col)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("definition lookup failed: %v", err),
		}, nil
	}

	if len(locations) == 0 {
		return &Result{
			Success: true,
			Output:  fmt.Sprintf("No definition found at %s:%d:%d", t.relPath(file), line, col),
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d definition(s):\n", len(locations)))
	for _, loc := range locations {
		sb.WriteString(fmt.Sprintf("  %s:%d:%d\n", t.relPath(loc.FilePath), loc.StartLine, loc.StartCol))
	}

	return &Result{
		Success: true,
		Output:  sb.String(),
		Data: map[string]interface{}{
			"locations": locations,
		},
	}, nil
}

// references handles the references operation.
func (t *LSPTool) references(ctx context.Context, file string, args map[string]interface{}) (*Result, error) {
	line, col, err := t.extractPosition(args)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	locations, err := t.manager.References(ctx, file, line, col)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("references lookup failed: %v", err),
		}, nil
	}

	if len(locations) == 0 {
		return &Result{
			Success: true,
			Output:  fmt.Sprintf("No references found at %s:%d:%d", t.relPath(file), line, col),
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d reference(s):\n", len(locations)))
	for _, loc := range locations {
		sb.WriteString(fmt.Sprintf("  %s:%d:%d\n", t.relPath(loc.FilePath), loc.StartLine, loc.StartCol))
	}

	return &Result{
		Success: true,
		Output:  sb.String(),
		Data: map[string]interface{}{
			"locations": locations,
		},
	}, nil
}

// hover handles the hover operation.
func (t *LSPTool) hover(ctx context.Context, file string, args map[string]interface{}) (*Result, error) {
	line, col, err := t.extractPosition(args)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	content, err := t.manager.Hover(ctx, file, line, col)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("hover lookup failed: %v", err),
		}, nil
	}

	if content == "" {
		return &Result{
			Success: true,
			Output:  fmt.Sprintf("No hover information at %s:%d:%d", t.relPath(file), line, col),
		}, nil
	}

	return &Result{
		Success: true,
		Output:  content,
		Data: map[string]interface{}{
			"content": content,
		},
	}, nil
}

// diagnostics handles the diagnostics operation.
func (t *LSPTool) diagnostics(ctx context.Context, file string) (*Result, error) {
	diags, err := t.manager.Diagnostics(ctx, file)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("diagnostics lookup failed: %v", err),
		}, nil
	}

	if len(diags) == 0 {
		return &Result{
			Success: true,
			Output:  fmt.Sprintf("No diagnostics for %s", t.relPath(file)),
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d diagnostic(s) in %s:\n\n", len(diags), t.relPath(file)))

	// Group by severity
	errors := []lsp.DiagnosticResult{}
	warnings := []lsp.DiagnosticResult{}
	other := []lsp.DiagnosticResult{}

	for _, d := range diags {
		switch d.Severity {
		case "error":
			errors = append(errors, d)
		case "warning":
			warnings = append(warnings, d)
		default:
			other = append(other, d)
		}
	}

	if len(errors) > 0 {
		sb.WriteString("## Errors\n")
		for _, d := range errors {
			sb.WriteString(fmt.Sprintf("- Line %d: %s", d.Line, d.Message))
			if d.Code != "" {
				sb.WriteString(fmt.Sprintf(" [%s]", d.Code))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(warnings) > 0 {
		sb.WriteString("## Warnings\n")
		for _, d := range warnings {
			sb.WriteString(fmt.Sprintf("- Line %d: %s", d.Line, d.Message))
			if d.Code != "" {
				sb.WriteString(fmt.Sprintf(" [%s]", d.Code))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(other) > 0 {
		sb.WriteString("## Info/Hints\n")
		for _, d := range other {
			sb.WriteString(fmt.Sprintf("- Line %d: %s", d.Line, d.Message))
			if d.Code != "" {
				sb.WriteString(fmt.Sprintf(" [%s]", d.Code))
			}
			sb.WriteString("\n")
		}
	}

	return &Result{
		Success: true,
		Output:  sb.String(),
		Data: map[string]interface{}{
			"diagnostics": diags,
		},
	}, nil
}

// symbols handles the symbols operation.
func (t *LSPTool) symbols(ctx context.Context, file string, args map[string]interface{}) (*Result, error) {
	scope := "file"
	if s, ok := args["scope"].(string); ok {
		scope = s
	}

	symbols, err := t.manager.Symbols(ctx, file, scope)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("symbols lookup failed: %v", err),
		}, nil
	}

	if len(symbols) == 0 {
		return &Result{
			Success: true,
			Output:  fmt.Sprintf("No symbols found in %s (scope: %s)", t.relPath(file), scope),
		}, nil
	}

	var sb strings.Builder
	if scope == "workspace" {
		sb.WriteString(fmt.Sprintf("Found %d symbol(s) in workspace:\n\n", len(symbols)))
	} else {
		sb.WriteString(fmt.Sprintf("Found %d symbol(s) in %s:\n\n", len(symbols), t.relPath(file)))
	}

	t.formatSymbols(&sb, symbols, 0)

	return &Result{
		Success: true,
		Output:  sb.String(),
		Data: map[string]interface{}{
			"symbols": symbols,
		},
	}, nil
}

// formatSymbols formats symbols with indentation for hierarchy.
func (t *LSPTool) formatSymbols(sb *strings.Builder, symbols []lsp.SymbolResult, indent int) {
	indentStr := strings.Repeat("  ", indent)
	for _, sym := range symbols {
		location := fmt.Sprintf("%s:%d", t.relPath(sym.FilePath), sym.Line)
		if sym.Detail != "" {
			sb.WriteString(fmt.Sprintf("%s- %s %s (%s) %s\n", indentStr, sym.Kind, sym.Name, sym.Detail, location))
		} else {
			sb.WriteString(fmt.Sprintf("%s- %s %s %s\n", indentStr, sym.Kind, sym.Name, location))
		}
		if len(sym.Children) > 0 {
			t.formatSymbols(sb, sym.Children, indent+1)
		}
	}
}

// extractPosition extracts line and column from args.
func (t *LSPTool) extractPosition(args map[string]interface{}) (int, int, error) {
	var line, col int

	switch v := args["line"].(type) {
	case float64:
		line = int(v)
	case int:
		line = v
	default:
		return 0, 0, fmt.Errorf("missing or invalid 'line' parameter (must be integer)")
	}

	switch v := args["column"].(type) {
	case float64:
		col = int(v)
	case int:
		col = v
	default:
		return 0, 0, fmt.Errorf("missing or invalid 'column' parameter (must be integer)")
	}

	if line < 1 {
		return 0, 0, fmt.Errorf("'line' must be >= 1 (1-indexed)")
	}
	if col < 1 {
		return 0, 0, fmt.Errorf("'column' must be >= 1 (1-indexed)")
	}

	return line, col, nil
}

// relPath returns a relative path for display.
func (t *LSPTool) relPath(absPath string) string {
	if rel, err := filepath.Rel(t.workspace, absPath); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return absPath
}

// Shutdown shuts down the LSP manager.
func (t *LSPTool) Shutdown(ctx context.Context) error {
	return t.manager.Shutdown(ctx)
}

// Metadata returns tool metadata for dynamic discovery.
func (t *LSPTool) Metadata() *ToolMetadata {
	return &ToolMetadata{
		Schema: &logits.JSONSchema{
			Type: "object",
			Properties: map[string]*logits.JSONSchema{
				"operation": {
					Type:        "string",
					Enum:        []interface{}{"definition", "references", "hover", "diagnostics", "symbols"},
					Description: "The LSP operation to perform",
				},
				"file": {
					Type:        "string",
					Description: "File path relative to workspace root",
				},
				"line": {
					Type:        "integer",
					Description: "Line number (1-indexed)",
				},
				"column": {
					Type:        "integer",
					Description: "Column position (1-indexed)",
				},
				"scope": {
					Type:        "string",
					Enum:        []interface{}{"file", "workspace"},
					Description: "Scope for symbols operation (default: file)",
				},
			},
			Required: []string{"operation", "file"},
		},
		Category:    CategoryCode,
		Optionality: ToolOptional,
		UsageHint: `Use LSP for precise code navigation:
- Before modifying a function, use 'references' to find all callers
- Use 'definition' to understand unfamiliar symbols
- Use 'hover' to get type information and documentation
- Use 'diagnostics' to check for errors after changes
- Use 'symbols' to get an overview of a file's structure`,
		Examples: []ToolExample{
			{
				Description: "Find definition of symbol at position",
				Input:       map[string]interface{}{"operation": "definition", "file": "main.go", "line": 42, "column": 15},
			},
			{
				Description: "Find all references to symbol",
				Input:       map[string]interface{}{"operation": "references", "file": "pkg/api/handler.go", "line": 25, "column": 6},
			},
			{
				Description: "Get diagnostics (errors/warnings) for a file",
				Input:       map[string]interface{}{"operation": "diagnostics", "file": "internal/server.go"},
			},
			{
				Description: "List symbols in a file",
				Input:       map[string]interface{}{"operation": "symbols", "file": "pkg/models/user.go"},
			},
		},
		RequiresCapabilities: []string{"lsp"},
		Produces:             []string{"locations", "diagnostics", "symbols", "hover_content"},
	}
}
