package tools

import (
	"os"
	"os/exec"
)

// Capability represents a system capability that tools may require
type Capability string

const (
	// CapabilityGit indicates git is available in PATH
	CapabilityGit Capability = "git"

	// CapabilityNetwork indicates network access is allowed
	CapabilityNetwork Capability = "network"

	// CapabilityNotionAPI indicates NOTION_TOKEN is set
	CapabilityNotionAPI Capability = "notion_api"

	// CapabilityBash indicates bash commands can be executed
	CapabilityBash Capability = "bash"

	// CapabilityGitHubAPI indicates GITHUB_TOKEN is set
	CapabilityGitHubAPI Capability = "github_api"

	// CapabilityGitLabAPI indicates GITLAB_TOKEN is set
	CapabilityGitLabAPI Capability = "gitlab_api"

	// CapabilityWhisper indicates whisper transcription service is available
	CapabilityWhisper Capability = "whisper"

	// CapabilityOllama indicates Ollama is available
	CapabilityOllama Capability = "ollama"
)

// CapabilityChecker checks for system capabilities at runtime
type CapabilityChecker interface {
	// Check returns true if the capability is available
	Check(cap Capability) bool

	// CheckAll returns a list of missing capabilities from the input
	CheckAll(caps []Capability) []Capability

	// Available returns all capabilities that are currently available
	Available() []Capability
}

// DefaultCapabilityChecker is the standard implementation that checks
// environment variables, PATH, and network availability
type DefaultCapabilityChecker struct {
	// Override allows tests to inject specific capability states
	Override map[Capability]bool
}

// NewCapabilityChecker creates a new default capability checker
func NewCapabilityChecker() *DefaultCapabilityChecker {
	return &DefaultCapabilityChecker{
		Override: make(map[Capability]bool),
	}
}

// Check returns true if the capability is available
func (c *DefaultCapabilityChecker) Check(cap Capability) bool {
	// Check override first (for testing)
	if override, ok := c.Override[cap]; ok {
		return override
	}

	switch cap {
	case CapabilityGit:
		return c.checkGit()
	case CapabilityNetwork:
		return c.checkNetwork()
	case CapabilityNotionAPI:
		return c.checkNotionAPI()
	case CapabilityBash:
		return c.checkBash()
	case CapabilityGitHubAPI:
		return c.checkGitHubAPI()
	case CapabilityGitLabAPI:
		return c.checkGitLabAPI()
	case CapabilityWhisper:
		return c.checkWhisper()
	case CapabilityOllama:
		return c.checkOllama()
	default:
		return false
	}
}

// CheckAll returns a list of missing capabilities
func (c *DefaultCapabilityChecker) CheckAll(caps []Capability) []Capability {
	var missing []Capability
	for _, cap := range caps {
		if !c.Check(cap) {
			missing = append(missing, cap)
		}
	}
	return missing
}

// Available returns all capabilities that are currently available
func (c *DefaultCapabilityChecker) Available() []Capability {
	allCaps := []Capability{
		CapabilityGit,
		CapabilityNetwork,
		CapabilityNotionAPI,
		CapabilityBash,
		CapabilityGitHubAPI,
		CapabilityGitLabAPI,
		CapabilityWhisper,
		CapabilityOllama,
	}

	var available []Capability
	for _, cap := range allCaps {
		if c.Check(cap) {
			available = append(available, cap)
		}
	}
	return available
}

// checkGit checks if git is available in PATH
func (c *DefaultCapabilityChecker) checkGit() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// checkNetwork always returns true (network is generally available)
// Override this in tests or specific environments
func (c *DefaultCapabilityChecker) checkNetwork() bool {
	return true
}

// checkNotionAPI checks if NOTION_TOKEN is set
func (c *DefaultCapabilityChecker) checkNotionAPI() bool {
	return os.Getenv("NOTION_TOKEN") != ""
}

// checkBash checks if bash is available in PATH
func (c *DefaultCapabilityChecker) checkBash() bool {
	_, err := exec.LookPath("bash")
	return err == nil
}

// checkGitHubAPI checks if GITHUB_TOKEN is set
func (c *DefaultCapabilityChecker) checkGitHubAPI() bool {
	return os.Getenv("GITHUB_TOKEN") != ""
}

// checkGitLabAPI checks if GITLAB_TOKEN is set
func (c *DefaultCapabilityChecker) checkGitLabAPI() bool {
	return os.Getenv("GITLAB_TOKEN") != ""
}

// checkWhisper checks if whisper is configured
// This is a simplified check - in practice you might ping the whisper service
func (c *DefaultCapabilityChecker) checkWhisper() bool {
	return os.Getenv("WHISPER_URL") != ""
}

// checkOllama checks if Ollama is available
func (c *DefaultCapabilityChecker) checkOllama() bool {
	_, err := exec.LookPath("ollama")
	return err == nil
}

// ListAvailable filters registry tools by their required capabilities
// Returns only tools whose RequiresCapabilities are all satisfied
func (r *ToolRegistry) ListAvailable(checker CapabilityChecker) []ExtendedTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var available []ExtendedTool
	for _, tool := range r.tools {
		meta := tool.Metadata()
		if meta == nil {
			// Tools without metadata are always available
			available = append(available, tool)
			continue
		}

		// Check if all required capabilities are available
		allAvailable := true
		for _, capStr := range meta.RequiresCapabilities {
			if !checker.Check(Capability(capStr)) {
				allAvailable = false
				break
			}
		}

		if allAvailable {
			available = append(available, tool)
		}
	}

	return available
}

// ListUnavailable returns tools that cannot be used due to missing capabilities
// along with their missing requirements
func (r *ToolRegistry) ListUnavailable(checker CapabilityChecker) map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	unavailable := make(map[string][]string)
	for name, tool := range r.tools {
		meta := tool.Metadata()
		if meta == nil {
			continue
		}

		var missing []string
		for _, capStr := range meta.RequiresCapabilities {
			if !checker.Check(Capability(capStr)) {
				missing = append(missing, capStr)
			}
		}

		if len(missing) > 0 {
			unavailable[name] = missing
		}
	}

	return unavailable
}
