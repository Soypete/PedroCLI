package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNavigateToolName(t *testing.T) {
	tool := NewNavigateTool("/tmp")
	if tool.Name() != "navigate" {
		t.Errorf("Name() = %v, want navigate", tool.Name())
	}
}

func TestNavigateToolDescription(t *testing.T) {
	tool := NewNavigateTool("/tmp")
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestListDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewNavigateTool(tmpDir)
	ctx := context.Background()

	// Create test structure
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "file1.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.py"), []byte("# python"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden"), 0644)

	tests := []struct {
		name     string
		args     map[string]interface{}
		validate func(*testing.T, *Result)
	}{
		{
			name: "list all files",
			args: map[string]interface{}{
				"action": "list_directory",
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "file1.go") {
					t.Errorf("Should contain file1.go")
				}
				if !strings.Contains(r.Output, "file2.py") {
					t.Errorf("Should contain file2.py")
				}
				if !strings.Contains(r.Output, "subdir/") {
					t.Errorf("Should contain subdir/")
				}
			},
		},
		{
			name: "filter by extension",
			args: map[string]interface{}{
				"action":    "list_directory",
				"extension": ".go",
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "file1.go") {
					t.Errorf("Should contain file1.go")
				}
				if strings.Contains(r.Output, "file2.py") {
					t.Errorf("Should not contain file2.py when filtering by .go")
				}
			},
		},
		{
			name: "show hidden files",
			args: map[string]interface{}{
				"action":      "list_directory",
				"show_hidden": true,
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, ".hidden") {
					t.Errorf("Should contain .hidden when show_hidden is true")
				}
			},
		},
		{
			name: "hide hidden files by default",
			args: map[string]interface{}{
				"action": "list_directory",
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if strings.Contains(r.Output, ".hidden") {
					t.Errorf("Should not contain .hidden by default")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
			}
			tt.validate(t, result)
		})
	}
}

func TestGetFileOutline(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewNavigateTool(tmpDir)
	ctx := context.Background()

	// Create Go test file
	goFile := filepath.Join(tmpDir, "main.go")
	goContent := `package main

import "fmt"

func HelloWorld() {
	fmt.Println("hello")
}

type MyStruct struct {
	field string
}

const MaxSize = 100

var counter int
`
	os.WriteFile(goFile, []byte(goContent), 0644)

	// Create Python test file
	pyFile := filepath.Join(tmpDir, "script.py")
	pyContent := `import sys

class MyClass:
    def __init__(self):
        pass

def hello_world():
    print("hello")
`
	os.WriteFile(pyFile, []byte(pyContent), 0644)

	tests := []struct {
		name     string
		path     string
		validate func(*testing.T, *Result)
	}{
		{
			name: "outline Go file",
			path: goFile,
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "func HelloWorld") {
					t.Errorf("Should find HelloWorld function")
				}
				if !strings.Contains(r.Output, "type MyStruct") {
					t.Errorf("Should find MyStruct type")
				}
				if !strings.Contains(r.Output, "const MaxSize") {
					t.Errorf("Should find MaxSize const")
				}
				if !strings.Contains(r.Output, "var counter") {
					t.Errorf("Should find counter var")
				}
			},
		},
		{
			name: "outline Python file",
			path: pyFile,
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "class MyClass") {
					t.Errorf("Should find MyClass class")
				}
				if !strings.Contains(r.Output, "def hello_world") {
					t.Errorf("Should find hello_world function")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"action": "get_file_outline",
				"path":   tt.path,
			}
			result, err := tool.Execute(ctx, args)
			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
			}
			tt.validate(t, result)
		})
	}
}

func TestFindImports(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewNavigateTool(tmpDir)
	ctx := context.Background()

	// Create Go test file
	goFile := filepath.Join(tmpDir, "main.go")
	goContent := `package main

import (
	"fmt"
	"strings"
)

func main() {}
`
	os.WriteFile(goFile, []byte(goContent), 0644)

	// Create Python test file
	pyFile := filepath.Join(tmpDir, "script.py")
	pyContent := `import os
import sys
from datetime import datetime

def main():
    pass
`
	os.WriteFile(pyFile, []byte(pyContent), 0644)

	// Create JavaScript test file
	jsFile := filepath.Join(tmpDir, "app.js")
	jsContent := `import React from 'react';
import { useState } from 'react';
const fs = require('fs');

function App() {}
`
	os.WriteFile(jsFile, []byte(jsContent), 0644)

	tests := []struct {
		name     string
		path     string
		validate func(*testing.T, *Result)
	}{
		{
			name: "find Go imports",
			path: goFile,
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				// Should find at least the import statement or the individual imports
				if !strings.Contains(r.Output, "import") && !strings.Contains(r.Output, "fmt") {
					t.Errorf("Should find import statement, got: %s", r.Output)
				}
			},
		},
		{
			name: "find Python imports",
			path: pyFile,
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "import os") {
					t.Errorf("Should find os import")
				}
				if !strings.Contains(r.Output, "from datetime import") {
					t.Errorf("Should find datetime import")
				}
			},
		},
		{
			name: "find JavaScript imports",
			path: jsFile,
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				// Should find either import or require statements
				if !strings.Contains(r.Output, "import") && !strings.Contains(r.Output, "require") {
					t.Errorf("Should find import or require statement, got: %s", r.Output)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"action": "find_imports",
				"path":   tt.path,
			}
			result, err := tool.Execute(ctx, args)
			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
			}
			tt.validate(t, result)
		})
	}
}

func TestGetTree(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewNavigateTool(tmpDir)
	ctx := context.Background()

	// Create test directory structure
	os.Mkdir(filepath.Join(tmpDir, "pkg"), 0755)
	os.Mkdir(filepath.Join(tmpDir, "pkg", "tools"), 0755)
	os.Mkdir(filepath.Join(tmpDir, "cmd"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "pkg", "config.go"), []byte("package pkg"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "pkg", "tools", "file.go"), []byte("package tools"), 0644)

	tests := []struct {
		name     string
		args     map[string]interface{}
		validate func(*testing.T, *Result)
	}{
		{
			name: "get tree default depth",
			args: map[string]interface{}{
				"action": "get_tree",
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "main.go") {
					t.Errorf("Should contain main.go")
				}
				if !strings.Contains(r.Output, "pkg") {
					t.Errorf("Should contain pkg directory")
				}
				if !strings.Contains(r.Output, "cmd") {
					t.Errorf("Should contain cmd directory")
				}
			},
		},
		{
			name: "get tree with depth limit",
			args: map[string]interface{}{
				"action":    "get_tree",
				"max_depth": float64(1),
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				// Should see pkg directory but not nested tools directory
				if !strings.Contains(r.Output, "pkg") {
					t.Errorf("Should contain pkg directory at depth 1")
				}
			},
		},
		{
			name: "get tree with specific directory",
			args: map[string]interface{}{
				"action":    "get_tree",
				"directory": "pkg",
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "config.go") {
					t.Errorf("Should contain config.go in pkg directory")
				}
				if !strings.Contains(r.Output, "tools") {
					t.Errorf("Should contain tools subdirectory")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
			}
			tt.validate(t, result)
		})
	}
}

func TestNavigateToolUnknownAction(t *testing.T) {
	tool := NewNavigateTool("/tmp")
	ctx := context.Background()

	args := map[string]interface{}{
		"action": "unknown",
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		t.Errorf("Execute() unexpected error = %v", err)
	}

	if result.Success {
		t.Error("Success = true, want false for unknown action")
	}
}
