// ABOUTME: System message widget for rendering system-level notifications
// ABOUTME: Handles commands, tool use, thinking indicators with appropriate formatting
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/harper/acp-relay/internal/tui/client"
	"github.com/harper/acp-relay/internal/tui/theme"
)

// FormatSystemMessage renders system messages based on their type.
func FormatSystemMessage(msg *client.Message, th theme.Theme) string {
	var sb strings.Builder

	// Icon and timestamp
	icon := msg.Type.Icon()
	timestamp := msg.Timestamp.Format("15:04")
	timestampStyled := th.DimStyle().Render(timestamp)

	// Build the header line: icon + timestamp
	header := fmt.Sprintf("%s %s", icon, timestampStyled)
	sb.WriteString(header)
	sb.WriteString("\n")

	// Format content based on message subtype
	switch msg.Type {
	case client.MessageTypeAvailableCommands:
		sb.WriteString(formatAvailableCommands(msg, th))
	case client.MessageTypeToolUse:
		sb.WriteString(formatToolUse(msg, th))
	case client.MessageTypeThinking:
		sb.WriteString(formatThinking(msg, th))
	case client.MessageTypeThoughtChunk:
		sb.WriteString(formatThoughtChunk(msg, th))
	default:
		// Generic system message
		content := th.DimStyle().Render(msg.Content)
		sb.WriteString(content)
	}

	sb.WriteString("\n")
	return sb.String()
}

// formatAvailableCommands renders available commands with bullet points.
func formatAvailableCommands(msg *client.Message, th theme.Theme) string {
	var sb strings.Builder

	// Title
	title := th.ChatViewStyle().Render("Available Commands Updated")
	sb.WriteString(title)
	sb.WriteString("\n")

	// Show first 5 commands with bullet points
	maxCommands := 5
	commandsToShow := msg.Commands
	if len(commandsToShow) > maxCommands {
		commandsToShow = msg.Commands[:maxCommands]
	}

	for _, cmd := range commandsToShow {
		bullet := th.DimStyle().Render("  • ")
		cmdName := th.ChatViewStyle().Bold(true).Render(cmd.Name)
		cmdDesc := ""
		if cmd.Description != "" {
			cmdDesc = " - " + th.DimStyle().Render(cmd.Description)
		}
		sb.WriteString(fmt.Sprintf("%s%s%s\n", bullet, cmdName, cmdDesc))
	}

	// Show "... and X more" if there are more commands
	if len(msg.Commands) > maxCommands {
		remaining := len(msg.Commands) - maxCommands
		moreText := th.DimStyle().Render(fmt.Sprintf("  ... and %d more", remaining))
		sb.WriteString(moreText)
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatToolUse renders tool use messages with magenta color.
func formatToolUse(msg *client.Message, th theme.Theme) string {
	// Use magenta color for tool use
	magenta := lipgloss.Color("#FF00FF")
	content := th.ChatViewStyle().
		Foreground(magenta).
		Render(fmt.Sprintf("Using tool: %s", msg.ToolName))
	return content
}

// formatThinking renders thinking indicator.
func formatThinking(_ *client.Message, th theme.Theme) string {
	content := th.DimStyle().Render("Agent is thinking...")
	return content
}

// formatThoughtChunk renders thought chunks (typically not shown in chat).
func formatThoughtChunk(msg *client.Message, th theme.Theme) string {
	// Thought chunks are typically shown in status bar, not in chat
	// But if we do show them, display dimmed
	content := th.DimStyle().Render(msg.Thought)
	return content
}
