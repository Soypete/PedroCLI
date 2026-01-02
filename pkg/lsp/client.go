package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Client represents an LSP client connected to a language server.
type Client struct {
	conn        *JSONRPCConn
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	stderr      io.ReadCloser
	language    string
	workspace   string
	initialized bool
	mu          sync.RWMutex

	// Diagnostics cache
	diagnostics   map[string][]Diagnostic
	diagnosticsMu sync.RWMutex

	// Document tracking
	openDocs   map[string]int // URI -> version
	openDocsMu sync.RWMutex
}

// InitializeParams contains LSP initialize request parameters.
type InitializeParams struct {
	ProcessID    int                `json:"processId"`
	RootURI      string             `json:"rootUri"`
	RootPath     string             `json:"rootPath"`
	Capabilities ClientCapabilities `json:"capabilities"`
}

// ClientCapabilities represents client capabilities.
type ClientCapabilities struct {
	TextDocument TextDocumentClientCapabilities `json:"textDocument,omitempty"`
	Workspace    WorkspaceClientCapabilities    `json:"workspace,omitempty"`
}

// TextDocumentClientCapabilities represents text document capabilities.
type TextDocumentClientCapabilities struct {
	Synchronization    *TextDocumentSyncClientCapabilities `json:"synchronization,omitempty"`
	PublishDiagnostics *PublishDiagnosticsCapability       `json:"publishDiagnostics,omitempty"`
}

// TextDocumentSyncClientCapabilities represents sync capabilities.
type TextDocumentSyncClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	WillSave            bool `json:"willSave,omitempty"`
	WillSaveWaitUntil   bool `json:"willSaveWaitUntil,omitempty"`
	DidSave             bool `json:"didSave,omitempty"`
}

// PublishDiagnosticsCapability represents diagnostic capabilities.
type PublishDiagnosticsCapability struct {
	RelatedInformation     bool `json:"relatedInformation,omitempty"`
	TagSupport             bool `json:"tagSupport,omitempty"`
	VersionSupport         bool `json:"versionSupport,omitempty"`
	CodeDescriptionSupport bool `json:"codeDescriptionSupport,omitempty"`
	DataSupport            bool `json:"dataSupport,omitempty"`
}

// WorkspaceClientCapabilities represents workspace capabilities.
type WorkspaceClientCapabilities struct {
	ApplyEdit bool `json:"applyEdit,omitempty"`
}

// InitializeResult contains LSP initialize response.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

// ServerCapabilities represents server capabilities.
type ServerCapabilities struct {
	TextDocumentSync           interface{} `json:"textDocumentSync,omitempty"`
	HoverProvider              bool        `json:"hoverProvider,omitempty"`
	CompletionProvider         interface{} `json:"completionProvider,omitempty"`
	DefinitionProvider         bool        `json:"definitionProvider,omitempty"`
	ReferencesProvider         bool        `json:"referencesProvider,omitempty"`
	DocumentSymbolProvider     bool        `json:"documentSymbolProvider,omitempty"`
	WorkspaceSymbolProvider    bool        `json:"workspaceSymbolProvider,omitempty"`
	RenameProvider             interface{} `json:"renameProvider,omitempty"`
	DocumentFormattingProvider bool        `json:"documentFormattingProvider,omitempty"`
}

// NewClient creates a new LSP client and connects to the server.
func NewClient(ctx context.Context, command string, args []string, workspace string, language string) (*Client, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workspace

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start language server: %w", err)
	}

	client := &Client{
		cmd:         cmd,
		stdin:       stdin,
		stdout:      stdout,
		stderr:      stderr,
		language:    language,
		workspace:   workspace,
		diagnostics: make(map[string][]Diagnostic),
		openDocs:    make(map[string]int),
	}

	// Create JSON-RPC connection
	client.conn = NewJSONRPCConn(stdout, stdin)

	// Register diagnostic handler
	client.conn.OnNotification("textDocument/publishDiagnostics", client.handleDiagnostics)

	// Initialize the server
	if err := client.initialize(ctx); err != nil {
		client.Close()
		return nil, err
	}

	return client, nil
}

