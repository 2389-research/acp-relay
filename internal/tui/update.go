// ABOUTME: Update logic for the TUI (handles all messages and state transitions)
// ABOUTME: Implements the Elm architecture Update function
package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/harper/acp-relay/internal/tui/client"
)

// Custom message types for relay communication
type RelayMessageMsg struct {
	Data []byte
}

type RelayErrorMsg struct {
	Err error
}

type RelayConnectedMsg struct{}

type RelayDisconnectedMsg struct{}

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

	case RelayConnectedMsg:
		// Connection established, update status and start listening
		DebugLog("Update: RelayConnectedMsg - connection established")
		m.statusBar.SetConnectionStatus("connected")
		return m, m.waitForRelayMessage()

	case RelayMessageMsg:
		// Handle incoming WebSocket messages
		DebugLog("Update: RelayMessageMsg - received %d bytes", len(msg.Data))
		// TODO: Parse JSON-RPC response and route appropriately
		// For now, add as system message
		if m.activeSessionID != "" {
			sysMsg := &client.Message{
				SessionID: m.activeSessionID,
				Type:      client.MessageTypeSystem,
				Content:   string(msg.Data),
				Timestamp: time.Now(),
			}
			m.messageStore.AddMessage(sysMsg)
			m = m.updateChatView()
		} else {
			DebugLog("Update: RelayMessageMsg - no active session, message ignored")
		}
		// Continue listening for more messages
		return m, m.waitForRelayMessage()

	case RelayErrorMsg:
		// Handle WebSocket errors
		DebugLog("Update: RelayErrorMsg - %v", msg.Err)
		m.statusBar.SetConnectionStatus("disconnected")

		// Add error message to message store
		if m.activeSessionID != "" {
			errMsg := &client.Message{
				SessionID: m.activeSessionID,
				Type:      client.MessageTypeError,
				Content:   msg.Err.Error(),
				Timestamp: time.Now(),
			}
			m.messageStore.AddMessage(errMsg)
			m = m.updateChatView()
		}
		return m, nil

	case RelayDisconnectedMsg:
		// Handle disconnection
		DebugLog("Update: RelayDisconnectedMsg - connection closed")
		m.statusBar.SetConnectionStatus("disconnected")
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
			m = m.onSessionSelect()
		case "down", "j":
			m.sidebar.CursorDown()
			m = m.onSessionSelect()
		case "enter":
			m = m.onSessionSelect()
		case "n":
			m = m.onCreateSession()
		case "d":
			// TODO: Delete session
		case "r":
			// TODO: Rename session
		}

	case FocusChatView:
		// ChatView handles its own scrolling via viewport
		_, cmd = m.chatView.Update(msg)

	case FocusInputArea:
		// Check if Enter should send message (Shift+Enter will still insert newline)
		if msg.String() == "enter" && m.config.Input.SendOnEnter {
			DebugLog("handleFocusedInput: Enter pressed in InputArea, calling onSendMessage")
			m = m.onSendMessage()
		} else {
			DebugLog("handleFocusedInput: Passing key '%s' to InputArea", msg.String())
			_, cmd = m.inputArea.Update(msg)
		}
	}

	return m, cmd
}

// onSessionSelect updates the active session and loads its messages
func (m Model) onSessionSelect() Model {
	sess := m.sidebar.GetSelectedSession()
	if sess == nil {
		return m
	}

	m.activeSessionID = sess.ID
	m.statusBar.SetActiveSession(sess.DisplayName)
	m = m.updateChatView()
	return m
}

// onSendMessage sends the input area content as a message
func (m Model) onSendMessage() Model {
	DebugLog("onSendMessage: Called (activeSessionID=%s, focusedArea=%d)", m.activeSessionID, m.focusedArea)

	if m.activeSessionID == "" {
		DebugLog("onSendMessage: No active session, cannot send")
		return m
	}

	content := m.inputArea.GetValue()
	if content == "" {
		DebugLog("onSendMessage: Empty content, ignoring")
		return m
	}

	DebugLog("onSendMessage: Sending message (length=%d)", len(content))

	// Add user message to store
	userMsg := &client.Message{
		SessionID: m.activeSessionID,
		Type:      client.MessageTypeUser,
		Content:   content,
		Timestamp: time.Now(),
	}
	m.messageStore.AddMessage(userMsg)

	// Clear input
	m.inputArea.Clear()

	// Update chat view
	m = m.updateChatView()

	// Send message to relay server
	if m.relayClient.IsConnected() {
		DebugLog("onSendMessage: Sending message to relay (session=%s): %s", m.activeSessionID, content)
		// TODO: Construct proper JSON-RPC request
		// For now, send raw content
		jsonMsg := []byte(content)
		if err := m.relayClient.Send(jsonMsg); err != nil {
			DebugLog("onSendMessage: Send failed: %v", err)
			// Add error message
			errMsg := &client.Message{
				SessionID: m.activeSessionID,
				Type:      client.MessageTypeError,
				Content:   "Failed to send: " + err.Error(),
				Timestamp: time.Now(),
			}
			m.messageStore.AddMessage(errMsg)
			m = m.updateChatView()
		} else {
			DebugLog("onSendMessage: Message sent successfully")
		}
	} else {
		DebugLog("onSendMessage: Not connected, cannot send message")
		// Not connected, add warning
		warnMsg := &client.Message{
			SessionID: m.activeSessionID,
			Type:      client.MessageTypeError,
			Content:   "Not connected to relay server",
			Timestamp: time.Now(),
		}
		m.messageStore.AddMessage(warnMsg)
		m = m.updateChatView()
	}

	return m
}

// updateChatView refreshes the chat view with current session messages
func (m Model) updateChatView() Model {
	if m.activeSessionID == "" {
		m.chatView.SetMessages([]*client.Message{})
		return m
	}

	messages := m.messageStore.GetMessages(m.activeSessionID)
	m.chatView.SetMessages(messages)
	return m
}

// refreshSidebar updates the sidebar with current session list
func (m Model) refreshSidebar() Model {
	sessions := m.sessionManager.List()
	m.sidebar.SetSessions(sessions)
	return m
}

// onCreateSession creates a new session and makes it active
func (m Model) onCreateSession() Model {
	// Generate unique session ID
	sessionID := "sess_" + uuid.New().String()[:8]

	// Use config default working directory
	workingDir := m.config.Sessions.DefaultWorkingDir

	// Create display name with counter
	sessions := m.sessionManager.List()
	displayName := fmt.Sprintf("Session %d", len(sessions)+1)

	// Create the session
	sess, err := m.sessionManager.Create(sessionID, workingDir, displayName)
	if err != nil {
		DebugLog("onCreateSession: Failed to create session: %v", err)
		return m
	}

	DebugLog("onCreateSession: Created session %s (%s)", sess.ID, sess.DisplayName)

	// Update sidebar with new session list
	m = m.refreshSidebar()

	// Make it the active session
	m.activeSessionID = sess.ID
	m.statusBar.SetActiveSession(sess.DisplayName)

	// Clear chat view for new session
	m = m.updateChatView()

	// Add welcome message
	welcomeMsg := &client.Message{
		SessionID: sessionID,
		Type:      client.MessageTypeSystem,
		Content:   fmt.Sprintf("Session created: %s\nWorking directory: %s", displayName, workingDir),
		Timestamp: time.Now(),
	}
	m.messageStore.AddMessage(welcomeMsg)
	m = m.updateChatView()

	return m
}
