# TUI Feature Parity Plan - Python Textual Chat Audit

> **For Claude:** Execute using superpowers:subagent-driven-development

**Goal:** Bring golang TUI to feature parity with Python Textual chat implementation by adding critical missing features: session selection, session resume, permission system, SQLite history, and rich message types.

**Context:** Audit of `clients/textual_chat.py` (876 lines) identified 15 missing features. This plan addresses the 11 critical/important features in 3 phases.

**Base Implementation:** Existing golang TUI has basic architecture, WebSocket client, message store, components. See git log for recent work.

**Tech Stack:** Go 1.24, Bubbletea v0.25, Bubbles v0.18, Lipgloss v0.9, gorilla/websocket v1.5, mattn/go-sqlite3 v1.14

---

## Phase 1: Core Functionality (Critical Features)

### Task 1.1: SQLite Integration for Session History

**Priority:** Critical (P1A)
**Impact:** Required for session resume and history loading
**Dependencies:** None

**Files:**
- Create: `internal/tui/client/database.go`
- Create: `internal/tui/client/database_test.go`
- Modify: `go.mod` (add sqlite3 dependency)

**Requirements:**

1. Add SQLite dependency:
```bash
go get github.com/mattn/go-sqlite3@latest
go mod tidy
```

2. Create `internal/tui/client/database.go`:
   - Type `DatabaseClient` struct with:
     - `dbPath string` - path to SQLite database
     - `db *sql.DB` - database connection
   - Constructor `NewDatabaseClient(dbPath string) (*DatabaseClient, error)`:
     - Default path: `~/.local/share/acp-relay/db.sqlite`
     - Open database with `sql.Open("sqlite3", dbPath)`
     - Return error if connection fails
   - Method `GetAllSessions() ([]Session, error)`:
     - Query: `SELECT id, working_directory, created_at, closed_at FROM sessions ORDER BY created_at DESC LIMIT 20`
     - Return slice of Session structs with: ID, WorkingDirectory, CreatedAt, ClosedAt, IsActive (computed from closed_at IS NULL)
   - Method `GetSessionMessages(sessionID string) ([]*Message, error)`:
     - Query: `SELECT direction, message_type, method, raw_message, timestamp FROM messages WHERE session_id = ? ORDER BY timestamp ASC`
     - Parse JSON from raw_message column
     - Convert to Message structs with proper Type (User/Agent/System)
     - Handle both client_to_relay (user messages) and relay_to_client (agent chunks, system updates)
   - Method `MarkSessionClosed(sessionID string) error`:
     - Update: `UPDATE sessions SET closed_at = ? WHERE id = ?`
     - Use current timestamp
   - Method `Close() error`:
     - Close database connection

3. Write tests in `internal/tui/client/database_test.go`:
   - Test opening database (success + failure cases)
   - Test GetAllSessions with mock data (active + closed sessions)
   - Test GetSessionMessages with various message types
   - Test MarkSessionClosed
   - Use in-memory SQLite for tests (`:memory:`)

**Verification:**
```bash
cd internal/tui/client
go test -v -run TestDatabase
```

Expected: All database tests pass (5+/5+)

**Commit Message:**
```
feat(tui): add SQLite integration for session history

- Add DatabaseClient for reading relay server database
- Implement GetAllSessions, GetSessionMessages, MarkSessionClosed
- Support both active and closed sessions
- Add comprehensive database tests

Enables session resume and history loading for TUI.
```

---

### Task 1.2: Session Resume JSON-RPC Support

**Priority:** Critical (P1A)
**Impact:** Required to resume existing sessions
**Dependencies:** Task 1.1 (database)

**Files:**
- Modify: `internal/tui/client/relay_client.go`
- Modify: `internal/tui/client/relay_client_test.go`
- Modify: `internal/tui/update.go`

**Requirements:**

