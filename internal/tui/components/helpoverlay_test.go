// ABOUTME: Tests for HelpOverlay component
// ABOUTME: Verifies keyboard shortcut modal overlay display and interaction
package components

import (
	"strings"
	"testing"

	"github.com/harper/acp-relay/internal/tui/theme"
)

func TestNewHelpOverlay(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"small size", 40, 20},
		{"medium size", 80, 24},
		{"large size", 120, 40},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overlay := NewHelpOverlay(tt.width, tt.height, theme.DefaultTheme)

			if overlay == nil {
				t.Fatal("NewHelpOverlay returned nil")
			}

			if overlay.width != tt.width {
				t.Errorf("width = %d, want %d", overlay.width, tt.width)
			}

			if overlay.height != tt.height {
				t.Errorf("height = %d, want %d", overlay.height, tt.height)
			}

			if overlay.visible {
				t.Error("expected overlay to be hidden by default")
			}

			if len(overlay.shortcuts) == 0 {
				t.Error("expected default shortcuts to be populated")
			}

			// Verify default shortcuts are present
			expectedShortcuts := []string{"Tab", "Ctrl+B", "n", "d", "r", "Ctrl+S", "q", "Ctrl+C", "?"}
			foundShortcuts := make(map[string]bool)
			for _, sc := range overlay.shortcuts {
				foundShortcuts[sc.key] = true
			}

			for _, expected := range expectedShortcuts {
				if !foundShortcuts[expected] {
					t.Errorf("missing expected shortcut: %s", expected)
				}
			}
		})
	}
}

func TestHelpOverlay_Show(t *testing.T) {
	overlay := NewHelpOverlay(80, 24, theme.DefaultTheme)

	if overlay.visible {
		t.Error("overlay should start hidden")
	}

	overlay.Show()

	if !overlay.visible {
		t.Error("overlay should be visible after Show()")
	}

	// Multiple Show() calls should be idempotent
	overlay.Show()
	if !overlay.visible {
		t.Error("overlay should still be visible after second Show()")
	}
}

func TestHelpOverlay_Hide(t *testing.T) {
	overlay := NewHelpOverlay(80, 24, theme.DefaultTheme)
	overlay.Show()

	if !overlay.visible {
		t.Error("overlay should be visible after Show()")
	}

	overlay.Hide()

	if overlay.visible {
		t.Error("overlay should be hidden after Hide()")
	}

	// Multiple Hide() calls should be idempotent
	overlay.Hide()
	if overlay.visible {
		t.Error("overlay should still be hidden after second Hide()")
	}
}

func TestHelpOverlay_IsVisible(t *testing.T) {
	overlay := NewHelpOverlay(80, 24, theme.DefaultTheme)

	if overlay.IsVisible() {
		t.Error("IsVisible() should return false initially")
	}

	overlay.Show()
	if !overlay.IsVisible() {
		t.Error("IsVisible() should return true after Show()")
	}

	overlay.Hide()
	if overlay.IsVisible() {
		t.Error("IsVisible() should return false after Hide()")
	}
}

func TestHelpOverlay_Toggle(t *testing.T) {
	overlay := NewHelpOverlay(80, 24, theme.DefaultTheme)

	// Start hidden
	if overlay.IsVisible() {
		t.Error("overlay should start hidden")
	}

	// Toggle to show
	overlay.Toggle()
	if !overlay.IsVisible() {
		t.Error("Toggle() should make overlay visible")
	}

	// Toggle to hide
	overlay.Toggle()
	if overlay.IsVisible() {
		t.Error("Toggle() should hide overlay")
	}

	// Toggle again to show
	overlay.Toggle()
	if !overlay.IsVisible() {
		t.Error("Toggle() should show overlay again")
	}
}

func TestHelpOverlay_View(t *testing.T) {
	overlay := NewHelpOverlay(80, 24, theme.DefaultTheme)

	t.Run("hidden overlay returns empty string", func(t *testing.T) {
		view := overlay.View()
		if view != "" {
			t.Errorf("View() should return empty string when hidden, got: %s", view)
		}
	})

	t.Run("visible overlay renders content", func(t *testing.T) {
		overlay.Show()
		view := overlay.View()

		if view == "" {
			t.Error("View() should return content when visible")
		}

		// Check for title
		if !strings.Contains(view, "Keyboard Shortcuts") {
			t.Error("View() should contain title 'Keyboard Shortcuts'")
		}

		// Check for some expected shortcuts
		expectedContent := []string{
			"Tab",
			"Ctrl+B",
			"Quit",
			"Toggle help",
		}

		for _, expected := range expectedContent {
			if !strings.Contains(view, expected) {
				t.Errorf("View() should contain '%s'", expected)
			}
		}
	})

	t.Run("view after hiding returns empty string", func(t *testing.T) {
		overlay.Show()
		overlay.Hide()
		view := overlay.View()

		if view != "" {
			t.Errorf("View() should return empty string after hiding, got: %s", view)
		}
	})
}

func TestHelpOverlay_SetSize(t *testing.T) {
	overlay := NewHelpOverlay(80, 24, theme.DefaultTheme)

	if overlay.width != 80 {
		t.Errorf("initial width = %d, want 80", overlay.width)
	}
	if overlay.height != 24 {
		t.Errorf("initial height = %d, want 24", overlay.height)
	}

	overlay.SetSize(120, 40)

	if overlay.width != 120 {
		t.Errorf("width after SetSize = %d, want 120", overlay.width)
	}
	if overlay.height != 40 {
		t.Errorf("height after SetSize = %d, want 40", overlay.height)
	}

	// Verify resize works when visible
	overlay.Show()
	overlay.SetSize(100, 30)

	if overlay.width != 100 {
		t.Errorf("width after SetSize while visible = %d, want 100", overlay.width)
	}
	if overlay.height != 30 {
		t.Errorf("height after SetSize while visible = %d, want 30", overlay.height)
	}
}
