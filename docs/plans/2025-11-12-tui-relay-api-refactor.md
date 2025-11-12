# TUI Relay API Refactor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor Python TUI to use relay WebSocket API instead of direct SQLite database access for listing sessions and viewing history.

**Architecture:** Add two new WebSocket methods to the relay server (`session/list` and `session/history`) that expose existing database functionality. Update TUI to call these methods via WebSocket instead of directly accessing `~/.local/share/acp-relay/db.sqlite`.

**Tech Stack:** Go (relay server), Python (TUI client), WebSocket JSON-RPC, SQLite

---

## Task 1: Add session/list endpoint to relay WebSocket server

**Files:**
- Modify: `internal/websocket/server.go:102-236` (add new case in switch statement)
- Test: `internal/websocket/server_test.go`

**Step 1: Write failing test for session/list**

Add to `internal/websocket/server_test.go`:

```go
func TestWebSocketSessionList(t *testing.T) {
	// Setup test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	defer database.Close()

	// Create some test sessions
	require.NoError(t, database.CreateSession("sess_test1", "/tmp/workspace1"))
	require.NoError(t, database.CreateSession("sess_test2", "/tmp/workspace2"))
	require.NoError(t, database.CloseSession("sess_test1"))

	// Setup manager
	mgr := session.NewManager(session.ManagerConfig{
		Mode:         "process",
		AgentCommand: "/bin/echo",
	}, database)

	// Setup WebSocket server
	ws := NewServer(mgr)

	// Create test server
	server := httptest.NewServer(ws)
	defer server.Close()

	// Connect via WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Send session/list request
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/list",
		"params":  map[string]interface{}{},
		"id":      1,
	}
	require.NoError(t, conn.WriteJSON(req))

	// Read response
	var resp map[string]interface{}
	require.NoError(t, conn.ReadJSON(&resp))

	// Verify response structure
	assert.Equal(t, "2.0", resp["jsonrpc"])
	assert.Equal(t, float64(1), resp["id"])

	// Verify result contains sessions
	result, ok := resp["result"].(map[string]interface{})
	require.True(t, ok, "result should be an object")

	sessions, ok := result["sessions"].([]interface{})
	require.True(t, ok, "sessions should be an array")
	assert.Equal(t, 2, len(sessions), "should have 2 sessions")

	// Verify first session structure
	sess1 := sessions[0].(map[string]interface{})
	assert.Contains(t, sess1, "id")
	assert.Contains(t, sess1, "workingDirectory")
	assert.Contains(t, sess1, "createdAt")
	assert.Contains(t, sess1, "closedAt")
	assert.Contains(t, sess1, "isActive")
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/harper/Public/src/2389/acp-relay
go test ./internal/websocket -run TestWebSocketSessionList -v
```

Expected output: `FAIL` - method not implemented

**Step 3: Implement session/list in WebSocket server**

In `internal/websocket/server.go`, add new case after line 163 (after `session/resume` case):

```go
		case "session/list":
			// List all sessions from database
			if currentSession == nil || currentSession.DB == nil {
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewInternalError("database not available"), req.ID)
				continue
			}

			sessions, err := currentSession.DB.GetAllSessions()
			if err != nil {
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewInternalError(fmt.Sprintf("failed to get sessions: %v", err)), req.ID)
				continue
			}

			// Convert to JSON-friendly format
			sessionList := make([]map[string]interface{}, len(sessions))
			for i, sess := range sessions {
				sessionList[i] = map[string]interface{}{
					"id":               sess.ID,
					"agentSessionId":   sess.AgentSessionID,
					"workingDirectory": sess.WorkingDirectory,
					"createdAt":        sess.CreatedAt.Format(time.RFC3339),
					"closedAt":         nil,
					"isActive":         sess.ClosedAt == nil,
				}
				if sess.ClosedAt != nil {
					sessionList[i]["closedAt"] = sess.ClosedAt.Format(time.RFC3339)
					sessionList[i]["isActive"] = false
				}
			}

			result := map[string]interface{}{
				"sessions": sessionList,
			}
			s.sendResponseSafe(conn, currentSession, currentClientID, result, req.ID)
```

