// ABOUTME: Tests for ChatView component rendering and message display
// ABOUTME: Verifies message formatting, scrolling, and empty states
package components

import (
	"strings"
	"testing"
	"time"

	"github.com/harper/acp-relay/internal/tui/client"
	"github.com/harper/acp-relay/internal/tui/theme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChatView(t *testing.T) {
	width, height := 80, 24
	th := theme.DefaultTheme
	cv := NewChatView(width, height, th)

	require.NotNil(t, cv)
	assert.Equal(t, width, cv.width)
	assert.Equal(t, height, cv.height)
	assert.Equal(t, th, cv.theme)
	assert.NotNil(t, cv.messages)
	assert.Empty(t, cv.messages)
}

func TestChatView_AddMessage(t *testing.T) {
	cv := NewChatView(80, 24, theme.DefaultTheme)

	msg := &client.Message{
		SessionID: "test-session",
		Type:      client.MessageTypeUser,
		Content:   "Hello, world!",
		Timestamp: time.Now(),
	}

	cv.AddMessage(msg)

	assert.Len(t, cv.messages, 1)
	assert.Equal(t, msg, cv.messages[0])

	// Add another message
	msg2 := &client.Message{
		SessionID: "test-session",
		Type:      client.MessageTypeAgent,
		Content:   "Hi there!",
		Timestamp: time.Now(),
	}

	cv.AddMessage(msg2)

	assert.Len(t, cv.messages, 2)
	assert.Equal(t, msg, cv.messages[0])
	assert.Equal(t, msg2, cv.messages[1])
}

func TestChatView_SetMessages(t *testing.T) {
	cv := NewChatView(80, 24, theme.DefaultTheme)

	messages := []*client.Message{
		{
			SessionID: "test-session",
			Type:      client.MessageTypeUser,
			Content:   "Message 1",
			Timestamp: time.Now(),
		},
		{
			SessionID: "test-session",
			Type:      client.MessageTypeAgent,
			Content:   "Message 2",
			Timestamp: time.Now(),
		},
	}

	cv.SetMessages(messages)

	assert.Len(t, cv.messages, 2)
	assert.Equal(t, messages[0], cv.messages[0])
	assert.Equal(t, messages[1], cv.messages[1])

	// SetMessages should replace, not append
	newMessages := []*client.Message{
		{
			SessionID: "test-session",
			Type:      client.MessageTypeTool,
			Content:   "New message",
			Timestamp: time.Now(),
		},
	}

	cv.SetMessages(newMessages)

	assert.Len(t, cv.messages, 1)
	assert.Equal(t, newMessages[0], cv.messages[0])
}

func TestChatView_FormatMessage(t *testing.T) {
	cv := NewChatView(80, 24, theme.DefaultTheme)

	tests := []struct {
		name     string
		msgType  client.MessageType
		content  string
		wantIcon string
	}{
		{
			name:     "User message",
			msgType:  client.MessageTypeUser,
			content:  "Hello!",
			wantIcon: "👤",
		},
		{
			name:     "Agent message",
			msgType:  client.MessageTypeAgent,
			content:  "Hi there!",
			wantIcon: "🤖",
		},
		{
			name:     "Tool message",
			msgType:  client.MessageTypeTool,
			content:  "Processing...",
			wantIcon: "🔧",
		},
		{
			name:     "Error message",
			msgType:  client.MessageTypeError,
			content:  "Something went wrong",
			wantIcon: "⚠️",
		},
		{
			name:     "System message",
			msgType:  client.MessageTypeSystem,
			content:  "Connection established",
			wantIcon: "ℹ️",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &client.Message{
				SessionID: "test-session",
				Type:      tt.msgType,
				Content:   tt.content,
				Timestamp: time.Now(),
			}

			formatted := cv.formatMessage(msg)

			// Check that it contains the icon
			assert.Contains(t, formatted, tt.wantIcon)

			// Check that it contains the content
			assert.Contains(t, formatted, tt.content)

			// Check that it contains a timestamp (hour:minute format)
			assert.Regexp(t, `\d{2}:\d{2}`, formatted)
		})
	}
}

func TestChatView_EmptyState(t *testing.T) {
	cv := NewChatView(80, 24, theme.DefaultTheme)

	view := cv.View()

	assert.Contains(t, view, "No messages yet")
}

func TestChatView_ViewWithMessages(t *testing.T) {
	cv := NewChatView(80, 24, theme.DefaultTheme)

	messages := []*client.Message{
		{
			SessionID: "test-session",
			Type:      client.MessageTypeUser,
			Content:   "Hello!",
			Timestamp: time.Now(),
		},
		{
			SessionID: "test-session",
			Type:      client.MessageTypeAgent,
			Content:   "Hi there!",
			Timestamp: time.Now(),
		},
	}

	cv.SetMessages(messages)

	view := cv.View()

	// View should contain both messages
	assert.Contains(t, view, "Hello!")
	assert.Contains(t, view, "Hi there!")
	assert.Contains(t, view, "👤")
	assert.Contains(t, view, "🤖")
}

func TestChatView_SetSize(t *testing.T) {
	cv := NewChatView(80, 24, theme.DefaultTheme)

	newWidth, newHeight := 100, 30
	cv.SetSize(newWidth, newHeight)

	assert.Equal(t, newWidth, cv.width)
	assert.Equal(t, newHeight, cv.height)
}

func TestChatView_MultilineContent(t *testing.T) {
	cv := NewChatView(80, 24, theme.DefaultTheme)

	msg := &client.Message{
		SessionID: "test-session",
		Type:      client.MessageTypeAgent,
		Content:   "Line 1\nLine 2\nLine 3",
		Timestamp: time.Now(),
	}

	cv.AddMessage(msg)
	view := cv.View()

	// All lines should be present
	assert.Contains(t, view, "Line 1")
	assert.Contains(t, view, "Line 2")
	assert.Contains(t, view, "Line 3")
}

func TestChatView_LongContent(t *testing.T) {
	cv := NewChatView(80, 24, theme.DefaultTheme)

	// Create a very long message
	longContent := strings.Repeat("This is a very long message. ", 20)

	msg := &client.Message{
		SessionID: "test-session",
		Type:      client.MessageTypeUser,
		Content:   longContent,
		Timestamp: time.Now(),
	}

	cv.AddMessage(msg)

	// Should not panic
	view := cv.View()
	assert.NotEmpty(t, view)
}
