// ABOUTME: View rendering for the TUI (converts model state to terminal output)
// ABOUTME: Implements the Elm architecture View function
package tui

import (
	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Build the main content layout
	var mainContent string

	if m.sidebarVisible {
		// Layout: sidebar | (chatView / inputArea)
		chatAndInput := lipgloss.JoinVertical(
			lipgloss.Top,
			m.chatView.View(),
			m.inputArea.View(),
		)

		mainContent = lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.sidebar.View(),
			chatAndInput,
		)
	} else {
		// Layout: chatView / inputArea (no sidebar)
		mainContent = lipgloss.JoinVertical(
			lipgloss.Top,
			m.chatView.View(),
			m.inputArea.View(),
		)
	}

	// Add status bar at the bottom
	fullView := lipgloss.JoinVertical(
		lipgloss.Top,
		mainContent,
		m.statusBar.View(),
	)

	// Overlay session selection modal if visible
	if m.sessionModal != nil {
		// Render session selection modal on top of the base view
		return m.sessionModal.View()
	}

	// Overlay help if visible
	if m.helpOverlay.IsVisible() {
		// Render help overlay on top of the base view
		// The help overlay already handles its own positioning and centering
		return m.helpOverlay.View()
	}

	// Render notifications in top-right corner if any exist
	notificationView := m.notifications.View()
	if notificationView != "" {
		// Use Lipgloss Place to position notifications in top-right
		fullView = lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Right,
			lipgloss.Top,
			notificationView,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.NoColor{}),
		) + "\n" + fullView
	}

	return fullView
}
