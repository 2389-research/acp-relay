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
	"github.com/harper/acp-relay/clients/tui/client"
	"github.com/harper/acp-relay/clients/tui/screens"
	"github.com/harper/acp-relay/clients/tui/theme"
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

// Session selection modal message types.
type showSessionSelectionMsg struct {
	Sessions []client.ManagementSession
}

type sessionSelectedMsg struct {
	Session client.ManagementSession
}

type createNewSessionMsg struct{}

type createNewSessionAfterConnectMsg struct {
	retryCount int
}

type resumeSessionAfterConnectMsg struct {
	Session    client.ManagementSession
	retryCount int
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

		// Also update modal if visible
		if m.sessionModal != nil {
			_, cmd = m.sessionModal.Update(msg)
			cmds = append(cmds, cmd)
		}

		return m, tea.Batch(cmds...)

	case showSessionSelectionMsg:
		// Show session selection modal
		DebugLog("Update: showSessionSelectionMsg - showing modal with %d sessions", len(msg.Sessions))

		// Create modal with current dimensions
		m.sessionModal = createSessionSelectionScreen(msg.Sessions, m.width, m.height, m.theme)

		// Call Init() on the modal to get any initialization commands and trigger redraw
		cmd := m.sessionModal.Init()
		return m, cmd

	case sessionSelectedMsg:
		// User selected a session from modal
		DebugLog("Update: sessionSelectedMsg - session selected: %s (active=%v)", msg.Session.ID, msg.Session.IsActive)

		// Close modal
		m.sessionModal = nil

		// If session is closed, load history in read-only mode (no connection needed)
		if !msg.Session.IsActive {
			DebugLog("Update: Session is closed, loading history in read-only mode")
			m.activeSessionID = msg.Session.ID
			m.readOnlyMode = true

			// Disable input area
			m.inputArea.SetDisabled(true)

			// Set status bar to read-only mode
			m.statusBar.SetReadOnlyMode(true)

			// Note: Message history for closed sessions is not available in read-only mode
			// TODO: Add API endpoint to fetch message history for closed sessions

			// Update subtitle with read-only indicator
			sessionIDShort := msg.Session.ID
			if len(sessionIDShort) > 12 {
				sessionIDShort = sessionIDShort[:12]
			}
			m.statusBar.SetActiveSession(sessionIDShort + " (Read-Only)")

			// Update chat view
			m = m.updateChatView()

			return m, nil
		}

		// Session is active - need to connect to relay first
		if !m.relayClient.IsConnected() {
			DebugLog("Update: Connecting to relay before resuming session")
			return m, tea.Batch(
				m.connectToRelay(),
				// After connection, resume session
				func() tea.Msg {
					return resumeSessionAfterConnectMsg{
						Session:    msg.Session,
						retryCount: 0,
					}
				},
			)
		}

		// Already connected, resume session immediately
		DebugLog("Update: Session is active, attempting resume")
		return m, func() tea.Msg {
			err := m.relayClient.ResumeSession(msg.Session.ID)
			return SessionResumeResultMsg{
				SessionID: msg.Session.ID,
				Err:       err,
			}
		}

	case createNewSessionMsg:
		// User wants to create a new session
		DebugLog("Update: createNewSessionMsg - creating new session")

		// Close modal
		m.sessionModal = nil

		// Connect to relay and create session
		if !m.relayClient.IsConnected() {
			DebugLog("Update: Connecting to relay before creating session")
			return m, tea.Batch(
				m.connectToRelay(),
				// Send message to create session after connection completes
				func() tea.Msg {
					return createNewSessionAfterConnectMsg{retryCount: 0}
				},
			)
		}

		// Already connected, create session immediately
		m = m.onCreateSession()
		return m, m.waitForRelayMessage()

	case createNewSessionAfterConnectMsg:
		// Wait for connection to be established before creating session
		if !m.relayClient.IsConnected() {
			// Check if we've exceeded max retries (20 retries = 1 second)
			if msg.retryCount >= 20 {
				DebugLog("Update: createNewSessionAfterConnectMsg - connection timeout after %d retries", msg.retryCount)
				notifCmd := m.notifications.Show("Connection timeout - please try again", "error")
				return m, notifCmd
			}

			DebugLog("Update: createNewSessionAfterConnectMsg - waiting for connection (retry %d/20)", msg.retryCount+1)
			// Connection not ready yet, check again shortly
			return m, func() tea.Msg {
				time.Sleep(50 * time.Millisecond)
				return createNewSessionAfterConnectMsg{retryCount: msg.retryCount + 1}
			}
		}
		// Connection established, now create session
		DebugLog("Update: createNewSessionAfterConnectMsg - creating session after connection")
		m = m.onCreateSession()
		return m, m.waitForRelayMessage()

	case resumeSessionAfterConnectMsg:
		// Wait for connection to be established before resuming session
		if !m.relayClient.IsConnected() {
			// Check if we've exceeded max retries (20 retries = 1 second)
			if msg.retryCount >= 20 {
				DebugLog("Update: resumeSessionAfterConnectMsg - connection timeout after %d retries", msg.retryCount)
				notifCmd := m.notifications.Show("Connection timeout - please try again", "error")
				return m, notifCmd
			}

			DebugLog("Update: resumeSessionAfterConnectMsg - waiting for connection (retry %d/20)", msg.retryCount+1)
			// Connection not ready yet, check again shortly
			return m, func() tea.Msg {
				time.Sleep(50 * time.Millisecond)
				return resumeSessionAfterConnectMsg{
					Session:    msg.Session,
					retryCount: msg.retryCount + 1,
				}
			}
		}
		// Connection established, now resume session
		DebugLog("Update: resumeSessionAfterConnectMsg - resuming session after connection: %s", msg.Session.ID)
		return m, func() tea.Msg {
			err := m.relayClient.ResumeSession(msg.Session.ID)
			return SessionResumeResultMsg{
				SessionID: msg.Session.ID,
				Err:       err,
			}
		}

	case tea.KeyMsg:
		// If modal is visible, route all keys to it
		if m.sessionModal != nil {
			var modalModel tea.Model
			modalModel, cmd = m.sessionModal.Update(msg)
			m.sessionModal = modalModel
			return m, cmd
		}
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

			// Don't save sessions to disk - relay is the source of truth

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

		// Show success notification
		notifCmd := m.notifications.Show("Connected to relay server", "success")

		return m, tea.Batch(m.waitForRelayMessage(), notifCmd)

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

		// Track whether this message was handled
		handled := false

		// Check if this is a response (has result/error) or notification (has method)
		//nolint:nestif // message routing requires nested checks for different response types
		if method, hasMethod := response["method"].(string); hasMethod {
			// This is a notification
			params, _ := response["params"].(map[string]interface{})

			// Extract sessionId from params (all session notifications include this)
			messageSessionID, _ := params["sessionId"].(string)

			DebugLog("Update: RelayMessageMsg - notification method=%s, sessionId=%s, active=%s", method, messageSessionID, m.activeSessionID)

			switch method {
			case "session/chunk":
				// Agent is streaming a response chunk
				handled = true
				// Only process if message is for our active session
				if m.activeSessionID != "" && messageSessionID == m.activeSessionID {
					if content, ok := params["content"].(string); ok {
						// Accumulate response for typing indicator
						m.currentResponse += content

						// Update typing indicator with accumulated text
						m.chatView.UpdateTyping(m.currentResponse)

						// Advance progress bar
						m.statusBar.AdvanceProgress(2.0)
					}
				} else if m.activeSessionID != "" && messageSessionID != m.activeSessionID {
					DebugLog("Update: Ignoring session/chunk for inactive session %s (active: %s)", messageSessionID, m.activeSessionID)
				}

			case "session/complete":
				// Agent finished responding
				handled = true
				DebugLog("Update: session/complete received")

				// Only process if message is for our active session
				if m.activeSessionID != "" && messageSessionID == m.activeSessionID {
					// Stop typing indicator (adds final message)
					m.chatView.StopTyping()
					m.currentResponse = ""

					// Hide progress bar
					m.statusBar.HideProgress()
				} else if m.activeSessionID != "" && messageSessionID != m.activeSessionID {
					DebugLog("Update: Ignoring session/complete for inactive session %s (active: %s)", messageSessionID, m.activeSessionID)
				}

			case "session/update":
				// Session status update (available_commands, tool_use, thinking, thought_chunk, agent_message_chunk)
				handled = true
				DebugLog("Update: session/update received")

				// Only process if message is for our active session
				if m.activeSessionID == "" || messageSessionID != m.activeSessionID {
					if messageSessionID != "" {
						DebugLog("Update: Ignoring session/update for inactive session %s (active: %s)", messageSessionID, m.activeSessionID)
					}
					break
				}

				// Extract the update object
				if update, ok := params["update"].(map[string]interface{}); ok {
					sessionUpdate, _ := update["sessionUpdate"].(string)
					DebugLog("Update: sessionUpdate type=%s", sessionUpdate)

					switch sessionUpdate {
					case "available_commands_update":
						// Extract available commands
						if availableCommands, ok := update["availableCommands"].([]interface{}); ok {
							commands := make([]client.Command, 0, len(availableCommands))
							for _, cmd := range availableCommands {
								if cmdMap, ok := cmd.(map[string]interface{}); ok {
									name, _ := cmdMap["name"].(string)
									desc, _ := cmdMap["description"].(string)
									commands = append(commands, client.Command{
										Name:        name,
										Description: desc,
									})
								}
							}

							// Create system message with command count
							var content string
							if len(commands) <= 5 {
								// Show all command names
								cmdNames := make([]string, len(commands))
								for i, cmd := range commands {
									cmdNames[i] = cmd.Name
								}
								content = fmt.Sprintf("Commands updated: %v", cmdNames)
							} else {
								// Show count only
								content = fmt.Sprintf("%d commands available", len(commands))
							}

							if m.activeSessionID != "" {
								cmdMsg := &client.Message{
									SessionID: m.activeSessionID,
									Type:      client.MessageTypeAvailableCommands,
									Content:   content,
									Commands:  commands,
									Timestamp: time.Now(),
								}
								m.messageStore.AddMessage(cmdMsg)
								m = m.updateChatView()
							}
						}

					case "tool_use":
						// Extract tool name
						if tool, ok := update["tool"].(map[string]interface{}); ok {
							toolName, _ := tool["name"].(string)

							if m.activeSessionID != "" {
								toolMsg := &client.Message{
									SessionID: m.activeSessionID,
									Type:      client.MessageTypeToolUse,
									Content:   fmt.Sprintf("Using tool: %s", toolName),
									ToolName:  toolName,
									Timestamp: time.Now(),
								}
								m.messageStore.AddMessage(toolMsg)
								m = m.updateChatView()
							}
						}

					case "agent_thinking":
						// Create thinking indicator message
						if m.activeSessionID != "" {
							thinkingMsg := &client.Message{
								SessionID: m.activeSessionID,
								Type:      client.MessageTypeThinking,
								Content:   "Agent is thinking...",
								Timestamp: time.Now(),
							}
							m.messageStore.AddMessage(thinkingMsg)
							m = m.updateChatView()

							// Update status bar
							m.statusBar.SetStatus("Agent is thinking...")
						}

					case "agent_thought_chunk":
						// Extract thought text and accumulate
						if content, ok := update["content"].(map[string]interface{}); ok {
							if text, ok := content["text"].(string); ok {
								// Accumulate thought text
								m.currentThought += text

								// Update status bar with truncated preview (first 50 chars)
								preview := m.currentThought
								if len(preview) > 50 {
									preview = preview[:50] + "..."
								}
								m.statusBar.SetStatus(fmt.Sprintf("💭 %s", preview))

								// Advance progress bar
								m.statusBar.AdvanceProgress(1.0)
							}
						}

					case "agent_message_chunk":
						// Extract message text and accumulate for typing indicator
						if content, ok := update["content"].(map[string]interface{}); ok {
							if text, ok := content["text"].(string); ok {
								if m.activeSessionID != "" {
									// Accumulate response for typing indicator
									m.currentResponse += text

									// Update typing indicator with accumulated text
									m.chatView.UpdateTyping(m.currentResponse)

									// Advance progress bar
									m.statusBar.AdvanceProgress(2.0)
								}
							}
						}
					}
				}

			case "session/request_permission":
				// Agent requesting permission to use a tool
				handled = true
				DebugLog("Update: session/request_permission received")

				// Only process if message is for our active session
				if m.activeSessionID == "" || messageSessionID != m.activeSessionID {
					if messageSessionID != "" {
						DebugLog("Update: Ignoring session/request_permission for inactive session %s (active: %s)", messageSessionID, m.activeSessionID)
					}
					break
				}

				// Extract request ID for response
				requestID, hasID := response["id"]

				// Extract toolCall information
				if toolCall, ok := params["toolCall"].(map[string]interface{}); ok {
					toolCallID, _ := toolCall["toolCallId"].(string)
					rawInput, _ := toolCall["rawInput"].(map[string]interface{})

					// Determine tool name from rawInput or params
					toolName := "Unknown"
					if name, ok := toolCall["name"].(string); ok {
						toolName = name
					}

					// Create permission request message for display
					if m.activeSessionID != "" {
						permMsg := &client.Message{
							SessionID:  m.activeSessionID,
							Type:       client.MessageTypePermissionRequest,
							ToolCallID: toolCallID,
							Content:    toolName,
							RawInput:   rawInput,
							Timestamp:  time.Now(),
						}
						m.messageStore.AddMessage(permMsg)
						m = m.updateChatView()
					}

					// Auto-approve permission (send response)
					if hasID {
						responseMsg := map[string]interface{}{
							"jsonrpc": "2.0",
							"id":      requestID,
							"result": map[string]interface{}{
								"outcome": map[string]interface{}{
									"outcome":  "selected",
									"optionId": "allow",
								},
							},
						}

						responseJSON, err := json.Marshal(responseMsg)
						if err != nil {
							DebugLog("Update: Failed to marshal permission response: %v", err)
						} else if err := m.relayClient.Send(responseJSON); err != nil {
							DebugLog("Update: Sending permission approval: %s", string(responseJSON))
							DebugLog("Update: Failed to send permission response: %v", err)
						} else if m.activeSessionID != "" {
							// Show info notification
							_ = m.notifications.Show("Permission approved", "info")

							// Add permission response message to display
							respMsg := &client.Message{
								SessionID:  m.activeSessionID,
								Type:       client.MessageTypePermissionResponse,
								ToolCallID: toolCallID,
								Content:    toolName,
								RawInput: map[string]interface{}{
									"outcome": "allow",
								},
								Timestamp: time.Now(),
							}
							m.messageStore.AddMessage(respMsg)
							m = m.updateChatView()
						}
					}
				}

			default:
				// Unknown notification - will be handled as unhandled below
				// Don't set handled = true for unknown methods
			}
		} else if result, hasResult := response["result"]; hasResult {
			// This is a successful response
			handled = true
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

						// Show info notification
						_ = m.notifications.Show("Session created", "info")

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
			handled = true
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

		// If message was not handled and debug mode is enabled, create unhandled message
		if !handled && m.debugMode && m.activeSessionID != "" {
			m = m.handleUnhandledMessage(msg.Data, response)
		}

		// Continue listening for more messages
		return m, m.waitForRelayMessage()

	case RelayErrorMsg:
		// Handle WebSocket errors
		DebugLog("Update: RelayErrorMsg - %v", msg.Err)
		m.statusBar.SetConnectionStatus("disconnected")

		// Show error notification
		notifCmd := m.notifications.Show(fmt.Sprintf("Connection error: %s", msg.Err.Error()), "error")

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
		return m, notifCmd

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

			// Show warning notification
			_ = m.notifications.Show("Failed to resume session", "warning")

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

			// Show success notification
			_ = m.notifications.Show("Session resumed", "success")

			// Set as active session (NOT read-only)
			m.activeSessionID = msg.SessionID
			m.readOnlyMode = false
			m.inputArea.SetDisabled(false)
			m.statusBar.SetReadOnlyMode(false)

			// Find the session in sidebar and update display name
			sessions := m.sessionManager.List()
			for _, sess := range sessions {
				if sess.ID == msg.SessionID {
					m.statusBar.SetActiveSession(sess.DisplayName)
					break
				}
			}

			// Note: Message history will be populated as messages arrive from relay

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

	// Always update notifications (handles DismissNotificationMsg)
	cmd = m.notifications.Update(msg)
	cmds = append(cmds, cmd)

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
	DebugLog("onSendMessage: Called (activeSessionID=%s, focusedArea=%d, readOnlyMode=%v)", m.activeSessionID, m.focusedArea, m.readOnlyMode)

	// Prevent sending messages in read-only mode
	if m.readOnlyMode {
		DebugLog("onSendMessage: Read-only mode active, cannot send")
		return m
	}

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

	// Show progress bar and start typing indicator
	m.statusBar.ShowProgress()
	m.chatView.StartTyping()
	m.currentResponse = ""
	m.currentThought = ""

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

	// Create workspace directory if auto_create_workspace is enabled
	if m.config.Sessions.AutoCreateWorkspace {
		if err := os.MkdirAll(workingDir, 0750); err != nil {
			DebugLog("onCreateSession: Failed to create workspace directory %s: %v", workingDir, err)
			notifCmd := m.notifications.Show(fmt.Sprintf("Failed to create workspace: %s", err.Error()), "error")
			// Execute notification command inline
			_ = notifCmd()
			return m
		}
		DebugLog("onCreateSession: Ensured workspace directory exists: %s", workingDir)
	}

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

// createSessionSelectionScreen creates a session selection modal and wraps message types.
func createSessionSelectionScreen(sessions []client.ManagementSession, width, height int, th theme.Theme) tea.Model {
	// We need to import the screens package - will be added at compile time
	// This is a wrapper that converts between internal message types and screens message types
	return &sessionSelectionWrapper{
		sessions: sessions,
		width:    width,
		height:   height,
		theme:    th,
	}
}

// sessionSelectionWrapper wraps the screens.SessionSelectionScreen to convert message types.
type sessionSelectionWrapper struct {
	sessions []client.ManagementSession
	width    int
	height   int
	theme    theme.Theme
	screen   tea.Model
}

func (w *sessionSelectionWrapper) Init() tea.Cmd {
	// Import screens package and create screen
	// This will be done via direct import at the top of the file
	return nil
}

func (w *sessionSelectionWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Lazy initialization
	if w.screen == nil {
		// We'll need to add the import - for now use a placeholder
		// This will be fixed when we add the import statement
		w.screen = newSessionSelectionScreenFromUpdate(w.sessions, w.width, w.height, w.theme)
	}

	// Forward to wrapped screen
	updatedScreen, cmd := w.screen.Update(msg)
	w.screen = updatedScreen

	// Convert message types if needed
	if cmd != nil {
		return w, func() tea.Msg {
			msg := cmd()
			// Convert screens package messages to internal messages
			return convertScreensMessage(msg)
		}
	}

	return w, nil
}

func (w *sessionSelectionWrapper) View() string {
	if w.screen == nil {
		return ""
	}
	return w.screen.View()
}

// newSessionSelectionScreenFromUpdate creates a SessionSelectionScreen from the screens package.
func newSessionSelectionScreenFromUpdate(sessions []client.ManagementSession, width, height int, th theme.Theme) tea.Model {
	return screens.NewSessionSelectionScreen(sessions, width, height, th)
}

// convertScreensMessage converts screens package messages to internal TUI messages.
func convertScreensMessage(msg tea.Msg) tea.Msg {
	switch msg := msg.(type) {
	case screens.SessionSelectedMsg:
		return sessionSelectedMsg{Session: msg.Session}
	case screens.CreateNewSessionMsg:
		return createNewSessionMsg{}
	default:
		// Pass through other messages (like tea.QuitMsg)
		return msg
	}
}

// handleUnhandledMessage creates and stores an unhandled message for debug display.
func (m Model) handleUnhandledMessage(rawData []byte, response map[string]interface{}) Model {
	DebugLog("Update: Unhandled message detected: %s", string(rawData))

	// Determine message type/content for display
	messageType := extractMessageType(response)

	// Format JSON with indentation for readability
	formattedJSON := formatJSONForDisplay(rawData)

	// Create unhandled message
	unhandledMsg := &client.Message{
		SessionID: m.activeSessionID,
		Type:      client.MessageTypeUnhandled,
		Content:   messageType,
		RawJSON:   formattedJSON,
		Timestamp: time.Now(),
	}
	m.messageStore.AddMessage(unhandledMsg)
	m = m.updateChatView()
	DebugLog("Update: Added unhandled message to store: type=%s", messageType)

	return m
}

// extractMessageType extracts the message type from a JSON-RPC response.
func extractMessageType(response map[string]interface{}) string {
	if method, ok := response["method"].(string); ok {
		return method
	}
	if id, ok := response["id"]; ok {
		return fmt.Sprintf("id: %v", id)
	}
	return "unknown"
}

// formatJSONForDisplay formats JSON with indentation for readability.
func formatJSONForDisplay(rawData []byte) string {
	var jsonData map[string]interface{}
	if err := json.Unmarshal(rawData, &jsonData); err == nil {
		if formatted, err := json.MarshalIndent(jsonData, "  ", "  "); err == nil {
			return string(formatted)
		}
	}
	return string(rawData)
}
