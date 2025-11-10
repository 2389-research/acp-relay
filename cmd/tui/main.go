// ABOUTME: Entry point for the ACP-Relay TUI client
// ABOUTME: Initializes configuration, theme, and starts the Bubbletea application
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/harper/acp-relay/internal/tui"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	// Create initial model
	m := tui.NewModel()

	// Create Bubbletea program
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Run program
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
