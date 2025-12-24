package fileio

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNewPromptBuffer(t *testing.T) {
	fs := NewFileSystem()
	pb := NewPromptBuffer(fs, "/project", 10000)

	if pb == nil {
		t.Fatal("NewPromptBuffer returned nil")
	}

	if pb.maxTokens != 10000 {
		t.Errorf("expected maxTokens 10000, got %d", pb.maxTokens)
	}

	if pb.basePath != "/project" {
		t.Errorf("expected basePath /project, got %s", pb.basePath)
	}
}

func TestAddFile(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	content := "package main\n\nfunc main() {\n}\n"
	err := fs.WriteFileString(testFile, content)
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	pb := NewPromptBuffer(fs, tmpDir, 10000)

	err = pb.AddFile(testFile)
	if err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	files := pb.GetFileList()
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}

	// Adding same file again should be a no-op
	err = pb.AddFile(testFile)
	if err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	files = pb.GetFileList()
	if len(files) != 1 {
		t.Errorf("expected 1 file after duplicate add, got %d", len(files))
	}
}

func TestAddLines(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	content := "package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n"
	err := fs.WriteFileString(testFile, content)
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	pb := NewPromptBuffer(fs, tmpDir, 10000)

	err = pb.AddLines(testFile, 3, 5, "main function")
	if err != nil {
		t.Fatalf("AddLines failed: %v", err)
	}

	if pb.GetBlockCount() != 1 {
		t.Errorf("expected 1 block, got %d", pb.GetBlockCount())
	}
}

func TestAddCodeBlock(t *testing.T) {
	fs := NewFileSystem()
	pb := NewPromptBuffer(fs, "/project", 10000)

	block := CodeBlock{
		FilePath:    "/project/main.go",
		StartLine:   10,
		EndLine:     20,
		Content:     "func test() {\n\treturn\n}",
		Language:    "go",
		Description: "Test function",
	}

	pb.AddCodeBlock(block)

	if pb.GetBlockCount() != 1 {
		t.Errorf("expected 1 block, got %d", pb.GetBlockCount())
	}
}

func TestRemoveFile(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	err := fs.WriteFileString(testFile, "package main")
	if err != nil {
		t.Fatalf("WriteFileString failed: %v", err)
	}

	pb := NewPromptBuffer(fs, tmpDir, 10000)
	pb.AddFile(testFile)

	if len(pb.GetFileList()) != 1 {
		t.Error("expected 1 file")
	}

	pb.RemoveFile(testFile)

	if len(pb.GetFileList()) != 0 {
		t.Error("expected 0 files after removal")
	}
}

func TestClear(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	fs.WriteFileString(testFile, "package main")

	pb := NewPromptBuffer(fs, tmpDir, 10000)
	pb.AddFile(testFile)
	pb.AddCodeBlock(CodeBlock{Content: "test"})

	pb.Clear()

	if len(pb.GetFileList()) != 0 {
		t.Error("expected 0 files after clear")
	}
	if pb.GetBlockCount() != 0 {
		t.Error("expected 0 blocks after clear")
	}
	if pb.GetTokenCount() != 0 {
		t.Error("expected 0 tokens after clear")
	}
}

func TestGetTokenCount(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	content := strings.Repeat("word ", 100) // ~500 chars = ~125 tokens
	fs.WriteFileString(testFile, content)

	pb := NewPromptBuffer(fs, tmpDir, 10000)
	pb.AddFile(testFile)

	tokens := pb.GetTokenCount()
	// Token estimate is len/4
	expected := len(content) / 4
	if tokens != expected {
		t.Errorf("expected ~%d tokens, got %d", expected, tokens)
	}
}

func TestHasCapacity(t *testing.T) {
	fs := NewFileSystem()
	pb := NewPromptBuffer(fs, "/project", 100)

	if !pb.HasCapacity(50) {
		t.Error("should have capacity for 50 tokens")
	}

	if !pb.HasCapacity(100) {
		t.Error("should have capacity for exactly 100 tokens")
	}

	if pb.HasCapacity(101) {
		t.Error("should not have capacity for 101 tokens")
	}
}

func TestFormatForPrompt(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "main.go")
	content := "package main\n\nfunc main() {\n}\n"
	fs.WriteFileString(testFile, content)

	pb := NewPromptBuffer(fs, tmpDir, 10000)
	pb.AddFile(testFile)
	pb.AddCodeBlock(CodeBlock{
		FilePath:    testFile,
		StartLine:   3,
		EndLine:     4,
		Content:     "func main() {\n}",
		Language:    "go",
		Description: "Main function",
	})

	output := pb.FormatForPrompt()

	// Check file section
	if !strings.Contains(output, "## Files") {
		t.Error("output should contain '## Files' header")
	}

	// Check file path
	if !strings.Contains(output, "main.go") {
		t.Error("output should contain file path")
	}

	// Check language
	if !strings.Contains(output, "Language: go") {
		t.Error("output should contain language info")
	}

	// Check code blocks section
	if !strings.Contains(output, "## Code Blocks") {
		t.Error("output should contain '## Code Blocks' header")
	}

	// Check code fence
	if !strings.Contains(output, "```go") {
		t.Error("output should contain code fence with language")
	}
}

