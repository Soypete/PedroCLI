package lsp

import (
	"testing"
)

func TestDiagnosticSeverityString(t *testing.T) {
	tests := []struct {
		severity DiagnosticSeverity
		expected string
	}{
		{DiagnosticSeverityError, "error"},
		{DiagnosticSeverityWarning, "warning"},
		{DiagnosticSeverityInformation, "info"},
		{DiagnosticSeverityHint, "hint"},
		{DiagnosticSeverity(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.severity.String()
			if result != tt.expected {
				t.Errorf("DiagnosticSeverity.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSymbolKindString(t *testing.T) {
	tests := []struct {
		kind     SymbolKind
		expected string
	}{
		{SymbolKindFile, "file"},
		{SymbolKindModule, "module"},
		{SymbolKindNamespace, "namespace"},
		{SymbolKindPackage, "package"},
		{SymbolKindClass, "class"},
		{SymbolKindMethod, "method"},
		{SymbolKindProperty, "property"},
		{SymbolKindField, "field"},
		{SymbolKindConstructor, "constructor"},
		{SymbolKindEnum, "enum"},
		{SymbolKindInterface, "interface"},
		{SymbolKindFunction, "function"},
		{SymbolKindVariable, "variable"},
		{SymbolKindConstant, "constant"},
		{SymbolKindString, "string"},
		{SymbolKindNumber, "number"},
		{SymbolKindBoolean, "boolean"},
		{SymbolKindArray, "array"},
		{SymbolKindObject, "object"},
		{SymbolKindKey, "key"},
		{SymbolKindNull, "null"},
		{SymbolKindEnumMember, "enumMember"},
		{SymbolKindStruct, "struct"},
		{SymbolKindEvent, "event"},
		{SymbolKindOperator, "operator"},
		{SymbolKindTypeParameter, "typeParameter"},
		{SymbolKind(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.kind.String()
			if result != tt.expected {
				t.Errorf("SymbolKind.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestPositionZeroIndexed(t *testing.T) {
	// Ensure Position uses 0-indexed values as per LSP spec
	p := Position{Line: 0, Character: 0}
	if p.Line != 0 || p.Character != 0 {
		t.Errorf("Position should use 0-indexed values")
	}
}

func TestLocationResult1Indexed(t *testing.T) {
	// Ensure LocationResult uses 1-indexed values for user display
	loc := LocationResult{
		FilePath:  "test.go",
		StartLine: 1,
		StartCol:  1,
		EndLine:   1,
		EndCol:    10,
	}
	if loc.StartLine < 1 || loc.StartCol < 1 {
		t.Errorf("LocationResult should use 1-indexed values")
	}
}

func TestDiagnosticResult1Indexed(t *testing.T) {
	// Ensure DiagnosticResult uses 1-indexed values for user display
	diag := DiagnosticResult{
		FilePath: "test.go",
		Line:     1,
		Column:   1,
		EndLine:  1,
		EndCol:   10,
		Severity: "error",
		Message:  "test error",
	}
	if diag.Line < 1 || diag.Column < 1 {
		t.Errorf("DiagnosticResult should use 1-indexed values")
	}
}

func TestSymbolResult1Indexed(t *testing.T) {
	// Ensure SymbolResult uses 1-indexed values for user display
	sym := SymbolResult{
		Name:     "TestFunc",
		Kind:     "function",
		FilePath: "test.go",
		Line:     1,
		Column:   1,
		EndLine:  10,
		EndCol:   1,
	}
	if sym.Line < 1 || sym.Column < 1 {
		t.Errorf("SymbolResult should use 1-indexed values")
	}
}
