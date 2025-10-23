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
	err := os.WriteFile("test_env_case.yaml", []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("test_env_case.yaml")

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
