// ABOUTME: InputArea component for multi-line text input in the TUI
// ABOUTME: Wraps bubbles/textarea with theme styling and focus management
package components

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/harper/acp-relay/clients/tui/theme"
)

// InputArea represents a multi-line text input area.
type InputArea struct {
	width    int
	height   int
	theme    theme.Theme
	textarea textarea.Model
	focused  bool
	disabled bool
}

// NewInputArea creates a new InputArea with the specified dimensions and theme.
func NewInputArea(width, height int, th theme.Theme) *InputArea {
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Enter to send, Shift+Enter for new line)"
	ta.SetWidth(width - 2) // Account for padding
	ta.SetHeight(height)
	ta.ShowLineNumbers = false
	ta.CharLimit = 0 // No character limit

	// Apply theme colors
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()

	return &InputArea{
		width:    width,
		height:   height,
		theme:    th,
		textarea: ta,
		focused:  false,
	}
}

// SetValue sets the text content of the input area.
func (ia *InputArea) SetValue(value string) {
	ia.textarea.SetValue(value)
}

// GetValue returns the current text content.
func (ia *InputArea) GetValue() string {
	return ia.textarea.Value()
}

// Clear resets the input area to empty.
func (ia *InputArea) Clear() {
	ia.textarea.Reset()
}

// Focus sets the focused state to true.
func (ia *InputArea) Focus() {
	ia.focused = true
	ia.textarea.Focus()
}

// Blur removes focus from the input area.
func (ia *InputArea) Blur() {
	ia.focused = false
	ia.textarea.Blur()
}

// SetSize updates the dimensions of the input area.
func (ia *InputArea) SetSize(width, height int) {
	ia.width = width
	ia.height = height
	ia.textarea.SetWidth(width - 2) // Account for padding
	ia.textarea.SetHeight(height)
}

// SetDisabled enables or disables the input area.
func (ia *InputArea) SetDisabled(disabled bool) {
	ia.disabled = disabled
	if disabled {
		ia.textarea.Placeholder = "Session is closed (read-only)"
	} else {
		ia.textarea.Placeholder = "Type your message... (Enter to send, Shift+Enter for new line)"
	}
}

// Init initializes the component (Bubbletea lifecycle).
func (ia *InputArea) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles messages and updates the component (Bubbletea lifecycle).
func (ia *InputArea) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Ignore all input when disabled
	if ia.disabled {
		return ia, nil
	}

	var cmd tea.Cmd
	ia.textarea, cmd = ia.textarea.Update(msg)
	return ia, cmd
}

// View renders the input area with theme styling.
func (ia *InputArea) View() string {
	baseStyle := ia.theme.InputAreaStyle().
		Width(ia.width).
		Height(ia.height)

	// Apply gray background when disabled
	if ia.disabled {
		baseStyle = baseStyle.
			Background(ia.theme.Dim).
			Foreground(lipgloss.Color("#666666"))
	}

	return baseStyle.Render(ia.textarea.View())
}
