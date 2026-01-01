package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof" // Import pprof for debugging
	"os"
	"os/signal"
	"syscall"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/httpbridge"
	depcheck "github.com/soypete/pedrocli/pkg/init"
)

const version = "0.3.0-dev"

func main() {
	// Load configuration
	cfg, err := config.LoadDefault()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Check dependencies (unless skipped)
	if !cfg.Init.SkipChecks {
		checker := depcheck.NewChecker(cfg)
		_, err := checker.CheckAll()

		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		if cfg.Init.Verbose {
			fmt.Println("All dependencies OK")
		}
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create HTTP server with embedded LLM backend and job manager
	server, err := httpbridge.NewServer(cfg, ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
		os.Exit(1)
	}

	port := 8080
	if cfg.Web.Port > 0 {
		port = cfg.Web.Port
	}

	host := "0.0.0.0" // Bind to all interfaces for Tailscale access
	if cfg.Web.Host != "" {
		host = cfg.Web.Host
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	fmt.Printf("PedroCLI HTTP Server v%s\n", version)
	fmt.Printf("Listening on http://%s\n", addr)
	fmt.Printf("pprof available at http://%s/debug/pprof/\n", addr)

	// Start pprof server on a separate port for debugging
	go func() {
		pprofAddr := fmt.Sprintf("%s:6060", host)
		fmt.Printf("pprof debug server on http://%s/debug/pprof/\n", pprofAddr)
		if err := http.ListenAndServe(pprofAddr, nil); err != nil {
			fmt.Fprintf(os.Stderr, "pprof server error: %v\n", err)
		}
	}()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		// Close server (including database connections)
		if err := server.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing server: %v\n", err)
		}
		cancel()
		os.Exit(0)
	}()

	if err := server.Run(addr); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
