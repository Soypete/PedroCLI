package main

import (
	"fmt"
	"os"

	"github.com/soypete/pedrocli/pkg/config"
)

func main() {
	// Load configuration
	cfg, err := config.LoadDefault()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		fmt.Fprintf(os.Stderr, "Create a .pedroceli.json file. See .pedroceli.json.example for reference.\n")
		os.Exit(1)
	}

	fmt.Printf("Pedroceli CLI v0.1.0\n")
	fmt.Printf("Model: %s (%s)\n", cfg.Model.Type, cfg.Model.ModelPath)
	fmt.Printf("Context: %d tokens (usable: %d)\n", cfg.Model.ContextSize, cfg.Model.UsableContext)
	fmt.Printf("\nCLI implementation coming soon in Phase 2!\n")
	fmt.Printf("For now, use the MCP server directly: pedroceli-server\n")
}
