// ABOUTME: Message store for maintaining per-session chat history
// ABOUTME: Implements FIFO queue with configurable history limits
package client

import (
	"sync"
	"time"
)

type MessageType int

const (
	MessageTypeUser MessageType = iota
	MessageTypeAgent
	MessageTypeTool
	MessageTypeSystem
	MessageTypeError
	MessageTypePermissionRequest
	MessageTypePermissionResponse
)

func (mt MessageType) String() string {
	switch mt {
	case MessageTypeUser:
		return "User"
	case MessageTypeAgent:
		return "Agent"
	case MessageTypeTool:
		return "Tool"
	case MessageTypeSystem:
		return "System"
	case MessageTypeError:
		return "Error"
	case MessageTypePermissionRequest:
		return "PermissionRequest"
	case MessageTypePermissionResponse:
		return "PermissionResponse"
	default:
		return "Unknown"
	}
}

func (mt MessageType) Icon() string {
	switch mt {
	case MessageTypeUser:
		return "👤"
	case MessageTypeAgent:
		return "🤖"
	case MessageTypeTool:
		return "🔧"
	case MessageTypeSystem:
		return "ℹ️"
	case MessageTypeError:
		return "⚠️"
	case MessageTypePermissionRequest:
		return "🔐"
	case MessageTypePermissionResponse:
		return "✅"
	default:
		return "❓"
	}
}

type Message struct {
	SessionID  string
	Type       MessageType
	Content    string
	Timestamp  time.Time
	ToolCallID string                 // For permission requests
	RawInput   map[string]interface{} // For permission requests (file_path, content, etc.)
}

type MessageStore struct {
	messages map[string][]*Message
	limit    int
	mu       sync.RWMutex
}

func NewMessageStore(limit int) *MessageStore {
	return &MessageStore{
		messages: make(map[string][]*Message),
		limit:    limit,
	}
}

func (ms *MessageStore) AddMessage(msg *Message) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	sessionMsgs := ms.messages[msg.SessionID]
	sessionMsgs = append(sessionMsgs, msg)

	// Enforce history limit (FIFO)
	if len(sessionMsgs) > ms.limit {
		sessionMsgs = sessionMsgs[len(sessionMsgs)-ms.limit:]
	}

	ms.messages[msg.SessionID] = sessionMsgs
}

func (ms *MessageStore) GetMessages(sessionID string) []*Message {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	messages := ms.messages[sessionID]
	// Return copy to prevent external modification
	result := make([]*Message, len(messages))
	copy(result, messages)
	return result
}

func (ms *MessageStore) Clear(sessionID string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	delete(ms.messages, sessionID)
}

func (ms *MessageStore) ClearAll() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.messages = make(map[string][]*Message)
}
