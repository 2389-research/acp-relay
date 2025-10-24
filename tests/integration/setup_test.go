// ABOUTME: Integration tests for setup subcommand
// ABOUTME: Validates config generation and XDG path handling

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/harper/acp-relay/internal/config"
	"github.com/harper/acp-relay/internal/runtime"
	"github.com/harper/acp-relay/internal/xdg"
)

func TestSetup_ConfigGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temp directory for config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Simulate setup by detecting runtime and generating config
	best := runtime.DetectBest()
	if best == nil {
		t.Skip("No runtime available")
	}

	dataPath := filepath.Join(tmpDir, "data")
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
  mode: "container"
  container:
    image: "alpine:latest"
    docker_host: "unix://` + best.SocketPath + `"
    workspace_host_base: "` + dataPath + `/workspaces"
    workspace_container_path: "/workspace"

database:
  path: "` + dataPath + `/db.sqlite"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Load config and verify
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load generated config: %v", err)
	}

	// Verify runtime settings
	if cfg.Agent.Container.DockerHost != "unix://"+best.SocketPath {
		t.Errorf("DockerHost = %q, want %q", cfg.Agent.Container.DockerHost, "unix://"+best.SocketPath)
	}

	// Verify paths
	expectedDB := filepath.Join(dataPath, "db.sqlite")
	if cfg.Database.Path != expectedDB {
		t.Errorf("Database.Path = %q, want %q", cfg.Database.Path, expectedDB)
	}
}

func TestSetup_XDGPaths(t *testing.T) {
	// Verify XDG functions return expected paths
	configHome := xdg.ConfigHome()
	if !filepath.IsAbs(configHome) {
		t.Errorf("ConfigHome() returned relative path: %q", configHome)
	}

	dataHome := xdg.DataHome()
	if !filepath.IsAbs(dataHome) {
		t.Errorf("DataHome() returned relative path: %q", dataHome)
	}

	cacheHome := xdg.CacheHome()
	if !filepath.IsAbs(cacheHome) {
		t.Errorf("CacheHome() returned relative path: %q", cacheHome)
	}
}
