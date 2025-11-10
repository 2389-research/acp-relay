// ABOUTME: TUI configuration system with XDG-compliant file loading
// ABOUTME: Handles config loading, validation, defaults, and theme selection
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/harper/acp-relay/internal/xdg"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Relay       RelayConfig       `yaml:"relay"`
	UI          UIConfig          `yaml:"ui"`
	Input       InputConfig       `yaml:"input"`
	Sessions    SessionsConfig    `yaml:"sessions"`
	Keybindings KeybindingsConfig `yaml:"keybindings"`
	Logging     LoggingConfig     `yaml:"logging"`
}

type RelayConfig struct {
	URL               string `yaml:"url"`
	ReconnectAttempts int    `yaml:"reconnect_attempts"`
	TimeoutSeconds    int    `yaml:"timeout_seconds"`
}

type UIConfig struct {
	Theme                 string `yaml:"theme"`
	SidebarWidth          int    `yaml:"sidebar_width"`
	SidebarDefaultVisible bool   `yaml:"sidebar_default_visible"`
	ChatHistoryLimit      int    `yaml:"chat_history_limit"`
}

type InputConfig struct {
	MultilineMinHeight int  `yaml:"multiline_min_height"`
	MultilineMaxHeight int  `yaml:"multiline_max_height"`
	SendOnEnter        bool `yaml:"send_on_enter"`
	VimMode            bool `yaml:"vim_mode"`
}

type SessionsConfig struct {
	DefaultWorkingDir   string `yaml:"default_working_dir"`
	AutoCreateWorkspace bool   `yaml:"auto_create_workspace"`
}

type KeybindingsConfig struct {
	ToggleSidebar string `yaml:"toggle_sidebar"`
	NewSession    string `yaml:"new_session"`
	DeleteSession string `yaml:"delete_session"`
	RenameSession string `yaml:"rename_session"`
	SendMessage   string `yaml:"send_message"`
	Quit          string `yaml:"quit"`
	Help          string `yaml:"help"`
}

type LoggingConfig struct {
	Enabled bool   `yaml:"enabled"`
	Level   string `yaml:"level"`
	File    string `yaml:"file"`
}

func DefaultConfig() *Config {
	return &Config{
		Relay: RelayConfig{
			URL:               "ws://localhost:8081",
			ReconnectAttempts: 5,
			TimeoutSeconds:    30,
		},
		UI: UIConfig{
			Theme:                 "default",
			SidebarWidth:          25,
			SidebarDefaultVisible: true,
			ChatHistoryLimit:      1000,
		},
		Input: InputConfig{
			MultilineMinHeight: 3,
			MultilineMaxHeight: 10,
			SendOnEnter:        false,
			VimMode:            false,
		},
		Sessions: SessionsConfig{
			DefaultWorkingDir:   "~/acp-workspaces",
			AutoCreateWorkspace: true,
		},
		Keybindings: KeybindingsConfig{
			ToggleSidebar: "ctrl+b",
			NewSession:    "n",
			DeleteSession: "d",
			RenameSession: "r",
			SendMessage:   "ctrl+s",
			Quit:          "ctrl+c",
			Help:          "?",
		},
		Logging: LoggingConfig{
			Enabled: true,
			Level:   "info",
			File:    "~/.local/share/acp-tui/tui.log",
		},
	}
}

func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	// Determine config file location
	if configPath == "" {
		configPath = filepath.Join(xdg.ConfigHome(), "acp-tui", "config.yaml")
	}

	// If file doesn't exist, create it with defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := saveDefault(cfg, configPath); err != nil {
			// Log warning but continue with defaults
			return cfg, nil
		}
		return cfg, nil
	}

	// Load existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.Validate()
	return cfg, nil
}

func (c *Config) Validate() {
	// Clamp sidebar width
	if c.UI.SidebarWidth < 20 {
		c.UI.SidebarWidth = 20
	}
	if c.UI.SidebarWidth > 40 {
		c.UI.SidebarWidth = 40
	}

	// Clamp chat history
	if c.UI.ChatHistoryLimit < 100 {
		c.UI.ChatHistoryLimit = 100
	}
	if c.UI.ChatHistoryLimit > 10000 {
		c.UI.ChatHistoryLimit = 10000
	}

	// Expand ~ in paths
	c.Sessions.DefaultWorkingDir = xdg.ExpandPath(c.Sessions.DefaultWorkingDir)
	c.Logging.File = xdg.ExpandPath(c.Logging.File)
}

func saveDefault(cfg *Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
