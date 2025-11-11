// ABOUTME: HelpOverlay component for displaying keyboard shortcuts
// ABOUTME: Shows a centered modal dialog with list of available keyboard shortcuts
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/harper/acp-relay/internal/tui/theme"
)

// Shortcut represents a keyboard shortcut with its description
type Shortcut struct {
	key         string
	description string
}

// HelpOverlay displays a modal overlay with keyboard shortcuts
type HelpOverlay struct {
	width     int
	height    int
	theme     theme.Theme
	visible   bool
	shortcuts []Shortcut
}

// NewHelpOverlay creates a new help overlay with default shortcuts
func NewHelpOverlay(width, height int, t theme.Theme) *HelpOverlay {
	return &HelpOverlay{
		width:   width,
		height:  height,
		theme:   t,
		visible: false,
		shortcuts: []Shortcut{
			{"Tab", "Switch focus between areas"},
			{"Ctrl+B", "Toggle sidebar"},
			{"n", "New session"},
			{"d", "Delete session"},
			{"r", "Rename session"},
			{"Ctrl+S", "Send message"},
			{"q", "Quit"},
			{"Ctrl+C", "Quit"},
			{"?", "Toggle help"},
		},
	}
}

// Show makes the overlay visible
func (h *HelpOverlay) Show() {
	h.visible = true
}

// Hide makes the overlay invisible
func (h *HelpOverlay) Hide() {
	h.visible = false
}

// IsVisible returns whether the overlay is currently visible
func (h *HelpOverlay) IsVisible() bool {
	return h.visible
}

// Toggle toggles the overlay visibility
func (h *HelpOverlay) Toggle() {
	h.visible = !h.visible
}

// SetSize updates the overlay dimensions
func (h *HelpOverlay) SetSize(width, height int) {
	h.width = width
	h.height = height
}

// View renders the help overlay
func (h *HelpOverlay) View() string {
	if !h.visible {
		return ""
	}

	// Build the content
	var content strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(h.theme.Primary).
		Align(lipgloss.Center)

	content.WriteString(titleStyle.Render("Keyboard Shortcuts"))
	content.WriteString("\n\n")

	// Find max key length for alignment
	maxKeyLen := 0
	for _, sc := range h.shortcuts {
		if len(sc.key) > maxKeyLen {
			maxKeyLen = len(sc.key)
		}
	}

	// Render shortcuts
	keyStyle := lipgloss.NewStyle().
		Foreground(h.theme.Success).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(h.theme.Foreground)

	for _, sc := range h.shortcuts {
		// Pad the key to align descriptions
		paddedKey := sc.key + strings.Repeat(" ", maxKeyLen-len(sc.key))
		content.WriteString(fmt.Sprintf("  %s  %s\n",
			keyStyle.Render(paddedKey),
			descStyle.Render(sc.description)))
	}

	// Calculate modal dimensions
	modalWidth := 50
	if modalWidth > h.width-4 {
		modalWidth = h.width - 4
	}

	// Create the modal box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(h.theme.Primary).
		Background(h.theme.Background).
		Padding(1, 2).
		Width(modalWidth)

	modal := boxStyle.Render(content.String())

	// Center the modal on screen
	modalLines := strings.Split(modal, "\n")
	modalHeight := len(modalLines)

	// Calculate vertical centering
	verticalPadding := (h.height - modalHeight) / 2
	if verticalPadding < 0 {
		verticalPadding = 0
	}

	// Add vertical padding
	var result strings.Builder
	for i := 0; i < verticalPadding; i++ {
		result.WriteString("\n")
	}

	// Center each line horizontally
	for _, line := range modalLines {
		// Get the actual visible width of the line (strip ANSI codes)
		visibleWidth := getVisibleWidth(line)
		horizontalPadding := (h.width - visibleWidth) / 2
		if horizontalPadding < 0 {
			horizontalPadding = 0
		}
		result.WriteString(strings.Repeat(" ", horizontalPadding))
		result.WriteString(line)
		result.WriteString("\n")
	}

	return result.String()
}

// getVisibleWidth calculates the visible width of a string, ignoring ANSI escape codes
func getVisibleWidth(s string) int {
	width := 0
	inEscape := false

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		width++
	}

	return width
}
