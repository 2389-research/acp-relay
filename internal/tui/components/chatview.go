// ABOUTME: ChatView component for displaying messages with scrolling
// ABOUTME: Uses bubbles viewport for scrolling and formats messages with icons
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/harper/acp-relay/internal/tui/client"
	"github.com/harper/acp-relay/internal/tui/theme"
)

type ChatView struct {
	width    int
	height   int
	theme    theme.Theme
	viewport viewport.Model
	messages []*client.Message
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
	cv.updateViewport()
}

func (cv *ChatView) AddMessage(msg *client.Message) {
	cv.messages = append(cv.messages, msg)
	cv.updateViewport()
	cv.scrollToBottom()
}

func (cv *ChatView) formatMessage(msg *client.Message) string {
	var sb strings.Builder

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
	if len(cv.messages) == 0 {
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

	cv.viewport.SetContent(sb.String())
}

func (cv *ChatView) scrollToBottom() {
	cv.viewport.GotoBottom()
}

func (cv *ChatView) View() string {
	if len(cv.messages) == 0 {
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