Add imports at top of file if not present:

```go
import (
	"fmt"
	"time"
	// ... existing imports
)
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/websocket -run TestWebSocketSessionList -v
```

Expected output: `PASS`

**Step 5: Run all websocket tests**

```bash
go test ./internal/websocket -v
```

Expected output: All tests `PASS`

**Step 6: Commit**

```bash
git add internal/websocket/server.go internal/websocket/server_test.go
git commit -m "feat(relay): add session/list WebSocket endpoint

- Add session/list method to WebSocket server
- Returns all sessions from database with metadata
- Includes isActive flag based on closedAt field
- Add comprehensive test coverage"
```

---

## Task 2: Add session/history endpoint to relay WebSocket server

**Files:**
- Modify: `internal/websocket/server.go:102-236` (add new case in switch statement)
- Test: `internal/websocket/server_test.go`

**Step 1: Write failing test for session/history**

Add to `internal/websocket/server_test.go`:

```go
func TestWebSocketSessionHistory(t *testing.T) {
	// Setup test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	defer database.Close()

	// Create test session
	sessionID := "sess_test1"
	require.NoError(t, database.CreateSession(sessionID, "/tmp/workspace"))

	// Log some test messages
	msg1 := []byte(`{"jsonrpc":"2.0","method":"session/prompt","params":{"content":[{"type":"text","text":"hello"}]},"id":1}`)
	require.NoError(t, database.LogMessage(sessionID, db.DirectionClientToRelay, msg1))

	msg2 := []byte(`{"jsonrpc":"2.0","result":{"content":[{"type":"text","text":"hi"}]},"id":1}`)
	require.NoError(t, database.LogMessage(sessionID, db.DirectionRelayToClient, msg2))

	// Setup manager
	mgr := session.NewManager(session.ManagerConfig{
		Mode:         "process",
		AgentCommand: "/bin/echo",
	}, database)

	// Setup WebSocket server
	ws := NewServer(mgr)

	// Create test server
	server := httptest.NewServer(ws)
	defer server.Close()

	// Connect via WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Send session/history request
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/history",
		"params": map[string]interface{}{
			"sessionId": sessionID,
		},
		"id": 1,
	}
	require.NoError(t, conn.WriteJSON(req))

	// Read response
	var resp map[string]interface{}
	require.NoError(t, conn.ReadJSON(&resp))

	// Verify response structure
	assert.Equal(t, "2.0", resp["jsonrpc"])
	assert.Equal(t, float64(1), resp["id"])

	// Verify result contains messages
	result, ok := resp["result"].(map[string]interface{})
	require.True(t, ok, "result should be an object")

	messages, ok := result["messages"].([]interface{})
	require.True(t, ok, "messages should be an array")
	assert.Equal(t, 2, len(messages), "should have 2 messages")

	// Verify first message structure
	msg := messages[0].(map[string]interface{})
	assert.Contains(t, msg, "id")
	assert.Contains(t, msg, "direction")
	assert.Contains(t, msg, "messageType")
	assert.Contains(t, msg, "method")
	assert.Contains(t, msg, "rawMessage")
	assert.Contains(t, msg, "timestamp")
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/websocket -run TestWebSocketSessionHistory -v
```

Expected output: `FAIL` - method not implemented

**Step 3: Implement session/history in WebSocket server**

In `internal/websocket/server.go`, add new case after the `session/list` case:

