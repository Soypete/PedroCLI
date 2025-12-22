package fileio

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// CodeBlock represents a block of code from a file
type CodeBlock struct {
	FilePath    string // Absolute path to the file
	StartLine   int    // 1-indexed start line
	EndLine     int    // 1-indexed end line (inclusive)
	Content     string // The actual code content
	Language    string // Language ID (e.g., "go", "python")
	Description string // Optional description of the code block
}

// FileContext represents a file's content for prompt inclusion
type FileContext struct {
	FilePath      string
	RelativePath  string
	Language      string
	LineCount     int
	Content       string
	Blocks        []CodeBlock
	TokenEstimate int
}

// PromptBuffer manages code context for inclusion in LLM prompts
type PromptBuffer struct {
	mu            sync.RWMutex
	fs            *FileSystem
	files         map[string]*FileContext
	blocks        []CodeBlock
	basePath      string
	maxTokens     int
	currentTokens int
}

// NewPromptBuffer creates a new prompt buffer
func NewPromptBuffer(fs *FileSystem, basePath string, maxTokens int) *PromptBuffer {
	return &PromptBuffer{
		fs:        fs,
		files:     make(map[string]*FileContext),
		blocks:    make([]CodeBlock, 0),
		basePath:  basePath,
		maxTokens: maxTokens,
	}
}

// AddFile adds an entire file to the prompt buffer
func (pb *PromptBuffer) AddFile(path string) error {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if already added
	if _, exists := pb.files[absPath]; exists {
		return nil // Already added
	}

	content, err := pb.fs.ReadFileString(absPath)
	if err != nil {
		return err
	}

	// Get relative path for display
	relPath := absPath
	if pb.basePath != "" {
		if rel, err := filepath.Rel(pb.basePath, absPath); err == nil {
			relPath = rel
		}
	}

	// Get language info
	ext := filepath.Ext(absPath)
	lang := pb.fs.extensionReg.GetLanguage(ext)

	// Estimate tokens (rough: 1 token per 4 chars)
	tokenEstimate := len(content) / 4

	fc := &FileContext{
		FilePath:      absPath,
		RelativePath:  relPath,
		Language:      lang,
		LineCount:     countLines([]byte(content)),
		Content:       content,
		TokenEstimate: tokenEstimate,
	}

	pb.files[absPath] = fc
	pb.currentTokens += tokenEstimate

	return nil
}

// AddLines adds specific lines from a file to the buffer as a code block
func (pb *PromptBuffer) AddLines(path string, startLine, endLine int, description string) error {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	content, err := pb.fs.ReadLinesString(absPath, startLine, endLine)
	if err != nil {
		return err
	}

	ext := filepath.Ext(absPath)
	lang := pb.fs.extensionReg.GetLanguage(ext)

	block := CodeBlock{
		FilePath:    absPath,
		StartLine:   startLine,
		EndLine:     endLine,
		Content:     content,
		Language:    lang,
		Description: description,
	}

	pb.blocks = append(pb.blocks, block)
	pb.currentTokens += len(content) / 4

	return nil
}

// AddCodeBlock adds a code block directly
func (pb *PromptBuffer) AddCodeBlock(block CodeBlock) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.blocks = append(pb.blocks, block)
	pb.currentTokens += len(block.Content) / 4
}

// RemoveFile removes a file from the buffer
func (pb *PromptBuffer) RemoveFile(path string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	absPath, _ := filepath.Abs(path)
	if fc, exists := pb.files[absPath]; exists {
		pb.currentTokens -= fc.TokenEstimate
		delete(pb.files, absPath)
	}
}

// Clear removes all content from the buffer
func (pb *PromptBuffer) Clear() {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.files = make(map[string]*FileContext)
	pb.blocks = make([]CodeBlock, 0)
	pb.currentTokens = 0
}

// GetTokenCount returns the current token estimate
func (pb *PromptBuffer) GetTokenCount() int {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	return pb.currentTokens
}

