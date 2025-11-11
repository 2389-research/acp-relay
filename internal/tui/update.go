// ABOUTME: Update logic for the TUI (handles all messages and state transitions)
// ABOUTME: Implements the Elm architecture Update function
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/harper/acp-relay/internal/tui/client"
)

// Custom message types for relay communication
type RelayMessageMsg struct {
	Data []byte
}

type RelayErrorMsg struct {
	Err error
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateComponentSizes()
		return m, nil

	case tea.KeyMsg:
		// Help overlay gets priority
		if m.helpOverlay.IsVisible() {
			if msg.String() == "?" || msg.String() == "esc" {
				m.helpOverlay.Toggle()
			}
			return m, nil
		}

		// Global shortcuts
		switch msg.String() {
		case "ctrl+c", "q":
			// Close relay client before quitting
			if m.relayClient != nil {
				m.relayClient.Close()
			}
			return m, tea.Quit

		case "?":
			m.helpOverlay.Toggle()
			return m, nil

		case "ctrl+b":
			m.sidebarVisible = !m.sidebarVisible
			m.updateComponentSizes()
			return m, nil

		case "tab":
			m.cycleFocus()
			return m, nil
		}

		// Route to focused component
		return m.handleFocusedInput(msg)

	case RelayMessageMsg:
		// Handle incoming WebSocket messages
		// TODO: Parse and route to appropriate handler
		return m, nil

	case RelayErrorMsg:
		// Handle WebSocket errors
		// Add error message to message store
		if m.activeSessionID != "" {
			errMsg := &client.Message{
				SessionID: m.activeSessionID,
				Type:      client.MessageTypeError,
				Content:   msg.Err.Error(),
				Timestamp: time.Now(),
			}
			m.messageStore.AddMessage(errMsg)
			m.updateChatView()
		}
		return m, nil
	}

	// Update components that need to receive all messages (like viewport scrolling)
	if m.focusedArea == FocusChatView {
		_, cmd = m.chatView.Update(msg)
		cmds = append(cmds, cmd)
	}

	if m.focusedArea == FocusInputArea {
		_, cmd = m.inputArea.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// updateComponentSizes recalculates and applies sizes to all components based on window dimensions
func (m *Model) updateComponentSizes() {
	if m.width == 0 || m.height == 0 {
		return
	}

	// Reserve space for status bar (1 line)
	statusBarHeight := 1
	availableHeight := m.height - statusBarHeight

	// Calculate sidebar width
	sidebarWidth := 0
	if m.sidebarVisible {
		sidebarWidth = m.width / 4
		if sidebarWidth < 25 {
			sidebarWidth = 25
		}
		if sidebarWidth > 40 {
			sidebarWidth = 40
		}
	}

	// Calculate main area dimensions
	mainWidth := m.width - sidebarWidth
	inputAreaHeight := 5
	if inputAreaHeight > availableHeight/3 {
		inputAreaHeight = availableHeight / 3
	}
	chatViewHeight := availableHeight - inputAreaHeight

	// Update component sizes
	if m.sidebarVisible {
		m.sidebar.SetSize(sidebarWidth, availableHeight)
	}
	m.chatView.SetSize(mainWidth, chatViewHeight)
	m.inputArea.SetSize(mainWidth, inputAreaHeight)
	m.statusBar.SetSize(m.width)
	m.helpOverlay.SetSize(m.width, m.height)
}

// cycleFocus moves focus to the next component
func (m *Model) cycleFocus() {
	// Blur current component
	switch m.focusedArea {
	case FocusInputArea:
		m.inputArea.Blur()
	}

	// Move to next focus area
	m.focusedArea = (m.focusedArea + 1) % 3

	// Skip sidebar if not visible
	if m.focusedArea == FocusSidebar && !m.sidebarVisible {
		m.focusedArea = FocusChatView
	}

	// Focus new component
	switch m.focusedArea {
	case FocusInputArea:
		m.inputArea.Focus()
	}
}

// handleFocusedInput routes key messages to the currently focused component
func (m Model) handleFocusedInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.focusedArea {
	case FocusSidebar:
		switch msg.String() {
		case "up", "k":
			m.sidebar.CursorUp()
			m.onSessionSelect()
		case "down", "j":
			m.sidebar.CursorDown()
			m.onSessionSelect()
		case "enter":
			m.onSessionSelect()
		case "n":
			// TODO: Create new session
		case "d":
			// TODO: Delete session
		case "r":
			// TODO: Rename session
		}

	case FocusChatView:
		// ChatView handles its own scrolling via viewport
		_, cmd = m.chatView.Update(msg)

	case FocusInputArea:
		switch msg.String() {
		case "ctrl+s":
			m.onSendMessage()
		default:
			_, cmd = m.inputArea.Update(msg)
		}
	}

	return m, cmd
}

// onSessionSelect updates the active session and loads its messages
func (m *Model) onSessionSelect() {
	sess := m.sidebar.GetSelectedSession()
	if sess == nil {
		return
	}

	m.activeSessionID = sess.ID
	m.statusBar.SetActiveSession(sess.DisplayName)
	m.updateChatView()
}

// onSendMessage sends the input area content as a message
func (m *Model) onSendMessage() {
	if m.activeSessionID == "" {
		return
	}

	content := m.inputArea.GetValue()
	if content == "" {
		return
	}

	// Add user message to store
	msg := &client.Message{
		SessionID: m.activeSessionID,
		Type:      client.MessageTypeUser,
		Content:   content,
		Timestamp: time.Now(),
	}
	m.messageStore.AddMessage(msg)

	// Clear input
	m.inputArea.Clear()

	// Update chat view
	m.updateChatView()

	// TODO: Send message to relay server
}

// updateChatView refreshes the chat view with current session messages
func (m *Model) updateChatView() {
	if m.activeSessionID == "" {
		m.chatView.SetMessages([]*client.Message{})
		return
	}

	messages := m.messageStore.GetMessages(m.activeSessionID)
	m.chatView.SetMessages(messages)
}
