// ABOUTME: End-to-end security test for environment isolation
// ABOUTME: Validates success criterion #1 (no host env leakage)

package e2e

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/harper/acp-relay/internal/config"
	"github.com/harper/acp-relay/internal/container"
)

func TestSecurity_EnvironmentIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Set sensitive host environment variable
	t.Setenv("HOST_SECRET", "very-sensitive-value")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-should-not-leak")
	t.Setenv("TERM", "xterm-256color")

	// Create container session
	cfg := config.ContainerConfig{
		Image:                  "alpine:latest",
		DockerHost:             "unix:///var/run/docker.sock",
		NetworkMode:            "bridge",
		WorkspaceHostBase:      t.TempDir(),
		WorkspaceContainerPath: "/workspace",
		AutoRemove:             false,
	}

	env := map[string]string{
		"TERM":              "xterm-256color",
		"HOST_SECRET":       "very-sensitive-value",
		"ANTHROPIC_API_KEY": "sk-ant-should-not-leak",
	}

	m, err := container.NewManager(cfg, "/bin/sh", []string{"-c", "env && sleep 30"}, env, nil)
	if err != nil {
		t.Skip("Docker not available:", err)
	}

	ctx := context.Background()
	sessionID := "security-test"

	components, err := m.CreateSession(ctx, sessionID, "/workspace")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	defer m.StopContainer(sessionID)

	// Give container time to start
	time.Sleep(2 * time.Second)

	// Execute env command in container
	execCmd := exec.CommandContext(ctx, "docker", "exec", components.ContainerID, "env")
	output, err := execCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to exec in container: %v", err)
	}

	envOutput := string(output)

	// Verify TERM is present (allowed)
	if !strings.Contains(envOutput, "TERM=xterm-256color") {
		t.Error("TERM not found in container (should be allowed)")
	}

	// Verify HOST_SECRET is NOT present (blocked)
	if strings.Contains(envOutput, "HOST_SECRET") {
		t.Error("❌ HOST_SECRET found in container (should be filtered) - SUCCESS CRITERION #1 FAILED")
	} else {
		t.Log("✅ HOST_SECRET not leaked to container")
	}

	// Verify ANTHROPIC_API_KEY is NOT present (blocked)
	if strings.Contains(envOutput, "ANTHROPIC_API_KEY") {
		t.Error("❌ ANTHROPIC_API_KEY found in container (should be filtered) - SUCCESS CRITERION #1 FAILED")
	} else {
		t.Log("✅ ANTHROPIC_API_KEY not leaked to container")
	}

	t.Log("✅ Success criterion #1 met: no host environment leakage")
}
