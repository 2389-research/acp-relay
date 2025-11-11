// ABOUTME: Unit tests for permission widget rendering
// ABOUTME: Tests permission request and response formatting with icons and colors
package components

import (
	"testing"
	"time"

	"github.com/harper/acp-relay/internal/tui/client"
	"github.com/harper/acp-relay/internal/tui/theme"
	"github.com/stretchr/testify/assert"
)

func TestFormatPermissionRequest(t *testing.T) {
	th := theme.GetTheme("default", nil)

	msg := &client.Message{
		SessionID:  "sess-1",
		Type:       client.MessageTypePermissionRequest,
		ToolCallID: "tool-123",
		Content:    "Write",
		RawInput: map[string]interface{}{
			"file_path": "/tmp/test.txt",
			"content":   "Hello, world!",
		},
		Timestamp: time.Now(),
	}

	output := FormatPermissionRequest(msg, th)

	// Should contain icon
	assert.Contains(t, output, "🔐")
	// Should contain title
	assert.Contains(t, output, "Permission Request")
	// Should contain tool name
	assert.Contains(t, output, "Write")
	// Should contain arguments
	assert.Contains(t, output, "file_path")
	assert.Contains(t, output, "/tmp/test.txt")
}

func TestFormatPermissionRequest_TruncatesLongArguments(t *testing.T) {
	th := theme.GetTheme("default", nil)

	longContent := string(make([]byte, 150)) // More than 100 chars
	for i := range longContent {
		longContent = longContent[:i] + "a" + longContent[i+1:]
	}

	msg := &client.Message{
		SessionID:  "sess-1",
		Type:       client.MessageTypePermissionRequest,
		ToolCallID: "tool-123",
		Content:    "Write",
		RawInput: map[string]interface{}{
			"file_path": "/tmp/test.txt",
			"content":   longContent,
		},
		Timestamp: time.Now(),
	}

	output := FormatPermissionRequest(msg, th)

	// Should truncate long arguments
	assert.Contains(t, output, "...")
}

func TestFormatPermissionResponse_Allow(t *testing.T) {
	th := theme.GetTheme("default", nil)

	msg := &client.Message{
		SessionID:  "sess-1",
		Type:       client.MessageTypePermissionResponse,
		ToolCallID: "tool-123",
		Content:    "Write", // Tool name
		RawInput: map[string]interface{}{
			"outcome": "allow",
		},
		Timestamp: time.Now(),
	}

	output := FormatPermissionResponse(msg, th)

	// Should contain approval icon
	assert.Contains(t, output, "✅")
	// Should contain outcome
	assert.Contains(t, output, "Allowed")
	// Should contain tool name
	assert.Contains(t, output, "Write")
}

func TestFormatPermissionResponse_Deny(t *testing.T) {
	th := theme.GetTheme("default", nil)

	msg := &client.Message{
		SessionID:  "sess-1",
		Type:       client.MessageTypePermissionResponse,
		ToolCallID: "tool-123",
		Content:    "Write",
		RawInput: map[string]interface{}{
			"outcome": "deny",
		},
		Timestamp: time.Now(),
	}

	output := FormatPermissionResponse(msg, th)

	// Should contain deny icon
	assert.Contains(t, output, "❌")
	// Should contain outcome
	assert.Contains(t, output, "Denied")
	// Should contain tool name
	assert.Contains(t, output, "Write")
}