1. Add resume method to `RelayClient`:
   - Method signature: `ResumeSession(sessionID string) error`
   - Send JSON-RPC request:
     ```json
     {
       "jsonrpc": "2.0",
       "method": "session/resume",
       "params": {"sessionId": "..."},
       "id": 1
     }
     ```
   - Wait for response on incoming channel
   - Parse response for result/error
   - Return error if resume fails, nil if succeeds
   - Timeout after 5 seconds

2. Add message type to `update.go`:
   - New type: `SessionResumeResultMsg struct { SessionID string; Err error }`
   - Return from resume command

3. Handle resume in Update():
   - On 'r' key in sidebar (when session selected):
     - Call `m.relayClient.ResumeSession(selectedSessionID)`
     - Return `SessionResumeResultMsg`
   - On `SessionResumeResultMsg`:
     - If error: show error in status bar, create new session fallback
     - If success: set activeSessionID, load history from database, update chat view

4. Write tests:
   - Test successful resume with mock WebSocket
   - Test resume failure (session not found)
   - Test resume timeout
   - Test Update() handler for SessionResumeResultMsg

**Verification:**
```bash
# Unit tests
go test -v ./internal/tui/client -run TestResumeSession
go test -v ./internal/tui -run TestUpdateSessionResume

# Manual test (requires relay server running)
./bin/acp-tui --debug
# Press 'r' on an existing session in sidebar
# Check ~/.local/share/acp-tui/debug.log for resume attempt
```

Expected: Tests pass, debug log shows successful resume or clear error

**Commit Message:**
```
feat(tui): add session resume via JSON-RPC

- Implement RelayClient.ResumeSession with timeout
- Handle session/resume in Update loop
- Fallback to new session on resume failure
- Add resume tests with mock WebSocket

Allows resuming existing sessions from sidebar.
```

---

### Task 1.3: Session Selection Modal Screen

**Priority:** Critical (P1B)
**Impact:** Essential UX - replaces blank startup with session selector
**Dependencies:** Task 1.1 (database), Task 1.2 (resume)

**Files:**
- Create: `internal/tui/screens/session_selection.go`
- Create: `internal/tui/screens/session_selection_test.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/update.go`

**Requirements:**

1. Create `internal/tui/screens/session_selection.go`:
   - Type `SessionSelectionScreen` struct:
     - `sessions []Session` - list of available sessions
     - `selectedIndex int` - currently highlighted session
     - `width, height int` - dimensions
     - `theme theme.Theme`
   - Constructor `NewSessionSelectionScreen(sessions []Session, w, h int, th theme.Theme) *SessionSelectionScreen`
   - Method `Update(msg tea.Msg) (*SessionSelectionScreen, tea.Cmd)`:
     - Handle up/down arrow keys to change selectedIndex
     - Handle Enter key to select session (return SelectionMade message)
     - Handle 'n' key to create new session (return CreateNewSession message)
     - Handle 'q' or Esc to quit
   - Method `View() string`:
     - Render modal dialog with border
     - Title: "🔄 Select or Create Session"
     - List first 15 sessions with:
       - Index number (1-15)
       - Status icon (✅ active / 💤 closed)
       - Session ID (first 12 chars)
       - Created timestamp
     - Highlight selected session with background color
     - Show keybindings at bottom: "↑↓: Navigate | Enter: Select | n: New | q: Quit"
     - Center dialog on screen

2. Add message types to `internal/tui/update.go`:
   - `ShowSessionSelectionMsg struct { Sessions []Session }`
   - `SessionSelectedMsg struct { Session Session }`
   - `CreateNewSessionMsg struct {}`

3. Modify Model.Init() in `internal/tui/model.go`:
   - After loading sessions, instead of connecting immediately:
   - Query database for all sessions
   - Return `ShowSessionSelectionMsg` with sessions list
   - Don't connect to relay yet

4. Handle modal in Update():
   - On `ShowSessionSelectionMsg`:
     - Create SessionSelectionScreen component
     - Set model state to show modal (add `sessionModal *SessionSelectionScreen` field)
   - When modal is visible, route all input to modal.Update()
   - On `SessionSelectedMsg`:
     - Close modal
     - If session.IsActive: attempt resume (Task 1.2)
     - If session closed: load history in read-only mode
   - On `CreateNewSessionMsg`:
     - Close modal
     - Connect to relay and create new session (existing behavior)

