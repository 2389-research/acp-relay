// ABOUTME: ConnectionManager manages multiple WebSocket clients attached to a session
// ABOUTME: Handles message broadcasting and per-client buffering with independent flow control

package session

import (
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type ConnectionManager struct {
	mu          sync.RWMutex
	connections map[string]*ClientConnection
	session     *Session
}

type ClientConnection struct {
	id           string
	conn         *websocket.Conn
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
				log.Printf("[%s] Broadcaster stopped (context canceled)", cm.session.ID[:8])
				return
			}
		}
	}()
}

func (cm *ConnectionManager) broadcastToClients(msg []byte) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

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
	client.buffer = nil
	cm.mu.Unlock()

	for i, msg := range pending {
		if client.conn == nil {
			// Mock connection, skip actual write
			continue
		}

		if err := client.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
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