// HasCapacity returns true if adding more tokens would exceed the limit
func (pb *PromptBuffer) HasCapacity(additionalTokens int) bool {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	return pb.currentTokens+additionalTokens <= pb.maxTokens
}

// FormatForPrompt formats all content for inclusion in an LLM prompt
func (pb *PromptBuffer) FormatForPrompt() string {
	pb.mu.RLock()
	defer pb.mu.RUnlock()

	var buf bytes.Buffer

	// Sort files by path for consistent output
	paths := make([]string, 0, len(pb.files))
	for path := range pb.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	// Add full files
	if len(paths) > 0 {
		buf.WriteString("## Files\n\n")
		for _, path := range paths {
			fc := pb.files[path]
			buf.WriteString(fmt.Sprintf("### File: %s\n", fc.RelativePath))
			buf.WriteString(fmt.Sprintf("Language: %s | Lines: %d\n\n", fc.Language, fc.LineCount))
			buf.WriteString("```")
			buf.WriteString(fc.Language)
			buf.WriteString("\n")
			buf.WriteString(fc.Content)
			if !strings.HasSuffix(fc.Content, "\n") {
				buf.WriteString("\n")
			}
			buf.WriteString("```\n\n")
		}
	}

	// Add code blocks
	if len(pb.blocks) > 0 {
		buf.WriteString("## Code Blocks\n\n")
		for i, block := range pb.blocks {
			relPath := block.FilePath
			if pb.basePath != "" {
				if rel, err := filepath.Rel(pb.basePath, block.FilePath); err == nil {
					relPath = rel
				}
			}

			buf.WriteString(fmt.Sprintf("### Block %d: %s (lines %d-%d)\n",
				i+1, relPath, block.StartLine, block.EndLine))
			if block.Description != "" {
				buf.WriteString(fmt.Sprintf("Description: %s\n", block.Description))
			}
			buf.WriteString("\n```")
			buf.WriteString(block.Language)
			buf.WriteString("\n")
			buf.WriteString(block.Content)
			if !strings.HasSuffix(block.Content, "\n") {
				buf.WriteString("\n")
			}
			buf.WriteString("```\n\n")
		}
	}

	return buf.String()
}

// FormatCompact formats content in a more compact format for smaller context
func (pb *PromptBuffer) FormatCompact() string {
	pb.mu.RLock()
	defer pb.mu.RUnlock()

	var buf bytes.Buffer

	// Sort files by path
	paths := make([]string, 0, len(pb.files))
	for path := range pb.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	// Add files with line numbers
	for _, path := range paths {
		fc := pb.files[path]
		buf.WriteString(fmt.Sprintf("=== %s ===\n", fc.RelativePath))
		lines := strings.Split(fc.Content, "\n")
		for i, line := range lines {
			buf.WriteString(fmt.Sprintf("%4d: %s\n", i+1, line))
		}
		buf.WriteString("\n")
	}

	// Add blocks with line numbers
	for _, block := range pb.blocks {
		relPath := block.FilePath
		if pb.basePath != "" {
			if rel, err := filepath.Rel(pb.basePath, block.FilePath); err == nil {
				relPath = rel
			}
		}
		buf.WriteString(fmt.Sprintf("=== %s [%d-%d] ===\n", relPath, block.StartLine, block.EndLine))
		lines := strings.Split(block.Content, "\n")
		for i, line := range lines {
			buf.WriteString(fmt.Sprintf("%4d: %s\n", block.StartLine+i, line))
		}
		buf.WriteString("\n")
	}

	return buf.String()
}

