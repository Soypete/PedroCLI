package fileio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewFileSystem(t *testing.T) {
	fs := NewFileSystem()
	if fs == nil {
		t.Fatal("NewFileSystem returned nil")
	}
	if fs.maxFileSize != DefaultMaxFileSize {
		t.Errorf("expected maxFileSize %d, got %d", DefaultMaxFileSize, fs.maxFileSize)
	}
}

func TestNewFileSystemWithOptions(t *testing.T) {
	fs := NewFileSystemWithOptions(
		WithMaxFileSize(1024*1024),
		WithBackup("/tmp/backup"),
	)
	if fs.maxFileSize != 1024*1024 {
		t.Errorf("expected maxFileSize 1048576, got %d", fs.maxFileSize)
	}
	if !fs.enableBackup {
		t.Error("expected backup to be enabled")
	}
	if fs.backupDir != "/tmp/backup" {
		t.Errorf("expected backupDir /tmp/backup, got %s", fs.backupDir)
	}
}

func TestReadWriteFile(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")
	content := "Hello, World!\nThis is a test."

	// Write file
	err := fs.WriteFileString(testFile, content)
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	// Read file
	readContent, err := fs.ReadFileString(testFile)
	if err != nil {
		t.Fatalf("ReadFileString failed: %v", err)
	}

	if readContent != content {
		t.Errorf("content mismatch:\nexpected: %q\ngot: %q", content, readContent)
	}
}

func TestReadLines(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")
	lines := []string{"Line 1", "Line 2", "Line 3", "Line 4", "Line 5"}
	content := strings.Join(lines, "\n")

	err := fs.WriteFileString(testFile, content)
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	// Read lines 2-4
	readLines, err := fs.ReadLines(testFile, 2, 4)
	if err != nil {
		t.Fatalf("ReadLines failed: %v", err)
	}

	expected := []string{"Line 2", "Line 3", "Line 4"}
	if len(readLines) != len(expected) {
		t.Fatalf("expected %d lines, got %d", len(expected), len(readLines))
	}
	for i, line := range readLines {
		if line != expected[i] {
			t.Errorf("line %d: expected %q, got %q", i+2, expected[i], line)
		}
	}
}

func TestReadLinesInvalidRange(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")
	err := fs.WriteFileString(testFile, "line 1\nline 2\nline 3")
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	// Invalid start line
	_, err = fs.ReadLines(testFile, 0, 3)
	if err == nil {
		t.Error("expected error for start_line < 1")
	}

	// End before start
	_, err = fs.ReadLines(testFile, 3, 1)
	if err == nil {
		t.Error("expected error for end_line < start_line")
	}
}

func TestEditLines(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	original := "package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n"
	err := fs.WriteFileString(testFile, original)
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	// Edit line 4
	err = fs.EditLines(testFile, 4, 4, "\tfmt.Println(\"World\")")
	if err != nil {
		t.Fatalf("EditLines failed: %v", err)
	}

	content, err := fs.ReadFileString(testFile)
	if err != nil {
		t.Fatalf("ReadFileString failed: %v", err)
	}

	if !strings.Contains(content, "World") {
		t.Error("edited content should contain 'World'")
	}
	if strings.Contains(content, "Hello") {
		t.Error("edited content should not contain 'Hello'")
	}
}

