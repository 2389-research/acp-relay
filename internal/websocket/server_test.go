package websocket

import (
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/harper/acp-relay/internal/db"
	"github.com/harper/acp-relay/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebSocketConnection(t *testing.T) {
	// Get absolute path to mock agent
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	mockAgentPath := filepath.Join(projectRoot, "testdata", "mock_agent.py")

	mgr := session.NewManager(session.ManagerConfig{
		AgentCommand: "python3",
		AgentArgs:    []string{mockAgentPath},
		AgentEnv:     map[string]string{},
	}, nil) // nil db for test

	srv := NewServer(mgr)

	// Create test server
	httpSrv := httptest.NewServer(srv)
	defer httpSrv.Close()

	// Connect via WebSocket
	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil) //nolint:bodyclose // websocket connection, not HTTP response
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = ws.Close() }()

	// Send session/new
	tmpDir := t.TempDir()
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/new",
		"params": map[string]interface{}{
			"workingDirectory": tmpDir,
		},
		"id": 1,
	}

	if err := ws.WriteJSON(req); err != nil {
		t.Fatalf("failed to send: %v", err)
	}

	// Read response
	var resp map[string]interface{}
	if err := ws.ReadJSON(&resp); err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatal("expected result object")
	}

	if result["sessionId"] == "" {
		t.Error("expected sessionId in result")
	}
}

//nolint:funlen // integration test with multiple clients
func TestMultiClientResume(t *testing.T) {
	// Get absolute path to mock agent
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	mockAgentPath := filepath.Join(projectRoot, "testdata", "mock_agent.py")

	mgr := session.NewManager(session.ManagerConfig{
		AgentCommand: "python3",
		AgentArgs:    []string{mockAgentPath},
		AgentEnv:     map[string]string{},
	}, nil) // nil db for test

	srv := NewServer(mgr)

	// Create test server
	httpSrv := httptest.NewServer(srv)
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")

	// === Client 1: Create session ===
	t.Log("Client 1: Creating session...")
	ws1, _, err := websocket.DefaultDialer.Dial(wsURL, nil) //nolint:bodyclose // websocket connection, not HTTP response
	if err != nil {
		t.Fatalf("Client 1: failed to connect: %v", err)
	}
	defer func() { _ = ws1.Close() }()

	tmpDir := t.TempDir()
	req1 := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/new",
		"params": map[string]interface{}{
			"workingDirectory": tmpDir,
		},
		"id": 1,
	}

	if err := ws1.WriteJSON(req1); err != nil {
		t.Fatalf("Client 1: failed to send session/new: %v", err)
	}

	var resp1 map[string]interface{}
	if err := ws1.ReadJSON(&resp1); err != nil {
		t.Fatalf("Client 1: failed to read session/new response: %v", err)
	}

	result1, ok := resp1["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Client 1: expected result object, got %v", resp1)
	}

	sessionID, ok := result1["sessionId"].(string)
	if !ok || sessionID == "" {
		t.Fatalf("Client 1: expected sessionId in result")
	}

	clientID1, ok := result1["clientId"].(string)
	if !ok || clientID1 == "" {
		t.Fatalf("Client 1: expected clientId in result")
	}

	t.Logf("✓ Session created: %s, Client 1 ID: %s", sessionID, clientID1)

	// Note: initialize and session/new responses are consumed by the relay during
	// session creation handshake, so they won't be broadcast to WebSocket clients.
	// Only messages AFTER session creation will be broadcast.

	// === Client 2: Resume session ===
	t.Log("Client 2: Resuming session...")
	ws2, _, err := websocket.DefaultDialer.Dial(wsURL, nil) //nolint:bodyclose // websocket connection, not HTTP response
	if err != nil {
		t.Fatalf("Client 2: failed to connect: %v", err)
	}
	defer func() { _ = ws2.Close() }()

	req2 := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/resume",
		"params": map[string]interface{}{
			"sessionId": sessionID,
		},
		"id": 2,
	}

	if err := ws2.WriteJSON(req2); err != nil {
		t.Fatalf("Client 2: failed to send session/resume: %v", err)
	}

	var resp2 map[string]interface{}
	if err := ws2.ReadJSON(&resp2); err != nil {
		t.Fatalf("Client 2: failed to read session/resume response: %v", err)
	}

	result2, ok := resp2["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Client 2: expected result object, got %v", resp2)
	}

	resumedSessionID, ok := result2["sessionId"].(string)
	if !ok || resumedSessionID != sessionID {
		t.Fatalf("Client 2: expected sessionId %s, got %v", sessionID, result2["sessionId"])
	}

	clientID2, ok := result2["clientId"].(string)
	if !ok || clientID2 == "" {
		t.Fatalf("Client 2: expected clientId in result")
	}

	if clientID1 == clientID2 {
		t.Fatalf("Expected different client IDs, got same: %s", clientID1)
	}

	t.Logf("✓ Client 2 ID: %s (resumed session %s)", clientID2, sessionID)

	// === Send a test message to agent ===
	t.Log("Sending test message to agent...")
	testReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "test",
		"id":      3,
	}

	if err := ws1.WriteJSON(testReq); err != nil {
		t.Fatalf("Failed to send test message: %v", err)
	}

	// Both clients should receive the response
	var msg1, msg2 map[string]interface{}

	if err := ws1.ReadJSON(&msg1); err != nil {
		t.Fatalf("Client 1: failed to read test response: %v", err)
	}

	if err := ws2.ReadJSON(&msg2); err != nil {
		t.Fatalf("Client 2: failed to read test response: %v", err)
	}

	// Both should receive the same message
	if msg1["id"] != msg2["id"] {
		t.Errorf("Messages differ: Client 1 got %v, Client 2 got %v", msg1, msg2)
	}

	t.Logf("✓ Both clients received the same message (id=%v)", msg1["id"])

	// Close client 1
	t.Log("Closing Client 1...")
	_ = ws1.Close()

	// Client 2 should still work
	testReq2 := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "test",
		"id":      4,
	}

	if err := ws2.WriteJSON(testReq2); err != nil {
		t.Fatalf("Client 2: failed to send after Client 1 closed: %v", err)
	}

	var msg3 map[string]interface{}
	if err := ws2.ReadJSON(&msg3); err != nil {
		t.Fatalf("Client 2: failed to read after Client 1 closed: %v", err)
	}

	t.Logf("✓ Client 2 still works after Client 1 disconnected")
}

func TestWebSocketSessionList(t *testing.T) {
	// Setup test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = database.Close() }()

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
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil) //nolint:bodyclose // websocket connection, not HTTP response
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

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
