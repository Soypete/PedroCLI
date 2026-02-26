package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
)

// FormatGoSource takes raw Go source code (as the agent produces it),
// parses it through go/parser for validation, and returns gofmt-compliant output.
// If the source has parse errors, it returns the original source with the error.
func FormatGoSource(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return src, fmt.Errorf("parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, astFile); err != nil {
		return src, fmt.Errorf("AST print error: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.Bytes(), fmt.Errorf("gofmt error: %w", err)
	}

	return formatted, nil
}

// ValidateGoSource checks whether Go source code is syntactically valid.
// Returns nil if valid, or a parse error describing the issue.
func ValidateGoSource(src []byte) error {
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	return err
}
