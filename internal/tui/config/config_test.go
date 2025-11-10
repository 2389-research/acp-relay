// ABOUTME: Unit tests for TUI configuration loading and validation
// ABOUTME: Tests default config, file loading, validation, and XDG path expansion
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "ws://localhost:8081", cfg.Relay.URL)
	assert.Equal(t, 5, cfg.Relay.ReconnectAttempts)
	assert.Equal(t, "default", cfg.UI.Theme)
	assert.Equal(t, 25, cfg.UI.SidebarWidth)
	assert.True(t, cfg.UI.SidebarDefaultVisible)
}

func TestLoadConfig_NoFile(t *testing.T) {
	// Set XDG to temp dir
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg, err := Load("")
	require.NoError(t, err)

	// Should return defaults
	assert.Equal(t, "default", cfg.UI.Theme)

	// Should create default config file
	configPath := filepath.Join(tmpDir, "acp-tui", "config.yaml")
	_, err = os.Stat(configPath)
	assert.NoError(t, err, "config file should be created")
}

func TestValidate_SidebarWidth(t *testing.T) {
	cfg := DefaultConfig()

	// Too small
	cfg.UI.SidebarWidth = 10
	cfg.Validate()
	assert.Equal(t, 20, cfg.UI.SidebarWidth, "should clamp to 20")

	// Too large
	cfg.UI.SidebarWidth = 50
	cfg.Validate()
	assert.Equal(t, 40, cfg.UI.SidebarWidth, "should clamp to 40")
}
