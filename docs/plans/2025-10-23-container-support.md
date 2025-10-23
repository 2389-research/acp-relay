# Container Support Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Docker container-based agent execution alongside existing process-based execution, with config-based mode selection.

**Architecture:** Separate container package implements adapter pattern. Session manager delegates to either process or container path based on config.agent.mode. Both paths return identical Session interface.

**Tech Stack:** Go 1.23, Docker SDK (github.com/docker/docker), stdcopy for stream demuxing

---

## Task 1: Add Container Configuration

**Files:**
- Modify: `internal/config/config.go`
- Test: Manual verification with config.yaml

**Step 1: Add container config structs**

In `internal/config/config.go`, add after the `AgentConfig` struct:

```go
type ContainerConfig struct {
	Image                  string            `yaml:"image"`
	DockerHost             string            `yaml:"docker_host"`
	NetworkMode            string            `yaml:"network_mode"`
	MemoryLimit            string            `yaml:"memory_limit"`
	CPULimit               float64           `yaml:"cpu_limit"`
	WorkspaceHostBase      string            `yaml:"workspace_host_base"`
	WorkspaceContainerPath string            `yaml:"workspace_container_path"`
	AutoRemove             bool              `yaml:"auto_remove"`
	StartupTimeoutSeconds  int               `yaml:"startup_timeout_seconds"`
}
```

**Step 2: Add Mode field to AgentConfig**

In `internal/config/config.go`, add Mode field to `AgentConfig` struct (after Command field):

```go
type AgentConfig struct {
	Command                 string            `yaml:"command"`
	Mode                    string            `yaml:"mode"` // NEW: "process" or "container"
	Args                    []string          `yaml:"args"`
	Env                     map[string]string `yaml:"env"`
	Container               ContainerConfig   `yaml:"container"` // NEW
	StartupTimeoutSeconds   int               `yaml:"startup_timeout_seconds"`
	MaxConcurrentSessions   int               `yaml:"max_concurrent_sessions"`
}
```

**Step 3: Add default mode logic**

In `internal/config/config.go`, in the `LoadConfig` function after loading, add:

```go
// Default to process mode if not specified
if cfg.Agent.Mode == "" {
	cfg.Agent.Mode = "process"
}

// Validate mode
if cfg.Agent.Mode != "process" && cfg.Agent.Mode != "container" {
	return nil, fmt.Errorf("invalid agent.mode: %s (must be 'process' or 'container')", cfg.Agent.Mode)
}
```

**Step 4: Update config.yaml with container example**

Add to `config.yaml` after the agent section:

```yaml
# Container mode config (set mode: "container" to use)
# container:
#   image: "acp-relay-agent:latest"
#   docker_host: "unix:///var/run/docker.sock"
#   network_mode: "bridge"
#   memory_limit: "512m"
#   cpu_limit: 1.0
#   workspace_host_base: "/tmp/acp-workspaces"
#   workspace_container_path: "/workspace"
#   auto_remove: true
#   startup_timeout_seconds: 10
```

**Step 5: Build and verify config loads**

Run: `go build -o acp-relay ./cmd/relay`
Expected: Clean build with no errors

**Step 6: Commit**

```bash
git add internal/config/config.go config.yaml
git commit -m "feat(config): add container mode configuration

- Add ContainerConfig struct for Docker settings
- Add mode field to AgentConfig (process/container)
- Default to process mode for backward compatibility
- Add example container config to config.yaml"
```

---

## Task 2: Create Container Package Structure

**Files:**
- Create: `internal/container/errors.go`
- Create: `internal/container/stream.go`

**Step 1: Create container package directory**

Run: `mkdir -p internal/container`

**Step 2: Write container errors**

Create `internal/container/errors.go`:

```go
// ABOUTME: Container-specific error types with helpful messages
// ABOUTME: Provides actionable error messages for Docker issues

package container

import "fmt"

type ContainerError struct {
	Type    string
	Message string
	Cause   error
}

func (e *ContainerError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func NewDockerUnavailableError(cause error) *ContainerError {
	return &ContainerError{
		Type:    "docker_unavailable",
		Message: "Cannot connect to Docker daemon. Is Docker running? Check: docker ps",
		Cause:   cause,
	}
}

func NewImageNotFoundError(image string, cause error) *ContainerError {
	return &ContainerError{
		Type:    "image_not_found",
		Message: fmt.Sprintf("Docker image '%s' not found. Build it with:\n  docker build -t %s .", image, image),
		Cause:   cause,
	}
}

func NewAttachFailedError(cause error) *ContainerError {
	return &ContainerError{
		Type:    "attach_failed",
		Message: "Failed to attach to container stdio",
		Cause:   cause,
	}
}
```

