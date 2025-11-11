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

	// Overlay help if visible
	if m.helpOverlay.IsVisible() {
		// Create a base layer with the main view
		baseStyle := lipgloss.NewStyle().
			Width(m.width).
			Height(m.height)

		base := baseStyle.Render(fullView)

		// Overlay the help on top
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			m.helpOverlay.View(),
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(m.theme.Background),
		) + "\n" + base
	}

	return fullView
}