// initialize performs LSP initialization handshake.
func (c *Client) initialize(ctx context.Context) error {
	params := InitializeParams{
		ProcessID: os.Getpid(),
		RootURI:   "file://" + c.workspace,
		RootPath:  c.workspace,
		Capabilities: ClientCapabilities{
			TextDocument: TextDocumentClientCapabilities{
				Synchronization: &TextDocumentSyncClientCapabilities{
					DidSave: true,
				},
				PublishDiagnostics: &PublishDiagnosticsCapability{
					RelatedInformation: true,
				},
			},
			Workspace: WorkspaceClientCapabilities{
				ApplyEdit: false,
			},
		},
	}

	var result InitializeResult
	if err := c.conn.Call("initialize", params, &result); err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	// Send initialized notification
	if err := c.conn.Notify("initialized", struct{}{}); err != nil {
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	c.mu.Lock()
	c.initialized = true
	c.mu.Unlock()

	return nil
}

// Close shuts down the client and language server.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		// Try to send shutdown request
		_ = c.conn.Call("shutdown", nil, nil)
		_ = c.conn.Notify("exit", nil)
		c.conn.Close()
	}

	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.stdout != nil {
		c.stdout.Close()
	}
	if c.stderr != nil {
		c.stderr.Close()
	}

	if c.cmd != nil && c.cmd.Process != nil {
		// Give the process a moment to exit gracefully
		done := make(chan error, 1)
		go func() {
			done <- c.cmd.Wait()
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			c.cmd.Process.Kill()
		}
	}

	return nil
}

// IsReady returns whether the client is initialized and ready.
func (c *Client) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.initialized
}

// OpenFile opens a file in the language server.
func (c *Client) OpenFile(ctx context.Context, filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	uri := pathToURI(filePath)

	c.openDocsMu.Lock()
	version := 1
	c.openDocs[uri] = version
	c.openDocsMu.Unlock()

	params := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: c.language,
			Version:    version,
			Text:       string(content),
		},
	}

	return c.conn.Notify("textDocument/didOpen", params)
}

// CloseFile closes a file in the language server.
func (c *Client) CloseFile(ctx context.Context, filePath string) error {
	uri := pathToURI(filePath)

	c.openDocsMu.Lock()
	delete(c.openDocs, uri)
	c.openDocsMu.Unlock()

	params := DidCloseTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}

	return c.conn.Notify("textDocument/didClose", params)
}

// Definition returns the definition location for a symbol.
func (c *Client) Definition(ctx context.Context, filePath string, line, col int) ([]LocationResult, error) {
	uri := pathToURI(filePath)

	// Ensure file is open
	if err := c.ensureOpen(ctx, filePath); err != nil {
		return nil, err
	}

	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line - 1, Character: col - 1}, // Convert to 0-indexed
	}

	var result json.RawMessage
	if err := c.conn.Call("textDocument/definition", params, &result); err != nil {
		return nil, fmt.Errorf("definition request failed: %w", err)
	}

	return c.parseLocations(result)
}

// References returns all references to a symbol.
func (c *Client) References(ctx context.Context, filePath string, line, col int, includeDeclaration bool) ([]LocationResult, error) {
	uri := pathToURI(filePath)

	// Ensure file is open
	if err := c.ensureOpen(ctx, filePath); err != nil {
		return nil, err
	}

	params := ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line - 1, Character: col - 1},
		Context:      ReferenceContext{IncludeDeclaration: includeDeclaration},
	}

	var result json.RawMessage
	if err := c.conn.Call("textDocument/references", params, &result); err != nil {
		return nil, fmt.Errorf("references request failed: %w", err)
	}

	return c.parseLocations(result)
}

