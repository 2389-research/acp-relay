// ABOUTME: Container manager for creating and managing Docker-based agent sessions
// ABOUTME: Handles Docker client, container lifecycle, and stdio attachment

package container

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/harper/acp-relay/internal/config"
	"github.com/harper/acp-relay/internal/db"
)

// ContainerInfo tracks a running container for a session
type ContainerInfo struct {
	ContainerID string
	SessionID   string
}

// SessionComponents contains the IO streams and metadata needed to create a session
type SessionComponents struct {
	ContainerID string
	Stdin       io.WriteCloser
	Stdout      io.ReadCloser
	Stderr      io.ReadCloser
}

type Manager struct {
	config       config.ContainerConfig
	agentCommand string
	agentArgs    []string
	agentEnv     map[string]string
	dockerClient *client.Client
	containers   map[string]*ContainerInfo // sessionID -> container info
	mu           sync.RWMutex
	db           *db.DB
}

func NewManager(cfg config.ContainerConfig, agentCommand string, agentArgs []string, agentEnv map[string]string, database *db.DB) (*Manager, error) {
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
		agentCommand: agentCommand,
		agentArgs:    agentArgs,
		agentEnv:     agentEnv,
		dockerClient: dockerClient,
		containers:   make(map[string]*ContainerInfo),
		db:           database,
	}, nil
}

// filterAllowedEnvVars returns only safe host environment variables
func (m *Manager) filterAllowedEnvVars(env map[string]string) map[string]string {
	// Allowlist: only safe terminal and locale vars
	allowlist := []string{"TERM", "COLORTERM"}

	result := make(map[string]string)
	for k, v := range env {
		// Check exact match
		allowed := false
		for _, prefix := range allowlist {
			if k == prefix {
				allowed = true
				break
			}
		}

		// Check LC_* prefix
		if strings.HasPrefix(k, "LC_") || k == "LANG" {
			allowed = true
		}

		if allowed {
			result[k] = v
		}
	}

	return result
}

// buildContainerLabels creates Docker labels for container tracking
func (m *Manager) buildContainerLabels(sessionID string) map[string]string {
	return map[string]string{
		"managed-by": "acp-relay",
		"session-id": sessionID,
		"created-at": time.Now().UTC().Format(time.RFC3339),
	}
}

// sanitizeContainerName produces valid Docker container name
func (m *Manager) sanitizeContainerName(sessionID string) string {
	// Docker names: [a-zA-Z0-9][a-zA-Z0-9_.-]*
	name := strings.ToLower(sessionID)
	name = strings.ReplaceAll(name, ".", "-")
	return "acp-relay-" + name
}

// findExistingContainer checks for existing container by labels
func (m *Manager) findExistingContainer(ctx context.Context, sessionID string) (string, error) {
	// Query containers with our labels
	filters := filters.NewArgs()
	filters.Add("label", "managed-by=acp-relay")
	filters.Add("label", fmt.Sprintf("session-id=%s", sessionID))

	containers, err := m.dockerClient.ContainerList(ctx, container.ListOptions{
		All:     true, // Include stopped containers
		Filters: filters,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		return "", nil // No existing container
	}

	// Return first match
	return containers[0].ID, nil
}

func (m *Manager) CreateSession(ctx context.Context, sessionID, workingDir string) (*SessionComponents, error) {
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

		// Warn if expansion resulted in empty value for critical env vars
		if expandedVal == "" && (k == "ANTHROPIC_API_KEY" || k == "OPENAI_API_KEY") {
			log.Printf("[%s] WARNING: %s is empty after expansion (template: %s)", sessionID, k, v)
			log.Printf("[%s] Make sure %s is set in your environment before starting the relay", sessionID, k)
		} else {
			log.Printf("[%s] Setting env: %s=%s (from template: %s)", sessionID, k, expandedVal, v)
		}
	}

	// 3. Create container config with runtime command
	cmd := append([]string{m.agentCommand}, m.agentArgs...)
	log.Printf("[%s] Container command: %v", sessionID, cmd)

	containerConfig := &container.Config{
		Image:     m.config.Image,
		Cmd:       cmd, // Runtime command override (not baked into image)
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
	binds := []string{
		fmt.Sprintf("%s:%s", hostWorkspace, m.config.WorkspaceContainerPath),
	}

	// Mount user's ~/.claude directory as read-only for agent configuration
	claudeDir := filepath.Join(os.Getenv("HOME"), ".claude")
	if _, err := os.Stat(claudeDir); err == nil {
		binds = append(binds, fmt.Sprintf("%s:/home/.claude:ro", claudeDir))
		log.Printf("[%s] Mounting ~/.claude directory as read-only", sessionID)
	} else {
		log.Printf("[%s] ~/.claude directory not found, skipping mount", sessionID)
	}

	hostConfig := &container.HostConfig{
		Binds:       binds,
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

	// 11. Store container info
	m.mu.Lock()
	m.containers[sessionID] = &ContainerInfo{
		ContainerID: resp.ID,
		SessionID:   sessionID,
	}
	m.mu.Unlock()

	// 12. Return session components for session manager to assemble
	return &SessionComponents{
		ContainerID: resp.ID,
		Stdin:       attachResp.Conn,
		Stdout:      stdoutReader,
		Stderr:      stderrReader,
	}, nil
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
	containerInfo, exists := m.containers[sessionID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("container not found for session: %s", sessionID)
	}
	delete(m.containers, sessionID)
	m.mu.Unlock()

	// Stop container
	timeout := 10
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout+5)*time.Second)
	defer cancel()

	if err := m.dockerClient.ContainerStop(ctx, containerInfo.ContainerID, container.StopOptions{
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
