// ABOUTME: StatusBar component for displaying connection status and session info
// ABOUTME: Shows connection state with colored indicators and keyboard shortcuts
package components

import (
	"fmt"
	"strings"

	"github.com/harper/acp-relay/clients/tui/theme"
)

type StatusBar struct {
	width             int
	theme             theme.Theme
	connectionStatus  string
	activeSessionName string
	customStatus      string // For temporary status messages like "Agent is thinking..."
	progressVisible   bool
	progressValue     float64
	readOnlyMode      bool
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

func (s *StatusBar) SetStatus(status string) {
	s.customStatus = status
}

func (s *StatusBar) SetReadOnlyMode(readOnly bool) {
	s.readOnlyMode = readOnly
}

func (s *StatusBar) SetSize(width int) {
	s.width = width
}

func (s *StatusBar) ShowProgress() {
	s.progressVisible = true
	s.progressValue = 0.0
}

func (s *StatusBar) HideProgress() {
	s.progressVisible = false
}

func (s *StatusBar) AdvanceProgress(amount float64) {
	s.progressValue += amount
	// Wrap at 100.0
	if s.progressValue >= 100.0 {
		s.progressValue = float64(int(s.progressValue) % 100)
	}
}

func (s *StatusBar) View() string {
	statusPart := s.formatConnectionStatus()
	sessionPart := s.formatSessionInfo()
	shortcuts := "Tab: Navigate, ?: Help, q: Quit"

	leftContent := fmt.Sprintf("%s %s", statusPart, sessionPart)
	fullContent := s.buildStatusLine(leftContent, shortcuts)

	statusLine := s.theme.StatusBarStyle().
		Width(s.width - 2).
		Render(fullContent)

	// Add progress bar below status line if visible
	if s.progressVisible {
		progressBar := s.renderProgressBar()
		return statusLine + "\n" + progressBar
	}

	return statusLine
}

// formatConnectionStatus returns the connection status icon and text.
func (s *StatusBar) formatConnectionStatus() string {
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

	return fmt.Sprintf("[%s %s]", statusIcon, statusText)
}

// formatSessionInfo returns the session info or custom status.
func (s *StatusBar) formatSessionInfo() string {
	switch {
	case s.readOnlyMode:
		// Read-only mode takes priority over custom status
		return "👀 Viewing closed session (read-only)"
	case s.customStatus != "":
		// Show custom status (takes priority)
		return s.customStatus
	case s.activeSessionName != "":
		return fmt.Sprintf("Session: %s", s.activeSessionName)
	default:
		return "No active session"
	}
}

// buildStatusLine constructs the status line with proper spacing.
func (s *StatusBar) buildStatusLine(leftContent, shortcuts string) string {
	plainLeft := stripAnsiForWidth(leftContent)
	plainShortcuts := stripAnsiForWidth(shortcuts)

	contentWidth := len(plainLeft) + len(plainShortcuts) + 3 // 3 for separator " | "
	padding := s.width - contentWidth - 4                    // 4 for the style padding

	if padding < 1 {
		padding = 1
	}

	spacer := strings.Repeat(" ", padding)
	return fmt.Sprintf("%s%s| %s", leftContent, spacer, shortcuts)
}

// renderProgressBar renders a 40-character width progress bar using █ and ░ characters.
func (s *StatusBar) renderProgressBar() string {
	const barWidth = 40
	filledWidth := int((s.progressValue / 100.0) * float64(barWidth))
	emptyWidth := barWidth - filledWidth

	filled := strings.Repeat("█", filledWidth)
	empty := strings.Repeat("░", emptyWidth)

	progressBar := filled + empty

	// Style the progress bar with primary color
	styledBar := s.theme.StatusBarStyle().
		Foreground(s.theme.Primary).
		Render(progressBar)

	return styledBar
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