```go
		case "session/history":
			// Get message history for a session
			var params struct {
				SessionID string `json:"sessionId"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewInvalidParamsError("sessionId", "string", "invalid or missing"), req.ID)
				continue
			}

			if currentSession == nil || currentSession.DB == nil {
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewInternalError("database not available"), req.ID)
				continue
			}

			messages, err := currentSession.DB.GetSessionMessages(params.SessionID)
			if err != nil {
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewInternalError(fmt.Sprintf("failed to get session messages: %v", err)), req.ID)
				continue
			}

			// Convert to JSON-friendly format
			messageList := make([]map[string]interface{}, len(messages))
			for i, msg := range messages {
				messageList[i] = map[string]interface{}{
					"id":          msg.ID,
					"direction":   string(msg.Direction),
					"messageType": msg.MessageType,
					"method":      msg.Method,
					"rawMessage":  msg.RawMessage,
					"timestamp":   msg.Timestamp.Format(time.RFC3339),
				}
				if msg.JSONRPCId != nil {
					messageList[i]["jsonrpcId"] = *msg.JSONRPCId
				}
			}

			result := map[string]interface{}{
				"sessionId": params.SessionID,
				"messages":  messageList,
			}
			s.sendResponseSafe(conn, currentSession, currentClientID, result, req.ID)
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/websocket -run TestWebSocketSessionHistory -v
```

Expected output: `PASS`

**Step 5: Run all websocket tests**

```bash
go test ./internal/websocket -v
```

Expected output: All tests `PASS`

**Step 6: Commit**

```bash
git add internal/websocket/server.go internal/websocket/server_test.go
git commit -m "feat(relay): add session/history WebSocket endpoint

- Add session/history method to WebSocket server
- Returns all messages for a session with metadata
- Includes direction, type, method, and raw JSON
- Add comprehensive test coverage"
```

---

## Task 3: Update API documentation

**Files:**
- Modify: `docs/api.md:220-429` (add new endpoints to WebSocket API section)

**Step 1: Add session/list documentation**

In `docs/api.md`, after the `session/resume` section (around line 163), add:

```markdown
#### session/list

List all sessions (active and closed).

**Client → Server:**

```json
{
  "jsonrpc": "2.0",
  "method": "session/list",
  "params": {},
  "id": 3
}
```

**Server → Client:**

```json
{
  "jsonrpc": "2.0",
  "result": {
    "sessions": [
      {
        "id": "sess_abc12345",
        "agentSessionId": "agent_xyz",
        "workingDirectory": "/tmp/workspace",
        "createdAt": "2025-11-12T10:30:00Z",
        "closedAt": null,
        "isActive": true
      },
      {
        "id": "sess_def67890",
        "agentSessionId": "agent_uvw",
        "workingDirectory": "/tmp/other",
        "createdAt": "2025-11-11T15:20:00Z",
        "closedAt": "2025-11-11T16:45:00Z",
        "isActive": false
      }
    ]
  },
  "id": 3
}
```

**Use Cases:**
- Display list of available sessions for resumption
- Show session history in UI
- Monitor active vs closed sessions
```

**Step 2: Add session/history documentation**

After the `session/list` section, add:

```markdown
#### session/history

Get message history for a specific session.

**Client → Server:**

```json
{
  "jsonrpc": "2.0",
  "method": "session/history",
  "params": {
    "sessionId": "sess_abc12345"
  },
  "id": 4
}
```

**Server → Client:**

```json
{
  "jsonrpc": "2.0",
  "result": {
    "sessionId": "sess_abc12345",
    "messages": [
      {
        "id": 1,
        "direction": "client_to_relay",
        "messageType": "request",
        "method": "session/prompt",
        "jsonrpcId": 2,
        "rawMessage": "{\"jsonrpc\":\"2.0\",\"method\":\"session/prompt\",...}",
        "timestamp": "2025-11-12T10:31:15Z"
      },
      {
        "id": 2,
        "direction": "relay_to_client",
        "messageType": "response",
        "method": "",
        "jsonrpcId": 2,
        "rawMessage": "{\"jsonrpc\":\"2.0\",\"result\":{...}}",
        "timestamp": "2025-11-12T10:31:20Z"
      }
    ]
  },
  "id": 4
}
```

**Message Directions:**
- `client_to_relay` - Client sent to relay
- `relay_to_agent` - Relay forwarded to agent
- `agent_to_relay` - Agent responded to relay
- `relay_to_client` - Relay forwarded to client

