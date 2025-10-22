// ABOUTME: Session data structure representing a client-agent connection
// ABOUTME: Each session has its own agent subprocess and working directory

package session

import (
	"context"
	"io"
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
