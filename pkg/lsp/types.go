// Package lsp provides Language Server Protocol client functionality for code intelligence.
package lsp

// Position represents a position in a text document.
type Position struct {
	Line      int `json:"line"`      // Zero-indexed line number
	Character int `json:"character"` // Zero-indexed character offset
}

// Range represents a range in a text document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a location in a document.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// LocationResult is a simplified location for tool output.
type LocationResult struct {
	FilePath  string `json:"file_path"`
	StartLine int    `json:"start_line"` // 1-indexed for user display
	StartCol  int    `json:"start_col"`  // 1-indexed for user display
	EndLine   int    `json:"end_line"`
	EndCol    int    `json:"end_col"`
}

// DiagnosticSeverity represents the severity of a diagnostic.
type DiagnosticSeverity int

const (
	DiagnosticSeverityError       DiagnosticSeverity = 1
	DiagnosticSeverityWarning     DiagnosticSeverity = 2
	DiagnosticSeverityInformation DiagnosticSeverity = 3
	DiagnosticSeverityHint        DiagnosticSeverity = 4
)

// String returns a string representation of the severity.
func (s DiagnosticSeverity) String() string {
	switch s {
	case DiagnosticSeverityError:
		return "error"
	case DiagnosticSeverityWarning:
		return "warning"
	case DiagnosticSeverityInformation:
		return "info"
	case DiagnosticSeverityHint:
		return "hint"
	default:
		return "unknown"
	}
}

// Diagnostic represents a diagnostic (error, warning, etc.).
type Diagnostic struct {
	Range    Range              `json:"range"`
	Severity DiagnosticSeverity `json:"severity,omitempty"`
	Code     interface{}        `json:"code,omitempty"` // Can be string or number
	Source   string             `json:"source,omitempty"`
	Message  string             `json:"message"`
}

// DiagnosticResult is a simplified diagnostic for tool output.
type DiagnosticResult struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`   // 1-indexed
	Column   int    `json:"column"` // 1-indexed
	EndLine  int    `json:"end_line"`
	EndCol   int    `json:"end_column"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Source   string `json:"source,omitempty"`
	Code     string `json:"code,omitempty"`
}

// SymbolKind represents the kind of a symbol.
type SymbolKind int

const (
	SymbolKindFile          SymbolKind = 1
	SymbolKindModule        SymbolKind = 2
	SymbolKindNamespace     SymbolKind = 3
	SymbolKindPackage       SymbolKind = 4
	SymbolKindClass         SymbolKind = 5
	SymbolKindMethod        SymbolKind = 6
	SymbolKindProperty      SymbolKind = 7
	SymbolKindField         SymbolKind = 8
	SymbolKindConstructor   SymbolKind = 9
	SymbolKindEnum          SymbolKind = 10
	SymbolKindInterface     SymbolKind = 11
	SymbolKindFunction      SymbolKind = 12
	SymbolKindVariable      SymbolKind = 13
	SymbolKindConstant      SymbolKind = 14
	SymbolKindString        SymbolKind = 15
	SymbolKindNumber        SymbolKind = 16
	SymbolKindBoolean       SymbolKind = 17
	SymbolKindArray         SymbolKind = 18
	SymbolKindObject        SymbolKind = 19
	SymbolKindKey           SymbolKind = 20
	SymbolKindNull          SymbolKind = 21
	SymbolKindEnumMember    SymbolKind = 22
	SymbolKindStruct        SymbolKind = 23
	SymbolKindEvent         SymbolKind = 24
	SymbolKindOperator      SymbolKind = 25
	SymbolKindTypeParameter SymbolKind = 26
)

// String returns a string representation of the symbol kind.
func (k SymbolKind) String() string {
	kinds := map[SymbolKind]string{
		SymbolKindFile:          "file",
		SymbolKindModule:        "module",
		SymbolKindNamespace:     "namespace",
		SymbolKindPackage:       "package",
		SymbolKindClass:         "class",
		SymbolKindMethod:        "method",
		SymbolKindProperty:      "property",
		SymbolKindField:         "field",
		SymbolKindConstructor:   "constructor",
		SymbolKindEnum:          "enum",
		SymbolKindInterface:     "interface",
		SymbolKindFunction:      "function",
		SymbolKindVariable:      "variable",
		SymbolKindConstant:      "constant",
		SymbolKindString:        "string",
		SymbolKindNumber:        "number",
		SymbolKindBoolean:       "boolean",
		SymbolKindArray:         "array",
		SymbolKindObject:        "object",
		SymbolKindKey:           "key",
		SymbolKindNull:          "null",
		SymbolKindEnumMember:    "enumMember",
		SymbolKindStruct:        "struct",
		SymbolKindEvent:         "event",
		SymbolKindOperator:      "operator",
		SymbolKindTypeParameter: "typeParameter",
	}
	if s, ok := kinds[k]; ok {
		return s
	}
	return "unknown"
}

// DocumentSymbol represents a symbol in a document.
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           SymbolKind       `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// SymbolInformation represents symbol information (alternative to DocumentSymbol).
type SymbolInformation struct {
	Name          string     `json:"name"`
	Kind          SymbolKind `json:"kind"`
	Location      Location   `json:"location"`
	ContainerName string     `json:"containerName,omitempty"`
}

// SymbolResult is a simplified symbol for tool output.
type SymbolResult struct {
	Name     string         `json:"name"`
	Kind     string         `json:"kind"`
	FilePath string         `json:"file_path"`
	Line     int            `json:"line"`   // 1-indexed
	Column   int            `json:"column"` // 1-indexed
	EndLine  int            `json:"end_line"`
	EndCol   int            `json:"end_column"`
	Detail   string         `json:"detail,omitempty"`
	Children []SymbolResult `json:"children,omitempty"`
}

// Hover represents hover information.
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// MarkupContent represents markup content.
type MarkupContent struct {
	Kind  string `json:"kind"` // "plaintext" or "markdown"
	Value string `json:"value"`
}

// TextDocumentIdentifier identifies a text document.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentPositionParams contains position parameters for a text document.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// ReferenceContext contains reference parameters.
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// ReferenceParams contains reference request parameters.
type ReferenceParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      ReferenceContext       `json:"context"`
}

// DocumentSymbolParams contains document symbol request parameters.
type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// WorkspaceSymbolParams contains workspace symbol request parameters.
type WorkspaceSymbolParams struct {
	Query string `json:"query"`
}

// PublishDiagnosticsParams contains diagnostic notification parameters.
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// TextDocumentItem represents a text document.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// DidOpenTextDocumentParams contains parameters for textDocument/didOpen.
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// VersionedTextDocumentIdentifier identifies a versioned text document.
type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

// TextDocumentContentChangeEvent represents a content change event.
type TextDocumentContentChangeEvent struct {
	Text string `json:"text"` // Full content for full sync
}

// DidChangeTextDocumentParams contains parameters for textDocument/didChange.
type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

// DidCloseTextDocumentParams contains parameters for textDocument/didClose.
type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}
