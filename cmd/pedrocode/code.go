package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/soypete/pedrocli/pkg/cli"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/repl"
)

func runCodeMode(debugMode bool) error {
	// Load config
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get working directory
	workdir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Create CLI bridge
	bridge, err := cli.NewCLIBridge(cli.CLIBridgeConfig{
		Config:  cfg,
		WorkDir: workdir,
	})
	if err != nil {
		return fmt.Errorf("failed to create CLI bridge: %w", err)
	}
	defer bridge.Close()

	// Create session
	sessionID := fmt.Sprintf("code-%s-%s", uuid.New().String()[:8], time.Now().Format("20060102-150405"))
	session, err := repl.NewSession(sessionID, cfg, bridge, "code", debugMode)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Check for incomplete jobs from previous sessions
	persister, err := repl.NewJobStatePersister()
	if err == nil {
		repl.CheckForIncompleteJobs(persister)
	}

	// Create REPL
	replInstance, err := repl.NewREPL(session)
	if err != nil {
		return fmt.Errorf("failed to create REPL: %w", err)
	}

	// Setup cleanup on exit
	defer func() {
		// Mark any running jobs as incomplete
		if persister != nil {
			for _, job := range session.JobManager.ActiveJobs() {
				_ = persister.MarkJobIncomplete(job.ID)
			}
		}
	}()

	// Run REPL
	return replInstance.Run()
}

// loadConfig loads the configuration file
func loadConfig() (*config.Config, error) {
	// Try current directory first
	configPath := ".pedrocli.json"
	if _, err := os.Stat(configPath); err != nil {
		// Try home directory
		homeDir, err := os.UserHomeDir()
		if err == nil {
			configPath = filepath.Join(homeDir, ".pedrocli.json")
		}
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		// If config not found, use defaults
		cfg, err = config.LoadDefault()
		if err != nil {
			return nil, err
		}
	}

	return cfg, nil
}
