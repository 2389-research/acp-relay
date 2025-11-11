// ABOUTME: Integration tests for container lifecycle management
// ABOUTME: Validates container labels and environment filtering

package integration

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/harper/acp-relay/internal/config"
	"github.com/harper/acp-relay/internal/container"
)

//nolint:funlen // container test with label validation
func TestContainerLabels_Present(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create manager and session
	cfg := config.ContainerConfig{
		Image:                  "alpine:latest",
		DockerHost:             "unix:///var/run/docker.sock",
		NetworkMode:            "bridge",
		WorkspaceHostBase:      t.TempDir(),
		WorkspaceContainerPath: "/workspace",
		AutoRemove:             false,
	}

	m, err := container.NewManager(cfg, "/bin/sh", []string{"-c", "sleep 30"}, map[string]string{}, nil)
	if err != nil {
		t.Skip("Docker not available:", err)
	}

	ctx := context.Background()
	sessionID := "labels-test-session"

	components, err := m.CreateSession(ctx, sessionID, "/workspace")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	defer func() { _ = m.StopContainer(sessionID) }()

	// Inspect container to verify labels using docker CLI
	//nolint:gosec // test docker inspect command with controlled container ID
	cmd := exec.Command("docker", "inspect", components.ContainerID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker inspect failed: %v, output: %s", err, output)
	}

	// Parse inspect output
	var inspectData []map[string]interface{}
	if err := json.Unmarshal(output, &inspectData); err != nil {
		t.Fatalf("Failed to parse inspect output: %v", err)
	}

	if len(inspectData) == 0 {
		t.Fatal("No inspect data returned")
	}

	// Get labels
	config, ok := inspectData[0]["Config"].(map[string]interface{})
	if !ok {
		t.Fatal("No Config in inspect data")
	}

	labelsRaw, ok := config["Labels"].(map[string]interface{})
	if !ok {
		t.Fatal("No Labels in Config")
	}

	// Convert to string map
	labels := make(map[string]string)
	for k, v := range labelsRaw {
		if str, ok := v.(string); ok {
			labels[k] = str
		}
	}

	// Verify labels
	if labels["managed-by"] != "acp-relay" {
		t.Errorf("labels[managed-by] = %q, want %q", labels["managed-by"], "acp-relay")
	}
	if labels["session-id"] != sessionID {
		t.Errorf("labels[session-id] = %q, want %q", labels["session-id"], sessionID)
	}
	if labels["created-at"] == "" {
		t.Error("labels[created-at] is empty")
	}
}

//nolint:funlen // test with extensive environment validation
func TestEnvironmentFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create manager with mixed environment variables
	cfg := config.ContainerConfig{
		Image:                  "alpine:latest",
		DockerHost:             "unix:///var/run/docker.sock",
		NetworkMode:            "bridge",
		WorkspaceHostBase:      t.TempDir(),
		WorkspaceContainerPath: "/workspace",
		AutoRemove:             false,
	}

	env := map[string]string{
		"TERM":       "xterm-256color",
		"LANG":       "en_US.UTF-8",
		"HOME":       "/home/user", // Should be filtered
		"SECRET_KEY": "sensitive",  // Should be filtered
	}

	m, err := container.NewManager(cfg, "/bin/sh", []string{"-c", "env && sleep 30"}, env, nil)
	if err != nil {
		t.Skip("Docker not available:", err)
	}

	ctx := context.Background()
	sessionID := "env-filter-test"

	components, err := m.CreateSession(ctx, sessionID, "/workspace")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	defer func() { _ = m.StopContainer(sessionID) }()

	//nolint:gosec // test docker inspect command with controlled container ID
	// Inspect container to verify env using docker CLI
	cmd := exec.Command("docker", "inspect", components.ContainerID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker inspect failed: %v, output: %s", err, output)
	}

	// Parse inspect output
	var inspectData []map[string]interface{}
	if err := json.Unmarshal(output, &inspectData); err != nil {
		t.Fatalf("Failed to parse inspect output: %v", err)
	}

	if len(inspectData) == 0 {
		t.Fatal("No inspect data returned")
	}

	// Get env
	configData, ok := inspectData[0]["Config"].(map[string]interface{})
	if !ok {
		t.Fatal("No Config in inspect data")
	}

	envRaw, ok := configData["Env"].([]interface{})
	if !ok {
		t.Fatal("No Env in Config")
	}

	// Parse environment
	envMap := make(map[string]string)
	for _, envVar := range envRaw {
		if envStr, ok := envVar.(string); ok {
			parts := strings.SplitN(envStr, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			}
		}
	}

	// Verify allowed vars present
	if _, ok := envMap["TERM"]; !ok {
		t.Error("TERM not in container environment (should be allowed)")
	}
	if _, ok := envMap["LANG"]; !ok {
		t.Error("LANG not in container environment (should be allowed)")
	}

	// Verify blocked vars absent
	if _, ok := envMap["HOME"]; ok {
		t.Error("HOME in container environment (should be filtered)")
	}
	if _, ok := envMap["SECRET_KEY"]; ok {
		t.Error("SECRET_KEY in container environment (should be filtered)")
	}
}
