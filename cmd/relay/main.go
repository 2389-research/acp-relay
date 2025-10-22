// ABOUTME: Main entry point for ACP relay server
// ABOUTME: Loads configuration and starts HTTP/WebSocket servers

package main

import (
	"flag"
	"log"

	"github.com/harper/acp-relay/internal/config"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Printf("loaded config: http_port=%d, ws_port=%d",
		cfg.Server.HTTPPort, cfg.Server.WebSocketPort)
}
