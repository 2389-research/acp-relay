// ABOUTME: Session data structure representing a client-agent connection
// ABOUTME: Each session has its own agent subprocess and working directory

package session

import (
	"bufio"
	"context"
	"io"
	"log"
	"os/exec"
)

type Session struct {
	ID          string
	WorkingDir  string
	AgentCmd    *exec.Cmd
	AgentStdin  io.WriteCloser
	AgentStdout io.ReadCloser
	AgentStderr io.ReadCloser
	ToAgent     chan []byte
	FromAgent   chan []byte
	Context     context.Context
	Cancel      context.CancelFunc
}

// StartStdioBridge starts goroutines to bridge channels and stdio
func (s *Session) StartStdioBridge() {
	// Goroutine: ToAgent channel -> AgentStdin
	go func() {
		for msg := range s.ToAgent {
			if _, err := s.AgentStdin.Write(msg); err != nil {
				log.Printf("error writing to agent stdin: %v", err)
				return
			}
		}
	}()

	// Goroutine: AgentStdout -> FromAgent channel
	go func() {
		scanner := bufio.NewScanner(s.AgentStdout)
		for scanner.Scan() {
			line := scanner.Bytes()
			// Make a copy since scanner reuses the buffer
			msg := make([]byte, len(line))
			copy(msg, line)

			select {
			case s.FromAgent <- msg:
			case <-s.Context.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("error reading agent stdout: %v", err)
		}
	}()

	// Goroutine: AgentStderr -> log
	go func() {
		scanner := bufio.NewScanner(s.AgentStderr)
		for scanner.Scan() {
			log.Printf("agent stderr [%s]: %s", s.ID, scanner.Text())
		}
	}()
}