5. Write tests:
   - Test navigation (up/down/wrap)
   - Test selection
   - Test new session creation
   - Test rendering with various session counts (0, 5, 20)
   - Test modal centering

**Verification:**
```bash
# Unit tests
go test -v ./internal/tui/screens -run TestSessionSelection

# Integration test
./bin/acp-tui --debug
# Should show session modal on startup
# Test navigation, selection, new session creation
```

Expected: Modal appears on startup, navigation works, can resume/view/create

**Commit Message:**
```
feat(tui): add session selection modal on startup

- Create SessionSelectionScreen component
- Show recent 15 sessions with status icons
- Support navigation, selection, and new session
- Integrate with resume and read-only view
- Match Python Textual UX pattern

Replaces blank startup with session picker.
```

---

### Task 1.4: Permission Request System

**Priority:** Critical (P1C)
**Impact:** Required for agent tool execution
**Dependencies:** None (independent)

**Files:**
- Modify: `internal/tui/update.go`
- Modify: `internal/tui/client/message.go`
- Create: `internal/tui/components/permission_widget.go`
- Create: `internal/tui/components/permission_widget_test.go`

**Requirements:**

1. Add message types to `client/message.go`:
   - `MessageTypePermissionRequest`
   - `MessageTypePermissionResponse`

2. Handle `session/request_permission` in Update():
   - Parse incoming notification:
     ```json
     {
       "method": "session/request_permission",
       "params": {
         "toolCall": {
           "toolCallId": "...",
           "rawInput": {"file_path": "...", "content": "..."}
         }
       },
       "id": 123
     }
     ```
   - Extract: requestID (id), toolCallId, tool name, arguments
   - Create PermissionRequest message for chat view
   - Auto-approve for now (future: manual approval)
   - Send response:
     ```json
     {
       "jsonrpc": "2.0",
       "id": 123,
       "result": {
         "outcome": {
           "outcome": "selected",
           "optionId": "allow"
         }
       }
     }
     ```
   - Add PermissionResponse message to chat view

3. Create `components/permission_widget.go`:
   - Render permission request with:
     - Icon: 🔐
     - Title: "Permission Request"
     - Tool name in bold
     - Arguments (truncated if > 100 chars)
     - Color: yellow border
   - Render permission response with:
     - Icon: ✅ (allow) or ❌ (deny)
     - Status in green/red
     - Tool name

4. Update ChatView to render PermissionRequest/Response messages using widget

5. Write tests:
   - Test parsing session/request_permission
   - Test auto-approval response generation
   - Test permission widget rendering
   - Test message storage with permission types

**Verification:**
```bash
# Unit tests
go test -v ./internal/tui -run TestPermission
go test -v ./internal/tui/components -run TestPermissionWidget

# Integration test (requires agent that uses tools)
./bin/acp-tui --debug
# Send prompt that triggers tool use: "create a file called test.txt with hello world"
# Should see permission request → approval → tool execution
```

Expected: Permission requests appear, auto-approved, tools execute

**Commit Message:**
```
feat(tui): implement permission request system

- Handle session/request_permission notifications
- Auto-approve permissions with proper JSON-RPC response
- Add PermissionWidget for visual display
- Support permission request/response message types
- Add icons and color coding (🔐 yellow, ✅ green)

Enables agent tool execution in TUI.
```

---

## Phase 2: Rich Message Handling (Important Features)

### Task 2.1: Advanced Session Update Types

**Priority:** Important (P2A)
**Impact:** Provides essential agent feedback for better UX
**Dependencies:** None (extends existing message handling)

**Files:**
- Modify: `internal/tui/update.go`
- Modify: `internal/tui/client/message.go`
- Create: `internal/tui/components/system_message_widget.go`
- Create: `internal/tui/components/system_message_widget_test.go`

