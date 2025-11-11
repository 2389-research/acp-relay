// ABOUTME: Permission widget for rendering permission requests and responses
// ABOUTME: Formats permission prompts with icons, tool names, and arguments
package components

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/harper/acp-relay/internal/tui/client"
	"github.com/harper/acp-relay/internal/tui/theme"
)

// FormatPermissionRequest renders a permission request message with icon, tool name, and arguments.
func FormatPermissionRequest(msg *client.Message, th theme.Theme) string {
	var sb strings.Builder

	// Icon and title
	sb.WriteString("🔐 ")
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(th.Warning)
	sb.WriteString(titleStyle.Render("Permission Request"))
	sb.WriteString("\n")

	// Tool name
	toolStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(th.Primary)
	sb.WriteString("Tool: ")
	sb.WriteString(toolStyle.Render(msg.Content))
	sb.WriteString("\n")

	// Arguments
	if len(msg.RawInput) > 0 {
		sb.WriteString("Arguments:\n")
		argsJSON, err := json.MarshalIndent(msg.RawInput, "  ", "  ")
		if err == nil {
			argsStr := string(argsJSON)
			// Truncate if too long
			if len(argsStr) > 100 {
				argsStr = argsStr[:100] + "..."
			}
			argStyle := lipgloss.NewStyle().
				Foreground(th.Dim)
			sb.WriteString(argStyle.Render("  " + argsStr))
		}
	}

	// Wrap in yellow border
	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(th.Warning).
		Padding(0, 1)

	return borderStyle.Render(sb.String())
}

// FormatPermissionResponse renders a permission response with outcome icon and tool name.
func FormatPermissionResponse(msg *client.Message, th theme.Theme) string {
	var sb strings.Builder

	// Determine outcome
	outcome := "unknown"
	if msg.RawInput != nil {
		if o, ok := msg.RawInput["outcome"].(string); ok {
			outcome = o
		}
	}

	// Icon and status based on outcome
	var icon string
	var statusStyle lipgloss.Style
	var statusText string

	switch outcome {
	case "allow", "selected":
		icon = "✅"
		statusStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(th.Success)
		statusText = "Allowed"
	case "deny", "rejected":
		icon = "❌"
		statusStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(th.Error)
		statusText = "Denied"
	default:
		icon = "❓"
		statusStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(th.Dim)
		statusText = "Unknown"
	}

	sb.WriteString(icon)
	sb.WriteString(" ")
	sb.WriteString(statusStyle.Render(statusText))
	sb.WriteString(": ")

	// Tool name
	toolStyle := lipgloss.NewStyle().
		Foreground(th.Primary)
	sb.WriteString(toolStyle.Render(msg.Content))

	return sb.String()
}
