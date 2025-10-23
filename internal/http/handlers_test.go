package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/harper/acp-relay/internal/session"
)

func TestSessionNew(t *testing.T) {
	// Get absolute path to mock agent
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	mockAgentPath := filepath.Join(projectRoot, "testdata", "mock_agent.py")

	mgr := session.NewManager(session.ManagerConfig{
		Mode:         "process",
		AgentCommand: "python3",
		AgentArgs:    []string{mockAgentPath},
		AgentEnv:     map[string]string{},
	}, nil)

	srv := NewServer(mgr)

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/new",
		"params": map[string]interface{}{
			"workingDirectory": t.TempDir(),
		},
		"id": 1,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/session/new", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatal("expected result object")
	}

	if result["sessionId"] == "" {
		t.Error("expected sessionId in result")
	}
}
