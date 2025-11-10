// ABOUTME: Sidebar component for displaying session list
// ABOUTME: Handles session navigation, rendering, and selection
package components

import (
	"fmt"
	"strings"

	"github.com/harper/acp-relay/internal/tui/client"
	"github.com/harper/acp-relay/internal/tui/theme"
)

type Sidebar struct {
	width    int
	height   int
	theme    theme.Theme
	sessions []*client.Session
	cursor   int
}

func NewSidebar(width, height int, t theme.Theme) *Sidebar {
	return &Sidebar{
		width:  width,
		height: height,
		theme:  t,
		cursor: 0,
	}
}

func (s *Sidebar) SetSessions(sessions []*client.Session) {
	s.sessions = sessions

	// Clamp cursor to valid range
	if s.cursor >= len(sessions) {
		s.cursor = len(sessions) - 1
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
}

func (s *Sidebar) CursorDown() {
	if len(s.sessions) == 0 {
		return
	}

	s.cursor++
	if s.cursor >= len(s.sessions) {
		s.cursor = 0 // Wrap to top
	}
}

func (s *Sidebar) CursorUp() {
	if len(s.sessions) == 0 {
		return
	}

	s.cursor--
	if s.cursor < 0 {
		s.cursor = len(s.sessions) - 1 // Wrap to bottom
	}
}

func (s *Sidebar) GetSelectedSession() *client.Session {
	if len(s.sessions) == 0 || s.cursor < 0 || s.cursor >= len(s.sessions) {
		return nil
	}
	return s.sessions[s.cursor]
}

func (s *Sidebar) SetCursor(index int) {
	if index >= 0 && index < len(s.sessions) {
		s.cursor = index
	}
}

func (s *Sidebar) View() string {
	if len(s.sessions) == 0 {
		emptyMsg := s.theme.DimStyle().Render("No sessions\n\nPress 'n' to create one")
		return s.theme.SidebarStyle().
			Width(s.width - 2).
			Height(s.height - 2).
			Render(emptyMsg)
	}

	var items []string

	// Title
	title := s.theme.ActiveSessionStyle().
		Width(s.width - 4).
		Render("SESSIONS")
	items = append(items, title, "")

	// Session list
	for i, sess := range s.sessions {
		icon := sess.Status.Icon()
		name := sess.DisplayName

		// Truncate long names
		maxLen := s.width - 8 // Account for padding and icon
		if len(name) > maxLen {
			name = name[:maxLen-3] + "..."
		}

		line := fmt.Sprintf("%s %s", icon, name)

		// Style based on selection
		if i == s.cursor {
			line = s.theme.ActiveSessionStyle().
				Width(s.width - 4).
				Render(line)
		} else {
			line = s.theme.InactiveSessionStyle().
				Width(s.width - 4).
				Render(line)
		}

		items = append(items, line)
	}

	// Help text at bottom
	help := s.theme.DimStyle().Render("\n↑↓: Navigate\nn: New\nd: Delete\nr: Rename")
	items = append(items, "", help)

	content := strings.Join(items, "\n")

	return s.theme.SidebarStyle().
		Width(s.width - 2).
		Height(s.height - 2).
		Render(content)
}

func (s *Sidebar) SetSize(width, height int) {
	s.width = width
	s.height = height
}
