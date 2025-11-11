// ABOUTME: Update logic for the TUI (handles all messages and state transitions)
// ABOUTME: Implements the Elm architecture Update function
package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/harper/acp-relay/internal/tui/client"
)

// Custom message types for relay communication.
type RelayMessageMsg struct {
	Data []byte
}

type RelayErrorMsg struct {
	Err error
}

type RelayConnectedMsg struct{}

type RelayDisconnectedMsg struct{}

type SessionResumeResultMsg struct {
	SessionID string
	Err       error
}

// Global message ID counter for JSON-RPC requests.
var messageIDCounter uint64

//nolint:gocognit,gocyclo,funlen // TUI event handling state machine requiring many conditional branches
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
				_ = m.relayClient.Close()
			}

			// Save sessions before quitting
			dataDir := os.ExpandEnv("$HOME/.local/share/acp-tui")
			if err := m.sessionManager.Save(dataDir); err != nil {
				DebugLog("Quit: Failed to save sessions: %v", err)
			} else {
				DebugLog("Quit: Saved %d sessions", len(m.sessionManager.List()))
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
		DebugLog("Update: RelayMessageMsg - received %d bytes: %s", len(msg.Data), string(msg.Data))

		// Parse JSON-RPC response
		var response map[string]interface{}
		if err := json.Unmarshal(msg.Data, &response); err != nil {
			DebugLog("Update: RelayMessageMsg - JSON parse error: %v", err)
			// Add error message
			if m.activeSessionID != "" {
				errMsg := &client.Message{
					SessionID: m.activeSessionID,
					Type:      client.MessageTypeError,
					Content:   "Failed to parse server response: " + err.Error(),
					Timestamp: time.Now(),
				}
				m.messageStore.AddMessage(errMsg)
				m = m.updateChatView()
			}
			return m, m.waitForRelayMessage()
		}

		// Check if this is a response (has result/error) or notification (has method)
		//nolint:nestif // message routing requires nested checks for different response types
		if method, hasMethod := response["method"].(string); hasMethod {
			// This is a notification
			params, _ := response["params"].(map[string]interface{})
			DebugLog("Update: RelayMessageMsg - notification method=%s", method)

			switch method {
			case "session/chunk":
				// Agent is streaming a response chunk
				if m.activeSessionID != "" {
					if content, ok := params["content"].(string); ok {
						agentMsg := &client.Message{
							SessionID: m.activeSessionID,
							Type:      client.MessageTypeAgent,
							Content:   content,
							Timestamp: time.Now(),
						}
						m.messageStore.AddMessage(agentMsg)
						m = m.updateChatView()
					}
				}

			case "session/complete":
				// Agent finished responding
				DebugLog("Update: session/complete received")

			default:
				// Unknown notification - log it
				if m.activeSessionID != "" {
					sysMsg := &client.Message{
						SessionID: m.activeSessionID,
						Type:      client.MessageTypeSystem,
						Content:   fmt.Sprintf("[%s] %s", method, string(msg.Data)),
						Timestamp: time.Now(),
					}
					m.messageStore.AddMessage(sysMsg)
					m = m.updateChatView()
				}
			}
		} else if result, hasResult := response["result"]; hasResult {
			// This is a successful response
			DebugLog("Update: RelayMessageMsg - response with result")

			// Check if it's a session/new response
			if resultMap, ok := result.(map[string]interface{}); ok {
				if sessionID, ok := resultMap["sessionId"].(string); ok {
					// Session created successfully!
					DebugLog("Update: Session created with ID: %s", sessionID)

					// Create local session
					workingDir := m.config.Sessions.DefaultWorkingDir
					sessions := m.sessionManager.List()
					displayName := fmt.Sprintf("Session %d", len(sessions)+1)

					sess, err := m.sessionManager.Create(sessionID, workingDir, displayName)
					if err != nil {
						DebugLog("Update: Failed to create local session: %v", err)
					} else {
						// Update sidebar
						m = m.refreshSidebar()

						// Make it active
						m.activeSessionID = sess.ID
						m.statusBar.SetActiveSession(sess.DisplayName)

						// Add welcome message
						welcomeMsg := &client.Message{
							SessionID: sessionID,
							Type:      client.MessageTypeSystem,
							Content:   fmt.Sprintf("Session created: %s\nWorking directory: %s", displayName, workingDir),
							Timestamp: time.Now(),
						}
						m.messageStore.AddMessage(welcomeMsg)
						m = m.updateChatView()

						// Save sessions
						dataDir := os.ExpandEnv("$HOME/.local/share/acp-tui")
						if err := m.sessionManager.Save(dataDir); err != nil {
							DebugLog("Update: Failed to save sessions: %v", err)
						}
					}
				}
			}
		} else if errorData, hasError := response["error"]; hasError {
			// This is an error response
			DebugLog("Update: RelayMessageMsg - response with error: %v", errorData)

			if errorMap, ok := errorData.(map[string]interface{}); ok {
				errorMsg := fmt.Sprintf("Error: %v", errorMap["message"])
				if m.activeSessionID != "" {
					msg := &client.Message{
						SessionID: m.activeSessionID,
						Type:      client.MessageTypeError,
						Content:   errorMsg,
						Timestamp: time.Now(),
					}
					m.messageStore.AddMessage(msg)
					m = m.updateChatView()
				}
			}
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

	case SessionResumeResultMsg:
		// Handle session resume result
		//nolint:nestif // session resume requires nested error handling and database operations
		if msg.Err != nil {
			// Resume failed - show error and create new session as fallback
			DebugLog("Update: SessionResumeResultMsg - resume failed: %v", msg.Err)

			// Add error message to display
			if m.activeSessionID != "" {
				errMsg := &client.Message{
					SessionID: m.activeSessionID,
					Type:      client.MessageTypeError,
					Content:   fmt.Sprintf("Failed to resume session: %s. Creating new session...", msg.Err.Error()),
					Timestamp: time.Now(),
				}
				m.messageStore.AddMessage(errMsg)
				m = m.updateChatView()
			}

			// Fallback: create new session
			m = m.onCreateSession()
		} else {
			// Resume successful - load history from database
			DebugLog("Update: SessionResumeResultMsg - resume succeeded for session %s", msg.SessionID)

			// Set as active session
			m.activeSessionID = msg.SessionID

			// Find the session in sidebar and update display name
			sessions := m.sessionManager.List()
			for _, sess := range sessions {
				if sess.ID == msg.SessionID {
					m.statusBar.SetActiveSession(sess.DisplayName)
					break
				}
			}

			// Load message history from database if available
			if m.dbClient != nil {
				messages, err := m.dbClient.GetSessionMessages(msg.SessionID)
				if err != nil {
					DebugLog("Update: SessionResumeResultMsg - failed to load history: %v", err)
				} else {
					DebugLog("Update: SessionResumeResultMsg - loaded %d messages from database", len(messages))
					// Add messages to store
					for _, historyMsg := range messages {
						m.messageStore.AddMessage(historyMsg)
					}
				}
			}

			// Add success message
			successMsg := &client.Message{
				SessionID: msg.SessionID,
				Type:      client.MessageTypeSystem,
				Content:   fmt.Sprintf("Session resumed: %s", msg.SessionID),
				Timestamp: time.Now(),
			}
			m.messageStore.AddMessage(successMsg)

			// Update chat view with loaded history
			m = m.updateChatView()
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

// updateComponentSizes recalculates and applies sizes to all components based on window dimensions.
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

// cycleFocus moves focus to the next component.
func (m *Model) cycleFocus() {
	// Blur current component
	if m.focusedArea == FocusInputArea {
		m.inputArea.Blur()
	}

	// Move to next focus area
	m.focusedArea = (m.focusedArea + 1) % 3

	// Skip sidebar if not visible
	if m.focusedArea == FocusSidebar && !m.sidebarVisible {
		m.focusedArea = FocusChatView
	}

	// Focus new component
	if m.focusedArea == FocusInputArea {
		m.inputArea.Focus()
	}
}

// handleFocusedInput routes key messages to the currently focused component.
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
		case "enter": //nolint:goconst // key string used in specific switch case
			m = m.onSessionSelect()
		case "n":
			m = m.onCreateSession()
		case "d":
			// TODO: Delete session
		case "r":
			// Resume selected session
			return m, m.onResumeSession()
		}

	case FocusChatView:
		// ChatView handles its own scrolling via viewport
		_, cmd = m.chatView.Update(msg)

	case FocusInputArea:
		// Check if Enter should send message (Shift+Enter will still insert newline)
		keyStr := msg.String()
		sendOnEnter := m.config.Input.SendOnEnter
		DebugLog("handleFocusedInput: key='%s', sendOnEnter=%v, match=%v", keyStr, sendOnEnter, keyStr == "enter")

		if keyStr == "enter" && sendOnEnter {
			DebugLog("handleFocusedInput: Enter pressed in InputArea, calling onSendMessage")
			m = m.onSendMessage()
		} else {
			DebugLog("handleFocusedInput: Passing key '%s' to InputArea", keyStr)
			_, cmd = m.inputArea.Update(msg)
		}
	}

	return m, cmd
}

