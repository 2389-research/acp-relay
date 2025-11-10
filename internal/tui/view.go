// ABOUTME: View rendering for the TUI (converts model state to terminal output)
// ABOUTME: Implements the Elm architecture View function
package tui

import "fmt"

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}
	return fmt.Sprintf("ACP-Relay TUI (%dx%d)\nPress 'q' to quit", m.width, m.height)
}