func TestInsertAtLine(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")
	err := fs.WriteFileString(testFile, "Line 1\nLine 2\nLine 3")
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	// Insert at line 2
	err = fs.InsertAtLine(testFile, 2, "Inserted Line")
	if err != nil {
		t.Fatalf("InsertAtLine failed: %v", err)
	}

	content, err := fs.ReadFileString(testFile)
	if err != nil {
		t.Fatalf("ReadFileString failed: %v", err)
	}

	lines := strings.Split(content, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
	if lines[1] != "Inserted Line" {
		t.Errorf("expected 'Inserted Line' at line 2, got %q", lines[1])
	}
}

func TestDeleteLines(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")
	err := fs.WriteFileString(testFile, "Line 1\nLine 2\nLine 3\nLine 4\nLine 5")
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	// Delete lines 2-4
	err = fs.DeleteLines(testFile, 2, 4)
	if err != nil {
		t.Fatalf("DeleteLines failed: %v", err)
	}

	content, err := fs.ReadFileString(testFile)
	if err != nil {
		t.Fatalf("ReadFileString failed: %v", err)
	}

	lines := strings.Split(content, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "Line 1" || lines[1] != "Line 5" {
		t.Errorf("unexpected content: %v", lines)
	}
}

func TestReplaceInFile(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")
	err := fs.WriteFileString(testFile, "foo bar foo baz foo")
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	count, err := fs.ReplaceInFile(testFile, "foo", "qux")
	if err != nil {
		t.Fatalf("ReplaceInFile failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 replacements, got %d", count)
	}

	content, err := fs.ReadFileString(testFile)
	if err != nil {
		t.Fatalf("ReadFileString failed: %v", err)
	}
	if content != "qux bar qux baz qux" {
		t.Errorf("unexpected content: %q", content)
	}
}

func TestAppendFile(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")
	err := fs.WriteFileString(testFile, "Initial content")
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	err = fs.AppendFileString(testFile, "\nAppended content")
	if err != nil {
		t.Fatalf("AppendFileString failed: %v", err)
	}

	content, err := fs.ReadFileString(testFile)
	if err != nil {
		t.Fatalf("ReadFileString failed: %v", err)
	}

	expected := "Initial content\nAppended content"
	if content != expected {
		t.Errorf("expected %q, got %q", expected, content)
	}
}

func TestDeleteFile(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")
	err := fs.WriteFileString(testFile, "content")
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	if !fs.FileExists(testFile) {
		t.Fatal("file should exist after write")
	}

	err = fs.DeleteFile(testFile)
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	if fs.FileExists(testFile) {
		t.Fatal("file should not exist after delete")
	}
}

func TestCopyFile(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")

	content := "Copy this content"
	err := fs.WriteFileString(srcFile, content)
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	err = fs.CopyFile(srcFile, dstFile)
	if err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	dstContent, err := fs.ReadFileString(dstFile)
	if err != nil {
		t.Fatalf("ReadFileString failed: %v", err)
	}

	if dstContent != content {
		t.Errorf("expected %q, got %q", content, dstContent)
	}
}

func TestMoveFile(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")

	content := "Move this content"
	err := fs.WriteFileString(srcFile, content)
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	err = fs.MoveFile(srcFile, dstFile)
	if err != nil {
		t.Fatalf("MoveFile failed: %v", err)
	}

	if fs.FileExists(srcFile) {
		t.Error("source file should not exist after move")
	}

	if !fs.FileExists(dstFile) {
		t.Error("destination file should exist after move")
	}

	dstContent, err := fs.ReadFileString(dstFile)
	if err != nil {
		t.Fatalf("ReadFileString failed: %v", err)
	}

	if dstContent != content {
		t.Errorf("expected %q, got %q", content, dstContent)
	}
}

func TestGetFileInfo(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	content := "package main\n\nfunc main() {\n}\n"
	err := fs.WriteFileString(testFile, content)
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	info, err := fs.GetFileInfo(testFile)
	if err != nil {
		t.Fatalf("GetFileInfo failed: %v", err)
	}

	if info.Extension != ".go" {
		t.Errorf("expected extension .go, got %s", info.Extension)
	}
	if info.Language != "go" {
		t.Errorf("expected language go, got %s", info.Language)
	}
	if info.LineCount != 4 {
		t.Errorf("expected 4 lines, got %d", info.LineCount)
	}
	if info.Size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), info.Size)
	}
}

func TestGetModifiedFiles(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	fs.WriteFileString(file1, "content1")
	fs.WriteFileString(file2, "content2")

	modified := fs.GetModifiedFiles()
	if len(modified) != 2 {
		t.Errorf("expected 2 modified files, got %d", len(modified))
	}

	fs.ClearModifiedFiles()
	modified = fs.GetModifiedFiles()
	if len(modified) != 0 {
		t.Errorf("expected 0 modified files after clear, got %d", len(modified))
	}
}

func TestLineEndingDetection(t *testing.T) {
	tests := []struct {
		content  string
		expected LineEnding
	}{
		{"line1\nline2\nline3", LineEndingUnix},
		{"line1\r\nline2\r\nline3", LineEndingWindows},
		{"line1\rline2\rline3", LineEndingClassicMac},
		{"line1\nline2\r\nline3", LineEndingMixed},
		{"single line", LineEndingUnix},
	}

	for _, tt := range tests {
		result := detectLineEnding([]byte(tt.content))
		if result != tt.expected {
			t.Errorf("detectLineEnding(%q) = %v, want %v", tt.content, result, tt.expected)
		}
	}
}