**Requirements:**

1. Add message types to `client/message.go`:
   - `MessageTypeAvailableCommands` - with `Commands []Command` field
   - `MessageTypeToolUse` - with `ToolName string` field
   - `MessageTypeThinking` - simple indicator
   - `MessageTypeThoughtChunk` - with `Thought string` field (streaming)

2. Extend `session/update` handler in `update.go`:
   - Parse `params.update.sessionUpdate` field
   - Handle new update types:

     **available_commands_update:**
     - Extract: `update.availableCommands` array
     - Create system message with command count
     - If ≤ 5 commands: show names in message
     - If > 5 commands: show "X commands available"
     - Icon: 📋

     **tool_use:**
     - Extract: `update.tool.name`
     - Create system message with tool name
     - Icon: 🔧
     - Color: magenta

     **agent_thinking:**
     - Create system message "Agent is thinking..."
     - Icon: 💭
     - Update status bar

     **agent_thought_chunk:**
     - Extract: `update.content.text`
     - Accumulate thought text in `currentThought string` field
     - Update status bar with truncated preview (first 50 chars)
     - Don't add to message store (transient display)

3. Create `components/system_message_widget.go`:
   - Render different system message types:
     - **Commands:** Show command list with bullet points
     - **Tool use:** Show tool name with wrench icon
     - **Thinking:** Show thinking indicator with pulse animation
   - Use consistent formatting: dimmed timestamp + icon + message

4. Update ChatView to use system_message_widget for rendering

5. Write tests:
   - Test parsing each session update type
   - Test message creation for each type
   - Test status bar updates for thinking/thoughts
   - Test widget rendering for each type
   - Test thought accumulation and truncation

**Verification:**
```bash
# Unit tests
go test -v ./internal/tui -run TestSessionUpdate
go test -v ./internal/tui/components -run TestSystemMessageWidget

# Integration test
./bin/acp-tui --debug
# Send prompt, observe:
# - Thinking indicator appears
# - Tool use messages show up
# - Commands update appears
# Check debug log for all update types received
```

Expected: All session update types displayed with proper formatting

**Commit Message:**
```
feat(tui): add advanced session update message types

- Handle available_commands_update with command list display
- Handle tool_use with wrench icon and tool name
- Handle agent_thinking with thinking indicator
- Handle agent_thought_chunk with status bar preview
- Add SystemMessageWidget for rich rendering
- Add icons: 📋 (commands), 🔧 (tool), 💭 (thinking)

Provides rich agent feedback matching Python Textual UX.
```

---

### Task 2.2: Progress Bar and Typing Indicators

**Priority:** Important (P2B)
**Impact:** Visual feedback improves perceived responsiveness
**Dependencies:** Task 2.1 (uses thought chunks)

**Files:**
- Modify: `internal/tui/components/statusbar.go`
- Modify: `internal/tui/components/statusbar_test.go`
- Modify: `internal/tui/components/chatview.go`
- Modify: `internal/tui/components/chatview_test.go`
- Modify: `internal/tui/update.go`

**Requirements:**

1. Add progress bar to StatusBar:
   - Add field `progressVisible bool`
   - Add field `progressValue float64` (0.0-100.0)
   - Method `ShowProgress()` - set progressVisible = true, progressValue = 0
   - Method `HideProgress()` - set progressVisible = false
   - Method `AdvanceProgress(amount float64)` - increment progressValue, wrap at 100
   - In View():
     - If progressVisible, render progress bar below status text
     - Use Lipgloss to create bar: filled portion (█) vs empty (░)
     - Width: 40 chars, percentage: progressValue/100
     - Color: theme primary

2. Add typing indicator to ChatView:
   - Add field `agentTyping bool`
   - Add field `typingText string`
   - Method `StartTyping()` - set agentTyping = true
   - Method `UpdateTyping(text string)` - set typingText = text
   - Method `StopTyping()` - set agentTyping = false, add final message
   - In View():
     - If agentTyping, show last line with blinking cursor: `{typingText} ▊`
     - Use Lipgloss blink style for cursor
     - Cursor blinks every 500ms

