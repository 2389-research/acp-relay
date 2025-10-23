// ABOUTME: Session data structure representing a client-agent connection
// ABOUTME: Each session has its own agent subprocess and working directory

package session

import (
	"bufio"
	"context"
	"io"
	"log"
	"os/exec"
	"sync"

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
