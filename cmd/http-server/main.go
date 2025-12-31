package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof" // Import pprof for debugging
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/httpbridge"
	depcheck "github.com/soypete/pedrocli/pkg/init"
	"github.com/soypete/pedrocli/pkg/mcp"
)

const version = "0.2.0-dev"

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
			fmt.Println("✓ All dependencies OK")
		}
	}

	// Start MCP client (SAME AS CLI)
	mcpClient, ctx, cancel, err := startMCPClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start MCP client: %v\n", err)
		os.Exit(1)
	}
	defer cancel()

	// Start HTTP server (NEW)
	server := httpbridge.NewServer(cfg, mcpClient, ctx)

	port := 8080
	if cfg.Web.Port > 0 {
		port = cfg.Web.Port
	}

	host := "0.0.0.0" // Bind to all interfaces for Tailscale access
	if cfg.Web.Host != "" {
		host = cfg.Web.Host
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	fmt.Printf("🚀 PedroCLI HTTP Server v%s\n", version)
	fmt.Printf("📡 Listening on http://%s\n", addr)
	fmt.Printf("🔧 MCP Server: Running\n")
	fmt.Printf("📊 pprof available at http://%s/debug/pprof/\n", addr)

	// Start pprof server on a separate port for debugging
	go func() {
		pprofAddr := fmt.Sprintf("%s:6060", host)
		fmt.Printf("📊 pprof debug server on http://%s/debug/pprof/\n", pprofAddr)
		if err := http.ListenAndServe(pprofAddr, nil); err != nil {
			fmt.Fprintf(os.Stderr, "pprof server error: %v\n", err)
		}
	}()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n🛑 Shutting down...")
		cancel()
		os.Exit(0)
	}()

	if err := server.Run(addr); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

// startMCPClient starts the MCP server and returns a client (SAME AS CLI)
func startMCPClient(cfg *config.Config) (*mcp.Client, context.Context, context.CancelFunc, error) {
	// Find the MCP server binary
	serverPath, err := findMCPServer()
	if err != nil {
		return nil, nil, nil, err
	}

	// Create client
	client := mcp.NewClient(serverPath, []string{})

	// Create context WITHOUT timeout - the HTTP server should run indefinitely
	// Individual requests can have their own timeouts
	ctx, cancel := context.WithCancel(context.Background())

	// Start server
	if err := client.Start(ctx); err != nil {
		cancel()
		return nil, nil, nil, fmt.Errorf("failed to start MCP server: %w", err)
	}

	return client, ctx, cancel, nil
}

// findMCPServer finds the MCP server binary (SAME AS CLI)
func findMCPServer() (string, error) {
	// Try current directory first
	localPath := "./pedrocli-server"
	if _, err := os.Stat(localPath); err == nil {
		abs, _ := filepath.Abs(localPath)
		return abs, nil
	}

	// Try in same directory as the HTTP server binary
	exePath, err := os.Executable()
	if err == nil {
		serverPath := filepath.Join(filepath.Dir(exePath), "pedrocli-server")
		if _, err := os.Stat(serverPath); err == nil {
			return serverPath, nil
		}
	}

	// Try $PATH
	serverPath, err := exec.LookPath("pedrocli-server")
	if err == nil {
		return serverPath, nil
	}

	return "", fmt.Errorf("pedrocli-server not found. Please build it with 'make build-server'")
}
