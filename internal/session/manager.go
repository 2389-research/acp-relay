// ABOUTME: Session manager for creating and managing agent subprocesses
// ABOUTME: Handles process lifecycle, stdio piping, and cleanup

package session

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/google/uuid"
	"github.com/harper/acp-relay/internal/config"
	"github.com/harper/acp-relay/internal/container"
	"github.com/harper/acp-relay/internal/db"
)

type ManagerConfig struct {
	Mode            string                 // "process" or "container"
	AgentCommand    string
	AgentArgs       []string
	AgentEnv        map[string]string
	ContainerConfig config.ContainerConfig
}

type Manager struct {
	config           ManagerConfig
	sessions         map[string]*Session
	mu               sync.RWMutex
	db               *db.DB
	containerManager *container.Manager // optional container manager
}

func NewManager(cfg ManagerConfig, database *db.DB) *Manager {
	m := &Manager{
		config:   cfg,
		sessions: make(map[string]*Session),
		db:       database,
	}

	// Initialize container manager if mode is "container"
	if cfg.Mode == "container" {
		containerMgr, err := container.NewManager(
			cfg.ContainerConfig,
			cfg.AgentCommand,
			cfg.AgentArgs,
			cfg.AgentEnv,
			database,
		)
		if err != nil {
			log.Fatalf("Failed to initialize container manager: %v", err)
		}
		m.containerManager = containerMgr
		log.Printf("Container manager initialized (image: %s, command: %s)", cfg.ContainerConfig.Image, cfg.AgentCommand)
	}

	return m
}

func (m *Manager) createProcessSession(ctx context.Context, sessionID, workingDir string) (*Session, error) {
	sessionCtx, cancel := context.WithCancel(ctx)

	// Create agent command
	cmd := exec.CommandContext(sessionCtx, m.config.AgentCommand, m.config.AgentArgs...)
	cmd.Dir = workingDir

	// Set up environment - inherit parent env and add custom vars
	cmd.Env = append(os.Environ(), "PWD="+workingDir)
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
		DB:          m.db,
	}

	// Log session creation to database
	if m.db != nil {
		if err := m.db.CreateSession(sessionID, workingDir); err != nil {
			cancel()
			return nil, fmt.Errorf("failed to log session creation: %w", err)
		}
	}

	return sess, nil
}

func (m *Manager) CreateSession(ctx context.Context, workingDir string) (*Session, error) {
	sessionID := "sess_" + uuid.New().String()[:8]

	var sess *Session
	var err error

	// Route based on mode
	if m.config.Mode == "container" {
		log.Printf("[%s] Creating container session (image: %s)", sessionID, m.config.ContainerConfig.Image)

		// Get container components
		components, err := m.containerManager.CreateSession(ctx, sessionID, workingDir)
		if err != nil {
			return nil, err
		}

		// Create context with cancel for container session
		sessionCtx, cancel := context.WithCancel(ctx)

		// Assemble Session from components
		sess = &Session{
			ID:          sessionID,
			WorkingDir:  workingDir,
			ContainerID: components.ContainerID,
			AgentStdin:  components.Stdin,
			AgentStdout: components.Stdout,
			AgentStderr: components.Stderr,
			ToAgent:     make(chan []byte, 10),
			FromAgent:   make(chan []byte, 10),
			Context:     sessionCtx,
			Cancel:      cancel,
			DB:          m.db,
		}

		// Log session creation to database
		if m.db != nil {
			if err := m.db.CreateSession(sessionID, workingDir); err != nil {
				cancel()  // Clean up context
				m.containerManager.StopContainer(sessionID)  // Clean up container
				return nil, fmt.Errorf("failed to log session creation: %w", err)
			}
		}
	} else {
		log.Printf("[%s] Creating process session (command: %s)", sessionID, m.config.AgentCommand)
		sess, err = m.createProcessSession(ctx, sessionID, workingDir)
		if err != nil {
			return nil, err
		}
	}

	// Common initialization (same for both modes)
	m.mu.Lock()
	m.sessions[sessionID] = sess
	m.mu.Unlock()

	// Start stdio bridge (works for both modes)
	go sess.StartStdioBridge()

	// Send ACP initialize
	if err := sess.SendInitialize(); err != nil {
		m.CloseSession(sessionID)
		return nil, fmt.Errorf("failed to initialize agent: %w", err)
	}

	// Send session/new to agent
	// For container mode, use the container workspace path, not the host path
	agentWorkingDir := workingDir
	if m.config.Mode == "container" {
		agentWorkingDir = m.config.ContainerConfig.WorkspaceContainerPath
		if agentWorkingDir == "" {
			m.CloseSession(sessionID)
			return nil, fmt.Errorf("container mode requires workspace_container_path to be set in config")
		}
	}
	if err := sess.SendSessionNew(agentWorkingDir); err != nil {
		m.CloseSession(sessionID)
		return nil, fmt.Errorf("failed to create agent session: %w", err)
	}

	log.Printf("[%s] Session ready (mode: %s)", sessionID, m.config.Mode)
	return sess, nil
}

func (m *Manager) CloseSession(sessionID string) error {
	// Common cleanup: remove session from map (both modes)
	m.mu.Lock()
	sess, exists := m.sessions[sessionID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	// Log session closure to database
	if m.db != nil {
		if err := m.db.CloseSession(sessionID); err != nil {
			// Log error but don't fail the close operation
			fmt.Printf("failed to log session closure: %v\n", err)
		}
	}

	// Mode-specific cleanup
	if m.config.Mode == "container" {
		// Cancel context to signal goroutines to stop
		sess.Cancel()
		// Delegate container cleanup to container manager
		return m.containerManager.StopContainer(sessionID)
	}

	// Process mode cleanup
	// Cancel context (kills process)
	sess.Cancel()

	// Wait for process to exit and all goroutines to finish
	sess.AgentCmd.Wait()

	// Close channels after process has exited to prevent race conditions
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
