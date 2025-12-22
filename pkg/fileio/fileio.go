// Package fileio provides comprehensive file I/O operations for PedroCLI.
// It offers robust file reading and writing using Go's standard library,
// with support for encoding detection, atomic writes, backup creation,
// file extension tracking, and LSP integration.
package fileio

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DefaultMaxFileSize is the default maximum file size (10MB)
const DefaultMaxFileSize = 10 * 1024 * 1024

// FileInfo contains metadata about a file
type FileInfo struct {
	Path        string
	Size        int64
	ModTime     time.Time
	IsDir       bool
	Extension   string
	Language    string
	LineCount   int
	Encoding    string
	LineEnding  LineEnding
	Permissions os.FileMode
	IsReadOnly  bool
}

// LineEnding represents the type of line ending used in a file
type LineEnding int

const (
	LineEndingUnix       LineEnding = iota // LF (\n)
	LineEndingWindows                      // CRLF (\r\n)
	LineEndingClassicMac                   // CR (\r)
	LineEndingMixed                        // Mixed line endings
)

// String returns the string representation of a line ending
func (le LineEnding) String() string {
	switch le {
	case LineEndingUnix:
		return "unix"
	case LineEndingWindows:
		return "windows"
	case LineEndingClassicMac:
		return "classic-mac"
	case LineEndingMixed:
		return "mixed"
	default:
		return "unknown"
	}
}

// Bytes returns the byte sequence for a line ending
func (le LineEnding) Bytes() []byte {
	switch le {
	case LineEndingWindows:
		return []byte("\r\n")
	case LineEndingClassicMac:
		return []byte("\r")
	default:
		return []byte("\n")
	}
}

// FileSystem provides the main interface for file operations
type FileSystem struct {
	maxFileSize   int64
	enableBackup  bool
	backupDir     string
	extensionReg  *ExtensionRegistry
	mu            sync.RWMutex
	modifiedFiles map[string]time.Time
}

// NewFileSystem creates a new FileSystem with default settings
func NewFileSystem() *FileSystem {
	return &FileSystem{
		maxFileSize:   DefaultMaxFileSize,
		enableBackup:  false,
		extensionReg:  NewExtensionRegistry(),
		modifiedFiles: make(map[string]time.Time),
	}
}

// Option configures a FileSystem
type Option func(*FileSystem)

// WithMaxFileSize sets the maximum file size
func WithMaxFileSize(size int64) Option {
	return func(fs *FileSystem) {
		fs.maxFileSize = size
	}
}

// WithBackup enables backup creation with a specified directory
func WithBackup(dir string) Option {
	return func(fs *FileSystem) {
		fs.enableBackup = true
		fs.backupDir = dir
	}
}

// NewFileSystemWithOptions creates a FileSystem with custom options
func NewFileSystemWithOptions(opts ...Option) *FileSystem {
	fs := NewFileSystem()
	for _, opt := range opts {
		opt(fs)
	}
	return fs
}

// GetFileInfo returns metadata about a file
func (fs *FileSystem) GetFileInfo(path string) (*FileInfo, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	ext := filepath.Ext(absPath)
	lang := fs.extensionReg.GetLanguage(ext)

	info := &FileInfo{
		Path:        absPath,
		Size:        stat.Size(),
		ModTime:     stat.ModTime(),
		IsDir:       stat.IsDir(),
		Extension:   ext,
		Language:    lang,
		Permissions: stat.Mode().Perm(),
		IsReadOnly:  stat.Mode().Perm()&0200 == 0, // Check write permission
	}

	if !stat.IsDir() && stat.Size() <= fs.maxFileSize {
		// Read file to get additional info
		content, err := os.ReadFile(absPath)
		if err == nil {
			info.LineCount = countLines(content)
			info.Encoding = detectEncoding(content)
			info.LineEnding = detectLineEnding(content)
		}
	}

	return info, nil
}

// ReadFile reads the entire contents of a file
func (fs *FileSystem) ReadFile(path string) ([]byte, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", absPath)
	}

	if stat.Size() > fs.maxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", stat.Size(), fs.maxFileSize)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return content, nil
}