3. Integrate in Update():
   - On user message submit:
     - Call `m.statusBar.ShowProgress()`
     - Call `m.chatView.StartTyping()`
   - On `session/chunk` notification:
     - Accumulate text in `currentResponse string`
     - Call `m.chatView.UpdateTyping(currentResponse)`
     - Call `m.statusBar.AdvanceProgress(2.0)` per chunk
   - On `session/complete` or final response:
     - Call `m.chatView.StopTyping()` (adds final message)
     - Call `m.statusBar.HideProgress()`
   - On `agent_thought_chunk`:
     - Call `m.statusBar.AdvanceProgress(1.0)` per chunk

4. Write tests:
   - Test progress bar visibility and advancement
   - Test progress wrapping at 100
   - Test typing indicator start/stop
   - Test typing text updates
   - Test cursor rendering
   - Test integration with message chunks

**Verification:**
```bash
# Unit tests
go test -v ./internal/tui/components -run TestStatusBarProgress
go test -v ./internal/tui/components -run TestChatViewTyping

# Integration test
./bin/acp-tui --debug
# Send message, observe:
# - Progress bar appears below status
# - Progress advances with each chunk
# - Typing indicator shows with blinking cursor
# - Progress hides when complete
```

Expected: Smooth progress bar, blinking cursor during streaming

**Commit Message:**
```
feat(tui): add progress bar and typing indicators

- Add progress bar to StatusBar (40-char width)
- Show/hide progress during agent responses
- Advance progress with each message/thought chunk
- Add typing indicator to ChatView with blinking cursor ▊
- Update typing text in real-time during streaming
- Auto-scroll to follow typing indicator

Improves perceived responsiveness matching Python UX.
```

---

### Task 2.3: Rich Message Formatting with Icons

**Priority:** Important (P2C)
**Impact:** Visual polish, easier to scan conversation history
**Dependencies:** Task 2.1 (system message types)

**Files:**
- Modify: `internal/tui/components/chatview.go`
- Modify: `internal/tui/components/chatview_test.go`
- Modify: `internal/tui/client/message.go`

**Requirements:**

1. Add icon constants to `client/message.go`:
   ```go
   const (
     IconUser       = "👤"
     IconAgent      = "🤖"
     IconSystem     = "ℹ️"
     IconError      = "❌"
     IconPermission = "🔐"
     IconApproved   = "✅"
     IconDenied     = "❌"
     IconTool       = "🔧"
     IconThinking   = "💭"
     IconCommands   = "📋"
   )
   ```

2. Enhance message rendering in ChatView.View():
   - For each message type, prepend appropriate icon:
     - **User:** `👤 You: {text}`
     - **Agent:** `🤖 Agent: {text}`
     - **System:** Depends on subtype:
       - Commands: `📋 Available Commands Updated`
       - Tool: `🔧 Tool Used: {toolName}`
       - Thinking: `💭 Agent is thinking...`
       - Generic: `ℹ️ System: {text}`
     - **Error:** `❌ Error: {text}`
     - **Permission Request:** `🔐 Permission Request: {toolName}`
     - **Permission Response:** `✅ Allowed: {toolName}` or `❌ Denied: {toolName}`

3. Add color coding by role:
   - User: Cyan (theme.Primary)
   - Agent: Green (theme.Success)
   - System: Yellow (theme.Warning)
   - Error: Red (theme.Error)
   - Permission request: Yellow
   - Permission approved: Green
   - Permission denied: Red

4. Format timestamps:
   - Show as `[HH:MM:SS]` before each message
   - Dim style (theme.Dimmed)

5. Truncate long arguments in permission requests:
   - If argument > 100 chars: show first 100 + "..."
   - Format arguments as indented JSON