**Step 3: Write stream demuxer**

Create `internal/container/stream.go`:

```go
// ABOUTME: Stream demuxing for Docker stdout/stderr separation
// ABOUTME: Fixes bug where Docker multiplexes streams with 8-byte headers

package container

import (
	"io"
	"log"

	"github.com/docker/docker/pkg/stdcopy"
)

// demuxStreams separates Docker's multiplexed stdout/stderr stream
// Returns two readers: one for stdout, one for stderr
func demuxStreams(multiplexed io.Reader) (stdout, stderr io.Reader) {
	stdoutPipe, stdoutWriter := io.Pipe()
	stderrPipe, stderrWriter := io.Pipe()

	// Background goroutine to demux
	go func() {
		defer stdoutWriter.Close()
		defer stderrWriter.Close()

		// stdcopy.StdCopy handles Docker's 8-byte header protocol
		_, err := stdcopy.StdCopy(stdoutWriter, stderrWriter, multiplexed)
		if err != nil && err != io.EOF {
			// Log error but don't crash - container might be stopping
			log.Printf("stream demux error: %v", err)
		}
	}()

	return stdoutPipe, stderrPipe
}
```

**Step 4: Add Docker SDK dependency**

Run: `go get github.com/docker/docker@latest`

**Step 5: Download dependencies**

Run: `go mod tidy`

**Step 6: Build to verify**

Run: `go build -o acp-relay ./cmd/relay`
Expected: Clean build

**Step 7: Commit**

```bash
git add internal/container/ go.mod go.sum
git commit -m "feat(container): add error types and stream demuxing

- Add helpful container error types
- Implement stream demuxing with stdcopy.StdCopy
- Add Docker SDK dependency"
```

---

## Task 3: Implement Container Manager

**Files:**
- Create: `internal/container/manager.go`

**Step 1: Write manager struct and constructor**

Create `internal/container/manager.go`:

```go
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
```

**Step 2: Add CreateSession method**

Add to `internal/container/manager.go`:

```go
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
```

**Step 3: Add helper methods**

Add to `internal/container/manager.go`:

```go
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
				logBytes, _ := io.ReadAll(logs)
				// Demux the logs since they're also multiplexed
				var stdout, stderr []byte
				stdoutBuf := &bytesBuffer{buf: &stdout}
				stderrBuf := &bytesBuffer{buf: &stderr}
				stdcopy.StdCopy(stdoutBuf, stderrBuf, logs)
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
```

**Step 4: Build to verify**

Run: `go build -o acp-relay ./cmd/relay`
Expected: Clean build

**Step 5: Commit**

```bash
git add internal/container/manager.go
git commit -m "feat(container): implement container manager

- Initialize Docker client with connectivity checks
- CreateSession: create container, attach stdio, demux streams
- StopContainer: graceful shutdown with log capture
- monitorContainer: background process to capture exit logs"
```

---

## Task 4: Update Session Struct for Container Support

**Files:**
- Modify: `internal/session/session.go`

**Step 1: Add ContainerID field to Session**

In `internal/session/session.go`, add ContainerID field after WorkingDir:

```go
type Session struct {
	ID          string
	WorkingDir  string
	ContainerID string // NEW: Docker container ID (empty for process mode)

	// Process mode fields (nil in container mode)
	AgentCmd *exec.Cmd

	// Common fields (both modes)
	AgentStdin  io.WriteCloser
	AgentStdout io.Reader
	AgentStderr io.Reader
	ToAgent     chan []byte
	FromAgent   chan []byte
	Context     context.Context
	Cancel      context.CancelFunc
	DB          *db.DB
}
```

**Step 2: Build to verify**

Run: `go build -o acp-relay ./cmd/relay`
Expected: Clean build

**Step 3: Commit**

```bash
git add internal/session/session.go
git commit -m "feat(session): add ContainerID field for container mode

- Add ContainerID field to Session struct
- Document that AgentCmd is nil in container mode
- Both process and container modes use same Session interface"
```

---

## Task 5: Integrate Container Manager into Session Manager

**Files:**
- Modify: `internal/session/manager.go`
- Modify: `cmd/relay/main.go`

