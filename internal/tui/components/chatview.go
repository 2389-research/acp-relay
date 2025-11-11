// ABOUTME: ChatView component for displaying messages with scrolling
// ABOUTME: Uses bubbles viewport for scrolling and formats messages with icons
package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/harper/acp-relay/internal/tui/client"
	"github.com/harper/acp-relay/internal/tui/theme"
)

type ChatView struct {
	width       int
	height      int
	theme       theme.Theme
	viewport    viewport.Model
	messages    []*client.Message
	agentTyping bool
	typingText  string
	sessionID   string // For creating messages when typing stops
}

func NewChatView(width, height int, t theme.Theme) *ChatView {
	vp := viewport.New(width, height)
	vp.Style = t.ChatViewStyle()

	return &ChatView{
		width:    width,
		height:   height,
		theme:    t,
		viewport: vp,
		messages: []*client.Message{},
	}
}

func (cv *ChatView) SetMessages(messages []*client.Message) {
	cv.messages = messages
	// Update sessionID from messages if available
	if len(messages) > 0 {
		cv.sessionID = messages[0].SessionID
	}
	cv.updateViewport()
}

func (cv *ChatView) AddMessage(msg *client.Message) {
	cv.messages = append(cv.messages, msg)
	// Update sessionID from message
	if cv.sessionID == "" {
		cv.sessionID = msg.SessionID
	}
	cv.updateViewport()
	cv.scrollToBottom()
}

func (cv *ChatView) StartTyping() {
	cv.agentTyping = true
	cv.typingText = ""
	cv.updateViewport()
}

func (cv *ChatView) UpdateTyping(text string) {
	cv.typingText = text
	cv.updateViewport()
}

func (cv *ChatView) StopTyping() {
	cv.agentTyping = false
	// Add final message if there's typing text
	if cv.typingText != "" {
		msg := &client.Message{
			SessionID: cv.sessionID,
			Type:      client.MessageTypeAgent,
			Content:   cv.typingText,
			Timestamp: time.Now(),
		}
		cv.messages = append(cv.messages, msg)
		cv.typingText = ""
		cv.updateViewport()
		cv.scrollToBottom()
	}
}

func (cv *ChatView) formatMessage(msg *client.Message) string {
	var sb strings.Builder

	// Handle permission messages specially using the permission widget
	if msg.Type == client.MessageTypePermissionRequest {
		return FormatPermissionRequest(msg, cv.theme)
	}
	if msg.Type == client.MessageTypePermissionResponse {
		return FormatPermissionResponse(msg, cv.theme)
	}

	// Handle system messages using the system message widget
	if msg.Type == client.MessageTypeAvailableCommands ||
		msg.Type == client.MessageTypeToolUse ||
		msg.Type == client.MessageTypeThinking ||
		msg.Type == client.MessageTypeThoughtChunk {
		return FormatSystemMessage(msg, cv.theme)
	}

	// Icon and timestamp
	icon := msg.Type.Icon()
	timestamp := msg.Timestamp.Format("15:04")
	timestampStyled := cv.theme.DimStyle().Render(timestamp)

	// Build the header line: icon + timestamp
	header := fmt.Sprintf("%s %s", icon, timestampStyled)

	sb.WriteString(header)
	sb.WriteString("\n")

	// Format content based on message type
	var contentStyle = cv.theme.ChatViewStyle()

	switch msg.Type {
	case client.MessageTypeUser:
		contentStyle = contentStyle.Foreground(cv.theme.UserMsg)
	case client.MessageTypeAgent:
		contentStyle = contentStyle.Foreground(cv.theme.AgentMsg)
	case client.MessageTypeError:
		contentStyle = cv.theme.ErrorStyle()
	case client.MessageTypeSystem:
		contentStyle = cv.theme.DimStyle()
	}

	// Render content
	content := contentStyle.Render(msg.Content)
	sb.WriteString(content)
	sb.WriteString("\n")

	return sb.String()
}

func (cv *ChatView) updateViewport() {
	if len(cv.messages) == 0 && !cv.agentTyping {
		cv.viewport.SetContent(cv.theme.DimStyle().Render("No messages yet"))
		return
	}

	var sb strings.Builder
	for i, msg := range cv.messages {
		sb.WriteString(cv.formatMessage(msg))
		// Add spacing between messages
		if i < len(cv.messages)-1 {
			sb.WriteString("\n")
		}
	}

	// Add typing indicator if agent is typing
	if cv.agentTyping {
		if len(cv.messages) > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(cv.renderTypingIndicator())
	}

	cv.viewport.SetContent(sb.String())
}

// renderTypingIndicator renders the typing indicator with blinking cursor.
func (cv *ChatView) renderTypingIndicator() string {
	// Create blinking cursor style using Lipgloss
	cursor := lipgloss.NewStyle().Blink(true).Render("▊")

	typingLine := fmt.Sprintf("%s %s", cv.typingText, cursor)

	// Style with agent color
	styled := cv.theme.ChatViewStyle().
		Foreground(cv.theme.AgentMsg).
		Render(typingLine)

	return styled
}

func (cv *ChatView) scrollToBottom() {
	cv.viewport.GotoBottom()
}

func (cv *ChatView) View() string {
	if len(cv.messages) == 0 && !cv.agentTyping {
		return cv.theme.ChatViewStyle().
			Width(cv.width - 2).
			Height(cv.height - 2).
			Render(cv.theme.DimStyle().Render("No messages yet"))
	}

	return cv.viewport.View()
}

func (cv *ChatView) SetSize(width, height int) {
	cv.width = width
	cv.height = height
	cv.viewport.Width = width
	cv.viewport.Height = height
	cv.updateViewport()
}

func (cv *ChatView) Init() tea.Cmd {
	return nil
}

func (cv *ChatView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	cv.viewport, cmd = cv.viewport.Update(msg)
	return cv, cmd
}
