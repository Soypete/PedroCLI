package init

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/platform"
)

// CheckResult represents the result of a dependency check
type CheckResult struct {
	Name     string
	Required bool
	Found    bool
	Path     string
	Version  string
	Error    string
}

// Checker validates dependencies before starting work
type Checker struct {
	config          *config.Config
	checkingWebDeps bool
}

// NewChecker creates a new dependency checker
func NewChecker(cfg *config.Config) *Checker {
	return &Checker{
		config:          cfg,
		checkingWebDeps: false,
	}
}

// CheckAll validates all dependencies
func (c *Checker) CheckAll() ([]CheckResult, error) {
	var results []CheckResult

	// Check inference backend
	if c.config.Model.Type == "llamacpp" {
		results = append(results, c.checkLlamaCpp())
		results = append(results, c.checkModelFile())
	} else if c.config.Model.Type == "ollama" {
		results = append(results, c.checkOllama())
	}

	// Check required CLI tools
	results = append(results, c.checkGit())
	results = append(results, c.checkGitHubCLI())

	// Check optional but recommended tools
	results = append(results, c.checkGo())

	// Check SSH access if using remote Spark
	if c.config.Execution.RunOnSpark {
		results = append(results, c.checkSparkSSH())
	}

	// Check platform-specific dependencies
	results = append(results, c.checkPlatformSpecific()...)

	// Validate any failures
	var failures []CheckResult
	for _, result := range results {
		if result.Required && !result.Found {
			failures = append(failures, result)
		}
	}

	if len(failures) > 0 {
		return results, c.formatErrors(failures)
	}

	return results, nil
}

// checkLlamaCpp checks if llama.cpp is available
func (c *Checker) checkLlamaCpp() CheckResult {
	path := c.config.Model.LlamaCppPath

	// Check if file exists
	if _, err := os.Stat(path); err != nil {
		return CheckResult{
			Name:     "llama.cpp",
			Required: true,
			Found:    false,
			Error:    fmt.Sprintf("llama-cli not found at %s", path),
		}
	}

	// Check if executable
	if !isExecutable(path) {
		return CheckResult{
			Name:     "llama.cpp",
			Required: true,
			Found:    false,
			Error:    fmt.Sprintf("%s is not executable", path),
		}
	}

	// Get version
	cmd := exec.Command(path, "--version")
	output, _ := cmd.CombinedOutput()
	version := strings.TrimSpace(string(output))

	return CheckResult{
		Name:     "llama.cpp",
		Required: true,
		Found:    true,
		Path:     path,
		Version:  version,
	}
}

// checkModelFile checks if the model file exists
func (c *Checker) checkModelFile() CheckResult {
	modelPath := c.config.Model.ModelPath

	if _, err := os.Stat(modelPath); err != nil {
		return CheckResult{
			Name:     "Model file",
			Required: true,
			Found:    false,
			Error:    fmt.Sprintf("Model not found at %s", modelPath),
		}
	}

	// Check file size (should be > 100MB for any real model)
	info, _ := os.Stat(modelPath)
	sizeMB := info.Size() / (1024 * 1024)

	if sizeMB < 100 {
		return CheckResult{
			Name:     "Model file",
			Required: true,
			Found:    false,
			Error:    fmt.Sprintf("Model file suspiciously small: %dMB", sizeMB),
		}
	}

	return CheckResult{
		Name:     "Model file",
		Required: true,
		Found:    true,
		Path:     modelPath,
		Version:  fmt.Sprintf("%dMB", sizeMB),
	}
}

// checkOllama checks if Ollama is available
func (c *Checker) checkOllama() CheckResult {
	path, err := exec.LookPath("ollama")
	if err != nil {
		return CheckResult{
			Name:     "Ollama",
			Required: true,
			Found:    false,
			Error:    "ollama not found in PATH. Install: curl -fsSL https://ollama.com/install.sh | sh",
		}
	}

	// Check if model is pulled
	cmd := exec.Command("ollama", "list")
	output, _ := cmd.CombinedOutput()

	modelName := c.config.Model.ModelName
	if !strings.Contains(string(output), modelName) {
		return CheckResult{
			Name:     "Ollama",
			Required: true,
			Found:    false,
			Error:    fmt.Sprintf("Model %s not found. Run: ollama pull %s", modelName, modelName),
		}
	}

	return CheckResult{
		Name:     "Ollama",
		Required: true,
		Found:    true,
		Path:     path,
		Version:  "OK (model available)",
	}
}