**Step 1: Add container manager field to Manager**

In `internal/session/manager.go`, add import:

```go
import (
	// ... existing imports
	"github.com/harper/acp-relay/internal/container"
)
```

Add field to Manager struct:

```go
type Manager struct {
	config           ManagerConfig
	sessions         map[string]*Session
	mu               sync.RWMutex
	db               *db.DB
	containerManager *container.Manager // NEW: optional container manager
}
```

**Step 2: Update ManagerConfig**

In `internal/session/manager.go`, add Mode and ContainerConfig to ManagerConfig:

```go
type ManagerConfig struct {
	Mode            string                 // NEW: "process" or "container"
	AgentCommand    string
	AgentArgs       []string
	AgentEnv        map[string]string
	ContainerConfig config.ContainerConfig // NEW
}
```

**Step 3: Initialize container manager in NewManager**

In `internal/session/manager.go`, update NewManager:

```go
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
			cfg.AgentEnv,
			database,
		)
		if err != nil {
			log.Fatalf("Failed to initialize container manager: %v", err)
		}
		m.containerManager = containerMgr
		log.Printf("Container manager initialized (image: %s)", cfg.ContainerConfig.Image)
	}

	return m
}
```

**Step 4: Extract process creation to helper**

In `internal/session/manager.go`, extract existing CreateSession logic to helper:

```go
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
```

**Step 5: Update CreateSession to route based on mode**

In `internal/session/manager.go`, replace CreateSession body:

```go
func (m *Manager) CreateSession(ctx context.Context, workingDir string) (*Session, error) {
	sessionID := "sess_" + uuid.New().String()[:8]

	var sess *Session
	var err error

	// Route based on mode
	if m.config.Mode == "container" {
		log.Printf("[%s] Creating container session (image: %s)", sessionID, m.config.ContainerConfig.Image)
		sess, err = m.containerManager.CreateSession(ctx, sessionID, workingDir)
	} else {
		log.Printf("[%s] Creating process session (command: %s)", sessionID, m.config.AgentCommand)
		sess, err = m.createProcessSession(ctx, sessionID, workingDir)
	}

	if err != nil {
		return nil, err
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
	if err := sess.SendSessionNew(workingDir); err != nil {
		m.CloseSession(sessionID)
		return nil, fmt.Errorf("failed to create agent session: %w", err)
	}

	log.Printf("[%s] Session ready (mode: %s)", sessionID, m.config.Mode)
	return sess, nil
}
```

**Step 6: Update CloseSession for container mode**

In `internal/session/manager.go`, update CloseSession:

```go
func (m *Manager) CloseSession(sessionID string) error {
	// If container mode, delegate to container manager
	if m.config.Mode == "container" {
		return m.containerManager.StopContainer(sessionID)
	}

	// Process mode (existing code)
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
			fmt.Printf("failed to log session closure: %v\n", err)
		}
	}

	// Cancel context (kills process)
	sess.Cancel()

	// Wait for process to exit
	sess.AgentCmd.Wait()

	// Close channels
	close(sess.ToAgent)
	close(sess.FromAgent)

	return nil
}
```

**Step 7: Update main.go to pass config to session manager**

In `cmd/relay/main.go`, update session manager initialization:

```go
// Create session manager
sessionMgr := session.NewManager(session.ManagerConfig{
	Mode:            cfg.Agent.Mode, // NEW
	AgentCommand:    cfg.Agent.Command,
	AgentArgs:       cfg.Agent.Args,
	AgentEnv:        cfg.Agent.Env,
	ContainerConfig: cfg.Agent.Container, // NEW
}, database)

log.Printf("Session manager initialized (mode: %s)", cfg.Agent.Mode)
```

**Step 8: Build to verify**

Run: `go build -o acp-relay ./cmd/relay`
Expected: Clean build

**Step 9: Commit**

```bash
git add internal/session/manager.go cmd/relay/main.go
git commit -m "feat(session): integrate container manager

- Add container manager field to Manager
- Route CreateSession based on mode (process/container)
- Extract process logic to createProcessSession helper
- Update CloseSession to handle container mode
- Update main.go to pass container config"
```

---

## Task 6: Create Dockerfile for Agent

**Files:**
- Create: `Dockerfile`
- Create: `.dockerignore`

**Step 1: Write Dockerfile**

Create `Dockerfile` in repo root:

