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

// SearchTool provides code search capabilities (grep, find files)
type SearchTool struct {
	workDir string
}

// NewSearchTool creates a new search tool
func NewSearchTool(workDir string) *SearchTool {
	return &SearchTool{
		workDir: workDir,
	}
}

// Name returns the tool name
func (s *SearchTool) Name() string {
	return "search"
}

// Description returns the tool description
func (s *SearchTool) Description() string {
	return "Search code: grep in files, find files by pattern, find definitions"
}

// Execute executes the search tool
func (s *SearchTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	action, ok := args["action"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'action' parameter"}, nil
	}

	switch action {
	case "grep":
		return s.grep(args)
	case "find_files":
		return s.findFiles(args)
	case "find_in_file":
		return s.findInFile(args)
	case "find_definition":
		return s.findDefinition(args)
	default:
		return &Result{Success: false, Error: fmt.Sprintf("unknown action: %s", action)}, nil
	}
}

// grep searches for a pattern across files
func (s *SearchTool) grep(args map[string]interface{}) (*Result, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'pattern' parameter"}, nil
	}

	// Optional: specific directory
	searchDir := s.workDir
	if dir, ok := args["directory"].(string); ok {
		searchDir = filepath.Join(s.workDir, dir)
	}

	// Optional: file glob pattern
	globPattern := "*"
	if glob, ok := args["file_pattern"].(string); ok {
		globPattern = glob
	}

	// Optional: case insensitive
	caseInsensitive := false
	if ci, ok := args["case_insensitive"].(bool); ok {
		caseInsensitive = ci
	}

	// Optional: max results
	maxResults := 100
	if max, ok := args["max_results"].(float64); ok {
		maxResults = int(max)
	}

	// Compile regex
	var re *regexp.Regexp
	var err error
	if caseInsensitive {
		re, err = regexp.Compile("(?i)" + pattern)
	} else {
		re, err = regexp.Compile(pattern)
	}

	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("invalid regex pattern: %s", err)}, nil
	}

	var matches []string
	matchCount := 0

	// Walk directory
	err = filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		if info.IsDir() {
			// Skip hidden directories and common ignore patterns
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "node_modules" || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file matches glob pattern
		matched, _ := filepath.Match(globPattern, info.Name())
		if !matched {
			return nil
		}

		// Search in file
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 1

		for scanner.Scan() {
			if matchCount >= maxResults {
				return filepath.SkipAll
			}

			line := scanner.Text()
			if re.MatchString(line) {
				relPath, _ := filepath.Rel(s.workDir, path)
				matches = append(matches, fmt.Sprintf("%s:%d:%s", relPath, lineNum, line))
				matchCount++
			}
			lineNum++
		}

		return nil
	})

	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	if len(matches) == 0 {
		return &Result{
			Success: true,
			Output:  fmt.Sprintf("No matches found for pattern: %s", pattern),
		}, nil
	}

	output := strings.Join(matches, "\n")
	if matchCount >= maxResults {
		output += fmt.Sprintf("\n... (limited to %d results)", maxResults)
	}

	return &Result{
		Success: true,
		Output:  output,
	}, nil
}

// findFiles finds files matching a glob pattern
func (s *SearchTool) findFiles(args map[string]interface{}) (*Result, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'pattern' parameter"}, nil
	}

	// Optional: specific directory
	searchDir := s.workDir
	if dir, ok := args["directory"].(string); ok {
		searchDir = filepath.Join(s.workDir, dir)
	}

	// Optional: max results
	maxResults := 100
	if max, ok := args["max_results"].(float64); ok {
		maxResults = int(max)
	}

	var files []string

	err := filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "node_modules" || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		if len(files) >= maxResults {
			return filepath.SkipAll
		}

		// Match pattern
		matched, _ := filepath.Match(pattern, info.Name())
		if matched {
			relPath, _ := filepath.Rel(s.workDir, path)
			files = append(files, relPath)
		}

		return nil
	})

	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	if len(files) == 0 {
		return &Result{
			Success: true,
			Output:  fmt.Sprintf("No files found matching pattern: %s", pattern),
		}, nil
	}

	output := strings.Join(files, "\n")
	if len(files) >= maxResults {
		output += fmt.Sprintf("\n... (limited to %d results)", maxResults)
	}

	return &Result{
		Success: true,
		Output:  output,
	}, nil
}

