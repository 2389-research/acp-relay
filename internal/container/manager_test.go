// ABOUTME: Tests for container manager, including helper functions and integration tests
// ABOUTME: Covers environment filtering, labeling, naming, and container reuse

package container

import (
	"context"
	"testing"

	"github.com/harper/acp-relay/internal/config"
)

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
