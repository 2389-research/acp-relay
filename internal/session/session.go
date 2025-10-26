// ABOUTME: Session data structure representing a client-agent connection
// ABOUTME: Each session has its own agent subprocess and working directory

package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/harper/acp-relay/internal/db"
)

type Session struct {
	ID             string
	AgentSessionID string // The session ID from the ACP agent
	WorkingDir     string
	ContainerID    string // Docker container ID (empty for process mode)

	// Process mode fields (nil in container mode)
	AgentCmd *exec.Cmd

	// Common fields (both modes)
	AgentStdin  io.WriteCloser
	AgentStdout io.ReadCloser
	AgentStderr io.ReadCloser
	ToAgent     chan []byte
	FromAgent   chan []byte
	Context     context.Context
	Cancel      context.CancelFunc
	DB          *db.DB

	// For HTTP: buffer messages from agent
	MessageBuffer [][]byte
	BufferMutex   sync.Mutex

	// NEW: Connection manager for multi-client support
	connMgr *ConnectionManager
}

// StartStdioBridge starts goroutines to bridge channels and stdio
func (s *Session) StartStdioBridge() {
	// Goroutine: ToAgent channel -> AgentStdin
	go func() {
		msgCount := 0
		for msg := range s.ToAgent {
			msgCount++
			preview := string(msg)
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			log.Printf("[%s] ToAgent #%d -> AgentStdin: %s", s.ID[:8], msgCount, preview)

			// Log message to database
			if s.DB != nil {
				if err := s.DB.LogMessage(s.ID, db.DirectionRelayToAgent, msg); err != nil {
					log.Printf("[%s] failed to log relay->agent message: %v", s.ID[:8], err)
				}
			}

			if _, err := s.AgentStdin.Write(msg); err != nil {
				log.Printf("[%s] error writing to agent stdin: %v", s.ID[:8], err)
				// Notify all connected clients that the agent has failed
				s.BroadcastError(fmt.Sprintf("agent stdin write failed: %v", err))
				return
			}
		}
		log.Printf("[%s] ToAgent channel closed, bridge stopped after %d messages", s.ID[:8], msgCount)
	}()

	// Goroutine: AgentStdout -> FromAgent channel
	go func() {
		scanner := bufio.NewScanner(s.AgentStdout)
		messageCount := 0
		for scanner.Scan() {
			line := scanner.Bytes()
			messageCount++

			// Make a copy since scanner reuses the buffer
			msg := make([]byte, len(line))
			copy(msg, line)

			// Log first 100 chars of message for debugging
			preview := string(msg)
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			log.Printf("[%s] AgentStdout->FromAgent #%d: %s", s.ID[:8], messageCount, preview)

			// Log message to database
			if s.DB != nil {
				if err := s.DB.LogMessage(s.ID, db.DirectionAgentToRelay, msg); err != nil {
					log.Printf("[%s] failed to log agent->relay message: %v", s.ID[:8], err)
				}
			}

			select {
			case s.FromAgent <- msg:
				log.Printf("[%s] Message #%d sent to FromAgent channel (buffer: %d/%d)",
					s.ID[:8], messageCount, len(s.FromAgent), cap(s.FromAgent))
			case <-s.Context.Done():
				log.Printf("[%s] Context done while sending message #%d", s.ID[:8], messageCount)
				return
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("[%s] error reading agent stdout: %v", s.ID[:8], err)
		}
		log.Printf("[%s] AgentStdout scanner finished, total messages: %d", s.ID[:8], messageCount)
	}()

	// Goroutine: AgentStderr -> log
	go func() {
		scanner := bufio.NewScanner(s.AgentStderr)
		for scanner.Scan() {
			select {
			case <-s.Context.Done():
				return
			default:
				log.Printf("agent stderr [%s]: %s", s.ID, scanner.Text())
			}
		}
	}()
}

// Connection manager delegation methods
// These methods allow the WebSocket handler to interact with the ConnectionManager
// without directly accessing the unexported connMgr field

// AttachClient attaches a WebSocket connection to this session and returns a client ID
func (s *Session) AttachClient(conn *websocket.Conn) string {
	return s.connMgr.AttachClient(conn)
}

// DetachClient removes a client from this session
func (s *Session) DetachClient(clientID string) {
	s.connMgr.DetachClient(clientID)
}

// StartBroadcaster starts the broadcaster goroutine if not already running
func (s *Session) StartBroadcaster() {
	s.connMgr.StartBroadcaster()
}

// SafeWriteMessage writes a message to a client's WebSocket using the connection's write mutex
// This ensures thread-safe writes when both the handler and delivery goroutine write to the same connection
func (s *Session) SafeWriteMessage(clientID string, messageType int, data []byte) error {
	return s.connMgr.SafeWriteMessage(clientID, messageType, data)
}

// BroadcastError sends an error notification to all connected clients
// This is typically called when the agent process crashes or exits unexpectedly
func (s *Session) BroadcastError(errorMessage string) {
	s.connMgr.BroadcastError(errorMessage)
}
