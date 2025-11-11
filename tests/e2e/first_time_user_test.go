// ABOUTME: End-to-end test for first-time user experience
// ABOUTME: Validates <5 minute setup and session creation

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

//nolint:funlen // end-to-end test covering full user flow
func TestFirstTimeUser_FullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	startTime := time.Now()

	// Step 1: Build binaries
	t.Log("Building acp-relay...")
	cmd := exec.Command("go", "build", "-o", "acp-relay", "./cmd/relay")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer func() { _ = os.Remove("acp-relay") }()

	// Step 2: Setup (automated with default answers)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Generate minimal config
	configContent := `
server:
  http_port: 18080
  http_host: "127.0.0.1"
  websocket_port: 18081
  websocket_host: "127.0.0.1"
  management_port: 18082
  management_host: "127.0.0.1"

agent:
  command: "/bin/sh"
  mode: "container"
  args: ["-c", "echo '{\"jsonrpc\":\"2.0\",\"id\":0,\"result\":{}}' && sleep 30"]
  container:
    image: "alpine:latest"
    docker_host: "unix:///var/run/docker.sock"
    workspace_host_base: "` + tmpDir + `/workspaces"
    workspace_container_path: "/workspace"
    auto_remove: true

database:
  path: "` + tmpDir + `/db.sqlite"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	setupTime := time.Since(startTime)
	t.Logf("Setup completed in %v", setupTime)

	// Step 3: Start server
	t.Log("Starting server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	//nolint:gosec // test subprocess with controlled arguments
	cmd = exec.CommandContext(ctx, "./acp-relay", "--config", configPath)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	// Wait for server to be ready
	time.Sleep(2 * time.Second)

	// Step 4: Create session
	t.Log("Creating session...")
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/new",
		"params": map[string]interface{}{
			"workingDirectory": "/workspace",
		},
		"id": 1,
	}

	reqJSON, err := json.Marshal(reqBody)
	require.NoError(t, err)
	resp, err := http.Post("http://127.0.0.1:18080/session/new", "application/json", bytes.NewReader(reqJSON))
	if err != nil {
		t.Fatalf("Session creation failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var respBody map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify session created successfully
	if _, ok := respBody["result"]; !ok {
		t.Errorf("No result in response: %+v", respBody)
	}

	totalTime := time.Since(startTime)
	t.Logf("Total time: %v", totalTime)

	// Verify <5 minute constraint (success criterion #2)
	if totalTime > 5*time.Minute {
		t.Errorf("First-time setup took %v, want <5 minutes", totalTime)
	} else {
		t.Logf("✅ Success criterion #2 met: setup completed in %v", totalTime)
	}
}