**Use Cases:**
- View conversation history for closed sessions
- Replay message flow for debugging
- Export session transcripts
```

**Step 3: Verify documentation**

Read through the updated documentation to ensure:
- Examples are valid JSON
- Field types match implementation
- Use cases are clear

**Step 4: Commit**

```bash
git add docs/api.md
git commit -m "docs(relay): add session/list and session/history API docs

- Document new session/list WebSocket method
- Document new session/history WebSocket method
- Include request/response examples
- Add use cases for each endpoint"
```

---

## Task 4: Refactor TUI to add relay API helper methods

**Files:**
- Modify: `clients/textual_chat.py:34-35` (remove DB_PATH constant)
- Modify: `clients/textual_chat.py:300-306` (add WebSocket helper methods)

**Step 1: Remove direct DB access constants**

In `clients/textual_chat.py`, delete line 34:

```python
DB_PATH = str(Path.home() / ".local" / "share" / "acp-relay" / "db.sqlite")
```

**Step 2: Add WebSocket helper methods to ACPChatApp class**

After the `send_message` method (around line 541), add these helper methods:

```python
    async def request_session_list(self) -> list:
        """Request list of all sessions from relay server"""
        if not self.websocket:
            return []

        # Send session/list request
        msg_id = self.msg_id
        self.msg_id += 1

        await self.send_message("session/list", {}, msg_id)

        # Wait for response
        try:
            raw_msg = await asyncio.wait_for(self.websocket.recv(), timeout=5.0)
            msg = json.loads(raw_msg)

            if msg.get("id") == msg_id and "result" in msg:
                sessions = msg["result"].get("sessions", [])
                return sessions
            elif "error" in msg:
                self.notify(f"Error getting sessions: {msg['error'].get('message', 'unknown')}", severity="error")
                return []
        except asyncio.TimeoutError:
            self.notify("Timeout getting session list", severity="warning")
            return []
        except Exception as e:
            self.notify(f"Error getting sessions: {e}", severity="error")
            return []

        return []

    async def request_session_history(self, session_id: str) -> list:
        """Request message history for a session from relay server"""
        if not self.websocket:
            return []

        # Send session/history request
        msg_id = self.msg_id
        self.msg_id += 1

        await self.send_message("session/history", {"sessionId": session_id}, msg_id)

        # Wait for response
        try:
            raw_msg = await asyncio.wait_for(self.websocket.recv(), timeout=10.0)
            msg = json.loads(raw_msg)

            if msg.get("id") == msg_id and "result" in msg:
                messages = msg["result"].get("messages", [])
                return messages
            elif "error" in msg:
                self.notify(f"Error getting history: {msg['error'].get('message', 'unknown')}", severity="error")
                return []
        except asyncio.TimeoutError:
            self.notify("Timeout getting session history", severity="warning")
            return []
        except Exception as e:
            self.notify(f"Error getting history: {e}", severity="error")
            return []

        return []
```

**Step 3: Remove sqlite3 import**

At the top of `clients/textual_chat.py`, remove line 21:

```python
import sqlite3
```

**Step 4: Test that file still loads**

```bash
cd /Users/harper/Public/src/2389/acp-relay
python3 -c "import sys; sys.path.insert(0, 'clients'); from textual_chat import ACPChatApp; print('OK')"
```

Expected output: `OK`

**Step 5: Commit**

```bash
git add clients/textual_chat.py
git commit -m "refactor(tui): add relay API helper methods

