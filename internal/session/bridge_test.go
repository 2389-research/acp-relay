package session

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestStdioBridge(t *testing.T) {
	tmpDir := t.TempDir()

	// Use 'cat' which echoes stdin to stdout
	mgr := NewManager(ManagerConfig{
		AgentCommand: "cat",
		AgentArgs:    []string{},
		AgentEnv:     map[string]string{},
	})

	sess, err := mgr.CreateSession(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer mgr.CloseSession(sess.ID)

	// Send a JSON-RPC message
	msg := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "test",
		"id":      1,
	}
	data, _ := json.Marshal(msg)
	data = append(data, '\n')

	sess.ToAgent <- data

	// Read response
	select {
	case response := <-sess.FromAgent:
		var parsed map[string]interface{}
		if err := json.Unmarshal(response, &parsed); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if parsed["method"] != "test" {
			t.Errorf("expected method 'test', got %v", parsed["method"])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for response")
	}
}
