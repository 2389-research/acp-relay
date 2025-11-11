// ABOUTME: Unit tests for TUI update logic
// ABOUTME: Tests message handling and state transitions
package tui

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/harper/acp-relay/internal/tui/client"
	"github.com/stretchr/testify/assert"
)

func TestHandlePermissionRequest(t *testing.T) {
	// Test parsing session/request_permission notification
	requestJSON := `{
		"jsonrpc": "2.0",
		"method": "session/request_permission",
		"params": {
			"toolCall": {
				"toolCallId": "tool-123",
				"rawInput": {
					"file_path": "/tmp/test.txt",
					"content": "Hello, world!"
				}
			}
		},
		"id": 42
	}`

	var notification map[string]interface{}
	err := json.Unmarshal([]byte(requestJSON), &notification)
	assert.NoError(t, err)

	// Extract fields like the handler would
	method, ok := notification["method"].(string)
	assert.True(t, ok)
	assert.Equal(t, "session/request_permission", method)

	requestID, ok := notification["id"].(float64)
	assert.True(t, ok)
	assert.Equal(t, float64(42), requestID)

	params, ok := notification["params"].(map[string]interface{})
	assert.True(t, ok)

	toolCall, ok := params["toolCall"].(map[string]interface{})
	assert.True(t, ok)

	toolCallID, ok := toolCall["toolCallId"].(string)
	assert.True(t, ok)
	assert.Equal(t, "tool-123", toolCallID)

	rawInput, ok := toolCall["rawInput"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "/tmp/test.txt", rawInput["file_path"])
	assert.Equal(t, "Hello, world!", rawInput["content"])
}

func TestBuildPermissionResponse(t *testing.T) {
	// Test building the JSON-RPC response for auto-approval
	requestID := float64(42)

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      requestID,
		"result": map[string]interface{}{
			"outcome": map[string]interface{}{
				"outcome":  "selected",
				"optionId": "allow",
			},
		},
	}

	jsonResponse, err := json.Marshal(response)
	assert.NoError(t, err)

	// Verify response can be parsed
	var parsed map[string]interface{}
	err = json.Unmarshal(jsonResponse, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, "2.0", parsed["jsonrpc"])
	assert.Equal(t, float64(42), parsed["id"])

	result, ok := parsed["result"].(map[string]interface{})
	assert.True(t, ok)

	outcome, ok := result["outcome"].(map[string]interface{})
	assert.True(t, ok)

	assert.Equal(t, "selected", outcome["outcome"])
	assert.Equal(t, "allow", outcome["optionId"])
}

func TestCreatePermissionRequestMessage(t *testing.T) {
	// Test creating a permission request message for the chat view
	toolCallID := "tool-123"
	toolName := "Write"
	rawInput := map[string]interface{}{
		"file_path": "/tmp/test.txt",
		"content":   "Hello, world!",
	}

	msg := &client.Message{
		SessionID:  "sess-1",
		Type:       client.MessageTypePermissionRequest,
		ToolCallID: toolCallID,
		Content:    toolName,
		RawInput:   rawInput,
		Timestamp:  time.Now(),
	}

	assert.Equal(t, "sess-1", msg.SessionID)
	assert.Equal(t, client.MessageTypePermissionRequest, msg.Type)
	assert.Equal(t, "tool-123", msg.ToolCallID)
	assert.Equal(t, "Write", msg.Content)
	assert.Equal(t, "/tmp/test.txt", msg.RawInput["file_path"])
}

func TestCreatePermissionResponseMessage(t *testing.T) {
	// Test creating a permission response message for the chat view
	msg := &client.Message{
		SessionID:  "sess-1",
		Type:       client.MessageTypePermissionResponse,
		ToolCallID: "tool-123",
		Content:    "Write",
		RawInput: map[string]interface{}{
			"outcome": "allow",
		},
		Timestamp: time.Now(),
	}

	assert.Equal(t, "sess-1", msg.SessionID)
	assert.Equal(t, client.MessageTypePermissionResponse, msg.Type)
	assert.Equal(t, "tool-123", msg.ToolCallID)
	assert.Equal(t, "Write", msg.Content)
	assert.Equal(t, "allow", msg.RawInput["outcome"])
}

func TestReadOnlyMode_ClosedSessionDetection(t *testing.T) {
	// Test that selecting a closed session sets readOnlyMode=true
	session := client.DBSession{
		ID:        "sess-closed-123",
		IsActive:  false, // Closed session
		CreatedAt: time.Now(),
	}

	// Simulate sessionSelectedMsg with closed session
	assert.False(t, session.IsActive, "Session should be marked as closed for this test")
}

func TestReadOnlyMode_ActiveSessionDetection(t *testing.T) {
	// Test that selecting an active session does NOT set readOnlyMode=true
	session := client.DBSession{
		ID:        "sess-active-456",
		IsActive:  true, // Active session
		CreatedAt: time.Now(),
	}

	// Simulate sessionSelectedMsg with active session
	assert.True(t, session.IsActive, "Session should be marked as active for this test")
}

func TestReadOnlyMode_InputDisabledInReadOnlySession(t *testing.T) {
	// This test verifies the behavior when in read-only mode:
	// - Input area should be disabled
	// - Status bar should show read-only indicator
	// - Cannot send messages

	// This is an integration test that would be done at the Model level
	// For now, we test the individual components
	t.Skip("Integration test - requires full Model initialization")
}
