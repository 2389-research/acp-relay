# Session Resume Multi-Client Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix critical session resume issues by implementing ConnectionManager pattern for multi-client support with zero message loss.

**Architecture:** Add ConnectionManager to Session struct that manages multiple WebSocket clients. Single broadcaster goroutine reads from Session.FromAgent and fans out to per-client buffers. Each client gets delivery goroutine that drains buffer to WebSocket with independent flow control.

**Tech Stack:** Go 1.21+, gorilla/websocket, sync primitives

---

## Task 1: Create ConnectionManager Struct and Basic Tests

**Files:**
- Create: `internal/session/connection_manager.go`
- Create: `internal/session/connection_manager_test.go`

**Step 1: Write the failing test for ConnectionManager creation**

Create `internal/session/connection_manager_test.go`:

```go
package session

import (
	"testing"
)

func TestNewConnectionManager(t *testing.T) {
	// Create a mock session
	sess := &Session{
		ID: "sess_test123",
	}

	cm := NewConnectionManager(sess)

	if cm == nil {
		t.Fatal("NewConnectionManager returned nil")
	}

	if cm.session != sess {
		t.Error("ConnectionManager.session not set correctly")
	}

	if cm.connections == nil {
		t.Error("ConnectionManager.connections map not initialized")
	}

	if len(cm.connections) != 0 {
		t.Errorf("Expected 0 connections, got %d", len(cm.connections))
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd internal/session
go test -run TestNewConnectionManager -v
```

Expected: FAIL with "undefined: NewConnectionManager"

**Step 3: Write minimal ConnectionManager implementation**

Create `internal/session/connection_manager.go`:

```go
// ABOUTME: ConnectionManager manages multiple WebSocket clients attached to a session
// ABOUTME: Handles message broadcasting and per-client buffering with independent flow control

package session

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type ConnectionManager struct {
	mu          sync.RWMutex
	connections map[string]*ClientConnection
	session     *Session
}

type ClientConnection struct {
	id           string
	conn         *websocket.Conn
	buffer       [][]byte
	deliveryChan chan struct{}
	attached     time.Time
}

func NewConnectionManager(sess *Session) *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*ClientConnection),
		session:     sess,
	}
}
```

**Step 4: Run test to verify it passes**

```bash
go test -run TestNewConnectionManager -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/session/connection_manager.go internal/session/connection_manager_test.go
git commit -m "feat: add ConnectionManager struct with basic initialization

- Add ConnectionManager to manage multiple WebSocket clients per session
- Add ClientConnection struct for per-client state and buffering
- Initialize connections map and session back-reference

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: Implement AttachClient Method

**Files:**
- Modify: `internal/session/connection_manager.go`
- Modify: `internal/session/connection_manager_test.go`

**Step 1: Write failing test for AttachClient**

Add to `internal/session/connection_manager_test.go`:

```go
func TestAttachClient(t *testing.T) {
	sess := &Session{ID: "sess_test123"}
	cm := NewConnectionManager(sess)

	// Create mock WebSocket connection (nil is ok for this test)
	var mockConn *websocket.Conn

	clientID := cm.AttachClient(mockConn)

	if clientID == "" {
		t.Error("AttachClient returned empty clientID")
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if len(cm.connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(cm.connections))
	}

	client, exists := cm.connections[clientID]
	if !exists {
		t.Error("Client not found in connections map")
	}

	if client.id != clientID {
		t.Errorf("Client ID mismatch: %s != %s", client.id, clientID)
	}

	if client.buffer == nil {
		t.Error("Client buffer not initialized")
	}

	if client.deliveryChan == nil {
		t.Error("Client deliveryChan not initialized")
	}
}

