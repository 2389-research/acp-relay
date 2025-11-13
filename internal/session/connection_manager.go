// ABOUTME: ConnectionManager manages multiple WebSocket clients attached to a session
// ABOUTME: Handles message broadcasting and per-client buffering with independent flow control

package session

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/harper/acp-relay/internal/db"
)

type ConnectionManager struct {
	mu              sync.RWMutex
	connections     map[string]*ClientConnection
	session         *Session
	broadcasterOnce sync.Once
}

type ClientConnection struct {
	id           string
	conn         *websocket.Conn
	writeMu      sync.Mutex // protects WebSocket writes (gorilla requires mutex)
	buffer       [][]byte
	deliveryChan chan struct{}
	attached     time.Time
}

func NewConnectionManager(sess *Session) *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*ClientConnection),
		session:     sess,
	}
}

func (cm *ConnectionManager) AttachClient(conn *websocket.Conn) string {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	clientID := uuid.New().String()[:8]

	client := &ClientConnection{
		id:           clientID,
		conn:         conn,
		buffer:       make([][]byte, 0, 100),
		deliveryChan: make(chan struct{}, 1),
		attached:     time.Now(),
	}

	cm.connections[clientID] = client

	// Start delivery goroutine
	cm.startClientDelivery(client)

	log.Printf("[%s] Client %s attached (%d total clients)",
		cm.session.ID[:8], clientID, len(cm.connections))

	return clientID
}

// SafeWriteMessage writes a message to a client's WebSocket using the write mutex
// This is used by the WebSocket handler for sending JSON-RPC responses.
func (cm *ConnectionManager) SafeWriteMessage(clientID string, messageType int, data []byte) error {
	cm.mu.RLock()
	client, exists := cm.connections[clientID]
	cm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}

	client.writeMu.Lock()
	defer client.writeMu.Unlock()

	return client.conn.WriteMessage(messageType, data)
}

func (cm *ConnectionManager) DetachClient(clientID string) {
	cm.mu.Lock()
	client, exists := cm.connections[clientID]
	if !exists {
		cm.mu.Unlock()
		return
	}

	delete(cm.connections, clientID)
	bufferedCount := len(client.buffer)
	remainingClients := len(cm.connections)
	cm.mu.Unlock()

	// Close delivery channel to stop delivery goroutine
	close(client.deliveryChan)

	log.Printf("[%s] Client %s detached, dropped %d buffered messages (%d clients remain)",
		cm.session.ID[:8], clientID, bufferedCount, remainingClients)
}

func (cm *ConnectionManager) StartBroadcaster() {
	// Use sync.Once to ensure broadcaster only starts once
	cm.broadcasterOnce.Do(func() {
		log.Printf("[%s] Starting broadcaster goroutine", cm.session.ID[:8])
		go func() {
			for {
				select {
				case msg, ok := <-cm.session.FromAgent:
					if !ok {
						log.Printf("[%s] Broadcaster stopped (FromAgent closed)", cm.session.ID[:8])
						return
					}
					cm.broadcastToClients(msg)
				case <-cm.session.Context.Done():
					log.Printf("[%s] Broadcaster stopped (context done)", cm.session.ID[:8])
					return
				}
			}
		}()
	})
}

func (cm *ConnectionManager) broadcastToClients(msg []byte) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Log message to database as relay->client (once per message, not per client)
	if cm.session.DB != nil {
		if err := cm.session.DB.LogMessage(cm.session.ID, db.DirectionRelayToClient, msg); err != nil {
			log.Printf("[%s] failed to log relay->client message: %v", cm.session.ID[:8], err)
		}
	}

	for clientID, client := range cm.connections {
		// Append to unlimited buffer
		client.buffer = append(client.buffer, msg)

		// Signal delivery goroutine (non-blocking)
		select {
		case client.deliveryChan <- struct{}{}:
			// Successfully signaled
		default:
			// Already has pending signal, skip
		}

		// Warn if buffer growing large
		if len(client.buffer) > 10000 {
			log.Printf("[WARN] Client %s buffer at %d messages (slow client?)",
				clientID, len(client.buffer))
		}
	}
}

// BroadcastError sends an error notification to all connected clients
// This is used when the agent process crashes or exits unexpectedly.
func (cm *ConnectionManager) BroadcastError(errorMessage string) {
	log.Printf("[%s] Broadcasting error to all clients: %s", cm.session.ID[:8], errorMessage)

	// Create JSON-RPC error notification
	errorNotification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/error",
		"params": map[string]interface{}{
			"error": errorMessage,
		},
	}

	errorData, err := json.Marshal(errorNotification)
	if err != nil {
		log.Printf("[%s] Failed to marshal error notification: %v", cm.session.ID[:8], err)
		return
	}

	cm.broadcastToClients(errorData)
}

func (cm *ConnectionManager) startClientDelivery(client *ClientConnection) {
	go func() {
		for range client.deliveryChan {
			cm.deliverPendingMessages(client)
		}
	}()
}

func (cm *ConnectionManager) deliverPendingMessages(client *ClientConnection) {
	cm.mu.Lock()
	pending := client.buffer
	client.buffer = make([][]byte, 0, 100) // Reset to empty slice with capacity
	cm.mu.Unlock()

	for i, msg := range pending {
		if client.conn == nil {
			// Mock connection, skip actual write
			continue
		}

		// gorilla/websocket requires mutex for concurrent writes
		client.writeMu.Lock()
		err := client.conn.WriteMessage(websocket.TextMessage, msg)
		client.writeMu.Unlock()

		if err != nil {
			// Client write failed - drop remaining messages
			dropped := len(pending) - i
			log.Printf("[%s] Client %s write failed, dropping %d messages: %v",
				cm.session.ID[:8], client.id, dropped, err)

			// Auto-detach dead client
			cm.DetachClient(client.id)
			return
		}
	}
}
