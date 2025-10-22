// ABOUTME: Main entry point for ACP relay server
// ABOUTME: Loads configuration and starts HTTP/WebSocket servers

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/harper/acp-relay/internal/config"
	httpserver "github.com/harper/acp-relay/internal/http"
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

	// Create session manager
	sessionMgr := session.NewManager(session.ManagerConfig{
		AgentCommand: cfg.Agent.Command,
		AgentArgs:    cfg.Agent.Args,
		AgentEnv:     cfg.Agent.Env,
	})

	// Create HTTP server
	httpSrv := httpserver.NewServer(sessionMgr)

	// Create WebSocket server
	wsSrv := wsserver.NewServer(sessionMgr)

	// Start HTTP server in goroutine
	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.Server.HTTPHost, cfg.Server.HTTPPort)
		log.Printf("Starting HTTP server on %s", addr)
		if err := http.ListenAndServe(addr, httpSrv); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Start WebSocket server on main goroutine
	wsAddr := fmt.Sprintf("%s:%d", cfg.Server.WebSocketHost, cfg.Server.WebSocketPort)
	log.Printf("Starting WebSocket server on %s", wsAddr)
	if err := http.ListenAndServe(wsAddr, wsSrv); err != nil {
		log.Fatalf("WebSocket server failed: %v", err)
	}
}
