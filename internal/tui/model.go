// ABOUTME: Core Bubbletea model and state management for the TUI
// ABOUTME: Implements the Model interface with Init, Update, and View methods
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/harper/acp-relay/internal/tui/config"
	"github.com/harper/acp-relay/internal/tui/theme"
)

type Model struct {
	config *config.Config
	theme  theme.Theme
	width  int
	height int
}

func NewModel(cfg *config.Config) Model {
	return Model{
		config: cfg,
		theme:  theme.GetTheme(cfg.UI.Theme, nil),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}