```dockerfile
FROM node:20-slim

# Install dependencies the agent might need
RUN apt-get update && apt-get install -y \
    git \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Set up working directory
WORKDIR /workspace

# Install the ACP agent globally
RUN npm install -g @zed-industries/claude-code-acp

# The agent expects stdio communication
ENTRYPOINT ["npx", "@zed-industries/claude-code-acp"]
```

**Step 2: Write .dockerignore**

Create `.dockerignore` in repo root:

```
# Binaries
acp-relay
relay
main

# Databases
*.db
*.db-shm
*.db-wal
relay-messages.*

# Git
.git
.github
.gitignore

# Docs
docs/
README.md

# Tests
tests/
testdata/

# Build artifacts
.worktrees/
```

**Step 3: Build Docker image**

Run: `docker build -t acp-relay-agent:latest .`
Expected: Successful build with final message "Successfully tagged acp-relay-agent:latest"

**Step 4: Verify image exists**

Run: `docker images | grep acp-relay-agent`
Expected: Shows acp-relay-agent:latest with size ~200MB

**Step 5: Commit**

```bash
git add Dockerfile .dockerignore
git commit -m "feat(docker): add Dockerfile for agent container

- Use node:20-slim as base
- Install git and curl for agent dependencies
- Install @zed-industries/claude-code-acp globally
- Set ENTRYPOINT for stdio communication"
```

---

## Task 7: Manual Testing

**Files:**
- Modify: `config.yaml` (temporary test changes)

**Step 1: Create test config for container mode**

Copy config.yaml to config-container-test.yaml:

```yaml
server:
  http_port: 8080
  http_host: "0.0.0.0"
  websocket_port: 8081
  websocket_host: "0.0.0.0"
  management_port: 8082
  management_host: "127.0.0.1"

agent:
  mode: "container"
  env:
    ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
    HOME: "${HOME}"
    PATH: "${PATH}"
  startup_timeout_seconds: 10
  max_concurrent_sessions: 100

  container:
    image: "acp-relay-agent:latest"
    docker_host: "unix:///var/run/docker.sock"
    network_mode: "bridge"
    memory_limit: "512m"
    cpu_limit: 1.0
    workspace_host_base: "/tmp/acp-workspaces"
    workspace_container_path: "/workspace"
    auto_remove: true
    startup_timeout_seconds: 10

database:
  path: "./relay-messages.db"
```

**Step 2: Test container mode initialization**

Run: `./acp-relay --config config-container-test.yaml`

Expected output:
```
Database initialized at ./relay-messages.db
Startup maintenance: marked 0 crashed/orphaned sessions as closed
Container manager initialized (image: acp-relay-agent:latest)
Session manager initialized (mode: container)
Starting management API on 127.0.0.1:8082
Starting HTTP server on 0.0.0.0:8080
Starting WebSocket server on 0.0.0.0:8081
```

**Step 3: Test session creation via curl (in new terminal)**

Run:
```bash
curl -X POST http://localhost:8080/session/new \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "session/new",
    "params": {
      "workingDirectory": "/tmp/test-workspace"
    },
    "id": 1
  }'
```

Expected: JSON response with `sessionId` field

**Step 4: Verify container was created**

Run: `docker ps`
Expected: Shows running container with name matching sess_* pattern

**Step 5: Verify workspace directory**

Run: `ls /tmp/acp-workspaces/`
Expected: Shows directory named sess_* (same as container name)

**Step 6: Test process mode still works**

Stop container relay (Ctrl+C)

Run: `./acp-relay --config config.yaml` (original config with process mode)
Expected: Starts successfully with "Session manager initialized (mode: process)"

**Step 7: Clean up**

Run: `docker ps -a | grep sess_ | awk '{print $1}' | xargs docker rm -f` (if any orphaned)
Run: `rm -rf /tmp/acp-workspaces/`
Run: `rm config-container-test.yaml`

---

## Task 8: Update Documentation

**Files:**
- Modify: `README.md`
- Create: `docs/container-mode.md`

**Step 1: Add container mode section to README**

In `README.md`, after the "Configuration Options" section, add:

