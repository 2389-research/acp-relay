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

	log.Printf("[%s] Client %s attached (%d total clients)",
		cm.session.ID[:8], clientID, len(cm.connections))

	return clientID
}