// findInFile searches for a pattern in a specific file with line numbers
func (s *SearchTool) findInFile(args map[string]interface{}) (*Result, error) {
	path, ok := args["path"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'path' parameter"}, nil
	}

	pattern, ok := args["pattern"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'pattern' parameter"}, nil
	}

	// Optional: case insensitive
	caseInsensitive := false
	if ci, ok := args["case_insensitive"].(bool); ok {
		caseInsensitive = ci
	}

	// Compile regex
	var re *regexp.Regexp
	var err error
	if caseInsensitive {
		re, err = regexp.Compile("(?i)" + pattern)
	} else {
		re, err = regexp.Compile(pattern)
	}

	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("invalid regex pattern: %s", err)}, nil
	}

	// Read file
	file, err := os.Open(path)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}
	defer file.Close()

	var matches []string
	scanner := bufio.NewScanner(file)
	lineNum := 1

	for scanner.Scan() {
		line := scanner.Text()
		if re.MatchString(line) {
			matches = append(matches, fmt.Sprintf("%d:%s", lineNum, line))
		}
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	if len(matches) == 0 {
		return &Result{
			Success: true,
			Output:  fmt.Sprintf("No matches found in %s", path),
		}, nil
	}

	return &Result{
		Success: true,
		Output:  strings.Join(matches, "\n"),
	}, nil
}

// findDefinition finds function/class definitions in code
func (s *SearchTool) findDefinition(args map[string]interface{}) (*Result, error) {
	name, ok := args["name"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'name' parameter"}, nil
	}

	// Optional: specific directory
	searchDir := s.workDir
	if dir, ok := args["directory"].(string); ok {
		searchDir = filepath.Join(s.workDir, dir)
	}

	// Optional: language hint
	language := ""
	if lang, ok := args["language"].(string); ok {
		language = lang
	}

	// Build patterns based on language
	patterns := s.buildDefinitionPatterns(name, language)

	var matches []string

	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}

		err = filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if info.IsDir() {
				if strings.HasPrefix(info.Name(), ".") || info.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}

			// Skip non-code files
			if !s.isCodeFile(info.Name()) {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			lineNum := 1

			for scanner.Scan() {
				line := scanner.Text()
				if re.MatchString(line) {
					relPath, _ := filepath.Rel(s.workDir, path)
					matches = append(matches, fmt.Sprintf("%s:%d:%s", relPath, lineNum, strings.TrimSpace(line)))
				}
				lineNum++
			}

			return nil
		})

		if err != nil {
			continue
		}
	}

	if len(matches) == 0 {
		return &Result{
			Success: true,
			Output:  fmt.Sprintf("No definition found for: %s", name),
		}, nil
	}

	return &Result{
		Success: true,
		Output:  strings.Join(matches, "\n"),
	}, nil
}

// buildDefinitionPatterns builds regex patterns for finding definitions
func (s *SearchTool) buildDefinitionPatterns(name string, language string) []string {
	patterns := []string{}

	switch language {
	case "go":
		patterns = append(patterns, fmt.Sprintf(`^\s*func\s+%s\s*\(`, name))
		patterns = append(patterns, fmt.Sprintf(`^\s*func\s+\(\w+\s+\*?\w+\)\s+%s\s*\(`, name))
		patterns = append(patterns, fmt.Sprintf(`^\s*type\s+%s\s+`, name))
	case "python":
		patterns = append(patterns, fmt.Sprintf(`^\s*def\s+%s\s*\(`, name))
		patterns = append(patterns, fmt.Sprintf(`^\s*class\s+%s\s*[\(:]`, name))
	case "javascript", "typescript":
		patterns = append(patterns, fmt.Sprintf(`^\s*function\s+%s\s*\(`, name))
		patterns = append(patterns, fmt.Sprintf(`^\s*const\s+%s\s*=`, name))
		patterns = append(patterns, fmt.Sprintf(`^\s*class\s+%s\s*`, name))
	default:
		// Generic patterns
		patterns = append(patterns, fmt.Sprintf(`^\s*func\s+%s\s*\(`, name))
		patterns = append(patterns, fmt.Sprintf(`^\s*def\s+%s\s*\(`, name))
		patterns = append(patterns, fmt.Sprintf(`^\s*function\s+%s\s*\(`, name))
		patterns = append(patterns, fmt.Sprintf(`^\s*class\s+%s\s*`, name))
		patterns = append(patterns, fmt.Sprintf(`^\s*type\s+%s\s+`, name))
	}

	return patterns
}

// isCodeFile checks if a file is a code file based on extension
func (s *SearchTool) isCodeFile(filename string) bool {
	codeExtensions := []string{
		".go", ".py", ".js", ".ts", ".tsx", ".jsx",
		".java", ".c", ".cpp", ".h", ".hpp",
		".rs", ".rb", ".php", ".cs", ".swift",
		".kt", ".scala", ".sh", ".bash",
	}

	ext := filepath.Ext(filename)
	for _, codeExt := range codeExtensions {
		if ext == codeExt {
			return true
		}
	}

	return false
}
