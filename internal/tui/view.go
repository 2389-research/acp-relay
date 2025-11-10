// ABOUTME: View rendering for the TUI (converts model state to terminal output)
// ABOUTME: Implements the Elm architecture View function
package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Apply theme styling
	title := m.theme.StatusBarStyle().Render("ACP-Relay TUI")
	info := m.theme.DimStyle().Render(fmt.Sprintf("(%dx%d)", m.width, m.height))
	help := m.theme.DimStyle().Render("Press 'q' to quit")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		info,
		help,
	)
}