- Add request_session_list() to get sessions via WebSocket
- Add request_session_history() to get history via WebSocket
- Remove direct DB access constants and imports
- Prepare for full relay API integration"
```

---

## Task 5: Replace get_all_sessions() with relay API call

**Files:**
- Modify: `clients/textual_chat.py:825-848` (replace get_all_sessions function)
- Modify: `clients/textual_chat.py:323-366` (update on_mount to use new method)

**Step 1: Create new async get_all_sessions_from_relay function**

Replace the `get_all_sessions()` function (lines 825-848) with:

```python
async def get_all_sessions_from_relay(websocket) -> list:
    """Get list of all sessions from the relay server via WebSocket"""
    try:
        # Send session/list request
        request = {
            "jsonrpc": "2.0",
            "method": "session/list",
            "params": {},
            "id": 999  # Use high ID to avoid conflicts
        }
        await websocket.send(json.dumps(request))

        # Wait for response
        raw_msg = await asyncio.wait_for(websocket.recv(), timeout=5.0)
        msg = json.loads(raw_msg)

        if msg.get("id") == 999 and "result" in msg:
            sessions = msg["result"].get("sessions", [])
            # Convert to format expected by SessionSelectionScreen
            formatted_sessions = []
            for s in sessions:
                formatted_sessions.append({
                    "id": s["id"],
                    "working_directory": s["workingDirectory"],
                    "created_at": s["createdAt"],
                    "closed_at": s["closedAt"],
                    "is_active": s["isActive"]
                })
            return formatted_sessions
        elif "error" in msg:
            print(f"Error getting sessions: {msg['error'].get('message', 'unknown')}")
            return []
    except (asyncio.TimeoutError, Exception) as e:
        print(f"Failed to get sessions from relay: {e}")
        return []

    return []
```

**Step 2: Update on_mount to connect before getting sessions**

In the `show_session_selector` function inside `on_mount` (around line 324-328), change:

```python
async def show_session_selector():
    try:
        # Get all sessions from database and show selector
        sessions = get_all_sessions()
```

To:

```python
async def show_session_selector():
    try:
        # Connect to relay first
        self.websocket = await websockets.connect(RELAY_WS_URL)

        # Get all sessions from relay server
        sessions = await get_all_sessions_from_relay(self.websocket)
```

**Step 3: Test the TUI**

```bash
# Start relay server in one terminal
./acp-relay

# Start TUI in another terminal
uv run clients/textual_chat.py
```

Expected behavior:
- TUI connects to relay
- Session selection modal shows sessions from relay API
- No SQLite errors

**Step 4: Commit**

```bash
git add clients/textual_chat.py
git commit -m "refactor(tui): use relay API for session list

- Replace get_all_sessions() with get_all_sessions_from_relay()
- Connect to relay before fetching session list
- Remove direct SQLite database access for session listing
- Maintain same UI behavior with relay API backend"
```

---

## Task 6: Replace load_session_history() with relay API call

**Files:**
- Modify: `clients/textual_chat.py:470-530` (replace load_session_history method)

**Step 1: Replace load_session_history implementation**

Replace the `load_session_history` method (lines 470-530) with:

```python
    async def load_session_history(self):
        """Load previous messages from the relay server via WebSocket"""
        try:
            if not self.websocket:
                self.notify("Not connected to relay", severity="error")
                return

            # Send session/history request
            msg_id = 998  # Use high ID to avoid conflicts
            await self.send_message("session/history", {"sessionId": self.session_id}, msg_id)

            # Wait for response
            raw_msg = await asyncio.wait_for(self.websocket.recv(), timeout=10.0)
            msg = json.loads(raw_msg)

            if msg.get("id") != msg_id or "result" not in msg:
                self.notify("Invalid response from relay", severity="error")
                return

            messages = msg["result"].get("messages", [])

            # Replay messages in the UI
            for message_record in messages:
                direction = message_record.get("direction")
                raw_msg = message_record.get("rawMessage", "{}")
                timestamp_str = message_record.get("timestamp", "")
                method = message_record.get("method", "")

                try:
                    parsed_msg = json.loads(raw_msg)

                    # Handle client->relay messages (user prompts)
                    if direction == "client_to_relay" and method == "session/prompt":
                        params = parsed_msg.get("params", {})
                        content = params.get("content", [])
                        if content and len(content) > 0:
                            text = content[0].get("text", "")
                            if text:
                                # Convert ISO timestamp to HH:MM:SS
                                try:
                                    dt = datetime.fromisoformat(timestamp_str.replace('Z', '+00:00'))
                                    display_time = dt.strftime("%H:%M:%S")
                                except:
                                    display_time = datetime.now().strftime("%H:%M:%S")

                                messages_container = self.query_one("#messages", ScrollableContainer)
                                messages_container.mount(ChatMessage("user", text, display_time))

                    # Handle relay->client messages (session updates)
                    elif direction == "relay_to_client" and "method" in parsed_msg:
                        if parsed_msg.get("method") == "session/update":
                            params = parsed_msg.get("params", {})
                            update = params.get("update", {})
                            session_update_type = update.get("sessionUpdate")

                            # Convert ISO timestamp to HH:MM:SS
                            try:
                                dt = datetime.fromisoformat(timestamp_str.replace('Z', '+00:00'))
                                display_time = dt.strftime("%H:%M:%S")
                            except:
                                display_time = datetime.now().strftime("%H:%M:%S")

                            if session_update_type == "available_commands_update":
                                commands = update.get("availableCommands", [])
                                self.add_system_message(
                                    "",
                                    msg_type="available_commands_update",
                                    data={"availableCommands": commands}
                                )

                except json.JSONDecodeError:
                    pass

            # Scroll to bottom
            messages_container = self.query_one("#messages", ScrollableContainer)
            messages_container.scroll_end(animate=False)

        except asyncio.TimeoutError:
            self.notify("Timeout loading session history", severity="warning")
        except Exception as e:
            self.notify(f"Could not load history: {e}", severity="warning")
