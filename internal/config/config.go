// ABOUTME: Configuration loading and management for ACP relay server
// ABOUTME: Supports YAML files and environment variable overrides

package config

import (
	"fmt"
	"os"

	"github.com/harper/acp-relay/internal/xdg"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Agent    AgentConfig    `mapstructure:"agent"`
	Database DatabaseConfig `mapstructure:"database"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type ServerConfig struct {
	HTTPPort       int    `mapstructure:"http_port"`
	HTTPHost       string `mapstructure:"http_host"`
	WebSocketPort  int    `mapstructure:"websocket_port"`
	WebSocketHost  string `mapstructure:"websocket_host"`
	ManagementPort int    `mapstructure:"management_port"`
	ManagementHost string `mapstructure:"management_host"`
}

type AgentConfig struct {
	Command               string            `mapstructure:"command"`
	Mode                  string            `mapstructure:"mode"` // "process" or "container"
	Args                  []string          `mapstructure:"args"`
	Env                   map[string]string `mapstructure:"env"`
	Container             ContainerConfig   `mapstructure:"container"`
	StartupTimeoutSeconds int               `mapstructure:"startup_timeout_seconds"`
	MaxConcurrentSessions int               `mapstructure:"max_concurrent_sessions"`
}

type ContainerConfig struct {
	Image                  string  `mapstructure:"image"`
	DockerHost             string  `mapstructure:"docker_host"`
	NetworkMode            string  `mapstructure:"network_mode"`
	MemoryLimit            string  `mapstructure:"memory_limit"`
	CPULimit               float64 `mapstructure:"cpu_limit"`
	WorkspaceHostBase      string  `mapstructure:"workspace_host_base"`
	WorkspaceContainerPath string  `mapstructure:"workspace_container_path"`
	AutoRemove             bool    `mapstructure:"auto_remove"`
	StartupTimeoutSeconds  int     `mapstructure:"startup_timeout_seconds"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// IMPORTANT: Viper lowercases all map keys, but environment variables are case-sensitive
	// Parse YAML directly to preserve original key case for agent.env
	//nolint:gosec // config file path from validated user input
	data, err := os.ReadFile(path)
	if err == nil {
		var rawConfig struct {
			Agent struct {
				Env map[string]string `yaml:"env"`
			} `yaml:"agent"`
		}
		if yaml.Unmarshal(data, &rawConfig) == nil && len(rawConfig.Agent.Env) > 0 {
			cfg.Agent.Env = rawConfig.Agent.Env
		}
	}

	// ENHANCEMENT: Expand XDG variables in database path
	cfg.Database.Path = xdg.ExpandPath(cfg.Database.Path)

	// Default to process mode if not specified
	if cfg.Agent.Mode == "" {
		cfg.Agent.Mode = "process"
	}

	// Validate mode
	if cfg.Agent.Mode != "process" && cfg.Agent.Mode != "container" {
		return nil, fmt.Errorf("invalid agent.mode: %s (must be 'process' or 'container')", cfg.Agent.Mode)
	}

	return &cfg, nil
}