6. Format command lists:
   - Show first 5 commands with bullet points:
     ```
     📋 Available Commands Updated
        • /command1 - description
        • /command2 - description
        ...
        ... and 10 more
     ```

7. Write tests:
   - Test icon rendering for each message type
   - Test color application
   - Test timestamp formatting
   - Test argument truncation
   - Test command list formatting
   - Test rendering with various message sequences

**Verification:**
```bash
# Unit tests
go test -v ./internal/tui/components -run TestChatViewFormatting

# Integration test
./bin/acp-tui --debug
# Send various messages, observe:
# - Icons appear before each message
# - Colors match message types
# - Timestamps shown
# - Permission requests formatted nicely
# - Command lists formatted with bullets
```

Expected: All messages have icons, colors, and proper formatting

**Commit Message:**
```
feat(tui): add rich message formatting with icons

- Add icons for all message types (👤🤖🔐✅❌🔧💭📋ℹ️)
- Apply color coding by role (cyan/green/yellow/red)
- Format timestamps as [HH:MM:SS] with dimmed style
- Truncate long arguments in permission requests (>100 chars)
- Format command lists with bullets (first 5 + count)
- Improve visual hierarchy and scannability

Matches Python Textual visual polish.
```

---

## Phase 3: Polish & Features (Nice to Have)

### Task 3.1: Read-Only Session Viewing

**Priority:** Nice to Have (P3A)
**Impact:** Allows reviewing closed sessions without resuming
**Dependencies:** Task 1.1 (database), Task 1.3 (session modal)

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/update.go`
- Modify: `internal/tui/components/inputarea.go`

**Requirements:**

1. Add state to Model:
   - Field `readOnlyMode bool`
   - When true, disable input and show indicator

2. In session selection modal:
   - If selected session is closed (IsActive = false):
     - Set `readOnlyMode = true`
     - Load session history from database
     - Don't attempt to connect to relay
     - Display messages in chat view
     - Update subtitle: "📖 {sessionID} (Read-Only)"

3. Modify InputArea:
   - Add field `disabled bool`
   - Method `SetDisabled(disabled bool)`
   - When disabled:
     - Don't accept input
     - Change placeholder to "Session is closed (read-only)"
     - Dim appearance (gray background)
   - In Update(): ignore all input when disabled

4. Add indicator to StatusBar:
   - When readOnlyMode = true:
     - Show "👀 Viewing closed session (read-only)" in status bar
     - Use different color (gray/dimmed)

5. Write tests:
   - Test readOnlyMode activation on closed session selection
   - Test input area disabled state
   - Test status bar read-only indicator
   - Test that input is ignored in read-only mode

**Verification:**
```bash
# Unit tests
go test -v ./internal/tui -run TestReadOnlyMode
go test -v ./internal/tui/components -run TestInputAreaDisabled

# Integration test
./bin/acp-tui --debug
# Select a closed session from modal
# Verify:
# - History loads and displays
# - Input area is disabled and grayed
# - Status bar shows read-only indicator
# - Can't send messages
# - Can quit with Ctrl+C
```

Expected: Closed sessions viewable without resume attempt, input disabled

**Commit Message:**
```
feat(tui): add read-only session viewing

- Detect closed sessions in selection modal
- Load history without relay connection
- Disable input area with visual indicator
- Show read-only status in status bar and subtitle
- Support viewing past conversations

