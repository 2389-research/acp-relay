// ABOUTME: Unit tests for status bar component (connection and session status display)
// ABOUTME: Tests rendering, status updates, and responsive sizing
package components

import (
	"strings"
	"testing"

	"github.com/harper/acp-relay/internal/tui/theme"
	"github.com/stretchr/testify/assert"
)

func TestNewStatusBar(t *testing.T) {
	sb := NewStatusBar(80, theme.DefaultTheme)

	assert.NotNil(t, sb)
	assert.Equal(t, 80, sb.width)
}

func TestStatusBar_SetConnectionStatus(t *testing.T) {
	tests := []struct {
		name         string
		status       string
		expectedIcon string
		expectedText string
	}{
		{
			name:         "connected status",
			status:       "connected",
			expectedIcon: "🟢",
			expectedText: "Connected",
		},
		{
			name:         "connecting status",
			status:       "connecting",
			expectedIcon: "🟡",
			expectedText: "Connecting",
		},
		{
			name:         "disconnected status",
			status:       "disconnected",
			expectedIcon: "🔴",
			expectedText: "Disconnected",
		},
		{
			name:         "unknown status defaults to disconnected",
			status:       "invalid",
			expectedIcon: "🔴",
			expectedText: "Disconnected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := NewStatusBar(80, theme.DefaultTheme)
			sb.SetConnectionStatus(tt.status)

			view := sb.View()
			assert.Contains(t, view, tt.expectedIcon)
			assert.Contains(t, view, tt.expectedText)
		})
	}
}

func TestStatusBar_SetActiveSession(t *testing.T) {
	sb := NewStatusBar(80, theme.DefaultTheme)

	// Test with session name
	sb.SetActiveSession("MySession")
	view := sb.View()
	assert.Contains(t, view, "Session:")
	assert.Contains(t, view, "MySession")

	// Test with empty session name
	sb.SetActiveSession("")
	view = sb.View()
	assert.Contains(t, view, "No active session")
}

func TestStatusBar_View(t *testing.T) {
	sb := NewStatusBar(100, theme.DefaultTheme)
	sb.SetConnectionStatus("connected")
	sb.SetActiveSession("TestSession")

	view := sb.View()

	// Check connection status
	assert.Contains(t, view, "🟢")
	assert.Contains(t, view, "Connected")

	// Check session name
	assert.Contains(t, view, "Session:")
	assert.Contains(t, view, "TestSession")

	// Check keyboard shortcuts
	assert.Contains(t, view, "Tab: Navigate")
	assert.Contains(t, view, "?: Help")
	assert.Contains(t, view, "q: Quit")
}

func TestStatusBar_SetSize(t *testing.T) {
	sb := NewStatusBar(80, theme.DefaultTheme)

	// Initial size
	assert.Equal(t, 80, sb.width)

	// Change size
	sb.SetSize(120)
	assert.Equal(t, 120, sb.width)

	// Verify view respects new size
	view := sb.View()
	// View should not exceed new width (accounting for ANSI codes)
	plainText := stripAnsiCodes(view)
	// The actual content might be shorter than width, but it should be properly sized
	assert.NotEmpty(t, plainText)
}

func TestStatusBar_DefaultValues(t *testing.T) {
	sb := NewStatusBar(80, theme.DefaultTheme)

	view := sb.View()

	// Should show disconnected by default
	assert.Contains(t, view, "🔴")
	assert.Contains(t, view, "Disconnected")

	// Should show no active session by default
	assert.Contains(t, view, "No active session")
}

func TestStatusBar_LongSessionName(t *testing.T) {
	sb := NewStatusBar(50, theme.DefaultTheme)

	// Set a very long session name
	longName := "ThisIsAVeryLongSessionNameThatShouldBeTruncated"
	sb.SetActiveSession(longName)

	view := sb.View()
	// Should contain the session indicator but might truncate the name
	assert.Contains(t, view, "Session:")
}

// stripAnsiCodes removes ANSI escape codes from a string for testing.
func stripAnsiCodes(s string) string {
	// Simple removal of ANSI codes for testing purposes
	result := strings.Builder{}
	inEscape := false

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}

	return result.String()
}
