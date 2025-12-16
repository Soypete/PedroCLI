package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// NavigateTool provides code navigation and structure analysis
type NavigateTool struct {
	workDir string
}

// NewNavigateTool creates a new navigate tool
func NewNavigateTool(workDir string) *NavigateTool {
	return &NavigateTool{
		workDir: workDir,
	}
}

// Name returns the tool name
func (n *NavigateTool) Name() string {
	return "navigate"
}

// Description returns the tool description
func (n *NavigateTool) Description() string {
	return "Navigate code structure: list files, get file outline, find imports"
}

// Execute executes the navigate tool
func (n *NavigateTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	action, ok := args["action"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'action' parameter"}, nil
	}

	switch action {
	case "list_directory":
		return n.listDirectory(args)
	case "get_file_outline":
		return n.getFileOutline(args)
	case "find_imports":
		return n.findImports(args)
	case "get_tree":
		return n.getTree(args)
	default:
		return &Result{Success: false, Error: fmt.Sprintf("unknown action: %s", action)}, nil
	}
}

// listDirectory lists files in a directory with optional filtering
func (n *NavigateTool) listDirectory(args map[string]interface{}) (*Result, error) {
	// Optional: specific directory
	dir := n.workDir
	if d, ok := args["directory"].(string); ok {
		dir = filepath.Join(n.workDir, d)
	}

	// Optional: show hidden files
	showHidden := false
	if sh, ok := args["show_hidden"].(bool); ok {
		showHidden = sh
	}

	// Optional: filter by extension
	var filterExt string
	if ext, ok := args["extension"].(string); ok {
		filterExt = ext
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	var files []string
	var dirs []string

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files unless requested
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}

		// Skip common ignore patterns
		if name == "node_modules" || name == "vendor" || name == ".git" {
			continue
		}

		if entry.IsDir() {
			dirs = append(dirs, name+"/")
		} else {
			// Filter by extension if specified
			if filterExt != "" && filepath.Ext(name) != filterExt {
				continue
			}
			files = append(files, name)
		}
	}

	var output []string
	if len(dirs) > 0 {
		output = append(output, "Directories:")
		output = append(output, dirs...)
	}
	if len(files) > 0 {
		if len(dirs) > 0 {
			output = append(output, "")
		}
		output = append(output, "Files:")
		output = append(output, files...)
	}

	if len(output) == 0 {
		return &Result{
			Success: true,
			Output:  "Directory is empty or all files filtered",
		}, nil
	}

	return &Result{
		Success: true,
		Output:  strings.Join(output, "\n"),
	}, nil
}

// getFileOutline extracts structure of a file (functions, classes, types)
func (n *NavigateTool) getFileOutline(args map[string]interface{}) (*Result, error) {
	path, ok := args["path"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'path' parameter"}, nil
	}

	// Determine language from extension
	ext := filepath.Ext(path)
	language := n.detectLanguage(ext)

	file, err := os.Open(path)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}
	defer func() { _ = file.Close() }()

	var outline []string
	scanner := bufio.NewScanner(file)
	lineNum := 1

	patterns := n.getOutlinePatterns(language)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern.regex)
			if re.MatchString(trimmed) {
				outline = append(outline, fmt.Sprintf("%d: [%s] %s", lineNum, pattern.name, trimmed))
				break
			}
		}

		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	if len(outline) == 0 {
		return &Result{
			Success: true,
			Output:  "No structure elements found",
		}, nil
	}

	return &Result{
		Success: true,
		Output:  strings.Join(outline, "\n"),
	}, nil
}

// findImports finds import statements in a file
func (n *NavigateTool) findImports(args map[string]interface{}) (*Result, error) {
	path, ok := args["path"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'path' parameter"}, nil
	}

	ext := filepath.Ext(path)
	language := n.detectLanguage(ext)

	file, err := os.Open(path)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}
	defer func() { _ = file.Close() }()

	var imports []string
	scanner := bufio.NewScanner(file)
	lineNum := 1

	patterns := n.getImportPatterns(language)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			if re.MatchString(line) {
				imports = append(imports, fmt.Sprintf("%d: %s", lineNum, line))
				break
			}
		}

		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	if len(imports) == 0 {
		return &Result{
			Success: true,
			Output:  "No imports found",
		}, nil
	}

	return &Result{
		Success: true,
		Output:  strings.Join(imports, "\n"),
	}, nil
}

