// ABOUTME: Main entry point for ACP relay server
// ABOUTME: Supports 'serve' and 'setup' subcommands with --verbose flag

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/harper/acp-relay/internal/config"
	"github.com/harper/acp-relay/internal/db"
	httpserver "github.com/harper/acp-relay/internal/http"
	"github.com/harper/acp-relay/internal/logger"
	mgmtserver "github.com/harper/acp-relay/internal/management"
	"github.com/harper/acp-relay/internal/session"
	wsserver "github.com/harper/acp-relay/internal/websocket"
	"github.com/joho/godotenv"
)

//nolint:funlen // main initialization function
func main() {
	// Load .env file if it exists (silently ignore if not found)
	_ = godotenv.Load()

	// Check for subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "setup":
			runSetup()
			return
		case "serve", "":
			// Continue to server setup
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
			fmt.Fprintf(os.Stderr, "Usage: acp-relay [serve|setup] [flags]\n")
			os.Exit(1)
		}
	}

	// Serve command flags
	serveFlags := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := serveFlags.String("config", "config.yaml", "path to config file")
	verbose := serveFlags.Bool("verbose", false, "enable verbose logging")

	// Parse flags after subcommand
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		if err := serveFlags.Parse(os.Args[2:]); err != nil {
			log.Fatalf("failed to parse serve flags: %v", err)
		}
	} else {
		if err := serveFlags.Parse(os.Args[1:]); err != nil {
			log.Fatalf("failed to parse flags: %v", err)
		}
	}

	// Set logger verbosity
	logger.SetVerbose(*verbose)
	if *verbose {
		logger.Info("Verbose logging enabled")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Open database for message logging
	database, err := db.Open(cfg.Database.Path)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	// Startup maintenance: mark all open sessions as closed (they crashed/were orphaned)
	closedCount, err := database.CloseAllOpenSessions()
	if err != nil {
		log.Printf("Warning: failed to close open sessions during startup: %v", err)
	} else if closedCount > 0 {
		log.Printf("Startup maintenance: marked %d crashed/orphaned sessions as closed", closedCount)
	}

	// Create session manager
	sessionMgr := session.NewManager(session.ManagerConfig{
		Mode:            cfg.Agent.Mode,
		AgentCommand:    cfg.Agent.Command,
		AgentArgs:       cfg.Agent.Args,
		AgentEnv:        cfg.Agent.Env,
		ContainerConfig: cfg.Agent.Container,
	}, database)

	logger.Info("Session manager initialized (mode: %s)", cfg.Agent.Mode)

	// Create HTTP server
	httpSrv := httpserver.NewServer(sessionMgr)

	// Create WebSocket server
	wsSrv := wsserver.NewServer(sessionMgr)

	// Create management server
	mgmtSrv := mgmtserver.NewServer(cfg, sessionMgr, database)

	// Start HTTP server in goroutine
	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.Server.HTTPHost, cfg.Server.HTTPPort)
		logger.Info("Starting HTTP server on %s", addr)
		//nolint:gosec // http server for relay API - timeouts configured via reverse proxy
		if err := http.ListenAndServe(addr, httpSrv); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Start WebSocket server in goroutine
	go func() {
		wsAddr := fmt.Sprintf("%s:%d", cfg.Server.WebSocketHost, cfg.Server.WebSocketPort)
		logger.Info("Starting WebSocket server on %s", wsAddr)
		//nolint:gosec // websocket server for agent communication - long-lived connections
		if err := http.ListenAndServe(wsAddr, wsSrv); err != nil {
			log.Fatalf("WebSocket server failed: %v", err)
		}
	}()

	// Start management server on main goroutine (localhost only for security)
	mgmtAddr := fmt.Sprintf("%s:%d", cfg.Server.ManagementHost, cfg.Server.ManagementPort)
	logger.Info("Starting management API on %s", mgmtAddr)
	//nolint:gosec // management API server - internal use only
	if err := http.ListenAndServe(mgmtAddr, mgmtSrv); err != nil {
		log.Printf("[ERROR] Management server failed: %v", err)
		os.Exit(1) //nolint:gocritic // intentional exit on critical error
	}
}
