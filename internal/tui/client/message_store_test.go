// ABOUTME: Unit tests for message store (per-session message history)
// ABOUTME: Tests message storage, retrieval, and history limits
package client

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewMessageStore(t *testing.T) {
	store := NewMessageStore(100)

	assert.NotNil(t, store)
	assert.Empty(t, store.GetMessages("sess-1"))
}

func TestMessageStore_AddMessage(t *testing.T) {
	store := NewMessageStore(100)

	msg := &Message{
		SessionID: "sess-1",
		Type:      MessageTypeUser,
		Content:   "Hello",
		Timestamp: time.Now(),
	}

	store.AddMessage(msg)

	messages := store.GetMessages("sess-1")
	assert.Len(t, messages, 1)
	assert.Equal(t, "Hello", messages[0].Content)
}

func TestMessageStore_HistoryLimit(t *testing.T) {
	store := NewMessageStore(3) // Limit to 3 messages

	// Add 5 messages
	for i := 0; i < 5; i++ {
		store.AddMessage(&Message{
			SessionID: "sess-1",
			Type:      MessageTypeUser,
			Content:   fmt.Sprintf("Message %d", i),
			Timestamp: time.Now(),
		})
	}

	messages := store.GetMessages("sess-1")
	assert.Len(t, messages, 3, "should only keep 3 most recent")

	// Should have messages 2, 3, 4 (oldest discarded)
	assert.Equal(t, "Message 2", messages[0].Content)
	assert.Equal(t, "Message 4", messages[2].Content)
}

func TestMessageStore_MultiplesSessions(t *testing.T) {
	store := NewMessageStore(100)

	store.AddMessage(&Message{SessionID: "sess-1", Content: "A"})
	store.AddMessage(&Message{SessionID: "sess-2", Content: "B"})
	store.AddMessage(&Message{SessionID: "sess-1", Content: "C"})

	sess1Msgs := store.GetMessages("sess-1")
	sess2Msgs := store.GetMessages("sess-2")

	assert.Len(t, sess1Msgs, 2)
	assert.Len(t, sess2Msgs, 1)
}

func TestMessageStore_Clear(t *testing.T) {
	store := NewMessageStore(100)

	store.AddMessage(&Message{SessionID: "sess-1", Content: "A"})
	store.Clear("sess-1")

	messages := store.GetMessages("sess-1")
	assert.Empty(t, messages)
}

func TestMessageType_PermissionRequest(t *testing.T) {
	msg := &Message{
		SessionID: "sess-1",
		Type:      MessageTypePermissionRequest,
		Content:   `{"toolCallId": "123", "toolName": "Write", "arguments": {"file_path": "/tmp/test.txt"}}`,
		Timestamp: time.Now(),
	}

	assert.Equal(t, MessageTypePermissionRequest, msg.Type)
	assert.Equal(t, "🔐", msg.Type.Icon())
	assert.Equal(t, "PermissionRequest", msg.Type.String())
}

func TestMessageType_PermissionResponse(t *testing.T) {
	msg := &Message{
		SessionID: "sess-1",
		Type:      MessageTypePermissionResponse,
		Content:   `{"outcome": "allow", "toolName": "Write"}`,
		Timestamp: time.Now(),
	}

	assert.Equal(t, MessageTypePermissionResponse, msg.Type)
	assert.Equal(t, "✅", msg.Type.Icon())
	assert.Equal(t, "PermissionResponse", msg.Type.String())
}