// getTree gets a directory tree structure
func (n *NavigateTool) getTree(args map[string]interface{}) (*Result, error) {
	// Optional: specific directory
	dir := n.workDir
	if d, ok := args["directory"].(string); ok {
		dir = filepath.Join(n.workDir, d)
	}

	// Optional: max depth
	maxDepth := 3
	if depth, ok := args["max_depth"].(float64); ok {
		maxDepth = int(depth)
	}

	var tree []string
	err := n.buildTree(dir, "", 0, maxDepth, &tree)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	return &Result{
		Success: true,
		Output:  strings.Join(tree, "\n"),
	}, nil
}

// buildTree recursively builds directory tree
func (n *NavigateTool) buildTree(dir string, prefix string, depth int, maxDepth int, tree *[]string) error {
	if depth > maxDepth {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for i, entry := range entries {
		name := entry.Name()

		// Skip hidden and common ignore patterns
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
			continue
		}

		isLast := i == len(entries)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		*tree = append(*tree, prefix+connector+name)

		if entry.IsDir() {
			newPrefix := prefix
			if isLast {
				newPrefix += "    "
			} else {
				newPrefix += "│   "
			}

			n.buildTree(filepath.Join(dir, name), newPrefix, depth+1, maxDepth, tree)
		}
	}

	return nil
}

// detectLanguage detects programming language from file extension
func (n *NavigateTool) detectLanguage(ext string) string {
	languages := map[string]string{
		".go":   "go",
		".py":   "python",
		".js":   "javascript",
		".ts":   "typescript",
		".tsx":  "typescript",
		".jsx":  "javascript",
		".java": "java",
		".c":    "c",
		".cpp":  "cpp",
		".h":    "c",
		".hpp":  "cpp",
		".rs":   "rust",
		".rb":   "ruby",
		".php":  "php",
	}

	if lang, ok := languages[ext]; ok {
		return lang
	}

	return "unknown"
}

// outlinePattern represents a pattern for extracting code structure
type outlinePattern struct {
	name  string
	regex string
}

// getOutlinePatterns returns patterns for extracting code structure
func (n *NavigateTool) getOutlinePatterns(language string) []outlinePattern {
	patterns := []outlinePattern{}

	switch language {
	case "go":
		patterns = append(patterns,
			outlinePattern{"func", `^func\s+`},
			outlinePattern{"type", `^type\s+`},
			outlinePattern{"const", `^const\s+`},
			outlinePattern{"var", `^var\s+`},
		)
	case "python":
		patterns = append(patterns,
			outlinePattern{"class", `^class\s+`},
			outlinePattern{"def", `^def\s+`},
		)
	case "javascript", "typescript":
		patterns = append(patterns,
			outlinePattern{"class", `^class\s+`},
			outlinePattern{"function", `^function\s+`},
			outlinePattern{"const", `^const\s+\w+\s*=\s*(\(|function)`},
			outlinePattern{"export", `^export\s+(class|function|const)`},
		)
	default:
		// Generic patterns
		patterns = append(patterns,
			outlinePattern{"func", `^(func|function|def)\s+`},
			outlinePattern{"class", `^class\s+`},
			outlinePattern{"type", `^type\s+`},
		)
	}

	return patterns
}

// getImportPatterns returns patterns for finding imports
func (n *NavigateTool) getImportPatterns(language string) []string {
	patterns := []string{}

	switch language {
	case "go":
		patterns = append(patterns, `^import\s+`, `^\s+".*"`)
	case "python":
		patterns = append(patterns, `^import\s+`, `^from\s+.*import`)
	case "javascript", "typescript":
		patterns = append(patterns, `^import\s+`, `^import\s+.*from`, `^require\(`)
	default:
		patterns = append(patterns, `^import\s+`, `^from\s+`, `^require\(`)
	}

	return patterns
}
