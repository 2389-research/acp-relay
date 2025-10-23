package websocket

import (
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/harper/acp-relay/internal/session"
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
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer ws.Close()

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
