// ABOUTME: Session manager for creating and managing agent subprocesses
// ABOUTME: Handles process lifecycle, stdio piping, and cleanup

package session

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/google/uuid"
)

type ManagerConfig struct {
	AgentCommand string
	AgentArgs    []string
	AgentEnv     map[string]string
}

type Manager struct {
	config   ManagerConfig
	sessions map[string]*Session
	mu       sync.RWMutex
}

func NewManager(cfg ManagerConfig) *Manager {
	return &Manager{
		config:   cfg,
		sessions: make(map[string]*Session),
	}
}

func (m *Manager) CreateSession(ctx context.Context, workingDir string) (*Session, error) {
	sessionID := "sess_" + uuid.New().String()[:8]

	sessionCtx, cancel := context.WithCancel(ctx)

	// Create agent command
	cmd := exec.CommandContext(sessionCtx, m.config.AgentCommand, m.config.AgentArgs...)
	cmd.Dir = workingDir

	// Set up environment
	cmd.Env = append(cmd.Env, "PWD="+workingDir)
	for k, v := range m.config.AgentEnv {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Create pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start agent: %w", err)
	}

	sess := &Session{
		ID:          sessionID,
		WorkingDir:  workingDir,
		AgentCmd:    cmd,
		AgentStdin:  stdin,
		AgentStdout: stdout,
		AgentStderr: stderr,
		ToAgent:     make(chan []byte, 10),
		FromAgent:   make(chan []byte, 10),
		Context:     sessionCtx,
		Cancel:      cancel,
	}

	m.mu.Lock()
	m.sessions[sessionID] = sess
	m.mu.Unlock()

	return sess, nil
}

func (m *Manager) CloseSession(sessionID string) error {
	m.mu.Lock()
	sess, exists := m.sessions[sessionID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	// Cancel context (kills process)
	sess.Cancel()

	// Wait for process to exit
	sess.AgentCmd.Wait()

	// Close channels
	close(sess.ToAgent)
	close(sess.FromAgent)

	return nil
}

func (m *Manager) GetSession(sessionID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sess, exists := m.sessions[sessionID]
	return sess, exists
}