// Hover returns hover information for a symbol.
func (c *Client) Hover(ctx context.Context, filePath string, line, col int) (string, error) {
	uri := pathToURI(filePath)

	// Ensure file is open
	if err := c.ensureOpen(ctx, filePath); err != nil {
		return "", err
	}

	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line - 1, Character: col - 1},
	}

	var result Hover
	if err := c.conn.Call("textDocument/hover", params, &result); err != nil {
		return "", fmt.Errorf("hover request failed: %w", err)
	}

	return result.Contents.Value, nil
}

// DocumentSymbols returns symbols in a document.
func (c *Client) DocumentSymbols(ctx context.Context, filePath string) ([]SymbolResult, error) {
	uri := pathToURI(filePath)

	// Ensure file is open
	if err := c.ensureOpen(ctx, filePath); err != nil {
		return nil, err
	}

	params := DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}

	var result json.RawMessage
	if err := c.conn.Call("textDocument/documentSymbol", params, &result); err != nil {
		return nil, fmt.Errorf("documentSymbol request failed: %w", err)
	}

	return c.parseSymbols(result, filePath)
}

// WorkspaceSymbols returns symbols matching a query in the workspace.
func (c *Client) WorkspaceSymbols(ctx context.Context, query string) ([]SymbolResult, error) {
	params := WorkspaceSymbolParams{Query: query}

	var result []SymbolInformation
	if err := c.conn.Call("workspace/symbol", params, &result); err != nil {
		return nil, fmt.Errorf("workspaceSymbol request failed: %w", err)
	}

	symbols := make([]SymbolResult, 0, len(result))
	for _, sym := range result {
		symbols = append(symbols, SymbolResult{
			Name:     sym.Name,
			Kind:     sym.Kind.String(),
			FilePath: uriToPath(sym.Location.URI),
			Line:     sym.Location.Range.Start.Line + 1,
			Column:   sym.Location.Range.Start.Character + 1,
			EndLine:  sym.Location.Range.End.Line + 1,
			EndCol:   sym.Location.Range.End.Character + 1,
		})
	}

	return symbols, nil
}

// GetDiagnostics returns cached diagnostics for a file.
func (c *Client) GetDiagnostics(ctx context.Context, filePath string) ([]DiagnosticResult, error) {
	// Ensure file is open to trigger diagnostics
	if err := c.ensureOpen(ctx, filePath); err != nil {
		return nil, err
	}

	// Give the server a moment to process diagnostics
	time.Sleep(500 * time.Millisecond)

	uri := pathToURI(filePath)

	c.diagnosticsMu.RLock()
	diags, ok := c.diagnostics[uri]
	c.diagnosticsMu.RUnlock()

	if !ok {
		return []DiagnosticResult{}, nil
	}

	results := make([]DiagnosticResult, 0, len(diags))
	for _, d := range diags {
		codeStr := ""
		if d.Code != nil {
			switch v := d.Code.(type) {
			case string:
				codeStr = v
			case float64:
				codeStr = fmt.Sprintf("%.0f", v)
			}
		}

		results = append(results, DiagnosticResult{
			FilePath: filePath,
			Line:     d.Range.Start.Line + 1,
			Column:   d.Range.Start.Character + 1,
			EndLine:  d.Range.End.Line + 1,
			EndCol:   d.Range.End.Character + 1,
			Severity: d.Severity.String(),
			Message:  d.Message,
			Source:   d.Source,
			Code:     codeStr,
		})
	}

	return results, nil
}

// handleDiagnostics handles incoming diagnostic notifications.
func (c *Client) handleDiagnostics(method string, params json.RawMessage) {
	var diagParams PublishDiagnosticsParams
	if err := json.Unmarshal(params, &diagParams); err != nil {
		return
	}

	c.diagnosticsMu.Lock()
	c.diagnostics[diagParams.URI] = diagParams.Diagnostics
	c.diagnosticsMu.Unlock()
}

