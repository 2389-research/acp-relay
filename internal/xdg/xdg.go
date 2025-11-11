// ABOUTME: XDG Base Directory Specification support for Linux/Unix standards
// ABOUTME: Handles config, data, and cache directories with HOME fallback

package xdg

import (
	"os"
	"path/filepath"
	"strings"
)

// ConfigHome returns ~/.config/acp-relay or respects XDG_CONFIG_HOME.
func ConfigHome() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "acp-relay")
	}
	home := getHome()
	return filepath.Join(home, ".config", "acp-relay")
}

// DataHome returns ~/.local/share/acp-relay or respects XDG_DATA_HOME.
func DataHome() string {
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, "acp-relay")
	}
	home := getHome()
	return filepath.Join(home, ".local", "share", "acp-relay")
}

// CacheHome returns ~/.cache/acp-relay or respects XDG_CACHE_HOME.
func CacheHome() string {
	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		return filepath.Join(xdgCache, "acp-relay")
	}
	home := getHome()
	return filepath.Join(home, ".cache", "acp-relay")
}

// TUIDataHome returns ~/.local/share/acp-tui or respects XDG_DATA_HOME.
func TUIDataHome() string {
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, "acp-tui")
	}
	home := getHome()
	return filepath.Join(home, ".local", "share", "acp-tui")
}

// ExpandPath expands $XDG_* variables and ~ in config paths.
func ExpandPath(path string) string {
	// Expand tilde
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(getHome(), path[2:])
	}

	// CRITICAL: Use strings.HasPrefix, not filepath.HasPrefix (Error #3 fix)
	// Expand generic XDG variables to their base directories (not app-specific)
	if strings.HasPrefix(path, "$XDG_DATA_HOME") {
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData == "" {
			xdgData = filepath.Join(getHome(), ".local", "share")
		}
		return strings.Replace(path, "$XDG_DATA_HOME", xdgData, 1)
	}
	if strings.HasPrefix(path, "$XDG_CONFIG_HOME") {
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig == "" {
			xdgConfig = filepath.Join(getHome(), ".config")
		}
		return strings.Replace(path, "$XDG_CONFIG_HOME", xdgConfig, 1)
	}
	if strings.HasPrefix(path, "$XDG_CACHE_HOME") {
		xdgCache := os.Getenv("XDG_CACHE_HOME")
		if xdgCache == "" {
			xdgCache = filepath.Join(getHome(), ".cache")
		}
		return strings.Replace(path, "$XDG_CACHE_HOME", xdgCache, 1)
	}

	// Non-XDG paths pass through unchanged
	return path
}

// getHome returns HOME with fallback chain (Error #2 fix).
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
