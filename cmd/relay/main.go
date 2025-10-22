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

	addr := fmt.Sprintf("%s:%d", cfg.Server.HTTPHost, cfg.Server.HTTPPort)
	log.Printf("Starting HTTP server on %s", addr)

	if err := http.ListenAndServe(addr, httpSrv); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
