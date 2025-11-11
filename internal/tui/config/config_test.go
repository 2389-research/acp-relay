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

	assert.Equal(t, "ws://localhost:23891", cfg.Relay.URL)
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

func TestLoadConfig_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create a custom config file
	configContent := `relay:
  url: "ws://custom-relay:9000"
  reconnect_attempts: 7
  timeout_seconds: 60
ui:
  theme: "dark"
  sidebar_width: 30
  sidebar_default_visible: false
  chat_history_limit: 500
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	// Load the config
	cfg, err := Load(configPath)
	require.NoError(t, err)

	// Verify custom values were loaded
	assert.Equal(t, "ws://custom-relay:9000", cfg.Relay.URL)
	assert.Equal(t, 7, cfg.Relay.ReconnectAttempts)
	assert.Equal(t, 60, cfg.Relay.TimeoutSeconds)
	assert.Equal(t, "dark", cfg.UI.Theme)
	assert.Equal(t, 30, cfg.UI.SidebarWidth)
	assert.False(t, cfg.UI.SidebarDefaultVisible)
	assert.Equal(t, 500, cfg.UI.ChatHistoryLimit)
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create malformed YAML
	invalidYAML := `relay:
  url: "ws://localhost:8081
  reconnect_attempts: not-a-number
ui:
    theme: [unclosed array
`
	require.NoError(t, os.WriteFile(configPath, []byte(invalidYAML), 0644))

	// Load should return error
	_, err := Load(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse config")
}

func TestValidate_ChatHistoryLimit(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"below minimum", 50, 100},
		{"at minimum", 100, 100},
		{"in range", 5000, 5000},
		{"at maximum", 10000, 10000},
		{"above maximum", 15000, 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.UI.ChatHistoryLimit = tt.input
			cfg.Validate()
			assert.Equal(t, tt.expected, cfg.UI.ChatHistoryLimit)
		})
	}
}

func TestValidate_MultilineHeights(t *testing.T) {
	tests := []struct {
		name        string
		minHeight   int
		maxHeight   int
		expectedMin int
		expectedMax int
	}{
		{"both valid", 3, 10, 3, 10},
		{"min too small", 0, 10, 1, 10},
		{"max too small", 3, 0, 1, 1},
		{"min > max", 10, 5, 5, 5},
		{"both zero", 0, 0, 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Input.MultilineMinHeight = tt.minHeight
			cfg.Input.MultilineMaxHeight = tt.maxHeight
			cfg.Validate()
			assert.Equal(t, tt.expectedMin, cfg.Input.MultilineMinHeight)
			assert.Equal(t, tt.expectedMax, cfg.Input.MultilineMaxHeight)
		})
	}
}

func TestValidate_ReconnectAttempts(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"below minimum", 0, 1},
		{"at minimum", 1, 1},
		{"in range", 5, 5},
		{"at maximum", 10, 10},
		{"above maximum", 20, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Relay.ReconnectAttempts = tt.input
			cfg.Validate()
			assert.Equal(t, tt.expected, cfg.Relay.ReconnectAttempts)
		})
	}
}

func TestValidate_TimeoutSeconds(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"below minimum", 2, 5},
		{"at minimum", 5, 5},
		{"in range", 60, 60},
		{"at maximum", 300, 300},
		{"above maximum", 500, 300},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Relay.TimeoutSeconds = tt.input
			cfg.Validate()
			assert.Equal(t, tt.expected, cfg.Relay.TimeoutSeconds)
		})
	}
}

func TestValidate_Keybindings(t *testing.T) {
	cfg := DefaultConfig()

	// Set all keybindings to empty
	cfg.Keybindings.ToggleSidebar = ""
	cfg.Keybindings.NewSession = ""
	cfg.Keybindings.DeleteSession = ""
	cfg.Keybindings.RenameSession = ""
	cfg.Keybindings.SendMessage = ""
	cfg.Keybindings.Quit = ""
	cfg.Keybindings.Help = ""

	cfg.Validate()

	// Should restore defaults
	assert.Equal(t, "ctrl+b", cfg.Keybindings.ToggleSidebar)
	assert.Equal(t, "n", cfg.Keybindings.NewSession)
	assert.Equal(t, "d", cfg.Keybindings.DeleteSession)
	assert.Equal(t, "r", cfg.Keybindings.RenameSession)
	assert.Equal(t, "ctrl+s", cfg.Keybindings.SendMessage)
	assert.Equal(t, "ctrl+c", cfg.Keybindings.Quit)
	assert.Equal(t, "?", cfg.Keybindings.Help)
}

func TestValidate_LogLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid debug", "debug", "debug"},
		{"valid info", "info", "info"},
		{"valid warn", "warn", "warn"},
		{"valid error", "error", "error"},
		{"invalid level", "trace", "info"},
		{"empty string", "", "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Logging.Level = tt.input
			cfg.Validate()
			assert.Equal(t, tt.expected, cfg.Logging.Level)
		})
	}
}