```markdown
## Container Mode

The ACP Relay Server supports running agents in Docker containers for isolation and reproducibility.

### Prerequisites

- Docker installed and running
- Docker image built: `docker build -t acp-relay-agent:latest .`

### Configuration

Set `agent.mode: "container"` in config.yaml:

```yaml
agent:
  mode: "container"
  env:
    ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"

  container:
    image: "acp-relay-agent:latest"
    docker_host: "unix:///var/run/docker.sock"
    network_mode: "bridge"
    memory_limit: "512m"
    cpu_limit: 1.0
    workspace_host_base: "/tmp/acp-workspaces"
    workspace_container_path: "/workspace"
    auto_remove: true
    startup_timeout_seconds: 10
```

### Building the Image

```bash
docker build -t acp-relay-agent:latest .
```

### Running

```bash
./acp-relay --config config.yaml
```

Sessions will be created in Docker containers with isolated workspaces.

### Troubleshooting

**"Cannot connect to Docker daemon"**
- Verify Docker is running: `docker ps`
- Check Docker socket path in config

**"Docker image not found"**
- Build the image: `docker build -t acp-relay-agent:latest .`
- Verify it exists: `docker images | grep acp-relay-agent`

**"Container exits immediately"**
- Check container logs: `docker logs <container-id>`
- Verify environment variables are set

See [docs/container-mode.md](docs/container-mode.md) for detailed documentation.
```

**Step 2: Create container mode guide**

Create `docs/container-mode.md`:

```markdown
# Container Mode Documentation

## Overview

Container mode runs each agent session in an isolated Docker container. This provides:

- **Isolation**: Each session has its own filesystem and resources
- **Reproducibility**: Same container image across environments
- **Resource Limits**: CPU and memory limits per session
- **Security**: Process isolation and network controls

## Architecture

```
Client Request
    ↓
Session Manager (checks agent.mode)
    ↓
Container Manager
    ↓
1. Create workspace directory on host
2. Create Docker container with mounts
3. Start container
4. Attach to stdin/stdout/stderr
5. Demux stdout/stderr (fixes Docker multiplexing)
6. Return Session (identical interface to process mode)
```

## Configuration

### Container Settings

```yaml
agent:
  mode: "container"

  container:
    # Docker image to use
    image: "acp-relay-agent:latest"

    # Docker daemon socket
    docker_host: "unix:///var/run/docker.sock"

    # Network mode: "bridge", "host", or "none"
    network_mode: "bridge"

    # Memory limit (examples: "256m", "1g", "512m")
    memory_limit: "512m"

    # CPU limit (cores: 0.5, 1.0, 2.0, etc.)
    cpu_limit: 1.0

    # Host directory prefix for workspaces
    workspace_host_base: "/tmp/acp-workspaces"

    # Mount point inside container
    workspace_container_path: "/workspace"

    # Remove container automatically on session close
    auto_remove: true

    # Timeout for container startup
    startup_timeout_seconds: 10
```

## Building Custom Images

### Basic Dockerfile

```dockerfile
FROM node:20-slim

RUN apt-get update && apt-get install -y git curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

RUN npm install -g @zed-industries/claude-code-acp

ENTRYPOINT ["npx", "@zed-industries/claude-code-acp"]
```

### Building

```bash
docker build -t acp-relay-agent:latest .
```

### Custom Agent

To use a different agent binary:

```dockerfile
FROM python:3.11-slim

WORKDIR /workspace

COPY my-agent /usr/local/bin/my-agent
RUN chmod +x /usr/local/bin/my-agent

ENTRYPOINT ["/usr/local/bin/my-agent"]
```

## Workspace Management

### Host Mounts

Containers mount host directories for workspace persistence:

```
Host: /tmp/acp-workspaces/sess_abc123/
        ↕ (bind mount)
Container: /workspace/
```

Files created by the agent appear on the host immediately and persist after container stops.

### Cleanup

Workspaces are NOT automatically deleted. Clean them manually:

```bash
rm -rf /tmp/acp-workspaces/sess_*
```

Or configure a cron job for automatic cleanup.

## Resource Limits

### Memory

Containers are limited by `memory_limit`:

```yaml
memory_limit: "512m"  # 512 MB
```

If exceeded, container is killed with OOM (exit code 137).

### CPU

Containers are limited by `cpu_limit`:

```yaml
cpu_limit: 1.0  # 1 CPU core
```

Values: 0.5 (half core), 1.0 (one core), 2.0 (two cores), etc.

## Network Isolation

### Bridge Mode (default)

```yaml
network_mode: "bridge"
```

Container can access internet but not host network.

### None Mode (isolated)

```yaml
network_mode: "none"
```

No network access. Agent cannot call APIs.

### Host Mode

```yaml
network_mode: "host"
```

Shares host network. Use with caution.

## Debugging

### View Container Logs

```bash
docker logs <container-id>
```

Container ID shown in relay logs as `[sess_abc123]`.

### List Running Containers

```bash
docker ps | grep sess_
```

### Exec Into Container

```bash
docker exec -it <container-id> /bin/sh
```

### Check Resource Usage

```bash
docker stats <container-id>
```

## Troubleshooting

### Container Exits Immediately

**Check logs:**
```bash
docker logs <container-id>
```

**Common causes:**
- Missing environment variables (ANTHROPIC_API_KEY)
- Agent binary not found in PATH
- Entrypoint command incorrect

### "Cannot Connect to Docker Daemon"

**Verify Docker running:**
```bash
docker ps
```

**Check socket path:**
- Mac/Linux: `unix:///var/run/docker.sock`
- Windows: `npipe:////./pipe/docker_engine`

