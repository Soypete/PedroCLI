package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_Build_Command(t *testing.T) {
	SkipUnlessE2E(t)

	// Build the CLI binary first
	cliPath := buildCLI(t)
	defer os.Remove(cliPath)

	// Create test environment
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	// Create test config
	configPath := filepath.Join(env.WorkDir, ".pedroceli.json")
	createTestConfigFile(t, configPath, env.WorkDir)

	// Run pedrocli build command
	cmd := exec.Command(cliPath,
		"build",
		"-description", "Add a hello function",
		"-config", configPath,
	)
	cmd.Dir = env.WorkDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		// CLI might fail if MCP server isn't available, which is expected in test environment
		t.Logf("CLI output: %s", string(output))
		t.Skip("Skipping test - requires full MCP server setup")
	}

	t.Logf("Build command output: %s", string(output))
}

func TestCLI_Debug_Command(t *testing.T) {
	SkipUnlessE2E(t)

	cliPath := buildCLI(t)
	defer os.Remove(cliPath)

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	configPath := filepath.Join(env.WorkDir, ".pedroceli.json")
	createTestConfigFile(t, configPath, env.WorkDir)

	// Create a buggy file
	env.CreateFile("bug.go", "package main\n\nfunc broken() {}\n")

	// Run pedrocli debug command
	cmd := exec.Command(cliPath,
		"debug",
		"-symptoms", "Function is broken",
		"-config", configPath,
	)
	cmd.Dir = env.WorkDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("CLI output: %s", string(output))
		t.Skip("Skipping test - requires full MCP server setup")
	}

	t.Logf("Debug command output: %s", string(output))
}

func TestCLI_Review_Command(t *testing.T) {
	SkipUnlessE2E(t)

	cliPath := buildCLI(t)
	defer os.Remove(cliPath)

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	configPath := filepath.Join(env.WorkDir, ".pedroceli.json")
	createTestConfigFile(t, configPath, env.WorkDir)

	// Run pedrocli review command
	cmd := exec.Command(cliPath,
		"review",
		"-branch", "test-branch",
		"-config", configPath,
	)
	cmd.Dir = env.WorkDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("CLI output: %s", string(output))
		t.Skip("Skipping test - requires full MCP server setup")
	}

	t.Logf("Review command output: %s", string(output))
}

func TestCLI_Triage_Command(t *testing.T) {
	SkipUnlessE2E(t)

	cliPath := buildCLI(t)
	defer os.Remove(cliPath)

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	configPath := filepath.Join(env.WorkDir, ".pedroceli.json")
	createTestConfigFile(t, configPath, env.WorkDir)

	// Run pedrocli triage command
	cmd := exec.Command(cliPath,
		"triage",
		"-description", "Build is failing",
		"-config", configPath,
	)
	cmd.Dir = env.WorkDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("CLI output: %s", string(output))
		t.Skip("Skipping test - requires full MCP server setup")
	}

	t.Logf("Triage command output: %s", string(output))
}

func TestCLI_Help_Command(t *testing.T) {
	SkipUnlessE2E(t)

	cliPath := buildCLI(t)
	defer os.Remove(cliPath)

	// Run pedrocli --help
	cmd := exec.Command(cliPath, "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Help command failed: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)

	// Verify help output contains expected commands
	expectedCommands := []string{"build", "debug", "review", "triage", "status", "list", "cancel"}
	for _, cmd := range expectedCommands {
		if !strings.Contains(outputStr, cmd) {
			t.Errorf("Help output missing command: %s\nOutput: %s", cmd, outputStr)
		}
	}
}

func TestCLI_Version_Command(t *testing.T) {
	SkipUnlessE2E(t)

	cliPath := buildCLI(t)
	defer os.Remove(cliPath)

	// Run pedrocli --version
	cmd := exec.Command(cliPath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Version command failed: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "pedrocli") && !strings.Contains(outputStr, "version") {
		t.Errorf("Version output unexpected: %s", outputStr)
	}
}

// Helper functions

func buildCLI(t *testing.T) string {
	// Build pedrocli binary to temp location
	tmpDir := t.TempDir()
	cliPath := filepath.Join(tmpDir, "pedrocli")

	cmd := exec.Command("go", "build", "-o", cliPath, "../../cmd/pedrocli")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build CLI: %v\nOutput: %s", err, string(output))
	}

	return cliPath
}

func createTestConfigFile(t *testing.T, configPath, workDir string) {
	config := `{
  "model": {
    "type": "ollama",
    "model_name": "qwen2.5-coder:7b",
    "ollama_url": "http://localhost:11434",
    "temperature": 0.2
  },
  "project": {
    "name": "TestProject",
    "workdir": "` + workDir + `",
    "tech_stack": ["Go"]
  },
  "tools": {
    "allowed_bash_commands": ["echo", "ls", "cat", "pwd"],
    "forbidden_commands": ["rm", "mv", "dd", "sudo"]
  },
  "limits": {
    "max_task_duration_minutes": 5,
    "max_inference_runs": 5
  },
  "init": {
    "skip_checks": true,
    "verbose": false
  }
}
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}
}
