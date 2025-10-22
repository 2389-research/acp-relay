package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create test config file
	content := `
server:
  http_port: 8080
  websocket_port: 8081
agent:
  command: "/usr/local/bin/test-agent"
`
	err := os.WriteFile("test_config.yaml", []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("test_config.yaml")

	cfg, err := Load("test_config.yaml")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Server.HTTPPort != 8080 {
		t.Errorf("expected http_port 8080, got %d", cfg.Server.HTTPPort)
	}

	if cfg.Agent.Command != "/usr/local/bin/test-agent" {
		t.Errorf("expected agent command '/usr/local/bin/test-agent', got %s", cfg.Agent.Command)
	}
}
