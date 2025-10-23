// ABOUTME: Container manager for creating and managing Docker-based agent sessions
// ABOUTME: Handles Docker client, container lifecycle, and stdio attachment

package container

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/harper/acp-relay/internal/config"
	"github.com/harper/acp-relay/internal/db"
	"github.com/harper/acp-relay/internal/session"
)

type Manager struct {
	config       config.ContainerConfig
	agentEnv     map[string]string
	dockerClient *client.Client
	sessions     map[string]*session.Session
	mu           sync.RWMutex
	db           *db.DB
}

func NewManager(cfg config.ContainerConfig, agentEnv map[string]string, database *db.DB) (*Manager, error) {
	// Initialize Docker client
	dockerClient, err := client.NewClientWithOpts(
		client.WithHost(cfg.DockerHost),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Verify Docker daemon is reachable
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = dockerClient.Ping(ctx)
	if err != nil {
		return nil, NewDockerUnavailableError(err)
	}

	// Verify image exists
	_, _, err = dockerClient.ImageInspectWithRaw(ctx, cfg.Image)
	if err != nil {
		return nil, NewImageNotFoundError(cfg.Image, err)
	}

	return &Manager{
		config:       cfg,
		agentEnv:     agentEnv,
		dockerClient: dockerClient,
		sessions:     make(map[string]*session.Session),
		db:           database,
	}, nil
}

func (m *Manager) CreateSession(ctx context.Context, sessionID, workingDir string) (*session.Session, error) {
	// 1. Create host workspace directory
	hostWorkspace := filepath.Join(m.config.WorkspaceHostBase, sessionID)
	if err := os.MkdirAll(hostWorkspace, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// 2. Format environment variables
	envVars := []string{}
	for k, v := range m.agentEnv {
		// Expand environment variable references
		expandedVal := os.ExpandEnv(v)
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, expandedVal))
	}

	// 3. Create container config
	containerConfig := &container.Config{
		Image:     m.config.Image,
		Env:       envVars,
		Tty:       false, // CRITICAL: must be false for stream demuxing
		OpenStdin: true,
		StdinOnce: false,
	}

	// 4. Parse memory limit
	memoryLimit, err := parseMemoryLimit(m.config.MemoryLimit)
	if err != nil {
		return nil, fmt.Errorf("invalid memory limit: %w", err)
	}

	// 5. Create host config with mounts and limits
	hostConfig := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:%s", hostWorkspace, m.config.WorkspaceContainerPath),
		},
		AutoRemove:  m.config.AutoRemove,
		NetworkMode: container.NetworkMode(m.config.NetworkMode),
		Resources: container.Resources{
			Memory:   memoryLimit,
			NanoCPUs: int64(m.config.CPULimit * 1e9),
		},
	}

	// 6. Create container
	resp, err := m.dockerClient.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		nil,
		nil,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// 7. Start container
	if err := m.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// 8. Attach to stdio
	attachResp, err := m.dockerClient.ContainerAttach(ctx, resp.ID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		m.dockerClient.ContainerStop(ctx, resp.ID, container.StopOptions{})
		return nil, NewAttachFailedError(err)
	}

	// 9. Demux stdout/stderr
	stdoutReader, stderrReader := demuxStreams(attachResp.Reader)

	// 10. Start background monitor
	go m.monitorContainer(ctx, resp.ID, sessionID)

	// 11. Create session
	sess := &session.Session{
		ID:          sessionID,
		WorkingDir:  workingDir,
		ContainerID: resp.ID,
		AgentStdin:  attachResp.Conn,
		AgentStdout: stdoutReader,
		AgentStderr: stderrReader,
		ToAgent:     make(chan []byte, 10),
		FromAgent:   make(chan []byte, 10),
		DB:          m.db,
	}

	// Store session
	m.mu.Lock()
	m.sessions[sessionID] = sess
	m.mu.Unlock()

	return sess, nil
}

func parseMemoryLimit(limit string) (int64, error) {
	if limit == "" {
		return 0, nil
	}

	// Simple parser for memory limits like "512m", "1g"
	var value float64
	var unit string
	_, err := fmt.Sscanf(limit, "%f%s", &value, &unit)
	if err != nil {
		return 0, err
	}

	switch unit {
	case "k", "K":
		return int64(value * 1024), nil
	case "m", "M":
		return int64(value * 1024 * 1024), nil
	case "g", "G":
		return int64(value * 1024 * 1024 * 1024), nil
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}
}

func (m *Manager) monitorContainer(ctx context.Context, containerID, sessionID string) {
	statusCh, errCh := m.dockerClient.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	select {
	case err := <-errCh:
		log.Printf("[%s] Container wait error: %v", sessionID, err)
	case status := <-statusCh:
		if status.StatusCode != 0 {
			// Container exited with error - grab logs
			logs, err := m.dockerClient.ContainerLogs(ctx, containerID, container.LogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Tail:       "50",
			})
			if err == nil {
				defer logs.Close()
				// Demux the logs since they're also multiplexed
				var stdout, stderr []byte
				stdoutBuf := &bytesBuffer{buf: &stdout}
				stderrBuf := &bytesBuffer{buf: &stderr}
				_, _ = stdcopy.StdCopy(stdoutBuf, stderrBuf, logs)
				log.Printf("[%s] Container exited with code %d. Last 50 lines:\nSTDOUT:\n%s\nSTDERR:\n%s",
					sessionID, status.StatusCode, string(stdout), string(stderr))
			}
		}
	}
}

// Helper for capturing logs
type bytesBuffer struct {
	buf *[]byte
}

func (b *bytesBuffer) Write(p []byte) (n int, err error) {
	*b.buf = append(*b.buf, p...)
	return len(p), nil
}

func (m *Manager) StopContainer(sessionID string) error {
	m.mu.Lock()
	sess, exists := m.sessions[sessionID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	// Stop container
	timeout := 10
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout+5)*time.Second)
	defer cancel()

	if err := m.dockerClient.ContainerStop(ctx, sess.ContainerID, container.StopOptions{
		Timeout: &timeout,
	}); err != nil {
		log.Printf("[%s] Failed to stop container: %v", sessionID, err)
	}

	// Close database session
	if m.db != nil {
		if err := m.db.CloseSession(sessionID); err != nil {
			log.Printf("[%s] Failed to close DB session: %v", sessionID, err)
		}
	}

	return nil
}