```

**Step 2: Test loading session history**

```bash
# Start relay server
./acp-relay

# Start TUI and select an existing session
uv run clients/textual_chat.py
```

Expected behavior:
- When viewing a closed session, history loads from relay API
- Messages display correctly in chronological order
- No SQLite errors

**Step 3: Commit**

```bash
git add clients/textual_chat.py
git commit -m "refactor(tui): use relay API for session history

- Replace load_session_history() to use relay WebSocket API
- Request session/history via WebSocket instead of DB query
- Parse and display messages using relay API response format
- Remove direct SQLite database access for history loading"
```

---

## Task 7: Remove mark_session_closed() function

**Files:**
- Modify: `clients/textual_chat.py:851-865` (remove mark_session_closed function)

**Step 1: Delete mark_session_closed function**

Remove the `mark_session_closed()` function entirely (lines 851-865):

```python
def mark_session_closed(session_id: str, reason: str = "stale"):
    """Mark a session as closed in the database"""
    try:
        conn = sqlite3.connect(DB_PATH)
        from datetime import datetime
        closed_at = datetime.now().isoformat()
        conn.execute("""
            UPDATE sessions
            SET closed_at = ?
            WHERE id = ?
        """, (closed_at, session_id))
        conn.commit()
        conn.close()
    except (sqlite3.Error, FileNotFoundError) as e:
        print(f"Failed to mark session as closed: {e}")
```

**Step 2: Search for uses of mark_session_closed**

```bash
cd /Users/harper/Public/src/2389/acp-relay
grep -n "mark_session_closed" clients/textual_chat.py
```

Expected output: No matches (function was defined but never called)

**Step 3: Verify TUI still works**

```bash
python3 -c "import sys; sys.path.insert(0, 'clients'); from textual_chat import ACPChatApp; print('OK')"
```

Expected output: `OK`

**Step 4: Commit**

```bash
git add clients/textual_chat.py
git commit -m "refactor(tui): remove mark_session_closed function

