// ABOUTME: XDG Base Directory Specification support for Linux/Unix standards
// ABOUTME: Handles config, data, and cache directories with HOME fallback

package xdg

import (
	"os"
	"path/filepath"
	"strings"
)

// ConfigHome returns ~/.config or respects XDG_CONFIG_HOME
func ConfigHome() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return xdgConfig
	}
	home := getHome()
	return filepath.Join(home, ".config")
}

// DataHome returns ~/.local/share/acp-relay
func DataHome() string {
	home := getHome()
	return filepath.Join(home, ".local", "share", "acp-relay")
}

// CacheHome returns ~/.cache/acp-relay
func CacheHome() string {
	home := getHome()
	return filepath.Join(home, ".cache", "acp-relay")
}

// ExpandPath expands $XDG_* variables and ~ in config paths
func ExpandPath(path string) string {
	// Expand tilde
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(getHome(), path[2:])
	}

	// CRITICAL: Use strings.HasPrefix, not filepath.HasPrefix (Error #3 fix)
	if strings.HasPrefix(path, "$XDG_DATA_HOME") {
		return strings.Replace(path, "$XDG_DATA_HOME", DataHome(), 1)
	}
	if strings.HasPrefix(path, "$XDG_CONFIG_HOME") {
		return strings.Replace(path, "$XDG_CONFIG_HOME", ConfigHome(), 1)
	}
	if strings.HasPrefix(path, "$XDG_CACHE_HOME") {
		return strings.Replace(path, "$XDG_CACHE_HOME", CacheHome(), 1)
	}

	// Non-XDG paths pass through unchanged
	return path
}

// getHome returns HOME with fallback chain (Error #2 fix)
func getHome() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}

	// Fallback to current directory if HOME not set
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}

	// Last resort
	return "."
}