// onSessionSelect updates the active session and loads its messages.
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
//
//nolint:funlen // message sending with protocol handling
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
		// Construct JSON-RPC 2.0 request
		msgID := atomic.AddUint64(&messageIDCounter, 1)

		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "session/prompt",
			"params": map[string]interface{}{
				"sessionId": m.activeSessionID,
				"content": []map[string]string{
					{
						"type": "text",
						"text": content,
					},
				},
			},
			"id": msgID,
		}

		jsonMsg, err := json.Marshal(request)
		if err != nil {
			DebugLog("onSendMessage: JSON marshal failed: %v", err)
			errMsg := &client.Message{
				SessionID: m.activeSessionID,
				Type:      client.MessageTypeError,
				Content:   "Failed to encode message: " + err.Error(),
				Timestamp: time.Now(),
			}
			m.messageStore.AddMessage(errMsg)
			m = m.updateChatView()
			return m
		}

		DebugLog("onSendMessage: Sending JSON-RPC request (id=%d, session=%s): %s", msgID, m.activeSessionID, string(jsonMsg))

		if err := m.relayClient.Send(jsonMsg); err != nil {
			DebugLog("onSendMessage: Send failed: %v", err)
			errMsg := &client.Message{
				SessionID: m.activeSessionID,
				Type:      client.MessageTypeError,
				Content:   "Failed to send: " + err.Error(),
				Timestamp: time.Now(),
			}
			m.messageStore.AddMessage(errMsg)
			m = m.updateChatView()
		} else {
			DebugLog("onSendMessage: Message sent successfully (id=%d)", msgID)
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

// updateChatView refreshes the chat view with current session messages.
func (m Model) updateChatView() Model {
	if m.activeSessionID == "" {
		m.chatView.SetMessages([]*client.Message{})
		return m
	}

	messages := m.messageStore.GetMessages(m.activeSessionID)
	m.chatView.SetMessages(messages)
	return m
}

// refreshSidebar updates the sidebar with current session list.
func (m Model) refreshSidebar() Model {
	sessions := m.sessionManager.List()
	m.sidebar.SetSessions(sessions)
	return m
}

// onCreateSession creates a new session on the relay server.
func (m Model) onCreateSession() Model {
	if !m.relayClient.IsConnected() {
		DebugLog("onCreateSession: Not connected to relay")
		// Could show error message
		return m
	}

	// Use config default working directory
	workingDir := m.config.Sessions.DefaultWorkingDir

	// Call relay server to create session
	msgID := atomic.AddUint64(&messageIDCounter, 1)

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/new",
		"params": map[string]interface{}{
			"workingDirectory": workingDir,
		},
		"id": msgID,
	}

	jsonMsg, err := json.Marshal(request)
	if err != nil {
		DebugLog("onCreateSession: JSON marshal failed: %v", err)
		return m
	}

	DebugLog("onCreateSession: Sending session/new request (id=%d): %s", msgID, string(jsonMsg))

	if err := m.relayClient.Send(jsonMsg); err != nil {
		DebugLog("onCreateSession: Send failed: %v", err)
		return m
	}

	// Store the display name temporarily so we can use it when we get the response
	// For now, we'll create the local session when we receive the response
	// TODO: Store pending session creation state

	return m
}

// onResumeSession attempts to resume the selected session from the sidebar.
func (m Model) onResumeSession() tea.Cmd {
	sess := m.sidebar.GetSelectedSession()
	if sess == nil {
		DebugLog("onResumeSession: No session selected")
		return nil
	}

	sessionID := sess.ID
	DebugLog("onResumeSession: Attempting to resume session %s", sessionID)

	if !m.relayClient.IsConnected() {
		DebugLog("onResumeSession: Not connected to relay")
		return func() tea.Msg {
			return SessionResumeResultMsg{
				SessionID: sessionID,
				Err:       fmt.Errorf("not connected to relay server"),
			}
		}
	}

	// Return a command that calls ResumeSession in a goroutine
	return func() tea.Msg {
		err := m.relayClient.ResumeSession(sessionID)
		return SessionResumeResultMsg{
			SessionID: sessionID,
			Err:       err,
		}
	}
}
