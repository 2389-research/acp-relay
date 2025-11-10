// ABOUTME: Core Bubbletea model and state management for the TUI
// ABOUTME: Implements the Model interface with Init, Update, and View methods
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/harper/acp-relay/internal/tui/config"
)

type Model struct {
	config *config.Config
	width  int
	height int
}

func NewModel(cfg *config.Config) Model {
	return Model{
		config: cfg,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}
