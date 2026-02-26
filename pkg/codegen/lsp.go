package codegen

import (
	"context"

	"github.com/soypete/pedrocli/pkg/lsp"
)

// LSPAwareWriter wraps the code writer with LSP intelligence.
// Before writing, it queries gopls for current state.
// After writing, it checks diagnostics for errors.
type LSPAwareWriter struct {
	manager *lsp.Manager
}

// NewLSPAwareWriter creates an LSPAwareWriter with the given LSP manager.
// If manager is nil, all LSP operations are no-ops.
func NewLSPAwareWriter(manager *lsp.Manager) *LSPAwareWriter {
	return &LSPAwareWriter{manager: manager}
}

// PreWriteReport contains information gathered before a write operation.
type PreWriteReport struct {
	TargetExists   bool
	ExistingKind   string
	ExistingLine   int
	ExistingErrors []lsp.DiagnosticResult
	References     []lsp.LocationResult
}

// PreWriteCheck queries LSP to validate the planned write operation.
func (w *LSPAwareWriter) PreWriteCheck(ctx context.Context, path string, targetName string) (*PreWriteReport, error) {
	if w.manager == nil {
		return &PreWriteReport{}, nil
	}

	report := &PreWriteReport{}

	// Check if target already exists via document symbols
	symbols, err := w.manager.Symbols(ctx, path, "file")
	if err == nil {
		for _, s := range symbols {
			if s.Name == targetName {
				report.TargetExists = true
				report.ExistingKind = s.Kind
				report.ExistingLine = s.Line
				break
			}
		}
	}

	// Get current diagnostics
	diags, err := w.manager.Diagnostics(ctx, path)
	if err == nil {
		report.ExistingErrors = diags
	}

	// If target exists, find references
	if report.TargetExists {
		refs, err := w.manager.References(ctx, path, report.ExistingLine, 1)
		if err == nil {
			report.References = refs
		}
	}

	return report, nil
}

// PostWriteValidate checks diagnostics after writing to catch issues.
func (w *LSPAwareWriter) PostWriteValidate(ctx context.Context, path string) ([]lsp.DiagnosticResult, error) {
	if w.manager == nil {
		return nil, nil
	}

	return w.manager.Diagnostics(ctx, path)
}
