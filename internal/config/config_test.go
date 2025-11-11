package config

import (
	"os"
	"path/filepath"
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
	err := os.WriteFile("test_config.yaml", []byte(content), 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove("test_config.yaml") }()

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

func TestLoadConfig_EnvCasePreservation(t *testing.T) {
	// Create test config with uppercase env var keys
	content := `
server:
  http_port: 8080
agent:
  mode: "container"
  env:
    ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
    HOME: "${HOME}"
    PATH: "${PATH}"
    lowercase_var: "test"
    MixedCase_Var: "test2"
`
	err := os.WriteFile("test_env_case.yaml", []byte(content), 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove("test_env_case.yaml") }()

	cfg, err := Load("test_env_case.yaml")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify that env var keys preserve their original case from YAML
	expectedKeys := map[string]bool{
		"ANTHROPIC_API_KEY": true,
		"HOME":              true,
		"PATH":              true,
		"lowercase_var":     true,
		"MixedCase_Var":     true,
	}

	if len(cfg.Agent.Env) != len(expectedKeys) {
		t.Errorf("expected %d env vars, got %d", len(expectedKeys), len(cfg.Agent.Env))
	}

	for key := range expectedKeys {
		if _, exists := cfg.Agent.Env[key]; !exists {
			t.Errorf("expected key %q to exist with exact case, but it doesn't", key)
		}
	}

	// Verify values are correct
	if cfg.Agent.Env["ANTHROPIC_API_KEY"] != "${ANTHROPIC_API_KEY}" {
		t.Errorf("expected ANTHROPIC_API_KEY value '${ANTHROPIC_API_KEY}', got %q", cfg.Agent.Env["ANTHROPIC_API_KEY"])
	}
}

func TestLoad_XDGExpansion(t *testing.T) {
	// Create temp config with XDG variable
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  http_port: 8080
  http_host: "0.0.0.0"
  websocket_port: 8081
  websocket_host: "0.0.0.0"
  management_port: 8082
  management_host: "127.0.0.1"

agent:
  command: "/bin/echo"
  mode: "process"

database:
  path: "$XDG_DATA_HOME/db.sqlite"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Should NOT contain literal $XDG_DATA_HOME
	if cfg.Database.Path == "$XDG_DATA_HOME/db.sqlite" {
		t.Error("XDG variable not expanded in database path")
	}

	// Should contain actual path
	home := os.Getenv("HOME")
	expectedPath := filepath.Join(home, ".local", "share", "acp-relay", "db.sqlite")
	if cfg.Database.Path != expectedPath {
		t.Errorf("Database.Path = %q, want %q", cfg.Database.Path, expectedPath)
	}
}

func TestLoad_NonXDGPathUnchanged(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  http_port: 8080
  http_host: "0.0.0.0"
  websocket_port: 8081
  websocket_host: "0.0.0.0"
  management_port: 8082
  management_host: "127.0.0.1"

agent:
  command: "/bin/echo"
  mode: "process"

database:
  path: "/absolute/path/db.sqlite"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Should remain unchanged
	if cfg.Database.Path != "/absolute/path/db.sqlite" {
		t.Errorf("Non-XDG path was modified: %q", cfg.Database.Path)
	}
}