func TestFormatCompact(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	content := "line1\nline2\nline3"
	fs.WriteFileString(testFile, content)

	pb := NewPromptBuffer(fs, tmpDir, 10000)
	pb.AddFile(testFile)

	output := pb.FormatCompact()

	// Should have line numbers
	if !strings.Contains(output, "   1:") {
		t.Error("output should contain line numbers")
	}

	// Should have file header
	if !strings.Contains(output, "===") {
		t.Error("output should contain file header markers")
	}
}

func TestTrimToFit(t *testing.T) {
	fs := NewFileSystem()
	pb := NewPromptBuffer(fs, "/project", 100)

	// Add blocks that exceed capacity
	for i := 0; i < 10; i++ {
		pb.AddCodeBlock(CodeBlock{
			Content: strings.Repeat("x", 50), // 12-13 tokens each
		})
	}

	if pb.GetTokenCount() <= 100 {
		t.Skip("test setup: need more tokens")
	}

	pb.TrimToFit()

	if pb.GetTokenCount() > 100 {
		t.Errorf("after TrimToFit, tokens should be <= 100, got %d", pb.GetTokenCount())
	}
}

func TestSummary(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	fs.WriteFileString(testFile, "package main")

	pb := NewPromptBuffer(fs, tmpDir, 10000)
	pb.AddFile(testFile)
	pb.AddCodeBlock(CodeBlock{Content: "test"})

	summary := pb.Summary()

	if !strings.Contains(summary, "Files: 1") {
		t.Error("summary should contain file count")
	}
	if !strings.Contains(summary, "Blocks: 1") {
		t.Error("summary should contain block count")
	}
	if !strings.Contains(summary, "Tokens:") {
		t.Error("summary should contain token count")
	}
}

func TestCodeContextBuilder(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "file1.go")
	file2 := filepath.Join(tmpDir, "file2.go")
	fs.WriteFileString(file1, "package main")
	fs.WriteFileString(file2, "package util")

	builder := NewCodeContextBuilder(fs, tmpDir, 10000)

	err := builder.AddFile(file1)
	if err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	err = builder.AddFile(file2)
	if err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	if builder.GetCurrentTokens() == 0 {
		t.Error("should have some tokens")
	}

	pb := builder.Build()
	if len(pb.GetFileList()) != 2 {
		t.Errorf("expected 2 files, got %d", len(pb.GetFileList()))
	}
}

func TestRelativePaths(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	subDir := filepath.Join(tmpDir, "src", "pkg")
	testFile := filepath.Join(subDir, "main.go")

	// Create directory and file
	fs.WriteFileString(testFile, "package pkg")

	pb := NewPromptBuffer(fs, tmpDir, 10000)
	pb.AddFile(testFile)

	output := pb.FormatForPrompt()

	// Should use relative path
	if !strings.Contains(output, "src/pkg/main.go") && !strings.Contains(output, "src\\pkg\\main.go") {
		t.Error("output should use relative path")
	}
}

func TestCodeBlockWithDescription(t *testing.T) {
	fs := NewFileSystem()
	pb := NewPromptBuffer(fs, "/project", 10000)

	pb.AddCodeBlock(CodeBlock{
		FilePath:    "/project/main.go",
		StartLine:   10,
		EndLine:     20,
		Content:     "func important() {}",
		Language:    "go",
		Description: "Important function for handling requests",
	})

	output := pb.FormatForPrompt()

	if !strings.Contains(output, "Description: Important function") {
		t.Error("output should contain block description")
	}
}

func TestEmptyBuffer(t *testing.T) {
	fs := NewFileSystem()
	pb := NewPromptBuffer(fs, "/project", 10000)

	output := pb.FormatForPrompt()
	if output != "" {
		t.Errorf("empty buffer should produce empty output, got %q", output)
	}

	summary := pb.Summary()
	if !strings.Contains(summary, "Files: 0") {
		t.Error("summary should show 0 files")
	}
}

func TestLanguageDetectionInBuffer(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	files := map[string]string{
		"main.go":    "package main",
		"script.py":  "print('hello')",
		"app.js":     "console.log('hi')",
		"styles.css": ".class { }",
	}

	pb := NewPromptBuffer(fs, tmpDir, 10000)

	for name, content := range files {
		path := filepath.Join(tmpDir, name)
		fs.WriteFileString(path, content)
		pb.AddFile(path)
	}

	output := pb.FormatForPrompt()

	// Check that each language is detected
	languages := []string{"go", "python", "javascript", "css"}
	for _, lang := range languages {
		if !strings.Contains(output, "Language: "+lang) && !strings.Contains(output, "```"+lang) {
			t.Errorf("should detect language: %s", lang)
		}
	}
}

func TestTokenEstimation(t *testing.T) {
	fs := NewFileSystem()
	tmpDir := t.TempDir()

	// Create file with known size
	content := strings.Repeat("word ", 400) // 2000 chars = ~500 tokens
	testFile := filepath.Join(tmpDir, "test.txt")
	fs.WriteFileString(testFile, content)

	pb := NewPromptBuffer(fs, tmpDir, 10000)
	pb.AddFile(testFile)

	tokens := pb.GetTokenCount()
	// Should be approximately len/4
	expected := 2000 / 4
	if tokens < expected-10 || tokens > expected+10 {
		t.Errorf("expected ~%d tokens, got %d", expected, tokens)
	}
}