### "Image Not Found"

**Build image:**
```bash
docker build -t acp-relay-agent:latest .
```

**Verify:**
```bash
docker images | grep acp-relay-agent
```

### High Memory Usage

**Check container stats:**
```bash
docker stats --no-stream
```

**Lower memory limit in config:**
```yaml
memory_limit: "256m"
```

### Orphaned Containers

**List all containers:**
```bash
docker ps -a | grep sess_
```

**Remove all:**
```bash
docker ps -a | grep sess_ | awk '{print $1}' | xargs docker rm -f
```

## Migration from Process Mode

1. Build Docker image
2. Update config.yaml (change mode to "container")
3. Restart relay
4. Test session creation
5. If issues, switch back to process mode

## Future Enhancements

- Container pooling for faster startup
- Custom Dockerfile generation per agent type
- Resource quotas per user/org
- Network egress policies
- Secrets management
```

**Step 3: Commit**

```bash
git add README.md docs/container-mode.md
git commit -m "docs: add container mode documentation

- Add container mode section to README
- Create comprehensive container-mode.md guide
- Document configuration, troubleshooting, debugging
- Add migration guide from process mode"
```

---

## Task 9: Final Testing and Verification

**Step 1: Run all unit tests**

Run: `go test ./internal/... -v`
Expected: All tests pass

**Step 2: Build release binary**

Run: `make build`
Expected: Binary created at ./acp-relay

**Step 3: Test process mode**

Run: `./acp-relay --config config.yaml`
Expected: Starts successfully with "mode: process"

**Step 4: Test container mode**

Edit config.yaml temporarily to set `mode: "container"` and add container config.

Run: `./acp-relay --config config.yaml`
Expected: Starts successfully with "mode: container"

**Step 5: Create test session in container mode**

Run curl command from Task 7 Step 3.
Expected: Session created, container running, workspace directory exists.

**Step 6: Verify container logs**

Run: `docker logs $(docker ps | grep sess_ | awk '{print $1}')`
Expected: Shows agent initialization logs

**Step 7: Clean up**

Stop relay (Ctrl+C)
Run: `docker ps -a | grep sess_ | awk '{print $1}' | xargs docker rm -f`
Run: `rm -rf /tmp/acp-workspaces/`

**Step 8: Restore config.yaml**

Revert `mode: "process"` in config.yaml

**Step 9: Final commit**

```bash
git add -A
git commit -m "test: verify container and process modes working

Manual testing complete:
- Process mode: sessions create and run
- Container mode: containers create, attach, cleanup
- Workspace directories persist on host
- Both modes coexist via config flag"
```

---

## Success Criteria

- [ ] Config loads with both process and container modes
- [ ] Process mode works unchanged (backward compatible)
- [ ] Container mode creates and attaches to containers
- [ ] Stdout/stderr properly separated (stream demuxing)
- [ ] Workspace files persist on host after container stops
- [ ] Containers clean up on session close (AutoRemove)
- [ ] Docker daemon errors fail fast with helpful messages
- [ ] Documentation updated with container mode guide
- [ ] All unit tests pass

---

## Related Documentation

- Design: @docs/plans/2025-10-23-container-support-design.md
- Container spec: @docs/container-spec.md
- Security gaps: @docs/container-spec-gaps.md

## Future Work (Not in This Plan)

- Container pooling for warm starts
- Dynamic Dockerfile generation
- Metrics and monitoring
- Security hardening (rootless Docker, secrets management)
- Kubernetes/containerd support
