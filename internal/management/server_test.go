package management

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/harper/acp-relay/internal/config"
	"github.com/harper/acp-relay/internal/session"
)

func TestHealthEndpoint(t *testing.T) {
	cfg := &config.Config{
		Agent: config.AgentConfig{
			Command: "cat",
		},
	}

	mgr := session.NewManager(session.ManagerConfig{
		AgentCommand: cfg.Agent.Command,
	}, nil) // nil db for test

	srv := NewServer(cfg, mgr, nil) // nil db for unit test

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var health map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &health)

	if health["status"] != "healthy" {
		t.Error("expected status healthy")
	}
}

func TestConfigEndpoint(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			HTTPPort:       8080,
			ManagementPort: 8082,
		},
		Agent: config.AgentConfig{
			Command: "cat",
		},
	}

	mgr := session.NewManager(session.ManagerConfig{
		AgentCommand: cfg.Agent.Command,
	}, nil) // nil db for test

	srv := NewServer(cfg, mgr, nil) // nil db for unit test

	req := httptest.NewRequest("GET", "/api/config", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var configResp config.Config
	if err := json.Unmarshal(rec.Body.Bytes(), &configResp); err != nil {
		t.Fatalf("failed to unmarshal config response: %v", err)
	}

	if configResp.Server.HTTPPort != 8080 {
		t.Errorf("expected http_port 8080, got %d", configResp.Server.HTTPPort)
	}

	if configResp.Agent.Command != "cat" {
		t.Errorf("expected agent command 'cat', got %s", configResp.Agent.Command)
	}
}