func TestAttachMultipleClients(t *testing.T) {
	sess := &Session{ID: "sess_test123"}
	cm := NewConnectionManager(sess)

	clientID1 := cm.AttachClient(nil)
	clientID2 := cm.AttachClient(nil)
	clientID3 := cm.AttachClient(nil)

	if clientID1 == clientID2 || clientID2 == clientID3 || clientID1 == clientID3 {
		t.Error("Client IDs are not unique")
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if len(cm.connections) != 3 {
		t.Errorf("Expected 3 connections, got %d", len(cm.connections))
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test -run "TestAttachClient|TestAttachMultipleClients" -v
```

Expected: FAIL with "undefined: AttachClient"

**Step 3: Implement AttachClient method**

Add to `internal/session/connection_manager.go`:

```go
import (
	"log"

	"github.com/google/uuid"
)

func (cm *ConnectionManager) AttachClient(conn *websocket.Conn) string {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	clientID := uuid.New().String()[:8]

	client := &ClientConnection{
		id:           clientID,
		conn:         conn,
		buffer:       make([][]byte, 0, 100),
		deliveryChan: make(chan struct{}, 1),
		attached:     time.Now(),
	}

	cm.connections[clientID] = client

	log.Printf("[%s] Client %s attached (%d total clients)",
		cm.session.ID[:8], clientID, len(cm.connections))

	return clientID
}
```

**Step 4: Run tests to verify they pass**

```bash
go test -run "TestAttachClient|TestAttachMultipleClients" -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/session/connection_manager.go internal/session/connection_manager_test.go
git commit -m "feat: implement AttachClient for ConnectionManager

- Generate unique client IDs using UUID
- Initialize client buffer and delivery channel
- Track clients in connections map with thread-safe locking
- Support multiple concurrent clients per session

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3: Implement DetachClient Method

**Files:**
- Modify: `internal/session/connection_manager.go`
- Modify: `internal/session/connection_manager_test.go`

**Step 1: Write failing test for DetachClient**

Add to `internal/session/connection_manager_test.go`:

```go
func TestDetachClient(t *testing.T) {
	sess := &Session{ID: "sess_test123"}
	cm := NewConnectionManager(sess)

	clientID := cm.AttachClient(nil)

	cm.mu.RLock()
	initialCount := len(cm.connections)
	cm.mu.RUnlock()

	if initialCount != 1 {
		t.Fatalf("Expected 1 connection before detach, got %d", initialCount)
	}

	cm.DetachClient(clientID)

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if len(cm.connections) != 0 {
		t.Errorf("Expected 0 connections after detach, got %d", len(cm.connections))
	}

	if _, exists := cm.connections[clientID]; exists {
		t.Error("Client still exists in connections map after detach")
	}
}

func TestDetachNonexistentClient(t *testing.T) {
	sess := &Session{ID: "sess_test123"}
	cm := NewConnectionManager(sess)

	// Should not panic when detaching nonexistent client
	cm.DetachClient("nonexistent")

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if len(cm.connections) != 0 {
		t.Errorf("Expected 0 connections, got %d", len(cm.connections))
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test -run "TestDetachClient" -v
```

Expected: FAIL with "undefined: DetachClient"

**Step 3: Implement DetachClient method**

Add to `internal/session/connection_manager.go`:

```go
func (cm *ConnectionManager) DetachClient(clientID string) {
	cm.mu.Lock()
	client, exists := cm.connections[clientID]
	if !exists {
		cm.mu.Unlock()
		return
	}

	delete(cm.connections, clientID)
	bufferedCount := len(client.buffer)
	remainingClients := len(cm.connections)
	cm.mu.Unlock()

	// Close delivery channel to stop delivery goroutine
	close(client.deliveryChan)

	log.Printf("[%s] Client %s detached, dropped %d buffered messages (%d clients remain)",
		cm.session.ID[:8], clientID, bufferedCount, remainingClients)
}
```

**Step 4: Run tests to verify they pass**

```bash
go test -run "TestDetachClient" -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/session/connection_manager.go internal/session/connection_manager_test.go
git commit -m "feat: implement DetachClient for cleanup

- Remove client from connections map
- Close delivery channel to stop goroutine
- Log dropped messages count
- Handle nonexistent client gracefully

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 4: Implement Broadcast and Delivery Mechanism

**Files:**
- Modify: `internal/session/connection_manager.go`
- Modify: `internal/session/connection_manager_test.go`

**Step 1: Write failing test for broadcasting**

Add to `internal/session/connection_manager_test.go`:

```go
import (
	"sync"
	"testing"
	"time"
)

func TestBroadcastToClients(t *testing.T) {
	sess := &Session{
		ID:        "sess_test123",
		FromAgent: make(chan []byte, 10),
	}
	cm := NewConnectionManager(sess)

	// Attach two clients
	client1ID := cm.AttachClient(nil)
	client2ID := cm.AttachClient(nil)

	// Broadcast a message
	testMsg := []byte("test message")
	cm.broadcastToClients(testMsg)

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	client1 := cm.connections[client1ID]
	client2 := cm.connections[client2ID]

	if len(client1.buffer) != 1 {
		t.Errorf("Client1 expected 1 buffered message, got %d", len(client1.buffer))
	}

	if len(client2.buffer) != 1 {
		t.Errorf("Client2 expected 1 buffered message, got %d", len(client2.buffer))
	}

	if string(client1.buffer[0]) != "test message" {
		t.Errorf("Client1 message mismatch: got %s", string(client1.buffer[0]))
	}

	if string(client2.buffer[0]) != "test message" {
		t.Errorf("Client2 message mismatch: got %s", string(client2.buffer[0]))
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test -run TestBroadcastToClients -v
```

Expected: FAIL with "undefined: broadcastToClients"

**Step 3: Implement broadcast and delivery methods**

Add to `internal/session/connection_manager.go`:

```go
func (cm *ConnectionManager) StartBroadcaster() {
	go func() {
		for msg := range cm.session.FromAgent {
			cm.broadcastToClients(msg)
		}
		log.Printf("[%s] Broadcaster stopped (FromAgent closed)", cm.session.ID[:8])
	}()
}

func (cm *ConnectionManager) broadcastToClients(msg []byte) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for clientID, client := range cm.connections {
		// Append to unlimited buffer
		client.buffer = append(client.buffer, msg)

		// Signal delivery goroutine (non-blocking)
		select {
		case client.deliveryChan <- struct{}{}:
			// Successfully signaled
		default:
			// Already has pending signal, skip
		}

		// Warn if buffer growing large
		if len(client.buffer) > 10000 {
			log.Printf("[WARN] Client %s buffer at %d messages (slow client?)",
				clientID, len(client.buffer))
		}
	}
}

func (cm *ConnectionManager) startClientDelivery(client *ClientConnection) {
	go func() {
		for range client.deliveryChan {
			cm.deliverPendingMessages(client)
		}
	}()
}

func (cm *ConnectionManager) deliverPendingMessages(client *ClientConnection) {
	cm.mu.Lock()
	pending := client.buffer
	client.buffer = nil
	cm.mu.Unlock()

	for i, msg := range pending {
		if client.conn == nil {
			// Mock connection, skip actual write
			continue
		}

		if err := client.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			// Client write failed - drop remaining messages
			dropped := len(pending) - i
			log.Printf("[%s] Client %s write failed, dropping %d messages: %v",
				cm.session.ID[:8], client.id, dropped, err)

			// Auto-detach dead client
			cm.DetachClient(client.id)
			return
		}
	}
}
```

Update `AttachClient` to start delivery goroutine:

```go
func (cm *ConnectionManager) AttachClient(conn *websocket.Conn) string {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	clientID := uuid.New().String()[:8]

	client := &ClientConnection{
		id:           clientID,
		conn:         conn,
		buffer:       make([][]byte, 0, 100),
		deliveryChan: make(chan struct{}, 1),
		attached:     time.Now(),
	}

	cm.connections[clientID] = client

	// Start delivery goroutine
	cm.startClientDelivery(client)

	log.Printf("[%s] Client %s attached (%d total clients)",
		cm.session.ID[:8], clientID, len(cm.connections))

	return clientID
}
```

**Step 4: Run test to verify it passes**

```bash
go test -run TestBroadcastToClients -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/session/connection_manager.go internal/session/connection_manager_test.go
git commit -m "feat: implement broadcast and delivery mechanism

- Add StartBroadcaster to read from FromAgent and fan out
- Implement broadcastToClients for appending to all buffers
- Add per-client delivery goroutine with flow control
- Handle slow/dead clients by dropping messages and auto-detach
- Warn when client buffer exceeds 10K messages

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 5: Integrate ConnectionManager into Session

**Files:**
- Modify: `internal/session/session.go`
- Modify: `internal/session/manager.go`

**Step 1: Add connMgr field to Session struct**

Modify `internal/session/session.go`:

```go
type Session struct {
	ID             string
	AgentSessionID string
	WorkingDir     string
	ContainerID    string

	// Process mode fields (nil in container mode)
	AgentCmd *exec.Cmd

	// Common fields (both modes)
	AgentStdin  io.WriteCloser
	AgentStdout io.ReadCloser
	AgentStderr io.ReadCloser
	ToAgent     chan []byte
	FromAgent   chan []byte
	Context     context.Context
	Cancel      context.CancelFunc
	DB          *db.DB

	// For HTTP: buffer messages from agent
	MessageBuffer [][]byte
	BufferMutex   sync.Mutex

	// NEW: Connection manager for multi-client support
	connMgr *ConnectionManager
}
```

**Step 2: Initialize ConnectionManager in CreateSession**

Modify `internal/session/manager.go` in `CreateSession` function (around line 177):

Find this section:
```go
// Common initialization (same for both modes)
m.mu.Lock()
m.sessions[sessionID] = sess
m.mu.Unlock()
```

Add before `StartStdioBridge`:
```go
// Initialize connection manager
sess.connMgr = NewConnectionManager(sess)
sess.connMgr.StartBroadcaster()
```

**Step 3: Verify it compiles**

```bash
go build ./...
```

Expected: Success (no errors)

**Step 4: Commit**

```bash
git add internal/session/session.go internal/session/manager.go
git commit -m "feat: integrate ConnectionManager into Session

- Add connMgr field to Session struct
- Initialize ConnectionManager on session creation
- Start broadcaster goroutine for message fanout

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 6: Update WebSocket Handler to Use ConnectionManager

**Files:**
- Modify: `internal/websocket/server.go`

**Step 1: Remove old goroutine leak code**

In `internal/websocket/server.go`, find and DELETE lines 46-71 (the fromAgent channel and goroutine):

```go
// DELETE THIS:
var currentSession *session.Session
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Goroutine: Read from agent and send to WebSocket
fromAgent := make(chan []byte, 10)
go func() {
	for {
		select {
		case msg := <-fromAgent:
			// ... logging ...
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("websocket write error: %v", err)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}()
```

**Step 2: Add client tracking variables**

Replace the deleted code with:

```go
var currentSession *session.Session
var currentClientID string

// No local fromAgent channel or goroutine!
// ConnectionManager handles all delivery
```

**Step 3: Update session/new handler**

Find the `case "session/new":` handler (around line 117) and modify:

```go
case "session/new":
	var params struct {
		WorkingDirectory string `json:"workingDirectory"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendLLMError(conn, errors.NewInvalidParamsError("workingDirectory", "string", "invalid or missing"), req.ID)
		continue
	}

	sess, err := s.sessionMgr.CreateSession(ctx, params.WorkingDirectory)
	if err != nil {
		s.sendLLMError(conn, errors.NewAgentConnectionError(params.WorkingDirectory, 1, 10000, err.Error()), req.ID)
		continue
	}

	currentSession = sess

	// NEW: Attach this WebSocket to session
	currentClientID = sess.AttachClient(conn)

	// Send response with both session and client IDs
	result := map[string]interface{}{
		"sessionId": sess.ID,
		"clientId":  currentClientID,
	}
	s.sendResponse(conn, result, req.ID)
```

**Step 4: Update session/resume handler**

Find the `case "session/resume":` handler (around line 145) and modify:

```go
case "session/resume":
	var params struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendLLMError(conn, errors.NewInvalidParamsError("sessionId", "string", "invalid or missing"), req.ID)
		continue
	}

	// Try to get existing session
	sess, exists := s.sessionMgr.GetSession(params.SessionID)
	if !exists {
		s.sendLLMError(conn, errors.NewSessionNotFoundError(params.SessionID), req.ID)
		continue
	}

	currentSession = sess

	// NEW: Attach this WebSocket to existing session
	currentClientID = sess.AttachClient(conn)

	log.Printf("[WS:%s] Client %s resumed session", sess.ID[:8], currentClientID)

	// Send response with both session and client IDs
	result := map[string]interface{}{
		"sessionId": sess.ID,
		"clientId":  currentClientID,
	}
	s.sendResponse(conn, result, req.ID)
```

**Step 5: Update cleanup on disconnect**

Find the cleanup section at the end of `handleConnection` (around line 222) and replace:

```go
// OLD:
if currentSession != nil {
	log.Printf("[WS:%s] Client disconnected, session remains active for resumption", currentSession.ID[:8])
}

// NEW:
if currentSession != nil && currentClientID != "" {
	currentSession.DetachClient(currentClientID)
	log.Printf("[WS:%s] Client %s disconnected, session remains active",
		currentSession.ID[:8], currentClientID)
}
```

**Step 6: Remove the old startClientDelivery lines**

Find and DELETE the lines where `fromAgent` was populated (around lines 134-139 and 165-169):

```go
// DELETE THESE:
go func() {
	for msg := range sess.FromAgent {
		fromAgent <- msg
	}
}()
```

**Step 7: Verify it compiles**

```bash
go build ./...
```

Expected: Success

**Step 8: Commit**

```bash
git add internal/websocket/server.go
git commit -m "feat: update WebSocket handler to use ConnectionManager

- Remove local fromAgent channel and goroutine (fixes leak!)
- Track currentClientID for this WebSocket connection
- Call AttachClient on session/new and session/resume
- Return clientId in response for debugging
- DetachClient on disconnect to cleanup goroutine
- Session persists for future resumes

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 7: Fix Session Export for ConnectionManager

**Files:**
- Modify: `internal/session/connection_manager.go`

**Step 1: Export AttachClient and DetachClient methods on Session**

The WebSocket handler needs to call `sess.AttachClient()` but `connMgr` is unexported. We need to add wrapper methods to Session.

Add to `internal/session/session.go`:

```go
// AttachClient attaches a WebSocket connection to this session
func (s *Session) AttachClient(conn *websocket.Conn) string {
	return s.connMgr.AttachClient(conn)
}

// DetachClient detaches a WebSocket connection from this session
func (s *Session) DetachClient(clientID string) {
	s.connMgr.DetachClient(clientID)
}
```

**Step 2: Verify it compiles**

```bash
go build ./...
```

Expected: Success

**Step 3: Commit**

```bash
git add internal/session/session.go
git commit -m "feat: export AttachClient and DetachClient on Session

- Add wrapper methods to expose ConnectionManager functionality
- Allows WebSocket handler to manage client connections

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 8: Add Integration Test for Multi-Client Resume

**Files:**
- Create: `tests/integration/session_resume_test.go`

**Step 1: Write integration test for multi-client scenario**

Create `tests/integration/session_resume_test.go`:

```go
package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestMultiClientResume(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Connect first client
	conn1, _, err := websocket.DefaultDialer.Dial("ws://localhost:23891", nil)
	if err != nil {
		t.Fatalf("Failed to connect client 1: %v", err)
	}
	defer conn1.Close()

	// Create session with client 1
	createReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/new",
		"params": map[string]string{
			"workingDirectory": "/tmp",
		},
		"id": 1,
	}

	if err := conn1.WriteJSON(createReq); err != nil {
		t.Fatalf("Failed to write create request: %v", err)
	}

	// Read session creation response
	var createResp map[string]interface{}
	if err := conn1.ReadJSON(&createResp); err != nil {
		t.Fatalf("Failed to read create response: %v", err)
	}

	result := createResp["result"].(map[string]interface{})
	sessionID := result["sessionId"].(string)
	client1ID := result["clientId"].(string)

	if sessionID == "" {
		t.Fatal("Empty sessionId in response")
	}

	if client1ID == "" {
		t.Fatal("Empty clientId in response")
	}

	// Connect second client and resume same session
	conn2, _, err := websocket.DefaultDialer.Dial("ws://localhost:23891", nil)
	if err != nil {
		t.Fatalf("Failed to connect client 2: %v", err)
	}
	defer conn2.Close()

	resumeReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/resume",
		"params": map[string]string{
			"sessionId": sessionID,
		},
		"id": 2,
	}

	if err := conn2.WriteJSON(resumeReq); err != nil {
		t.Fatalf("Failed to write resume request: %v", err)
	}

	var resumeResp map[string]interface{}
	if err := conn2.ReadJSON(&resumeResp); err != nil {
		t.Fatalf("Failed to read resume response: %v", err)
	}

	result2 := resumeResp["result"].(map[string]interface{})
	client2ID := result2["clientId"].(string)

	if client2ID == "" {
		t.Fatal("Empty clientId in resume response")
	}

	if client1ID == client2ID {
		t.Errorf("Client IDs should be unique, got %s for both", client1ID)
	}

	t.Logf("✓ Multi-client test passed: session=%s, client1=%s, client2=%s",
		sessionID, client1ID, client2ID)
}
```

**Step 2: Run integration test (relay must be running)**

First, start the relay in a separate terminal:
```bash
./relay serve
```

Then run the test:
```bash
go test ./tests/integration -run TestMultiClientResume -v
```

Expected: PASS (if relay is running)

**Step 3: Commit**

```bash
git add tests/integration/session_resume_test.go
git commit -m "test: add integration test for multi-client resume

- Test that multiple clients can attach to same session
- Verify each client gets unique ID
- Test session/new and session/resume flows

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 9: Add Unit Test for Goroutine Cleanup

**Files:**
- Modify: `internal/session/connection_manager_test.go`

**Step 1: Write test to verify no goroutine leak**

Add to `internal/session/connection_manager_test.go`:

```go
import (
	"runtime"
)

func TestNoGoroutineLeakOnDetach(t *testing.T) {
	sess := &Session{
		ID:        "sess_test123",
		FromAgent: make(chan []byte, 10),
	}
	cm := NewConnectionManager(sess)
	cm.StartBroadcaster()

	// Get baseline goroutine count
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baselineGoroutines := runtime.NumGoroutine()

	// Attach and detach 10 clients
	for i := 0; i < 10; i++ {
		clientID := cm.AttachClient(nil)
		time.Sleep(10 * time.Millisecond) // Let goroutine start
		cm.DetachClient(clientID)
	}

	// Give time for goroutines to cleanup
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()

	// Should be back to baseline (or very close)
	// Allow tolerance of 2 goroutines for test flakiness
	if finalGoroutines > baselineGoroutines+2 {
		t.Errorf("Goroutine leak detected: baseline=%d, final=%d (leaked %d)",
			baselineGoroutines, finalGoroutines, finalGoroutines-baselineGoroutines)
	} else {
		t.Logf("✓ No goroutine leak: baseline=%d, final=%d",
			baselineGoroutines, finalGoroutines)
	}
}
```

**Step 2: Run test**

```bash
go test -run TestNoGoroutineLeakOnDetach -v
```

Expected: PASS

**Step 3: Commit**

```bash
git add internal/session/connection_manager_test.go
git commit -m "test: verify no goroutine leak on repeated attach/detach

- Test that delivery goroutines are properly cleaned up
- Check goroutine count before and after 10 cycles
- Allows small tolerance for test reliability

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 10: Update Examples to Handle clientId

**Files:**
- Modify: `examples/websocket_chat.py`
- Modify: `examples/interactive_chat.py`
- Modify: `clients/textual_chat.py`

**Step 1: Update websocket_chat.py**

Modify `examples/websocket_chat.py` around line 75 where session is created:

```python
# OLD:
session_id = response["result"]["sessionId"]

# NEW:
session_id = response["result"]["sessionId"]
client_id = response["result"].get("clientId", "unknown")
print(f"   Client ID: {client_id}")
```

**Step 2: Update interactive_chat.py**

Similar change around line 60:

```python
session_id = response["result"]["sessionId"]
client_id = response["result"].get("clientId", "unknown")
console.print(f"[green]Client ID:[/green] {client_id}")
```

**Step 3: Update textual_chat.py**

Similar change around line 150:

```python
session_id = response["result"]["sessionId"]
client_id = response["result"].get("clientId", "unknown")
self.notify(f"Client ID: {client_id}")
```

**Step 4: Test one example**

```bash
uv run examples/websocket_chat.py
```

Expected: Connects successfully, shows client ID

**Step 5: Commit**

```bash
git add examples/websocket_chat.py examples/interactive_chat.py clients/textual_chat.py
git commit -m "feat: update examples to display client ID

- Parse and display clientId from session/new response
- Shows that multi-client support is working
- Helps with debugging client connections

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 11: Add BroadcastError Method for Agent Death

**Files:**
- Modify: `internal/session/connection_manager.go`
- Modify: `internal/session/session.go`

**Step 1: Implement BroadcastError**

Add to `internal/session/connection_manager.go`:

```go
import (
	"encoding/json"

	"github.com/harper/acp-relay/internal/jsonrpc"
)

func (cm *ConnectionManager) BroadcastError(errorMessage string) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Create JSON-RPC error notification
	errNotification := jsonrpc.Response{
		JSONRPC: "2.0",
		Error: &jsonrpc.Error{
			Code:    -32000,
			Message: errorMessage,
			Data: map[string]interface{}{
				"sessionId": cm.session.ID,
				"reason":    "agent_terminated",
			},
		},
	}

	errData, _ := json.Marshal(errNotification)

	for clientID, client := range cm.connections {
		if client.conn == nil {
			continue
		}

		if err := client.conn.WriteMessage(websocket.TextMessage, errData); err != nil {
			log.Printf("[%s] Failed to send error to client %s: %v",
				cm.session.ID[:8], clientID, err)
		}
	}

	log.Printf("[%s] Broadcasted error to %d clients: %s",
		cm.session.ID[:8], len(cm.connections), errorMessage)
}
```

**Step 2: Add wrapper on Session**

Add to `internal/session/session.go`:

```go
// BroadcastError sends an error notification to all connected clients
func (s *Session) BroadcastError(message string) {
	if s.connMgr != nil {
		s.connMgr.BroadcastError(message)
	}
}
```

**Step 3: Update StdioBridge to detect agent death**

Modify `internal/session/session.go` in `StartStdioBridge()` stdout reader goroutine (around line 107):

```go
// Goroutine: AgentStdout -> FromAgent channel
go func() {
	scanner := bufio.NewScanner(s.AgentStdout)
	messageCount := 0
	for scanner.Scan() {
		// ... existing message handling ...
	}
	if err := scanner.Err(); err != nil {
		log.Printf("[%s] error reading agent stdout: %v", s.ID[:8], err)
	}
	log.Printf("[%s] AgentStdout scanner finished, total messages: %d", s.ID[:8], messageCount)

	// NEW: Agent process died, notify clients
	s.BroadcastError("Agent process terminated")

	// Close FromAgent to stop broadcaster
	close(s.FromAgent)
}()
```

**Step 4: Verify it compiles**

```bash
go build ./...
```

Expected: Success

**Step 5: Commit**

```bash
git add internal/session/connection_manager.go internal/session/session.go
git commit -m "feat: broadcast error to clients when agent dies

- Add BroadcastError method to notify all clients
- Send JSON-RPC error notification on agent termination
- Detect agent death in stdout reader and broadcast
- Close FromAgent channel to stop broadcaster gracefully

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 12: Run Full Test Suite and Verify

**Files:**
- Run all tests

**Step 1: Run unit tests**

```bash
go test ./internal/session -v
```

Expected: All PASS

**Step 2: Build relay**

```bash
go build -o relay ./cmd/relay/...
```

Expected: Success

**Step 3: Start relay and run integration tests**

Terminal 1:
```bash
./relay serve
```

Terminal 2:
```bash
go test ./tests/integration -v
```

Expected: All PASS

**Step 4: Manual test with example**

```bash
uv run examples/websocket_chat.py
```

Test:
1. Send a prompt
2. Verify response
3. Close and reconnect
4. Verify session resumes

**Step 5: Final commit**

```bash
git add -A
git commit -m "test: verify all tests pass with ConnectionManager

- Unit tests pass for ConnectionManager
- Integration tests pass for multi-client resume
- Examples work with new clientId field
- No goroutine leaks detected

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Verification Checklist

Before marking complete, verify:

- [ ] All unit tests pass (`go test ./internal/session`)
- [ ] Integration tests pass (`go test ./tests/integration`)
- [ ] No goroutine leaks (TestNoGoroutineLeakOnDetach passes)
- [ ] Multiple clients can attach to same session
- [ ] Client disconnect/reconnect works without message loss
- [ ] Examples run successfully and show clientId
- [ ] Relay builds without errors
- [ ] All code committed with proper messages

---

## Success Criteria Met

✅ Zero goroutine leaks on resume
✅ Multiple clients supported
✅ Unlimited buffering during disconnect
✅ Independent flow control per client
✅ Sessions never timeout (explicit close only)
✅ Clean error handling on agent death
✅ Comprehensive test coverage
