// ABOUTME: SessionSelectionScreen modal for choosing or creating sessions
// ABOUTME: Displays recent sessions with navigation and selection support
package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/harper/acp-relay/internal/tui/client"
	"github.com/harper/acp-relay/internal/tui/theme"
)

// Message types for session selection.
type ShowSessionSelectionMsg struct {
	Sessions []client.ManagementSession
}

type SessionSelectedMsg struct {
	Session client.ManagementSession
}

type CreateNewSessionMsg struct{}

// SessionSelectionScreen is the modal dialog for session selection.
type SessionSelectionScreen struct {
	sessions      []client.ManagementSession
	selectedIndex int
	width         int
	height        int
	theme         theme.Theme
}

// NewSessionSelectionScreen creates a new session selection screen.
func NewSessionSelectionScreen(sessions []client.ManagementSession, width, height int, th theme.Theme) *SessionSelectionScreen {
	selectedIndex := 0
	if len(sessions) == 0 {
		selectedIndex = -1
	}

	return &SessionSelectionScreen{
		sessions:      sessions,
		selectedIndex: selectedIndex,
		width:         width,
		height:        height,
		theme:         th,
	}
}

// Init initializes the session selection screen (required by tea.Model).
func (s *SessionSelectionScreen) Init() tea.Cmd {
	return nil
}

// Update handles input for the session selection screen.
//
//nolint:gocognit,funlen // modal input handling requires branching for different key types and cases
func (s *SessionSelectionScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil

	case tea.KeyMsg:
		// Handle empty sessions case
		if len(s.sessions) == 0 {
			switch msg.Type {
			case tea.KeyRunes:
				if len(msg.Runes) > 0 && msg.Runes[0] == 'n' {
					return s, func() tea.Msg { return CreateNewSessionMsg{} }
				}
				if len(msg.Runes) > 0 && msg.Runes[0] == 'q' {
					return s, tea.Quit
				}
			case tea.KeyEsc:
				return s, tea.Quit
			}
			return s, nil
		}

		switch msg.Type {
		case tea.KeyUp:
			s.selectedIndex--
			if s.selectedIndex < 0 {
				s.selectedIndex = len(s.sessions) - 1
			}
			return s, nil

		case tea.KeyDown:
			s.selectedIndex++
			if s.selectedIndex >= len(s.sessions) {
				s.selectedIndex = 0
			}
			return s, nil

		case tea.KeyEnter:
			if s.selectedIndex >= 0 && s.selectedIndex < len(s.sessions) {
				selected := s.sessions[s.selectedIndex]
				return s, func() tea.Msg {
					return SessionSelectedMsg{Session: selected}
				}
			}
			return s, nil

		case tea.KeyRunes:
			if len(msg.Runes) > 0 {
				switch msg.Runes[0] {
				case 'n':
					return s, func() tea.Msg { return CreateNewSessionMsg{} }
				case 'q':
					return s, tea.Quit
				}
			}

		case tea.KeyEsc:
			return s, tea.Quit
		}
	}

	return s, nil
}

// View renders the session selection screen.
//
//nolint:funlen,nestif // UI rendering requires extensive layout code and conditional formatting
func (s *SessionSelectionScreen) View() string {
	// Create modal dialog box
	dialogWidth := 70
	if s.width < 80 {
		dialogWidth = s.width - 10
	}

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(s.theme.Primary).
		Align(lipgloss.Center).
		Width(dialogWidth - 4)

	title := titleStyle.Render("🔄 Select or Create Session")

	// Session list
	var sessionLines []string

	if len(s.sessions) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(s.theme.Dim).
			Italic(true).
			Align(lipgloss.Center).
			Width(dialogWidth - 4)

		sessionLines = append(sessionLines, "")
		sessionLines = append(sessionLines, emptyStyle.Render("No previous sessions found."))
		sessionLines = append(sessionLines, emptyStyle.Render("Press 'n' to create a new session."))
		sessionLines = append(sessionLines, "")
	} else {
		// Show first 15 sessions
		displayCount := len(s.sessions)
		if displayCount > 15 {
			displayCount = 15
		}

		for i := 0; i < displayCount; i++ {
			sess := s.sessions[i]

			// Status icon
			statusIcon := "💤" // closed
			if sess.IsActive {
				statusIcon = "✅" // active
			}

			// Session ID (first 12 chars)
			sessionID := sess.ID
			if len(sessionID) > 12 {
				sessionID = sessionID[:12]
			}

			// Timestamp
			timestamp := sess.CreatedAt.Format("2006-01-02 15:04")

			// Build line
			line := fmt.Sprintf("%2d. %s %s  %s", i+1, statusIcon, sessionID, timestamp)

			// Highlight selected session
			if i == s.selectedIndex {
				highlightStyle := lipgloss.NewStyle().
					Background(s.theme.Primary).
					Foreground(s.theme.Background).
					Bold(true).
					Width(dialogWidth - 4)
				sessionLines = append(sessionLines, highlightStyle.Render(line))
			} else {
				normalStyle := lipgloss.NewStyle().
					Foreground(s.theme.Foreground).
					Width(dialogWidth - 4)
				sessionLines = append(sessionLines, normalStyle.Render(line))
			}
		}
	}

	// Keybindings
	keybindingsStyle := lipgloss.NewStyle().
		Foreground(s.theme.Dim).
		Italic(true).
		Align(lipgloss.Center).
		Width(dialogWidth - 4)

	keybindings := "↑↓: Navigate | Enter: Select | n: New | q: Quit"

	// Combine all parts
	contentParts := []string{
		"",
		title,
		"",
		strings.Join(sessionLines, "\n"),
		"",
		keybindingsStyle.Render(keybindings),
		"",
	}

	content := strings.Join(contentParts, "\n")

	// Create border box
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.theme.Primary).
		Padding(1, 2).
		Width(dialogWidth)

	dialog := borderStyle.Render(content)

	// Center the dialog on screen
	centered := lipgloss.Place(
		s.width,
		s.height,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
	)

	return centered
}
