// ABOUTME: StatusBar component for displaying connection status and session info
// ABOUTME: Shows connection state with colored indicators and keyboard shortcuts
package components

import (
	"fmt"
	"strings"

	"github.com/harper/acp-relay/internal/tui/theme"
)

type StatusBar struct {
	width             int
	theme             theme.Theme
	connectionStatus  string
	activeSessionName string
}

func NewStatusBar(width int, t theme.Theme) *StatusBar {
	return &StatusBar{
		width:             width,
		theme:             t,
		connectionStatus:  "disconnected",
		activeSessionName: "",
	}
}

func (s *StatusBar) SetConnectionStatus(status string) {
	s.connectionStatus = status
}

func (s *StatusBar) SetActiveSession(name string) {
	s.activeSessionName = name
}

func (s *StatusBar) SetSize(width int) {
	s.width = width
}

func (s *StatusBar) View() string {
	// Connection status with icon
	var statusIcon string
	var statusText string

	switch s.connectionStatus {
	case "connected":
		statusIcon = "🟢"
		statusText = "Connected"
	case "connecting":
		statusIcon = "🟡"
		statusText = "Connecting"
	case "disconnected":
		statusIcon = "🔴"
		statusText = "Disconnected"
	default:
		statusIcon = "🔴"
		statusText = "Disconnected"
	}

	statusPart := fmt.Sprintf("[%s %s]", statusIcon, statusText)

	// Session info
	var sessionPart string
	if s.activeSessionName != "" {
		sessionPart = fmt.Sprintf("Session: %s", s.activeSessionName)
	} else {
		sessionPart = "No active session"
	}

	// Keyboard shortcuts
	shortcuts := "Tab: Navigate, ?: Help, q: Quit"

	// Build the status bar with separator
	leftContent := fmt.Sprintf("%s %s", statusPart, sessionPart)

	// Calculate spacing to right-align shortcuts
	// Account for visual width without ANSI codes
	plainLeft := stripAnsiForWidth(leftContent)
	plainShortcuts := stripAnsiForWidth(shortcuts)

	contentWidth := len(plainLeft) + len(plainShortcuts) + 3 // 3 for separator " | "
	padding := s.width - contentWidth - 4                    // 4 for the style padding

	if padding < 1 {
		padding = 1
	}

	spacer := strings.Repeat(" ", padding)
	fullContent := fmt.Sprintf("%s%s| %s", leftContent, spacer, shortcuts)

	return s.theme.StatusBarStyle().
		Width(s.width - 2).
		Render(fullContent)
}

// stripAnsiForWidth removes ANSI codes to calculate actual display width.
func stripAnsiForWidth(s string) string {
	result := strings.Builder{}
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
		result.WriteRune(r)
	}

	return result.String()
}
