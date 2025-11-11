// ABOUTME: Unit tests for system message widget formatting
// ABOUTME: Tests rendering of commands, tool use, thinking indicators
package components

import (
	"strings"
	"testing"
	"time"

	"github.com/harper/acp-relay/internal/tui/client"
	"github.com/harper/acp-relay/internal/tui/theme"
	"github.com/stretchr/testify/assert"
)

func TestFormatSystemMessage_AvailableCommands(t *testing.T) {
	th := theme.GetTheme("dark", nil)
	commands := []client.Command{
		{Name: "/help", Description: "Show help"},
		{Name: "/clear", Description: "Clear screen"},
		{Name: "/exit", Description: "Exit program"},
	}

	msg := &client.Message{
		SessionID: "sess-1",
		Type:      client.MessageTypeAvailableCommands,
		Content:   "Commands updated: [/help /clear /exit]",
		Commands:  commands,
		Timestamp: time.Now(),
	}

	result := FormatSystemMessage(msg, th)

	// Check that result contains command names
	assert.Contains(t, result, "/help")
	assert.Contains(t, result, "/clear")
	assert.Contains(t, result, "/exit")
	assert.Contains(t, result, "Available Commands Updated")
	assert.Contains(t, result, "📋") // Icon
}

func TestFormatSystemMessage_AvailableCommands_MoreThanFive(t *testing.T) {
	th := theme.GetTheme("dark", nil)
	commands := []client.Command{
		{Name: "/cmd1", Description: "Command 1"},
		{Name: "/cmd2", Description: "Command 2"},
		{Name: "/cmd3", Description: "Command 3"},
		{Name: "/cmd4", Description: "Command 4"},
		{Name: "/cmd5", Description: "Command 5"},
		{Name: "/cmd6", Description: "Command 6"},
		{Name: "/cmd7", Description: "Command 7"},
	}

	msg := &client.Message{
		SessionID: "sess-1",
		Type:      client.MessageTypeAvailableCommands,
		Content:   "7 commands available",
		Commands:  commands,
		Timestamp: time.Now(),
	}

	result := FormatSystemMessage(msg, th)

	// Check that only first 5 are shown
	assert.Contains(t, result, "/cmd1")
	assert.Contains(t, result, "/cmd5")
	assert.NotContains(t, result, "/cmd6")
	assert.NotContains(t, result, "/cmd7")
	// Check "... and X more" message
	assert.Contains(t, result, "and 2 more")
}

func TestFormatSystemMessage_ToolUse(t *testing.T) {
	th := theme.GetTheme("dark", nil)

	msg := &client.Message{
		SessionID: "sess-1",
		Type:      client.MessageTypeToolUse,
		Content:   "Using tool: Read",
		ToolName:  "Read",
		Timestamp: time.Now(),
	}

	result := FormatSystemMessage(msg, th)

	// Check that result contains tool name
	assert.Contains(t, result, "Using tool: Read")
	assert.Contains(t, result, "🔧") // Icon
}

func TestFormatSystemMessage_Thinking(t *testing.T) {
	th := theme.GetTheme("dark", nil)

	msg := &client.Message{
		SessionID: "sess-1",
		Type:      client.MessageTypeThinking,
		Content:   "Agent is thinking...",
		Timestamp: time.Now(),
	}

	result := FormatSystemMessage(msg, th)

	// Check that result contains thinking message
	assert.Contains(t, result, "Agent is thinking...")
	assert.Contains(t, result, "💭") // Icon
}

func TestFormatSystemMessage_ThoughtChunk(t *testing.T) {
	th := theme.GetTheme("dark", nil)

	msg := &client.Message{
		SessionID: "sess-1",
		Type:      client.MessageTypeThoughtChunk,
		Content:   "I need to analyze the code structure first",
		Thought:   "I need to analyze the code structure first",
		Timestamp: time.Now(),
	}

	result := FormatSystemMessage(msg, th)

	// Check that result contains thought text
	assert.Contains(t, result, "I need to analyze the code structure first")
	assert.Contains(t, result, "💭") // Icon
}

func TestFormatSystemMessage_Generic(t *testing.T) {
	th := theme.GetTheme("dark", nil)

	msg := &client.Message{
		SessionID: "sess-1",
		Type:      client.MessageTypeSystem,
		Content:   "Generic system message",
		Timestamp: time.Now(),
	}

	result := FormatSystemMessage(msg, th)

	// Check that result contains message content
	assert.Contains(t, result, "Generic system message")
	assert.Contains(t, result, "ℹ️") // Icon
}

func TestFormatAvailableCommands_WithDescriptions(t *testing.T) {
	th := theme.GetTheme("dark", nil)
	commands := []client.Command{
		{Name: "/help", Description: "Show help"},
		{Name: "/clear", Description: "Clear screen"},
	}

	msg := &client.Message{
		Commands: commands,
	}

	result := formatAvailableCommands(msg, th)

	// Check bullet points and descriptions
	assert.True(t, strings.Contains(result, "/help"))
	assert.True(t, strings.Contains(result, "Show help"))
	assert.True(t, strings.Contains(result, "/clear"))
	assert.True(t, strings.Contains(result, "Clear screen"))
}

func TestFormatAvailableCommands_WithoutDescriptions(t *testing.T) {
	th := theme.GetTheme("dark", nil)
	commands := []client.Command{
		{Name: "/help", Description: ""},
		{Name: "/clear", Description: ""},
	}

	msg := &client.Message{
		Commands: commands,
	}

	result := formatAvailableCommands(msg, th)

	// Check command names are present
	assert.True(t, strings.Contains(result, "/help"))
	assert.True(t, strings.Contains(result, "/clear"))
}
