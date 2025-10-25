// ABOUTME: ConnectionManager manages multiple WebSocket clients attached to a session
// ABOUTME: Handles message broadcasting and per-client buffering with independent flow control

package session

import (
	"sync"
	"time"

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
