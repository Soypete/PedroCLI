package main

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/soypete/pedrocli/pkg/cli"
	"github.com/soypete/pedrocli/pkg/repl"
)

func runPodcastMode(debugMode bool) error {
	// Load config
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if podcast mode is enabled
	if !cfg.Podcast.Enabled {
		return fmt.Errorf("podcast mode is not enabled in config")
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
	sessionID := fmt.Sprintf("podcast-%s-%s", uuid.New().String()[:8], time.Now().Format("20060102-150405"))
	session, err := repl.NewSession(sessionID, cfg, bridge, "podcast", debugMode)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Create REPL
	replInstance, err := repl.NewREPL(session)
	if err != nil {
		return fmt.Errorf("failed to create REPL: %w", err)
	}

	// Run REPL
	return replInstance.Run()
}
