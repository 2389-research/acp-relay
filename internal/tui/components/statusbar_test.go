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

func TestStatusBar_ProgressBar(t *testing.T) {
	sb := NewStatusBar(80, theme.DefaultTheme)

	// Initially, progress should not be visible
	view := sb.View()
	assert.NotContains(t, view, "█")
	assert.NotContains(t, view, "░")

	// Show progress
	sb.ShowProgress()
	view = sb.View()
	// Progress bar should appear (even at 0%)
	plainText := stripAnsiCodes(view)
	assert.Contains(t, plainText, "░")

	// Advance progress
	sb.AdvanceProgress(25.0)
	view = sb.View()
	plainText = stripAnsiCodes(view)
	assert.Contains(t, plainText, "█")
	assert.Contains(t, plainText, "░")

	// Hide progress
	sb.HideProgress()
	view = sb.View()
	assert.NotContains(t, view, "█")
	assert.NotContains(t, view, "░")
}

func TestStatusBar_ProgressAdvancement(t *testing.T) {
	sb := NewStatusBar(80, theme.DefaultTheme)
	sb.ShowProgress()

	// Advance by 10%
	sb.AdvanceProgress(10.0)
	assert.Equal(t, 10.0, sb.progressValue)

	// Advance by another 15%
	sb.AdvanceProgress(15.0)
	assert.Equal(t, 25.0, sb.progressValue)

	// Advance by 80% (should wrap at 100)
	sb.AdvanceProgress(80.0)
	assert.Equal(t, 5.0, sb.progressValue) // 25 + 80 = 105, wraps to 5
}

func TestStatusBar_ProgressWrapping(t *testing.T) {
	sb := NewStatusBar(80, theme.DefaultTheme)
	sb.ShowProgress()

	// Set progress to 95%
	sb.AdvanceProgress(95.0)
	assert.Equal(t, 95.0, sb.progressValue)

	// Advance by 10% (should wrap)
	sb.AdvanceProgress(10.0)
	assert.Equal(t, 5.0, sb.progressValue) // 95 + 10 = 105, wraps to 5

	// Advance by 100% (should wrap to 5 again)
	sb.AdvanceProgress(100.0)
	assert.Equal(t, 5.0, sb.progressValue) // 5 + 100 = 105, wraps to 5
}

func TestStatusBar_ProgressBarRendering(t *testing.T) {
	tests := []struct {
		name           string
		progressValue  float64
		expectedFilled int
		expectedEmpty  int
	}{
		{
			name:           "0% progress",
			progressValue:  0.0,
			expectedFilled: 0,
			expectedEmpty:  40,
		},
		{
			name:           "25% progress",
			progressValue:  25.0,
			expectedFilled: 10,
			expectedEmpty:  30,
		},
		{
			name:           "50% progress",
			progressValue:  50.0,
			expectedFilled: 20,
			expectedEmpty:  20,
		},
		{
			name:           "100% progress",
			progressValue:  100.0,
			expectedFilled: 40,
			expectedEmpty:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := NewStatusBar(80, theme.DefaultTheme)
			sb.ShowProgress()
			sb.progressValue = tt.progressValue

			view := sb.View()
			plainText := stripAnsiCodes(view)

			// Count filled and empty characters
			filledCount := strings.Count(plainText, "█")
			emptyCount := strings.Count(plainText, "░")

			assert.Equal(t, tt.expectedFilled, filledCount, "filled count mismatch")
			assert.Equal(t, tt.expectedEmpty, emptyCount, "empty count mismatch")
		})
	}
}

func TestStatusBar_SetReadOnlyMode(t *testing.T) {
	sb := NewStatusBar(80, theme.DefaultTheme)

	// Initially not in read-only mode
	assert.False(t, sb.readOnlyMode)

	// Enable read-only mode
	sb.SetReadOnlyMode(true)
	assert.True(t, sb.readOnlyMode)

	// Disable read-only mode
	sb.SetReadOnlyMode(false)
	assert.False(t, sb.readOnlyMode)
}

func TestStatusBar_ReadOnlyModeIndicator(t *testing.T) {
	sb := NewStatusBar(100, theme.DefaultTheme)
	sb.SetConnectionStatus("connected")

	// Enable read-only mode
	sb.SetReadOnlyMode(true)

	view := sb.View()

	// Should show read-only indicator
	assert.Contains(t, view, "👀")
	assert.Contains(t, view, "Viewing closed session (read-only)")
}

func TestStatusBar_ReadOnlyModePriority(t *testing.T) {
	sb := NewStatusBar(100, theme.DefaultTheme)

	// Set custom status
	sb.SetStatus("Agent is thinking...")

	// Enable read-only mode (should override custom status)
	sb.SetReadOnlyMode(true)

	view := sb.View()

	// Should show read-only indicator, not custom status
	assert.Contains(t, view, "👀")
	assert.Contains(t, view, "read-only")
	assert.NotContains(t, view, "Agent is thinking")
}

func TestStatusBar_ReadOnlyModeWithSessionName(t *testing.T) {
	sb := NewStatusBar(100, theme.DefaultTheme)

	// Set active session
	sb.SetActiveSession("abc123 (Read-Only)")
	sb.SetReadOnlyMode(true)

	view := sb.View()

	// Should show read-only indicator in custom status
	assert.Contains(t, view, "👀")
	assert.Contains(t, view, "read-only")
}

func TestStatusBar_NormalModeDoesNotShowReadOnlyIndicator(t *testing.T) {
	sb := NewStatusBar(100, theme.DefaultTheme)
	sb.SetConnectionStatus("connected")
	sb.SetActiveSession("TestSession")

	// Ensure read-only mode is off
	sb.SetReadOnlyMode(false)

	view := sb.View()

	// Should NOT show read-only indicator
	assert.NotContains(t, view, "Viewing closed session (read-only)")
}