func TestEncodingDetection(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected string
	}{
		{"UTF-8 BOM", []byte{0xEF, 0xBB, 0xBF, 'h', 'e', 'l', 'l', 'o'}, "utf-8-bom"},
		{"UTF-16 BE BOM", []byte{0xFE, 0xFF, 0, 'h'}, "utf-16-be"},
		{"UTF-16 LE BOM", []byte{0xFF, 0xFE, 'h', 0}, "utf-16-le"},
		{"Plain UTF-8", []byte("hello world"), "utf-8"},
		{"UTF-8 with emoji", []byte("hello 🌍"), "utf-8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectEncoding(tt.content)
			if result != tt.expected {
				t.Errorf("detectEncoding(%v) = %q, want %q", tt.content, result, tt.expected)
			}
		})
	}
}

func TestAtomicWrite(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "atomic.txt")

	// Write initial content
	err := fs.WriteFileString(testFile, "initial")
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	// Overwrite with new content
	err = fs.WriteFileString(testFile, "updated")
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	content, err := fs.ReadFileString(testFile)
	if err != nil {
		t.Fatalf("ReadFileString failed: %v", err)
	}

	if content != "updated" {
		t.Errorf("expected 'updated', got %q", content)
	}

	// Check that no temp file remains
	entries, _ := os.ReadDir(tmpDir)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("temp file should not remain: %s", entry.Name())
		}
	}
}

func TestBackupCreation(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")

	fs := NewFileSystemWithOptions(WithBackup(backupDir))

	testFile := filepath.Join(tmpDir, "backup_test.txt")

	// Write initial content
	err := fs.WriteFileString(testFile, "original content")
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	// Overwrite (should create backup)
	err = fs.WriteFileString(testFile, "new content")
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	// Check backup was created
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 backup file, got %d", len(entries))
	}
}

func TestCreateDirectories(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	// Write to nested directory that doesn't exist
	testFile := filepath.Join(tmpDir, "a", "b", "c", "test.txt")
	err := fs.WriteFileString(testFile, "content")
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	if !fs.FileExists(testFile) {
		t.Error("file should exist after write to nested directory")
	}
}

func TestIsDirectory(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "file.txt")
	fs.WriteFileString(testFile, "content")

	if !fs.IsDirectory(tmpDir) {
		t.Error("tmpDir should be a directory")
	}

	if fs.IsDirectory(testFile) {
		t.Error("testFile should not be a directory")
	}
}

func TestFileTooLarge(t *testing.T) {
	fs := NewFileSystemWithOptions(WithMaxFileSize(100))
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "large.txt")

	// Write file larger than max
	largeContent := strings.Repeat("x", 200)
	err := os.WriteFile(testFile, []byte(largeContent), 0644)
	if err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}

	// Try to read - should fail
	_, err = fs.ReadFile(testFile)
	if err == nil {
		t.Error("expected error reading file larger than max size")
	}
}

func TestFileNotFound(t *testing.T) {
	fs := NewFileSystem()

	_, err := fs.ReadFile("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		content  string
		expected int
	}{
		{"", 0},
		{"single line", 1},
		{"line1\nline2", 2},
		{"line1\nline2\n", 2},
		{"line1\nline2\nline3", 3},
	}

	for _, tt := range tests {
		result := countLines([]byte(tt.content))
		if result != tt.expected {
			t.Errorf("countLines(%q) = %d, want %d", tt.content, result, tt.expected)
		}
	}
}

func TestPreserveLineEndings(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	// Test Windows line endings
	testFile := filepath.Join(tmpDir, "windows.txt")
	content := "line1\r\nline2\r\nline3"
	err := fs.WriteFileString(testFile, content)
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	// Edit a line
	err = fs.EditLines(testFile, 2, 2, "edited")
	if err != nil {
		t.Fatalf("EditLines failed: %v", err)
	}

	// Read back
	result, err := fs.ReadFileString(testFile)
	if err != nil {
		t.Fatalf("ReadFileString failed: %v", err)
	}

	// Should preserve Windows line endings
	if !strings.Contains(result, "\r\n") {
		t.Error("Windows line endings should be preserved")
	}
}
