package session

import (
	"testing"

	"github.com/gorilla/websocket"
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

func TestBroadcastToClients(t *testing.T) {
	sess := &Session{
		ID:        "sess_test123",
		FromAgent: make(chan []byte, 10),
	}
	cm := NewConnectionManager(sess)

	// Create client connections directly WITHOUT starting delivery goroutines
	// This avoids the race condition where delivery goroutines clear buffers
	client1 := &ClientConnection{
		id:           "client1",
		conn:         nil,
		buffer:       make([][]byte, 0, 100),
		deliveryChan: make(chan struct{}, 1),
	}

	client2 := &ClientConnection{
		id:           "client2",
		conn:         nil,
		buffer:       make([][]byte, 0, 100),
		deliveryChan: make(chan struct{}, 1),
	}

	// Add clients to connection manager manually
	cm.mu.Lock()
	cm.connections["client1"] = client1
	cm.connections["client2"] = client2
	cm.mu.Unlock()

	// Broadcast a message
	testMsg := []byte("test message")
	cm.broadcastToClients(testMsg)

	// Verify buffers contain the message
	cm.mu.RLock()
	defer cm.mu.RUnlock()

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

func TestGoroutineCleanupOnDetach(t *testing.T) {
	sess := &Session{
		ID:        "sess_test123",
		FromAgent: make(chan []byte, 10),
	}

	cm := NewConnectionManager(sess)

	// Attach a client (this starts a delivery goroutine)
	clientID := cm.AttachClient(nil)

	// Get reference to the client before detaching
	cm.mu.RLock()
	client, exists := cm.connections[clientID]
	cm.mu.RUnlock()

	if !exists {
		t.Fatal("Client not found after attach")
	}

	// Verify deliveryChan is open initially
	select {
	case <-client.deliveryChan:
		t.Error("deliveryChan should not have any pending signals initially")
	default:
		// Good - channel is empty
	}

	// Detach the client
	cm.DetachClient(clientID)

	// Verify deliveryChan is closed (this signals the delivery goroutine to exit)
	_, ok := <-client.deliveryChan
	if ok {
		t.Error("deliveryChan should be closed after detach (to stop delivery goroutine)")
	}

	// Verify client is removed from connections map
	cm.mu.RLock()
	_, exists = cm.connections[clientID]
	cm.mu.RUnlock()

	if exists {
		t.Error("Client should be removed from connections map after detach")
	}
}