// ensureOpen ensures a file is open in the language server.
func (c *Client) ensureOpen(ctx context.Context, filePath string) error {
	uri := pathToURI(filePath)

	c.openDocsMu.RLock()
	_, isOpen := c.openDocs[uri]
	c.openDocsMu.RUnlock()

	if !isOpen {
		return c.OpenFile(ctx, filePath)
	}

	return nil
}

// parseLocations parses location results from a JSON response.
func (c *Client) parseLocations(raw json.RawMessage) ([]LocationResult, error) {
	if raw == nil || string(raw) == "null" {
		return []LocationResult{}, nil
	}

	// Try array of locations
	var locations []Location
	if err := json.Unmarshal(raw, &locations); err == nil {
		results := make([]LocationResult, 0, len(locations))
		for _, loc := range locations {
			results = append(results, LocationResult{
				FilePath:  uriToPath(loc.URI),
				StartLine: loc.Range.Start.Line + 1,
				StartCol:  loc.Range.Start.Character + 1,
				EndLine:   loc.Range.End.Line + 1,
				EndCol:    loc.Range.End.Character + 1,
			})
		}
		return results, nil
	}

	// Try single location
	var loc Location
	if err := json.Unmarshal(raw, &loc); err == nil {
		return []LocationResult{{
			FilePath:  uriToPath(loc.URI),
			StartLine: loc.Range.Start.Line + 1,
			StartCol:  loc.Range.Start.Character + 1,
			EndLine:   loc.Range.End.Line + 1,
			EndCol:    loc.Range.End.Character + 1,
		}}, nil
	}

	return []LocationResult{}, nil
}

// parseSymbols parses symbol results from a JSON response.
func (c *Client) parseSymbols(raw json.RawMessage, filePath string) ([]SymbolResult, error) {
	if raw == nil || string(raw) == "null" {
		return []SymbolResult{}, nil
	}

	// Try DocumentSymbol (hierarchical)
	var docSymbols []DocumentSymbol
	if err := json.Unmarshal(raw, &docSymbols); err == nil && len(docSymbols) > 0 {
		return convertDocumentSymbols(docSymbols, filePath), nil
	}

	// Try SymbolInformation (flat)
	var symInfo []SymbolInformation
	if err := json.Unmarshal(raw, &symInfo); err == nil {
		results := make([]SymbolResult, 0, len(symInfo))
		for _, sym := range symInfo {
			results = append(results, SymbolResult{
				Name:     sym.Name,
				Kind:     sym.Kind.String(),
				FilePath: uriToPath(sym.Location.URI),
				Line:     sym.Location.Range.Start.Line + 1,
				Column:   sym.Location.Range.Start.Character + 1,
				EndLine:  sym.Location.Range.End.Line + 1,
				EndCol:   sym.Location.Range.End.Character + 1,
			})
		}
		return results, nil
	}

	return []SymbolResult{}, nil
}

// convertDocumentSymbols converts DocumentSymbol to SymbolResult.
func convertDocumentSymbols(symbols []DocumentSymbol, filePath string) []SymbolResult {
	results := make([]SymbolResult, 0, len(symbols))
	for _, sym := range symbols {
		result := SymbolResult{
			Name:     sym.Name,
			Kind:     sym.Kind.String(),
			FilePath: filePath,
			Line:     sym.Range.Start.Line + 1,
			Column:   sym.Range.Start.Character + 1,
			EndLine:  sym.Range.End.Line + 1,
			EndCol:   sym.Range.End.Character + 1,
			Detail:   sym.Detail,
		}
		if len(sym.Children) > 0 {
			result.Children = convertDocumentSymbols(sym.Children, filePath)
		}
		results = append(results, result)
	}
	return results
}

// pathToURI converts a file path to a URI.
func pathToURI(path string) string {
	absPath, _ := filepath.Abs(path)
	return "file://" + absPath
}

// uriToPath converts a URI to a file path.
func uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return uri[7:]
	}
	return uri
}
