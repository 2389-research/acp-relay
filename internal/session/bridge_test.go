package session

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStdioBridge(t *testing.T) {
	tmpDir := t.TempDir()

	// Get absolute path to mock agent
	_, filename, _, _ := runtime.Caller(0)
	mockAgentPath := filepath.Join(filepath.Dir(filename), "testdata", "mock_agent.py")

	mgr := NewManager(ManagerConfig{
		AgentCommand: "python3",
		AgentArgs:    []string{mockAgentPath},
		AgentEnv:     map[string]string{},
	}, nil) // nil db for test

	sess, err := mgr.CreateSession(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer func() { _ = mgr.CloseSession(sess.ID) }()

	// Send a JSON-RPC message (non-initialize method)
	msg := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "test",
		"id":      2,
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	data = append(data, '\n')

	sess.ToAgent <- data

	// Read response
	select {
	case response := <-sess.FromAgent:
		var parsed map[string]interface{}
		if err := json.Unmarshal(response, &parsed); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		// Mock agent echoes back the request in the result
		result, ok := parsed["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected result object in response")
		}
		echo, ok := result["echo"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected echo in result")
		}
		if echo["method"] != "test" {
			t.Errorf("expected method 'test', got %v", echo["method"])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for response")
	}
}