Allows reviewing closed sessions matching Python UX.
```

---

### Task 3.2: Toast Notification System

**Priority:** Nice to Have (P3B)
**Impact:** Better feedback for async events and errors
**Dependencies:** None (independent)

**Files:**
- Create: `internal/tui/components/notification.go`
- Create: `internal/tui/components/notification_test.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/update.go`
- Modify: `internal/tui/view.go`

**Requirements:**

1. Create `components/notification.go`:
   - Type `Notification` struct:
     - `Message string`
     - `Severity string` (info/warning/error/success)
     - `Visible bool`
     - `CreatedAt time.Time`
   - Type `NotificationComponent` struct:
     - `notifications []*Notification` (max 3 visible)
     - `width int`
     - `theme theme.Theme`
   - Method `Show(message string, severity string)`:
     - Create new notification
     - Add to notifications slice
     - Start auto-dismiss timer (3 seconds)
   - Method `Dismiss(index int)`:
     - Remove notification from slice
   - Method `Update(msg tea.Msg) tea.Cmd`:
     - Handle DismissNotificationMsg
     - Return tea.Tick for auto-dismiss
   - Method `View() string`:
     - Render notifications as overlay in top-right corner
     - Stack vertically (max 3)
     - Use Lipgloss for styling:
       - Info: blue border, ℹ️ icon
       - Warning: yellow border, ⚠️ icon
       - Error: red border, ❌ icon
       - Success: green border, ✅ icon
     - Show message text (wrap at notification width)
     - Fade in animation (optional)

2. Add notification field to Model:
   - Field `notifications *NotificationComponent`
   - Initialize in NewModel()

3. Add notification calls in Update():
   - On RelayConnectedMsg: `m.notifications.Show("Connected to relay server", "success")`
   - On RelayErrorMsg: `m.notifications.Show("Connection error: " + err.Error(), "error")`
   - On session creation: `m.notifications.Show("Session created", "info")`
   - On session resume success: `m.notifications.Show("Session resumed", "success")`
   - On session resume failure: `m.notifications.Show("Failed to resume session", "warning")`
   - On permission auto-approved: `m.notifications.Show("Permission approved", "info")`

4. Render notifications in View():
   - Call `m.notifications.View()` after main view
   - Use Lipgloss Place to position in top-right corner

5. Write tests:
   - Test notification creation and dismissal
   - Test auto-dismiss after timeout
   - Test max 3 visible notifications
   - Test rendering with different severities
   - Test stacking and positioning

**Verification:**
```bash
# Unit tests
go test -v ./internal/tui/components -run TestNotification

# Integration test
./bin/acp-tui --debug
# Trigger various events:
# - Startup: should see connection notification
# - Create session: should see creation notification
# - Send message with permission: should see approval notification
# - Force error: should see error notification
# Verify notifications appear in top-right, auto-dismiss after 3s
```

Expected: Toast notifications appear for events, auto-dismiss, max 3 visible

**Commit Message:**
```
feat(tui): add toast notification system

- Create NotificationComponent with auto-dismiss (3s)
- Support info/warning/error/success severities
- Render as overlay in top-right corner (max 3 visible)
- Add icons and color coding per severity
- Show notifications for connection, session, permission events
- Use Lipgloss for styling and positioning

Provides async feedback matching Python UX.
```

---

### Task 3.3: Unhandled Message Display (Debug Mode)

**Priority:** Nice to Have (P3C)
**Impact:** Helps identify protocol issues during development
**Dependencies:** Task 2.3 (message formatting)

**Files:**
- Modify: `internal/tui/update.go`
- Modify: `internal/tui/client/message.go`
- Modify: `cmd/tui/main.go`

**Requirements:**

1. Add debug flag to Model:
   - Field `debugMode bool`
   - Set from CLI flag in main.go

2. Add unhandled message type:
   - `MessageTypeUnhandled` in `client/message.go`
   - Include `RawJSON string` field

3. Track handled messages in Update():
   - For each message received, set `handled = false`
   - After each handler, set `handled = true`
   - At end of message handling:
     - If `!handled && m.debugMode`:
       - Create unhandled message with full JSON
       - Add to message store
       - Log to debug log

4. Render unhandled messages:
   - In ChatView, format as:
     ```
     [HH:MM:SS] ⚠️ Unhandled Message:
     Type: session/update (or method/id)
     {
       "jsonrpc": "2.0",
       "method": "...",
       ...
     }
     ```
   - Use monospace font for JSON
   - Color: yellow (warning)
   - Indent JSON for readability

5. Write tests:
   - Test unhandled message detection
   - Test unhandled message rendering
   - Test that debug mode controls visibility
   - Test various unhandled message types

**Verification:**
```bash
# Unit tests
go test -v ./internal/tui -run TestUnhandledMessage

