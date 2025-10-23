// ABOUTME: Main entry point for ACP relay server
// ABOUTME: Loads configuration and starts HTTP/WebSocket servers

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/harper/acp-relay/internal/config"
	"github.com/harper/acp-relay/internal/db"
	httpserver "github.com/harper/acp-relay/internal/http"
	mgmtserver "github.com/harper/acp-relay/internal/management"
	"github.com/harper/acp-relay/internal/session"
	wsserver "github.com/harper/acp-relay/internal/websocket"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Open database for message logging
	database, err := db.Open(cfg.Database.Path)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

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

	log.Printf("Session manager initialized (mode: %s)", cfg.Agent.Mode)

	// Create HTTP server
	httpSrv := httpserver.NewServer(sessionMgr)

	// Create WebSocket server
	wsSrv := wsserver.NewServer(sessionMgr)

	// Create management server
	mgmtSrv := mgmtserver.NewServer(cfg, sessionMgr)

	// Start HTTP server in goroutine
	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.Server.HTTPHost, cfg.Server.HTTPPort)
		log.Printf("Starting HTTP server on %s", addr)
		if err := http.ListenAndServe(addr, httpSrv); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Start WebSocket server in goroutine
	go func() {
		wsAddr := fmt.Sprintf("%s:%d", cfg.Server.WebSocketHost, cfg.Server.WebSocketPort)
		log.Printf("Starting WebSocket server on %s", wsAddr)
		if err := http.ListenAndServe(wsAddr, wsSrv); err != nil {
			log.Fatalf("WebSocket server failed: %v", err)
		}
	}()

	// Start management server on main goroutine (localhost only for security)
	mgmtAddr := fmt.Sprintf("%s:%d", cfg.Server.ManagementHost, cfg.Server.ManagementPort)
	log.Printf("Starting management API on %s", mgmtAddr)
	if err := http.ListenAndServe(mgmtAddr, mgmtSrv); err != nil {
		log.Fatalf("Management server failed: %v", err)
	}
}