// GetFileList returns a list of files in the buffer
func (pb *PromptBuffer) GetFileList() []string {
	pb.mu.RLock()
	defer pb.mu.RUnlock()

	paths := make([]string, 0, len(pb.files))
	for path := range pb.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// GetBlockCount returns the number of code blocks
func (pb *PromptBuffer) GetBlockCount() int {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	return len(pb.blocks)
}

// TrimToFit removes content until the buffer fits within maxTokens
// Removes oldest blocks first, then files by size (largest first)
func (pb *PromptBuffer) TrimToFit() {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	for pb.currentTokens > pb.maxTokens && len(pb.blocks) > 0 {
		// Remove oldest block
		block := pb.blocks[0]
		pb.blocks = pb.blocks[1:]
		pb.currentTokens -= len(block.Content) / 4
	}

	if pb.currentTokens <= pb.maxTokens {
		return
	}

	// Sort files by size (largest first) for removal
	type fileSize struct {
		path   string
		tokens int
	}
	sizes := make([]fileSize, 0, len(pb.files))
	for path, fc := range pb.files {
		sizes = append(sizes, fileSize{path, fc.TokenEstimate})
	}
	sort.Slice(sizes, func(i, j int) bool {
		return sizes[i].tokens > sizes[j].tokens
	})

	// Remove largest files until we fit
	for _, fs := range sizes {
		if pb.currentTokens <= pb.maxTokens {
			break
		}
		if fc, exists := pb.files[fs.path]; exists {
			pb.currentTokens -= fc.TokenEstimate
			delete(pb.files, fs.path)
		}
	}
}

// Summary returns a summary of the buffer contents
func (pb *PromptBuffer) Summary() string {
	pb.mu.RLock()
	defer pb.mu.RUnlock()

	return fmt.Sprintf("Files: %d, Blocks: %d, Tokens: ~%d/%d",
		len(pb.files), len(pb.blocks), pb.currentTokens, pb.maxTokens)
}

// CodeContextBuilder helps build code context incrementally
type CodeContextBuilder struct {
	fs        *FileSystem
	basePath  string
	maxTokens int
	contexts  []*FileContext
	mu        sync.Mutex
}

// NewCodeContextBuilder creates a new context builder
func NewCodeContextBuilder(fs *FileSystem, basePath string, maxTokens int) *CodeContextBuilder {
	return &CodeContextBuilder{
		fs:        fs,
		basePath:  basePath,
		maxTokens: maxTokens,
		contexts:  make([]*FileContext, 0),
	}
}

// AddFile adds a file to the context
func (ccb *CodeContextBuilder) AddFile(path string) error {
	ccb.mu.Lock()
	defer ccb.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	content, err := ccb.fs.ReadFileString(absPath)
	if err != nil {
		return err
	}

	relPath := absPath
	if ccb.basePath != "" {
		if rel, err := filepath.Rel(ccb.basePath, absPath); err == nil {
			relPath = rel
		}
	}

	ext := filepath.Ext(absPath)
	lang := ccb.fs.extensionReg.GetLanguage(ext)

	fc := &FileContext{
		FilePath:      absPath,
		RelativePath:  relPath,
		Language:      lang,
		LineCount:     countLines([]byte(content)),
		Content:       content,
		TokenEstimate: len(content) / 4,
	}

	ccb.contexts = append(ccb.contexts, fc)
	return nil
}

// Build creates a PromptBuffer from the accumulated context
func (ccb *CodeContextBuilder) Build() *PromptBuffer {
	ccb.mu.Lock()
	defer ccb.mu.Unlock()

	pb := NewPromptBuffer(ccb.fs, ccb.basePath, ccb.maxTokens)
	for _, fc := range ccb.contexts {
		pb.files[fc.FilePath] = fc
		pb.currentTokens += fc.TokenEstimate
	}
	return pb
}

// GetCurrentTokens returns current token count
func (ccb *CodeContextBuilder) GetCurrentTokens() int {
	ccb.mu.Lock()
	defer ccb.mu.Unlock()

	total := 0
	for _, fc := range ccb.contexts {
		total += fc.TokenEstimate
	}
	return total
}