# Integration test
./bin/acp-tui --debug
# Send prompt, observe messages
# Inject unknown notification (manually edit relay response)
# Verify unhandled message appears in chat with JSON
# Verify it's only visible in debug mode
```

Expected: Unknown messages displayed in debug mode with full JSON

**Commit Message:**
```
feat(tui): add unhandled message display in debug mode

- Track handled vs unhandled messages in Update
- Add MessageTypeUnhandled with raw JSON
- Render unhandled messages with formatted JSON
- Only show in debug mode (--debug flag)
- Include message type and full payload
- Use yellow color and warning icon

Helps identify protocol issues during development.
```

---

## Final Review & Completion

After all tasks complete:

1. Run full test suite:
   ```bash
   make test
   ```

2. Run integration test:
   ```bash
   make build-tui
   ./bin/acp-tui --debug
   ```
   - Test session selection modal
   - Test session resume
   - Test read-only viewing
   - Test permission requests
   - Test progress bars and typing indicators
   - Test notifications
   - Test all message types (user, agent, system, tool, permission, commands)

3. Verify feature parity with Python Textual chat:
   - [ ] Session selection modal ✓
   - [ ] Session resume ✓
   - [ ] Session history loading ✓
   - [ ] Permission system ✓
   - [ ] Advanced message types ✓
   - [ ] Progress bar ✓
   - [ ] Typing indicator ✓
   - [ ] Rich formatting with icons ✓
   - [ ] Read-only viewing ✓
   - [ ] Toast notifications ✓
   - [ ] Unhandled message display ✓

4. Performance check:
   - Test with 1000+ message history
   - Test with 20+ sessions
   - Verify no memory leaks
   - Verify smooth scrolling

5. Commit final state:
   ```bash
   git add .
   git commit -m "feat(tui): achieve feature parity with Python Textual chat

   Complete implementation of all critical and important features:
   - Phase 1: Session selection, resume, history, permissions
   - Phase 2: Rich messages, progress, typing indicators, formatting
   - Phase 3: Read-only viewing, notifications, debug display

   Golang TUI now matches Python Textual functionality plus additional
   features (themes, help overlay, multi-line input, component tests).
   "
   ```

6. Use **superpowers:finishing-a-development-branch** skill to complete development

---

## Success Criteria

**Must Have (Phase 1):**
- ✅ SQLite integration works, reads relay database
- ✅ Session resume successful via JSON-RPC
- ✅ Session selection modal appears on startup
- ✅ Permission requests handled and auto-approved
- ✅ All Phase 1 tests pass (20+/20+)

**Should Have (Phase 2):**
- ✅ All advanced session update types displayed
- ✅ Progress bar shows during agent work
- ✅ Typing indicator with blinking cursor
- ✅ Rich formatting with icons and colors
- ✅ All Phase 2 tests pass (15+/15+)

**Nice to Have (Phase 3):**
- ✅ Read-only session viewing works
- ✅ Toast notifications appear and dismiss
- ✅ Unhandled messages shown in debug mode
- ✅ All Phase 3 tests pass (10+/10+)

**Overall:**
- ✅ Full test suite passes (45+/45+ tests)
- ✅ No regressions in existing functionality
- ✅ Feature parity with Python Textual chat achieved
- ✅ Documentation updated (if needed)
- ✅ Ready to replace Python implementation

---

## Notes

- This plan assumes existing TUI structure is working (based on recent commits)
- SQLite schema matches relay server database (sessions + messages tables)
- Auto-approval for permissions is temporary - future: manual approval UI
- Progress bar uses indeterminate style (wraps at 100%)
- Typing indicator accumulates chunks, final message added on complete
- Read-only mode doesn't connect to relay (saves resources)
- Notifications have 3-second auto-dismiss, can be dismissed manually
- Debug mode required for unhandled messages (prevents clutter)