// ReadFileString reads a file and returns its contents as a string
func (fs *FileSystem) ReadFileString(path string) (string, error) {
	content, err := fs.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// ReadLines reads specific lines from a file (1-indexed, inclusive)
func (fs *FileSystem) ReadLines(path string, startLine, endLine int) ([]string, error) {
	if startLine < 1 {
		return nil, fmt.Errorf("start_line must be >= 1")
	}
	if endLine < startLine {
		return nil, fmt.Errorf("end_line must be >= start_line")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	file, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var lines []string
	lineNum := 1

	for scanner.Scan() {
		if lineNum >= startLine && lineNum <= endLine {
			lines = append(lines, scanner.Text())
		}
		if lineNum > endLine {
			break
		}
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return lines, nil
}

// ReadLinesString reads specific lines and returns them as a single string
func (fs *FileSystem) ReadLinesString(path string, startLine, endLine int) (string, error) {
	lines, err := fs.ReadLines(path, startLine, endLine)
	if err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

// WriteFile writes content to a file, creating directories as needed
func (fs *FileSystem) WriteFile(path string, content []byte) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Create backup if enabled and file exists
	if fs.enableBackup {
		if _, err := os.Stat(absPath); err == nil {
			if err := fs.createBackup(absPath); err != nil {
				return fmt.Errorf("failed to create backup: %w", err)
			}
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to temporary file first (atomic write)
	tmpFile := absPath + ".tmp"
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Rename temp file to target (atomic on most systems)
	if err := os.Rename(tmpFile, absPath); err != nil {
		_ = os.Remove(tmpFile) // Clean up temp file
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Track modification
	fs.mu.Lock()
	fs.modifiedFiles[absPath] = time.Now()
	fs.mu.Unlock()

	return nil
}

// WriteFileString writes a string to a file
func (fs *FileSystem) WriteFileString(path string, content string) error {
	return fs.WriteFile(path, []byte(content))
}

// AppendFile appends content to a file
func (fs *FileSystem) AppendFile(path string, content []byte) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	f, err := os.OpenFile(absPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(content); err != nil {
		return fmt.Errorf("failed to append to file: %w", err)
	}

	// Track modification
	fs.mu.Lock()
	fs.modifiedFiles[absPath] = time.Now()
	fs.mu.Unlock()

	return nil
}

// AppendFileString appends a string to a file
func (fs *FileSystem) AppendFileString(path string, content string) error {
	return fs.AppendFile(path, []byte(content))
}

// DeleteFile removes a file
func (fs *FileSystem) DeleteFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Create backup if enabled
	if fs.enableBackup {
		if err := fs.createBackup(absPath); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}

	if err := os.Remove(absPath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Track deletion
	fs.mu.Lock()
	delete(fs.modifiedFiles, absPath)
	fs.mu.Unlock()

	return nil
}

// ReplaceInFile replaces all occurrences of old with new in a file
func (fs *FileSystem) ReplaceInFile(path, old, new string) (int, error) {
	content, err := fs.ReadFileString(path)
	if err != nil {
		return 0, err
	}

	count := strings.Count(content, old)
	if count == 0 {
		return 0, nil
	}

	newContent := strings.ReplaceAll(content, old, new)
	if err := fs.WriteFileString(path, newContent); err != nil {
		return 0, err
	}

	return count, nil
}

// EditLines replaces a range of lines with new content
func (fs *FileSystem) EditLines(path string, startLine, endLine int, newContent string) error {
	if startLine < 1 {
		return fmt.Errorf("start_line must be >= 1")
	}
	if endLine < startLine {
		return fmt.Errorf("end_line must be >= start_line")
	}

	content, err := fs.ReadFile(path)
	if err != nil {
		return err
	}

	lines := splitLines(content)

	if startLine > len(lines) {
		return fmt.Errorf("start_line %d exceeds file length %d", startLine, len(lines))
	}

	if endLine > len(lines) {
		endLine = len(lines)
	}

	// Detect original line ending
	lineEnding := detectLineEnding(content)

	// Build new file content
	var newLines []string
	newLines = append(newLines, lines[:startLine-1]...)
	newLines = append(newLines, splitLinesString(newContent)...)
	newLines = append(newLines, lines[endLine:]...)

	// Join with original line ending
	newFileContent := strings.Join(newLines, string(lineEnding.Bytes()))

	return fs.WriteFileString(path, newFileContent)
}

// InsertAtLine inserts content at a specific line number
func (fs *FileSystem) InsertAtLine(path string, lineNumber int, content string) error {
	if lineNumber < 1 {
		return fmt.Errorf("line_number must be >= 1")
	}

	fileContent, err := fs.ReadFile(path)
	if err != nil {
		return err
	}

	lines := splitLines(fileContent)
	lineEnding := detectLineEnding(fileContent)

	var newLines []string
	if lineNumber > len(lines) {
		// Append at end
		newLines = append(newLines, lines...)
		newLines = append(newLines, splitLinesString(content)...)
	} else {
		// Insert at position
		newLines = append(newLines, lines[:lineNumber-1]...)
		newLines = append(newLines, splitLinesString(content)...)
		newLines = append(newLines, lines[lineNumber-1:]...)
	}

	newFileContent := strings.Join(newLines, string(lineEnding.Bytes()))
	return fs.WriteFileString(path, newFileContent)
}

// DeleteLines removes a range of lines from a file
func (fs *FileSystem) DeleteLines(path string, startLine, endLine int) error {
	if startLine < 1 {
		return fmt.Errorf("start_line must be >= 1")
	}
	if endLine < startLine {
		return fmt.Errorf("end_line must be >= start_line")
	}

	content, err := fs.ReadFile(path)
	if err != nil {
		return err
	}

	lines := splitLines(content)

	if startLine > len(lines) {
		return fmt.Errorf("start_line %d exceeds file length %d", startLine, len(lines))
	}

	if endLine > len(lines) {
		endLine = len(lines)
	}

	lineEnding := detectLineEnding(content)

	var newLines []string
	newLines = append(newLines, lines[:startLine-1]...)
	newLines = append(newLines, lines[endLine:]...)

	newFileContent := strings.Join(newLines, string(lineEnding.Bytes()))
	return fs.WriteFileString(path, newFileContent)
}

// FileExists checks if a file exists
func (fs *FileSystem) FileExists(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	_, err = os.Stat(absPath)
	return err == nil
}

// IsDirectory checks if a path is a directory
func (fs *FileSystem) IsDirectory(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	stat, err := os.Stat(absPath)
	if err != nil {
		return false
	}
	return stat.IsDir()
}

// GetModifiedFiles returns a list of files modified through this FileSystem
func (fs *FileSystem) GetModifiedFiles() []string {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	files := make([]string, 0, len(fs.modifiedFiles))
	for path := range fs.modifiedFiles {
		files = append(files, path)
	}
	return files
}

// ClearModifiedFiles clears the list of modified files
func (fs *FileSystem) ClearModifiedFiles() {
	fs.mu.Lock()
	fs.modifiedFiles = make(map[string]time.Time)
	fs.mu.Unlock()
}

// createBackup creates a backup of a file
func (fs *FileSystem) createBackup(path string) error {
	if fs.backupDir == "" {
		fs.backupDir = filepath.Join(filepath.Dir(path), ".backup")
	}

	if err := os.MkdirAll(fs.backupDir, 0755); err != nil {
		return err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	basename := filepath.Base(path)
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(fs.backupDir, fmt.Sprintf("%s.%s.bak", basename, timestamp))

	return os.WriteFile(backupPath, content, 0644)
}

// Helper functions

// countLines counts the number of lines in content
func countLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	count := bytes.Count(content, []byte("\n"))
	// Add 1 if content doesn't end with newline
	if content[len(content)-1] != '\n' {
		count++
	}
	return count
}

// detectEncoding attempts to detect the file encoding
func detectEncoding(content []byte) string {
	// Check for BOM
	if len(content) >= 3 && content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF {
		return "utf-8-bom"
	}
	if len(content) >= 2 && content[0] == 0xFE && content[1] == 0xFF {
		return "utf-16-be"
	}
	if len(content) >= 2 && content[0] == 0xFF && content[1] == 0xFE {
		return "utf-16-le"
	}

	// Check if valid UTF-8
	if isValidUTF8(content) {
		return "utf-8"
	}

	return "binary"
}

// isValidUTF8 checks if content is valid UTF-8
func isValidUTF8(content []byte) bool {
	for i := 0; i < len(content); {
		if content[i] < 0x80 {
			i++
			continue
		}

		// Check for valid multi-byte sequences
		if content[i]&0xE0 == 0xC0 && i+1 < len(content) {
			if content[i+1]&0xC0 == 0x80 {
				i += 2
				continue
			}
		}
		if content[i]&0xF0 == 0xE0 && i+2 < len(content) {
			if content[i+1]&0xC0 == 0x80 && content[i+2]&0xC0 == 0x80 {
				i += 3
				continue
			}
		}
		if content[i]&0xF8 == 0xF0 && i+3 < len(content) {
			if content[i+1]&0xC0 == 0x80 && content[i+2]&0xC0 == 0x80 && content[i+3]&0xC0 == 0x80 {
				i += 4
				continue
			}
		}

		// Invalid UTF-8 sequence
		return false
	}
	return true
}

// detectLineEnding detects the line ending style used in content
func detectLineEnding(content []byte) LineEnding {
	crlfCount := bytes.Count(content, []byte("\r\n"))
	lfCount := bytes.Count(content, []byte("\n")) - crlfCount
	crCount := bytes.Count(content, []byte("\r")) - crlfCount

	// Determine dominant line ending
	if crlfCount > 0 && lfCount == 0 && crCount == 0 {
		return LineEndingWindows
	}
	if lfCount > 0 && crlfCount == 0 && crCount == 0 {
		return LineEndingUnix
	}
	if crCount > 0 && crlfCount == 0 && lfCount == 0 {
		return LineEndingClassicMac
	}
	if crlfCount > 0 || lfCount > 0 || crCount > 0 {
		return LineEndingMixed
	}

	// Default to Unix
	return LineEndingUnix
}

// splitLines splits content into lines, handling different line endings
func splitLines(content []byte) []string {
	// Normalize line endings to \n for splitting
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	normalized = bytes.ReplaceAll(normalized, []byte("\r"), []byte("\n"))
	return strings.Split(string(normalized), "\n")
}

// splitLinesString splits a string into lines
func splitLinesString(content string) []string {
	return splitLines([]byte(content))
}

// CopyFile copies a file from src to dst
func (fs *FileSystem) CopyFile(src, dst string) error {
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return fmt.Errorf("failed to get absolute source path: %w", err)
	}

	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return fmt.Errorf("failed to get absolute destination path: %w", err)
	}

	srcFile, err := os.Open(srcAbs)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() { _ = srcFile.Close() }()

	// Get source file info for permissions
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	if srcInfo.Size() > fs.maxFileSize {
		return fmt.Errorf("source file too large: %d bytes (max %d)", srcInfo.Size(), fs.maxFileSize)
	}

	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	dstFile, err := os.Create(dstAbs)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Preserve permissions
	if err := os.Chmod(dstAbs, srcInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Track modification
	fs.mu.Lock()
	fs.modifiedFiles[dstAbs] = time.Now()
	fs.mu.Unlock()

	return nil
}

// MoveFile moves a file from src to dst
func (fs *FileSystem) MoveFile(src, dst string) error {
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return fmt.Errorf("failed to get absolute source path: %w", err)
	}

	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return fmt.Errorf("failed to get absolute destination path: %w", err)
	}

	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Try direct rename first (works when on same filesystem)
	if err := os.Rename(srcAbs, dstAbs); err == nil {
		fs.mu.Lock()
		delete(fs.modifiedFiles, srcAbs)
		fs.modifiedFiles[dstAbs] = time.Now()
		fs.mu.Unlock()
		return nil
	}

	// Fall back to copy + delete
	if err := fs.CopyFile(srcAbs, dstAbs); err != nil {
		return err
	}

	if err := os.Remove(srcAbs); err != nil {
		return fmt.Errorf("failed to remove source after copy: %w", err)
	}

	fs.mu.Lock()
	delete(fs.modifiedFiles, srcAbs)
	fs.mu.Unlock()

	return nil
}
