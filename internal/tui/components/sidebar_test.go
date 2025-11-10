// ABOUTME: Unit tests for sidebar component (session list display)
// ABOUTME: Tests rendering, navigation, and session selection
package components

import (
	"testing"

	"github.com/harper/acp-relay/internal/tui/client"
	"github.com/harper/acp-relay/internal/tui/theme"
	"github.com/stretchr/testify/assert"
)

func TestNewSidebar(t *testing.T) {
	sb := NewSidebar(30, 20, theme.DefaultTheme)

	assert.NotNil(t, sb)
	assert.Equal(t, 30, sb.width)
	assert.Equal(t, 20, sb.height)
}

func TestSidebar_SetSessions(t *testing.T) {
	sb := NewSidebar(30, 20, theme.DefaultTheme)

	sessions := []*client.Session{
		{ID: "1", DisplayName: "Test", Status: client.StatusActive},
		{ID: "2", DisplayName: "Demo", Status: client.StatusIdle},
	}

	sb.SetSessions(sessions)

	view := sb.View()
	assert.Contains(t, view, "Test")
	assert.Contains(t, view, "Demo")
	assert.Contains(t, view, "⚡") // Active icon
	assert.Contains(t, view, "💤") // Idle icon
}

func TestSidebar_Navigation(t *testing.T) {
	sb := NewSidebar(30, 20, theme.DefaultTheme)

	sessions := []*client.Session{
		{ID: "1", DisplayName: "One"},
		{ID: "2", DisplayName: "Two"},
		{ID: "3", DisplayName: "Three"},
	}
	sb.SetSessions(sessions)

	// Start at 0
	assert.Equal(t, 0, sb.cursor)

	// Move down
	sb.CursorDown()
	assert.Equal(t, 1, sb.cursor)

	// Move down again
	sb.CursorDown()
	assert.Equal(t, 2, sb.cursor)

	// Wrap to top
	sb.CursorDown()
	assert.Equal(t, 0, sb.cursor)

	// Move up (wraps to bottom)
	sb.CursorUp()
	assert.Equal(t, 2, sb.cursor)
}

func TestSidebar_GetSelectedSession(t *testing.T) {
	sb := NewSidebar(30, 20, theme.DefaultTheme)

	sessions := []*client.Session{
		{ID: "1", DisplayName: "One"},
		{ID: "2", DisplayName: "Two"},
	}
	sb.SetSessions(sessions)

	// Default selection
	selected := sb.GetSelectedSession()
	assert.Equal(t, "1", selected.ID)

	// Move cursor
	sb.CursorDown()
	selected = sb.GetSelectedSession()
	assert.Equal(t, "2", selected.ID)
}

func TestSidebar_EmptySessions(t *testing.T) {
	sb := NewSidebar(30, 20, theme.DefaultTheme)

	view := sb.View()
	assert.Contains(t, view, "No sessions")
}
