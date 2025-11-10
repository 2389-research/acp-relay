// ABOUTME: Tests for XDG Base Directory Specification support
// ABOUTME: Includes regression tests for HOME variable handling

package xdg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigHome(t *testing.T) {
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("HOME not set")
	}

	got := ConfigHome()
	want := filepath.Join(home, ".config", "acp-relay")

	if got != want {
		t.Errorf("ConfigHome() = %q, want %q", got, want)
	}
}

func TestConfigHome_WithEnv(t *testing.T) {
	// Test XDG_CONFIG_HOME environment variable
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		if oldXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	testPath := "/tmp/custom-config"
	os.Setenv("XDG_CONFIG_HOME", testPath)

	got := ConfigHome()
	want := filepath.Join(testPath, "acp-relay")
	if got != want {
		t.Errorf("ConfigHome() with XDG_CONFIG_HOME = %q, want %q", got, want)
	}
}

func TestDataHome(t *testing.T) {
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("HOME not set")
	}

	got := DataHome()
	want := filepath.Join(home, ".local", "share", "acp-relay")

	if got != want {
		t.Errorf("DataHome() = %q, want %q", got, want)
	}
}

func TestCacheHome(t *testing.T) {
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("HOME not set")
	}

	got := CacheHome()
	want := filepath.Join(home, ".cache", "acp-relay")

	if got != want {
		t.Errorf("CacheHome() = %q, want %q", got, want)
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "XDG_DATA_HOME variable",
			input: "$XDG_DATA_HOME/db.sqlite",
			want:  filepath.Join(DataHome(), "db.sqlite"),
		},
		{
			name:  "XDG_CONFIG_HOME variable",
			input: "$XDG_CONFIG_HOME/config.yaml",
			want:  filepath.Join(ConfigHome(), "config.yaml"),
		},
		{
			name:  "XDG_CACHE_HOME variable",
			input: "$XDG_CACHE_HOME/cache.db",
			want:  filepath.Join(CacheHome(), "cache.db"),
		},
		{
			name:  "non-XDG path passes through",
			input: "/absolute/path/to/file",
			want:  "/absolute/path/to/file",
		},
		{
			name:  "relative path passes through",
			input: "relative/path/to/file",
			want:  "relative/path/to/file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandPath(tt.input)
			if got != tt.want {
				t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExpandPath_MissingHOME(t *testing.T) {
	// Regression test for Error #2 from previous implementation
	oldHome := os.Getenv("HOME")
	os.Unsetenv("HOME")
	defer os.Setenv("HOME", oldHome)

	// Should fall back to current directory
	got := ExpandPath("$XDG_DATA_HOME/db.sqlite")

	// Should not create path at root
	if filepath.IsAbs(got) && filepath.Dir(got) == "/" {
		t.Errorf("ExpandPath with missing HOME created root path: %q", got)
	}
}

func TestExpandPath_StringPrefix(t *testing.T) {
	// Regression test for Error #3 from previous implementation
	// Must use strings.HasPrefix, not filepath.HasPrefix
	input := "$XDG_DATA_HOME/db.sqlite"
	got := ExpandPath(input)

	// Should detect $XDG_* prefix correctly
	if got == input {
		t.Errorf("ExpandPath(%q) did not expand, returned %q", input, got)
	}
}