// checkGit checks if git is available
func (c *Checker) checkGit() CheckResult {
	path, err := exec.LookPath("git")
	if err != nil {
		return CheckResult{
			Name:     "Git",
			Required: true,
			Found:    false,
			Error:    "git not found. Install git to manage code changes.",
		}
	}

	cmd := exec.Command("git", "--version")
	output, _ := cmd.CombinedOutput()
	version := strings.TrimSpace(string(output))

	return CheckResult{
		Name:     "Git",
		Required: true,
		Found:    true,
		Path:     path,
		Version:  version,
	}
}

// checkGitHubCLI checks if GitHub CLI is available and authenticated
func (c *Checker) checkGitHubCLI() CheckResult {
	path, err := exec.LookPath("gh")
	if err != nil {
		return CheckResult{
			Name:     "GitHub CLI",
			Required: true,
			Found:    false,
			Error:    "gh not found. Install: https://cli.github.com/",
		}
	}

	// Check if authenticated
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		return CheckResult{
			Name:     "GitHub CLI",
			Required: true,
			Found:    false,
			Error:    "gh not authenticated. Run: gh auth login",
		}
	}

	cmd = exec.Command("gh", "--version")
	output, _ := cmd.CombinedOutput()
	version := strings.TrimSpace(strings.Split(string(output), "\n")[0])

	return CheckResult{
		Name:     "GitHub CLI",
		Required: true,
		Found:    true,
		Path:     path,
		Version:  version,
	}
}

// checkGo checks if Go is available
func (c *Checker) checkGo() CheckResult {
	path, err := exec.LookPath("go")
	if err != nil {
		return CheckResult{
			Name:     "Go",
			Required: false,
			Found:    false,
			Error:    "go not found (needed for Go projects)",
		}
	}

	cmd := exec.Command("go", "version")
	output, _ := cmd.CombinedOutput()
	version := strings.TrimSpace(string(output))

	return CheckResult{
		Name:     "Go",
		Required: false,
		Found:    true,
		Path:     path,
		Version:  version,
	}
}

// checkSparkSSH checks if SSH access to Spark is available
func (c *Checker) checkSparkSSH() CheckResult {
	sshHost := c.config.Execution.SparkSSH

	cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", sshHost, "echo OK")
	if err := cmd.Run(); err != nil {
		return CheckResult{
			Name:     "Spark SSH",
			Required: true,
			Found:    false,
			Error:    fmt.Sprintf("Cannot SSH to %s. Check SSH keys and config.", sshHost),
		}
	}

	return CheckResult{
		Name:     "Spark SSH",
		Required: true,
		Found:    true,
		Version:  "Connected",
	}
}

// checkPlatformSpecific checks platform-specific dependencies
func (c *Checker) checkPlatformSpecific() []CheckResult {
	var results []CheckResult

	switch platform.Current() {
	case platform.Linux:
		// Check if Ubuntu and version
		if c.isUbuntu() {
			results = append(results, c.checkUbuntuVersion())
		}
	}

	return results
}

// isUbuntu checks if running on Ubuntu
func (c *Checker) isUbuntu() bool {
	content, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return false
	}
	return strings.Contains(string(content), "Ubuntu")
}

// checkUbuntuVersion checks Ubuntu version
func (c *Checker) checkUbuntuVersion() CheckResult {
	content, _ := os.ReadFile("/etc/os-release")

	// Parse VERSION_ID
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "VERSION_ID=") {
			version := strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")

			// We target Ubuntu 24.04+
			if version >= "24.04" {
				return CheckResult{
					Name:     "Ubuntu Version",
					Required: false,
					Found:    true,
					Version:  version,
				}
			}

			return CheckResult{
				Name:     "Ubuntu Version",
				Required: false,
				Found:    true,
				Version:  version,
				Error:    "Ubuntu 24.04+ recommended",
			}
		}
	}

	return CheckResult{
		Name:     "Ubuntu Version",
		Required: false,
		Found:    false,
	}
}

// formatErrors formats dependency check errors
func (c *Checker) formatErrors(failures []CheckResult) error {
	var msg strings.Builder
	msg.WriteString("\n❌ Dependency check failed:\n\n")

	for _, failure := range failures {
		msg.WriteString(fmt.Sprintf("  ✗ %s: %s\n", failure.Name, failure.Error))
	}

	msg.WriteString("\nPlease install missing dependencies and try again.\n")

	return fmt.Errorf(msg.String())
}

// isExecutable checks if a file is executable
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&0111 != 0
}