- Remove unused mark_session_closed() function
- Sessions are now managed entirely by relay server
- Complete removal of direct database access from TUI"
```

---

## Task 8: Build and test complete integration

**Files:**
- Build: `acp-relay` binary
- Test: Full end-to-end workflow

**Step 1: Build relay server**

```bash
cd /Users/harper/Public/src/2389/acp-relay
make build
```

Expected output: `Build successful` or similar

**Step 2: Start relay server**

```bash
./acp-relay
```

Expected output:
```
Server starting...
HTTP server listening on 0.0.0.0:8080
WebSocket server listening on 0.0.0.0:8081
Management server listening on 127.0.0.1:8082
```

**Step 3: Test TUI - Create new session**

In another terminal:

```bash
cd /Users/harper/Public/src/2389/acp-relay
uv run clients/textual_chat.py
```

Actions:
1. Select "Create New Session"
2. Type a message
3. Verify agent responds
4. Press Ctrl+C to exit

Expected: All works, no errors

**Step 4: Test TUI - Resume session**

```bash
uv run clients/textual_chat.py
```

Actions:
1. Select session from list (should show the session just created)
2. Verify session resumes successfully
3. Type another message
4. Press Ctrl+C to exit

Expected: Session list appears, resume works

**Step 5: Test TUI - View closed session**

```bash
uv run clients/textual_chat.py
```

Actions:
1. Select a closed session from list
2. Verify history displays in read-only mode
3. Input should be disabled
4. Press Ctrl+C to exit

Expected: History displays, input disabled

**Step 6: Check logs for relay API usage**

In the relay server logs, look for:
- `session/list` requests
- `session/history` requests

Expected: Logs show TUI using new API endpoints

**Step 7: Document testing results**

Create `docs/manual-test-results.md` section or append:

```markdown
## TUI Relay API Integration - 2025-11-12

**Tested:**
- ✅ Session list via `session/list` API
- ✅ Session history via `session/history` API
- ✅ Create new session flow
- ✅ Resume active session flow
- ✅ View closed session (read-only) flow
- ✅ No direct database access from TUI
- ✅ All session data flows through relay WebSocket API

**Results:** All tests passing, TUI fully uses relay API instead of direct DB access.
```

**Step 8: Commit**

```bash
git add docs/manual-test-results.md
git commit -m "test(tui): verify relay API integration

- Test session list via relay API
- Test session history via relay API
- Verify all session operations work through relay
- Confirm no direct database access from TUI
- Document manual testing results"
```

---

## Task 9: Run automated test suite

**Files:**
- Test: All Go tests
- Test: Pre-commit hooks

**Step 1: Run all Go tests**

```bash
cd /Users/harper/Public/src/2389/acp-relay
go test ./... -v
```

Expected output: All tests `PASS`

**Step 2: Run pre-commit hooks**

```bash
pre-commit run --all-files
```

Expected output: All hooks `Passed`

**Step 3: If any tests fail, fix them**

For each failing test:
1. Read the error message
2. Identify the issue
3. Fix the code
4. Re-run the test
5. Repeat until all pass

**Step 4: Commit any fixes**

```bash
git add .
git commit -m "fix: address test failures from relay API refactor

[Describe what was fixed]"
```

---

## Task 10: Update README and close

**Files:**
- Modify: `README.md` (update TUI section if needed)

**Step 1: Review README TUI section**

Check if README mentions database access or needs updating.

**Step 2: Update if needed**

If README mentions direct database access, update to reflect relay API usage:

```markdown
### TUI Client

The Python TUI client (`clients/textual_chat.py`) provides a terminal user interface for interacting with the relay server. It communicates exclusively via WebSocket API - no direct database access.

Features:
- Session management via relay API
- History viewing via relay API
- Real-time message streaming
- Permission request handling
```

**Step 3: Final commit**

```bash
git add README.md
git commit -m "docs: update README to reflect TUI relay API usage

- Document TUI uses relay WebSocket API exclusively
- Remove references to direct database access
- Update architecture description"
```

**Step 4: Push all changes**

```bash
git push origin main
```

---

## Completion Checklist

After all tasks complete:

- [ ] All tests pass (`go test ./...`)
- [ ] Pre-commit hooks pass
- [ ] TUI works end-to-end with relay API
- [ ] No direct database imports in TUI code
- [ ] Documentation updated
- [ ] Changes committed and pushed

## Reference Skills

- **@superpowers:test-driven-development** - Use TDD for each task
- **@superpowers:systematic-debugging** - If tests fail, debug systematically
- **@superpowers:verification-before-completion** - Verify each step before marking complete
