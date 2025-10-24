// ABOUTME: Tests for container manager, including helper functions and integration tests
// ABOUTME: Covers environment filtering, labeling, naming, and container reuse

package container

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/harper/acp-relay/internal/config"
)

func TestEnvContains(t *testing.T) {
	tests := []struct {
		name     string
		envVars  []string
		key      string
		expected bool
	}{
		{
			name:     "empty list",
			envVars:  []string{},
			key:      "TERM",
			expected: false,
		},
		{
			name:     "key exists",
			envVars:  []string{"TERM=xterm", "LANG=en_US"},
			key:      "TERM",
			expected: true,
		},
		{
			name:     "key does not exist",
			envVars:  []string{"TERM=xterm", "LANG=en_US"},
			key:      "HOME",
			expected: false,
		},
		{
			name:     "partial key match should not match",
			envVars:  []string{"ANTHROPIC_API_KEY=sk-123"},
			key:      "API_KEY",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := envContains(tt.envVars, tt.key)
			if result != tt.expected {
				t.Errorf("envContains(%v, %q) = %v, want %v", tt.envVars, tt.key, result, tt.expected)
			}
		})
	}
}

func TestFilterAllowedEnvVars(t *testing.T) {
	m := &Manager{}

	input := map[string]string{
		"TERM":            "xterm-256color",
		"LANG":            "en_US.UTF-8",
		"LC_ALL":          "en_US.UTF-8",
		"COLORTERM":       "truecolor",
		"HOME":            "/home/user",           // Should be filtered
		"PATH":            "/usr/bin",             // Should be filtered
		"SECRET_API_KEY":  "sensitive",            // Should be filtered
		"ANTHROPIC_API_KEY": "sk-ant-api03-...", // Should be filtered
	}

	result := m.filterAllowedEnvVars(input)

	// Should only keep allowlisted vars
	allowed := []string{"TERM", "LANG", "LC_ALL", "COLORTERM"}
	if len(result) != len(allowed) {
		t.Errorf("filterAllowedEnvVars() returned %d vars, want %d", len(result), len(allowed))
	}

	for _, key := range allowed {
		if _, ok := result[key]; !ok {
			t.Errorf("filterAllowedEnvVars() missing allowed key: %s", key)
		}
	}

	// Should NOT include sensitive vars
	blocked := []string{"HOME", "PATH", "SECRET_API_KEY", "ANTHROPIC_API_KEY"}
	for _, key := range blocked {
		if _, ok := result[key]; ok {
			t.Errorf("filterAllowedEnvVars() included blocked key: %s", key)
		}
	}
}

func TestBuildContainerLabels(t *testing.T) {
	m := &Manager{}
	sessionID := "test-session-123"

	labels := m.buildContainerLabels(sessionID)

	// Should have required labels
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

func TestSanitizeContainerName(t *testing.T) {
	m := &Manager{}

	tests := []struct {
		input string
		want  string
	}{
		{"simple-session", "acp-relay-simple-session"},
		{"session_with_underscore", "acp-relay-session_with_underscore"},
		{"session.with.dots", "acp-relay-session-with-dots"},
		{"SESSION-UPPERCASE", "acp-relay-session-uppercase"},
	}

	for _, tt := range tests {
		got := m.sanitizeContainerName(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeContainerName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFindExistingContainer_NotFound(t *testing.T) {
	// This test requires Docker, skip if not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.ContainerConfig{
		Image:                  "alpine:latest",
		DockerHost:             "unix:///var/run/docker.sock",
		NetworkMode:            "bridge",
		MemoryLimit:            "512m",
		CPULimit:               1.0,
		WorkspaceHostBase:      t.TempDir(),
		WorkspaceContainerPath: "/workspace",
		AutoRemove:             true,
	}

	m, err := NewManager(cfg, "/bin/sh", []string{}, map[string]string{}, nil)
	if err != nil {
		t.Skip("Docker not available:", err)
	}

	ctx := context.Background()
	containerID, err := m.findExistingContainer(ctx, "nonexistent-session")

	if containerID != "" {
		t.Errorf("findExistingContainer() returned %q for nonexistent session", containerID)
	}
	if err != nil {
		t.Errorf("findExistingContainer() returned error: %v", err)
	}
}

func TestContainerReuse_FullFlow(t *testing.T) {
	// Regression test for Error #1 (state management bug)
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.ContainerConfig{
		Image:                  "alpine:latest",
		DockerHost:             "unix:///var/run/docker.sock",
		NetworkMode:            "bridge",
		MemoryLimit:            "512m",
		CPULimit:               1.0,
		WorkspaceHostBase:      t.TempDir(),
		WorkspaceContainerPath: "/workspace",
		AutoRemove:             false, // Keep container for reuse test
	}

	m, err := NewManager(cfg, "/bin/sh", []string{"-c", "sleep 30"}, map[string]string{}, nil)
	if err != nil {
		t.Skip("Docker not available:", err)
	}

	ctx := context.Background()
	sessionID := "reuse-test-session"

	// Create first session
	components1, err := m.CreateSession(ctx, sessionID, "/workspace")
	if err != nil {
		t.Fatalf("First CreateSession failed: %v", err)
	}
	containerID1 := components1.ContainerID

	// Verify container in manager's map
	m.mu.RLock()
	_, exists := m.containers[sessionID]
	m.mu.RUnlock()
	if !exists {
		t.Fatal("Container not in manager's map after creation")
	}

	// Create second session with same ID (should reuse)
	components2, err := m.CreateSession(ctx, sessionID, "/workspace")
	if err != nil {
		t.Fatalf("Second CreateSession failed: %v", err)
	}
	containerID2 := components2.ContainerID

	// Should be same container
	if containerID1 != containerID2 {
		t.Errorf("Container not reused: first=%s, second=%s", containerID1, containerID2)
	}

	// Verify still in manager's map (regression test for Error #1)
	m.mu.RLock()
	_, exists = m.containers[sessionID]
	m.mu.RUnlock()
	if !exists {
		t.Error("Container disappeared from manager's map after reuse (Error #1 regression)")
	}

	// Cleanup
	m.StopContainer(sessionID)
	m.dockerClient.ContainerRemove(ctx, containerID1, container.RemoveOptions{Force: true})
}
