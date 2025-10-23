# ACP Relay Server Container Integration Specification

## Table of Contents
1. [Overview & Architecture](#overview--architecture)
2. [Prerequisites & Setup](#prerequisites--setup)
3. [Container Design](#container-design)
4. [Code Implementation](#code-implementation)
5. [Configuration Management](#configuration-management)
6. [Error Handling](#error-handling)
7. [Testing Strategy](#testing-strategy)
8. [Deployment & Operations](#deployment--operations)
9. [Troubleshooting Guide](#troubleshooting-guide)

---

## 1. Overview & Architecture

### Current Architecture (Process-based)
```
Client → HTTP/WS → Relay → spawn process → Agent (same host)
                            ↓
                         stdio pipes
```

### New Architecture (Container-based)
```
Client → HTTP/WS → Relay → Docker API → Container → Agent (isolated)
                            ↓
                         attach API (stdio)
```

### Key Changes
1. **Process Spawn** → **Container Creation**
2. **OS Process** → **Docker Container** 
3. **Filesystem Access** → **Volume Mounts**
4. **Process Kill** → **Container Stop/Remove**

---

## 2. Prerequisites & Setup

### Required Software

```bash
# Check Docker is installed
docker --version  # Should be 24.0+ 

# Check Go version
go version  # Should be 1.23+

# Ensure Docker daemon is running
docker ps  # Should not error

# Current user needs Docker access
docker run hello-world  # Should succeed
```

### Add User to Docker Group (Linux)
```bash
sudo usermod -aG docker $USER
newgrp docker
```

### Install Docker Go SDK
```bash
cd acp-relay
go get github.com/docker/docker/client@v24.0.7
go get github.com/docker/docker/api/types@v24.0.7
go get github.com/docker/docker/api/types/container@v24.0.7
go get github.com/docker/docker/api/types/mount@v24.0.7
go get github.com/docker/go-connections/nat@v0.5.0
go mod tidy
```

---

## 3. Container Design

### 3.1 Agent Container Image

Create `docker/agent/Dockerfile`:

```dockerfile
# Base image with Node.js
FROM node:20-slim

# Install system dependencies
RUN apt-get update && apt-get install -y \
    # Basic utilities
    curl \
    wget \
    git \
    # Build tools (some agents might compile code)
    build-essential \
    python3 \
    python3-pip \
    # Editor for file operations
    nano \
    vim \
    # Process monitoring
    procps \
    && rm -rf /var/lib/apt/lists/*

# Create a non-root user for running the agent
RUN groupadd -g 1000 agent && \
    useradd -m -u 1000 -g agent agent

# Install the ACP agent globally
RUN npm install -g @zed-industries/claude-code-acp@latest

# Create workspace directory with correct permissions
RUN mkdir -p /workspace && \
    chown -R agent:agent /workspace

# Create directory for agent logs
RUN mkdir -p /var/log/agent && \
    chown -R agent:agent /var/log/agent

# Switch to non-root user
USER agent
WORKDIR /workspace

# Set environment variables
ENV NODE_ENV=production \
    AGENT_LOG_LEVEL=info \
    HOME=/home/agent

# Health check (optional but recommended)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
    CMD pgrep node || exit 1

# The agent will be started with command from relay
ENTRYPOINT ["npx", "@zed-industries/claude-code-acp"]
```

### 3.2 Build Script

Create `docker/build.sh`:

```bash
#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Building ACP Agent Container Image...${NC}"

# Get script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# Image name and tag
IMAGE_NAME="acp-agent"
IMAGE_TAG="${1:-latest}"
FULL_IMAGE="$IMAGE_NAME:$IMAGE_TAG"

echo -e "${YELLOW}Building image: $FULL_IMAGE${NC}"

# Build the image
docker build \
    --file agent/Dockerfile \
    --tag "$FULL_IMAGE" \
    --progress=plain \
    .

# Verify the image was built
if docker image inspect "$FULL_IMAGE" > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Image built successfully: $FULL_IMAGE${NC}"
    echo -e "${GREEN}Size: $(docker image ls $FULL_IMAGE --format 'table {{.Size}}' | tail -n 1)${NC}"
else
    echo -e "${RED}✗ Failed to build image${NC}"
    exit 1
fi

# Optional: Test the container starts
echo -e "${YELLOW}Testing container startup...${NC}"
if docker run --rm "$FULL_IMAGE" --version 2>/dev/null; then
    echo -e "${GREEN}✓ Container starts successfully${NC}"
else
    echo -e "${YELLOW}⚠ Could not verify container startup (this might be normal)${NC}"
fi
```

Make it executable:
```bash
chmod +x docker/build.sh
```

---

## 4. Code Implementation

### 4.1 New Container Package

Create `internal/container/types.go`:

```go
// ABOUTME: Docker container types and configuration for agent isolation
// ABOUTME: Defines how containers are configured and managed

package container

import (
    "time"
)

// Config holds Docker container configuration
type Config struct {
    // Image name for the agent container
    Image string `mapstructure:"image"`
    
    // Resource limits
    MemoryLimit string `mapstructure:"memory_limit"` // e.g., "2G", "512M"
    CPULimit    float64 `mapstructure:"cpu_limit"`    // e.g., 0.5 = 50% of one CPU
    
    // Networking
    NetworkMode string `mapstructure:"network_mode"` // "none", "bridge", "host"
    
    // Security
    ReadOnlyRootFS bool     `mapstructure:"readonly_rootfs"`
    DropCaps       []string `mapstructure:"drop_capabilities"`
    SecurityOpt    []string `mapstructure:"security_opt"`
    
    // Volumes
    WorkspaceBaseDir string   `mapstructure:"workspace_base_dir"` // Host directory for workspaces
    ExtraVolumes     []string `mapstructure:"extra_volumes"`      // Additional volume mounts
    
    // Behavior
    AutoRemove      bool          `mapstructure:"auto_remove"`
    PullPolicy      string        `mapstructure:"pull_policy"` // "always", "never", "missing"
    StartupTimeout  time.Duration `mapstructure:"startup_timeout"`
    ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
    
    // Logging
    LogDriver string            `mapstructure:"log_driver"`
    LogOpts   map[string]string `mapstructure:"log_opts"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
    return &Config{
        Image:            "acp-agent:latest",
        MemoryLimit:      "2G",
        CPULimit:         1.0,
        NetworkMode:      "none",
        ReadOnlyRootFS:   false,
        DropCaps:         []string{"NET_RAW", "SYS_ADMIN"},
        WorkspaceBaseDir: "/tmp/acp-workspaces",
        AutoRemove:       true,
        PullPolicy:       "missing",
        StartupTimeout:   30 * time.Second,
        ShutdownTimeout:  10 * time.Second,
        LogDriver:        "json-file",
        LogOpts: map[string]string{
            "max-size": "10m",
            "max-file": "3",
        },
    }
}

// ParseMemoryLimit converts string like "2G" to bytes
func ParseMemoryLimit(limit string) (int64, error) {
    if limit == "" {
        return 0, nil
    }
    
    // Simple parsing - expand this as needed
    multipliers := map[byte]int64{
        'K': 1024,
        'M': 1024 * 1024,
        'G': 1024 * 1024 * 1024,
    }
    
    lastChar := limit[len(limit)-1]
    if multiplier, ok := multipliers[lastChar]; ok {
        var value int64
        _, err := fmt.Sscanf(limit[:len(limit)-1], "%d", &value)
        if err != nil {
            return 0, fmt.Errorf("invalid memory limit format: %s", limit)
        }
        return value * multiplier, nil
    }
    
    // Assume bytes if no suffix
    var value int64
    _, err := fmt.Sscanf(limit, "%d", &value)
    return value, err
}
```

### 4.2 Container Manager

Create `internal/container/manager.go`:

```go
// ABOUTME: Docker container lifecycle management for ACP agents
// ABOUTME: Handles creation, attachment, and cleanup of agent containers

package container

import (
    "context"
    "fmt"
    "io"
    "log"
    "path/filepath"
    "sync"
    "time"
    
    "github.com/docker/docker/api/types"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/mount"
    "github.com/docker/docker/client"
    "github.com/google/uuid"
)

type Manager struct {
    client     *client.Client
    config     *Config
    containers map[string]*Container // sessionID -> Container
    mu         sync.RWMutex
}

type Container struct {
    ID          string
    SessionID   string
    WorkspaceDir string
    
    // Docker attachment hijacked connection
    Conn        types.HijackedResponse
    
    // Wrapped stdio
    Stdin       io.WriteCloser
    Stdout      io.ReadCloser
    Stderr      io.ReadCloser
    
    // Lifecycle
    StartedAt   time.Time
    Context     context.Context
    Cancel      context.CancelFunc
}

// NewManager creates a new container manager
func NewManager(cfg *Config) (*Manager, error) {
    // Create Docker client
    cli, err := client.NewClientWithOpts(
        client.FromEnv,
        client.WithAPIVersionNegotiation(),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create Docker client: %w", err)
    }
    
    // Verify Docker is accessible
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    _, err = cli.Ping(ctx)
    if err != nil {
        return nil, fmt.Errorf("Docker daemon not accessible: %w", err)
    }
    
    // Check if image exists (if pull policy is not "always")
    if cfg.PullPolicy != "always" {
        _, _, err = cli.ImageInspectWithRaw(ctx, cfg.Image)
        if err != nil && cfg.PullPolicy == "never" {
            return nil, fmt.Errorf("image %s not found and pull_policy is 'never'", cfg.Image)
        }
        if err != nil && cfg.PullPolicy == "missing" {
            log.Printf("Image %s not found locally, will pull", cfg.Image)
            // Pull the image
            if err := pullImage(cli, cfg.Image); err != nil {
                return nil, fmt.Errorf("failed to pull image: %w", err)
            }
        }
    }
    
    return &Manager{
        client:     cli,
        config:     cfg,
        containers: make(map[string]*Container),
    }, nil
}

// CreateContainer starts a new container for a session
func (m *Manager) CreateContainer(ctx context.Context, sessionID string, workingDir string) (*Container, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // Check if container already exists for this session
    if _, exists := m.containers[sessionID]; exists {
        return nil, fmt.Errorf("container already exists for session %s", sessionID)
    }
    
    // Create unique container name
    containerName := fmt.Sprintf("acp-agent-%s-%s", sessionID[:8], uuid.New().String()[:8])
    
    // Parse memory limit
    memoryLimit, err := ParseMemoryLimit(m.config.MemoryLimit)
    if err != nil {
        return nil, fmt.Errorf("invalid memory limit: %w", err)
    }
    
    // Create workspace directory on host
    hostWorkspace := filepath.Join(m.config.WorkspaceBaseDir, sessionID)
    if err := os.MkdirAll(hostWorkspace, 0755); err != nil {
        return nil, fmt.Errorf("failed to create workspace directory: %w", err)
    }
    
    // Container configuration
    containerConfig := &container.Config{
        Image:        m.config.Image,
        Hostname:     containerName,
        WorkingDir:   "/workspace",
        
        // Attach to stdio
        AttachStdin:  true,
        AttachStdout: true,
        AttachStderr: true,
        OpenStdin:    true,
        StdinOnce:    false,
        Tty:          false,
        
        // Environment variables
        Env: []string{
            fmt.Sprintf("SESSION_ID=%s", sessionID),
            "AGENT_CONTAINER=true",
            fmt.Sprintf("WORKSPACE=%s", "/workspace"),
        },
        
        // Labels for management
        Labels: map[string]string{
            "acp.relay":    "true",
            "acp.session":  sessionID,
            "acp.created":  time.Now().Format(time.RFC3339),
        },
    }
    
    // Host configuration
    hostConfig := &container.HostConfig{
        // Resource limits
        Resources: container.Resources{
            Memory:   memoryLimit,
            CPUQuota: int64(m.config.CPULimit * 100000), // Convert to microseconds
            CPUPeriod: 100000,
        },
        
        // Networking
        NetworkMode: container.NetworkMode(m.config.NetworkMode),
        
        // Volumes
        Mounts: []mount.Mount{
            {
                Type:   mount.TypeBind,
                Source: hostWorkspace,
                Target: "/workspace",
                BindOptions: &mount.BindOptions{
                    Propagation: mount.PropagationRPrivate,
                },
            },
        },
        
        // Security
        ReadonlyRootfs: m.config.ReadOnlyRootFS,
        CapDrop:       m.config.DropCaps,
        SecurityOpt:   m.config.SecurityOpt,
        
        // Cleanup
        AutoRemove: m.config.AutoRemove,
        
        // Logging
        LogConfig: container.LogConfig{
            Type:   m.config.LogDriver,
            Config: m.config.LogOpts,
        },
    }
    
    // Add extra volumes if configured
    for _, vol := range m.config.ExtraVolumes {
        parts := strings.Split(vol, ":")
        if len(parts) != 2 {
            log.Printf("Invalid volume format, skipping: %s", vol)
            continue
        }
        hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
            Type:   mount.TypeBind,
            Source: parts[0],
            Target: parts[1],
            ReadOnly: true,
        })
    }
    
    // Create the container
    createResp, err := m.client.ContainerCreate(
        ctx,
        containerConfig,
        hostConfig,
        nil, // NetworkingConfig
        nil, // Platform
        containerName,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create container: %w", err)
    }
    
    containerID := createResp.ID
    log.Printf("Created container %s for session %s", containerID[:12], sessionID[:8])
    
    // Start the container
    startCtx, cancel := context.WithTimeout(ctx, m.config.StartupTimeout)
    defer cancel()
    
    if err := m.client.ContainerStart(startCtx, containerID, types.ContainerStartOptions{}); err != nil {
        // Clean up container if start fails
        m.client.ContainerRemove(context.Background(), containerID, types.ContainerRemoveOptions{Force: true})
        return nil, fmt.Errorf("failed to start container: %w", err)
    }
    
    // Attach to container stdio
    attachResp, err := m.client.ContainerAttach(ctx, containerID, types.ContainerAttachOptions{
        Stream: true,
        Stdin:  true,
        Stdout: true,
        Stderr: true,
    })
    if err != nil {
        // Stop and remove container if attach fails
        m.client.ContainerStop(context.Background(), containerID, container.StopOptions{})
        return nil, fmt.Errorf("failed to attach to container: %w", err)
    }
    
    // Create container context
    containerCtx, containerCancel := context.WithCancel(ctx)
    
    // Create Container object
    cnt := &Container{
        ID:           containerID,
        SessionID:    sessionID,
        WorkspaceDir: hostWorkspace,
        Conn:         attachResp,
        Stdin:        attachResp.Conn,
        Stdout:       attachResp.Reader,
        Stderr:       attachResp.Reader, // Docker mixes stdout/stderr in attached mode
        StartedAt:    time.Now(),
        Context:      containerCtx,
        Cancel:       containerCancel,
    }
    
    // Store in map
    m.containers[sessionID] = cnt
    
    // Start monitoring goroutine
    go m.monitorContainer(cnt)
    
    log.Printf("Container %s ready for session %s", containerID[:12], sessionID[:8])
    return cnt, nil
}

// StopContainer stops and removes a container
func (m *Manager) StopContainer(sessionID string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    cnt, exists := m.containers[sessionID]
    if !exists {
        return fmt.Errorf("no container for session %s", sessionID)
    }
    
    log.Printf("Stopping container %s for session %s", cnt.ID[:12], sessionID[:8])
    
    // Cancel context
    cnt.Cancel()
    
    // Close connection
    cnt.Conn.Close()
    
    // Stop container with timeout
    stopCtx, cancel := context.WithTimeout(context.Background(), m.config.ShutdownTimeout)
    defer cancel()
    
    stopOptions := container.StopOptions{
        Timeout: int(m.config.ShutdownTimeout.Seconds()),
    }
    
    if err := m.client.ContainerStop(stopCtx, cnt.ID, stopOptions); err != nil {
        log.Printf("Error stopping container %s: %v", cnt.ID[:12], err)
        // Force remove if stop fails
        m.client.ContainerRemove(context.Background(), cnt.ID, types.ContainerRemoveOptions{
            Force: true,
        })
    }
    
    // Remove from map
    delete(m.containers, sessionID)
    
    // Clean up workspace if configured
    if m.config.AutoRemove {
        if err := os.RemoveAll(cnt.WorkspaceDir); err != nil {
            log.Printf("Failed to clean workspace %s: %v", cnt.WorkspaceDir, err)
        }
    }
    
    return nil
}

// GetContainer returns the container for a session
func (m *Manager) GetContainer(sessionID string) (*Container, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    cnt, exists := m.containers[sessionID]
    return cnt, exists
}

// monitorContainer watches container health
func (m *Manager) monitorContainer(cnt *Container) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-cnt.Context.Done():
            return
        case <-ticker.C:
            // Check if container is still running
            inspect, err := m.client.ContainerInspect(context.Background(), cnt.ID)
            if err != nil || !inspect.State.Running {
                log.Printf("Container %s no longer running, cleaning up", cnt.ID[:12])
                m.StopContainer(cnt.SessionID)
                return
            }
        }
    }
}

// pullImage pulls a Docker image
func pullImage(cli *client.Client, imageName string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()
    
    reader, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
    if err != nil {
        return err
    }
    defer reader.Close()
    
    // Read the output (important to consume it)
    _, err = io.Copy(io.Discard, reader)
    return err
}

// Cleanup stops all containers
func (m *Manager) Cleanup() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    var errors []error
    
    for sessionID := range m.containers {
        if err := m.StopContainer(sessionID); err != nil {
            errors = append(errors, fmt.Errorf("failed to stop container for session %s: %w", sessionID, err))
        }
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("cleanup errors: %v", errors)
    }
    
    return nil
}
```

### 4.3 Modified Session Manager

Update `internal/session/manager.go`:

```go
// Add to imports
import (
    "github.com/harper/acp-relay/internal/container"
)

// Update Manager struct
type Manager struct {
    config         ManagerConfig
    sessions       map[string]*Session
    mu             sync.RWMutex
    db             *db.DB
    containerMgr   *container.Manager // Add this
    useContainers  bool              // Add this
}

// Update NewManager
func NewManager(cfg ManagerConfig, database *db.DB, containerCfg *container.Config) *Manager {
    var containerMgr *container.Manager
    useContainers := false
    
    if containerCfg != nil && containerCfg.Image != "" {
        var err error
        containerMgr, err = container.NewManager(containerCfg)
        if err != nil {
            log.Printf("Failed to initialize container manager: %v", err)
            log.Printf("Falling back to process-based agents")
        } else {
            useContainers = true
            log.Printf("Container manager initialized, using Docker containers for agents")
        }
    }
    
    return &Manager{
        config:        cfg,
        sessions:      make(map[string]*Session),
        db:           database,
        containerMgr: containerMgr,
        useContainers: useContainers,
    }
}

// Update CreateSession
func (m *Manager) CreateSession(ctx context.Context, workingDir string) (*Session, error) {
    sessionID := "sess_" + uuid.New().String()[:8]
    
    if m.useContainers {
        return m.createContainerSession(ctx, sessionID, workingDir)
    } else {
        return m.createProcessSession(ctx, sessionID, workingDir)
    }
}

// New method: createContainerSession
func (m *Manager) createContainerSession(ctx context.Context, sessionID string, workingDir string) (*Session, error) {
    // Create container
    cnt, err := m.containerMgr.CreateContainer(ctx, sessionID, workingDir)
    if err != nil {
        return nil, fmt.Errorf("failed to create container: %w", err)
    }
    
    // Create session
    sess := &Session{
        ID:           sessionID,
        WorkingDir:   workingDir,
        ContainerID:  cnt.ID,
        AgentStdin:   cnt.Stdin,
        AgentStdout:  cnt.Stdout,
        AgentStderr:  cnt.Stderr,
        ToAgent:      make(chan []byte, 10),
        FromAgent:    make(chan []byte, 10),
        Context:      cnt.Context,
        Cancel:       cnt.Cancel,
        DB:           m.db,
        IsContainer:  true,
    }
    
    // Log session creation
    if m.db != nil {
        if err := m.db.CreateSession(sessionID, workingDir); err != nil {
            m.containerMgr.StopContainer(sessionID)
            return nil, fmt.Errorf("failed to log session creation: %w", err)
        }
    }
    
    m.mu.Lock()
    m.sessions[sessionID] = sess
    m.mu.Unlock()
    
    // Start stdio bridge
    go sess.StartStdioBridge()
    
    // Send initialize and session/new
    if err := sess.SendInitialize(); err != nil {
        m.CloseSession(sessionID)
        return nil, fmt.Errorf("failed to initialize agent: %w", err)
    }
    
    if err := sess.SendSessionNew(workingDir); err != nil {
        m.CloseSession(sessionID)
        return nil, fmt.Errorf("failed to create agent session: %w", err)
    }
    
    return sess, nil
}

// Rename existing CreateSession to createProcessSession
func (m *Manager) createProcessSession(ctx context.Context, sessionID string, workingDir string) (*Session, error) {
    // ... existing process-based code ...
}

// Update CloseSession
func (m *Manager) CloseSession(sessionID string) error {
    m.mu.Lock()
    sess, exists := m.sessions[sessionID]
    if !exists {
        m.mu.Unlock()
        return fmt.Errorf("session not found: %s", sessionID)
    }
    delete(m.sessions, sessionID)
    m.mu.Unlock()
    
    // Log closure
    if m.db != nil {
        if err := m.db.CloseSession(sessionID); err != nil {
            log.Printf("failed to log session closure: %v", err)
        }
    }
    
    // Handle container vs process cleanup
    if sess.IsContainer && m.containerMgr != nil {
        return m.containerMgr.StopContainer(sessionID)
    } else {
        // Existing process cleanup
        sess.Cancel()
        if sess.AgentCmd != nil {
            sess.AgentCmd.Wait()
        }
        close(sess.ToAgent)
        close(sess.FromAgent)
        return nil
    }
}
```

### 4.4 Update Session Type

Update `internal/session/session.go`:

```go
type Session struct {
    ID             string
    AgentSessionID string
    WorkingDir     string
    
    // Process mode fields
    AgentCmd       *exec.Cmd
    
    // Container mode fields
    ContainerID    string
    IsContainer    bool
    
    // Common fields
    AgentStdin     io.WriteCloser
    AgentStdout    io.ReadCloser
    AgentStderr    io.ReadCloser
    ToAgent        chan []byte
    FromAgent      chan []byte
    Context        context.Context
    Cancel         context.CancelFunc
    DB             *db.DB
    
    // For HTTP
    MessageBuffer [][]byte
    BufferMutex   sync.Mutex
}
```

---

## 5. Configuration Management

### 5.1 Update Config Structure

Update `internal/config/config.go`:

```go
type Config struct {
    Server    ServerConfig    `mapstructure:"server"`
    Agent     AgentConfig     `mapstructure:"agent"`
    Container *ContainerConfig `mapstructure:"container"` // Add this
    Database  DatabaseConfig  `mapstructure:"database"`
}

type ContainerConfig struct {
    Enabled          bool              `mapstructure:"enabled"`
    Image            string            `mapstructure:"image"`
    MemoryLimit      string            `mapstructure:"memory_limit"`
    CPULimit         float64           `mapstructure:"cpu_limit"`
    NetworkMode      string            `mapstructure:"network_mode"`
    WorkspaceBaseDir string            `mapstructure:"workspace_base_dir"`
    AutoRemove       bool              `mapstructure:"auto_remove"`
    PullPolicy       string            `mapstructure:"pull_policy"`
    ReadOnlyRootFS   bool              `mapstructure:"readonly_rootfs"`
    DropCaps         []string          `mapstructure:"drop_capabilities"`
    SecurityOpt      []string          `mapstructure:"security_opt"`
    ExtraVolumes     []string          `mapstructure:"extra_volumes"`
    LogDriver        string            `mapstructure:"log_driver"`
    LogOpts          map[string]string `mapstructure:"log_opts"`
}
```

### 5.2 Update config.yaml

```yaml
server:
  http_port: 8080
  http_host: "0.0.0.0"
  websocket_port: 8081
  websocket_host: "0.0.0.0"
  management_port: 8082
  management_host: "127.0.0.1"

# Traditional process-based agent (fallback)
agent:
  command: "npx"
  args: ["@zed-industries/claude-code-acp"]
  env:
    ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
  startup_timeout_seconds: 10
  max_concurrent_sessions: 100

# Container-based agent configuration
container:
  enabled: true  # Set to false to use process-based agents
  image: "acp-agent:latest"
  
  # Resource limits
  memory_limit: "2G"      # Memory limit (K, M, G suffix)
  cpu_limit: 1.0          # CPU cores (1.0 = 100% of 1 core)
  
  # Networking
  network_mode: "none"    # Options: none, bridge, host
  
  # Storage
  workspace_base_dir: "/var/lib/acp-relay/workspaces"
  auto_remove: true       # Remove container and workspace on session end
  
  # Docker behavior
  pull_policy: "missing"  # Options: always, never, missing
  
  # Security
  readonly_rootfs: false  # Make root filesystem read-only
  drop_capabilities:      # Linux capabilities to drop
    - "NET_RAW"
    - "SYS_ADMIN"
  security_opt: []        # Security options (e.g., "no-new-privileges")
  
  # Additional volumes to mount (read-only)
  extra_volumes:
    # - "/usr/share/docs:/docs:ro"
  
  # Logging
  log_driver: "json-file"
  log_opts:
    max-size: "10m"
    max-file: "3"

database:
  path: "./relay-messages.db"
```

### 5.3 Environment-Specific Configs

Create `config.development.yaml`:

```yaml
container:
  enabled: true
  image: "acp-agent:dev"
  memory_limit: "512M"
  cpu_limit: 0.5
  network_mode: "bridge"  # Allow network in dev
  workspace_base_dir: "/tmp/acp-dev"
  auto_remove: true
```

Create `config.production.yaml`:

```yaml
container:
  enabled: true
  image: "acp-agent:v1.0.0"  # Use specific version
  memory_limit: "4G"
  cpu_limit: 2.0
  network_mode: "none"  # No network in production
  workspace_base_dir: "/var/lib/acp-relay/workspaces"
  auto_remove: true
  readonly_rootfs: true
  drop_capabilities:
    - "ALL"  # Drop all capabilities
  security_opt:
    - "no-new-privileges"
    - "seccomp=default"
```

---

## 6. Error Handling

### 6.1 Container-Specific Errors

Create `internal/errors/container_errors.go`:

```go
package errors

import (
    "encoding/json"
    "fmt"
    "github.com/harper/acp-relay/internal/jsonrpc"
)

func NewContainerStartError(image string, details string) *jsonrpc.Error {
    message := fmt.Sprintf(
        "Failed to start agent container from image '%s'. This typically means "+
        "Docker is not running, the image doesn't exist, or there are insufficient resources.",
        image,
    )
    
    data := LLMErrorData{
        ErrorType:   "container_start_failed",
        Explanation: "The relay server could not create or start a Docker container for the agent.",
        PossibleCauses: []string{
            "Docker daemon is not running",
            "Docker image doesn't exist or couldn't be pulled",
            "Insufficient system resources (memory, disk space)",
            "Docker permissions issue (user not in docker group)",
            "Container configuration is invalid",
        },
        SuggestedActions: []string{
            "Check Docker is running: systemctl status docker",
            "Verify the image exists: docker images | grep " + image,
            "Check disk space: df -h",
            "Ensure user has Docker access: docker ps",
            "Check Docker logs: journalctl -u docker -n 50",
            "Try pulling the image manually: docker pull " + image,
        },
        RelevantState: map[string]interface{}{
            "image":   image,
            "details": details,
        },
        Recoverable: true,
        Details:     details,
    }
    
    dataBytes, _ := json.Marshal(data)
    return &jsonrpc.Error{
        Code:    jsonrpc.ServerError,
        Message: message,
        Data:    dataBytes,
    }
}

func NewContainerResourceError(resource string, limit string, details string) *jsonrpc.Error {
    message := fmt.Sprintf(
        "Container resource limit exceeded for %s (limit: %s). "+
        "The agent requires more %s than allocated.",
        resource, limit, resource,
    )
    
    data := LLMErrorData{
        ErrorType:   "container_resource_exceeded",
        Explanation: "The container hit a resource limit and was terminated or throttled.",
        PossibleCauses: []string{
            "Memory limit too low for agent operations",
            "CPU limit causing timeouts",
            "Disk space exhausted in workspace",
            "Too many concurrent containers",
        },
        SuggestedActions: []string{
            "Increase the " + resource + " limit in config.yaml",
            "Check current resource usage: docker stats",
            "Reduce concurrent sessions",
            "Clean up old containers: docker container prune",
        },
        RelevantState: map[string]interface{}{
            "resource": resource,
            "limit":    limit,
            "details":  details,
        },
        Recoverable: true,
    }
    
    dataBytes, _ := json.Marshal(data)
    return &jsonrpc.Error{
        Code:    jsonrpc.ServerError,
        Message: message,
        Data:    dataBytes,
    }
}

func NewDockerNotAvailableError(details string) *jsonrpc.Error {
    message := "Docker is not available or not properly configured. " +
        "The relay server requires Docker to run agent containers."
    
    data := LLMErrorData{
        ErrorType:   "docker_not_available",
        Explanation: "The relay cannot connect to the Docker daemon.",
        PossibleCauses: []string{
            "Docker is not installed",
            "Docker daemon is not running",
            "User doesn't have permission to access Docker socket",
            "Docker socket path is incorrect",
            "Running in environment without Docker",
        },
        SuggestedActions: []string{
            "Install Docker: https://docs.docker.com/engine/install/",
            "Start Docker: sudo systemctl start docker",
            "Add user to docker group: sudo usermod -aG docker $USER",
            "Verify Docker: docker version",
            "Check socket exists: ls -la /var/run/docker.sock",
            "Fall back to process mode: set container.enabled=false",
        },
        RelevantState: map[string]interface{}{
            "details": details,
        },
        Recoverable: false,
    }
    
    dataBytes, _ := json.Marshal(data)
    return &jsonrpc.Error{
        Code:    jsonrpc.ServerError,
        Message: message,
        Data:    dataBytes,
    }
}
```

---

## 7. Testing Strategy

### 7.1 Container Manager Tests

Create `internal/container/manager_test.go`:

```go
package container

import (
    "context"
    "testing"
    "time"
)

func TestContainerLifecycle(t *testing.T) {
    // Skip if Docker not available
    if !dockerAvailable() {
        t.Skip("Docker not available")
    }
    
    cfg := DefaultConfig()
    cfg.Image = "alpine:latest" // Use small test image
    
    mgr, err := NewManager(cfg)
    if err != nil {
        t.Fatalf("failed to create manager: %v", err)
    }
    
    // Test container creation
    ctx := context.Background()
    sessionID := "test_session_123"
    workDir := t.TempDir()
    
    cnt, err := mgr.CreateContainer(ctx, sessionID, workDir)
    if err != nil {
        t.Fatalf("failed to create container: %v", err)
    }
    
    // Verify container is running
    if cnt.ID == "" {
        t.Error("container ID is empty")
    }
    
    // Test writing to stdin
    testMsg := []byte("echo hello\n")
    _, err = cnt.Stdin.Write(testMsg)
    if err != nil {
        t.Errorf("failed to write to container: %v", err)
    }
    
    // Test stopping container
    err = mgr.StopContainer(sessionID)
    if err != nil {
        t.Errorf("failed to stop container: %v", err)
    }
    
    // Verify container is removed
    _, exists := mgr.GetContainer(sessionID)
    if exists {
        t.Error("container still exists after stop")
    }
}

func TestResourceLimits(t *testing.T) {
    if !dockerAvailable() {
        t.Skip("Docker not available")
    }
    
    cfg := DefaultConfig()
    cfg.Image = "alpine:latest"
    cfg.MemoryLimit = "100M"
    cfg.CPULimit = 0.5
    
    mgr, err := NewManager(cfg)
    if err != nil {
        t.Fatalf("failed to create manager: %v", err)
    }
    
    ctx := context.Background()
    cnt, err := mgr.CreateContainer(ctx, "test_limits", t.TempDir())
    if err != nil {
        t.Fatalf("failed to create container: %v", err)
    }
    defer mgr.StopContainer("test_limits")
    
    // Container should be created with limits
    // In real test, inspect container to verify limits
}

func dockerAvailable() bool {
    cli, err := client.NewClientWithOpts(client.FromEnv)
    if err != nil {
        return false
    }
    defer cli.Close()
    
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    
    _, err = cli.Ping(ctx)
    return err == nil
}
```

### 7.2 Integration Tests

Create `tests/container_integration_test.go`:

```go
package tests

import (
    "bytes"
    "encoding/json"
    "net/http"
    "testing"
    "time"
)

func TestContainerSession(t *testing.T) {
    // Start relay with container support
    startRelay(t, "test_config_containers.yaml")
    defer stopRelay(t)
    
    // Wait for startup
    time.Sleep(3 * time.Second)
    
    // Create session (should create container)
    sessionReq := map[string]interface{}{
        "jsonrpc": "2.0",
        "method":  "session/new",
        "params": map[string]interface{}{
            "workingDirectory": "/workspace",
        },
        "id": 1,
    }
    
    body, _ := json.Marshal(sessionReq)
    resp, err := http.Post(
        "http://localhost:8080/session/new",
        "application/json",
        bytes.NewReader(body),
    )
    if err != nil {
        t.Fatalf("failed to create session: %v", err)
    }
    
    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    
    sessionID := result["result"].(map[string]interface{})["sessionId"].(string)
    if sessionID == "" {
        t.Fatal("no session ID returned")
    }
    
    // Verify container was created
    containers := listContainers(t)
    found := false
    for _, cnt := range containers {
        if strings.Contains(cnt, sessionID[:8]) {
            found = true
            break
        }
    }
    
    if !found {
        t.Error("container not found for session")
    }
    
    // Test sending prompt
    promptReq := map[string]interface{}{
        "jsonrpc": "2.0",
        "method":  "session/prompt",
        "params": map[string]interface{}{
            "sessionId": sessionID,
            "content": []map[string]interface{}{
                {"type": "text", "text": "Hello from container"},
            },
        },
        "id": 2,
    }
    
    // ... rest of test
}

func listContainers(t *testing.T) []string {
    cmd := exec.Command("docker", "ps", "--format", "{{.Names}}")
    output, err := cmd.Output()
    if err != nil {
        t.Fatalf("failed to list containers: %v", err)
    }
    return strings.Split(string(output), "\n")
}
```

---

## 8. Deployment & Operations

### 8.1 Pre-flight Checks Script

Create `scripts/preflight.sh`:

```bash
#!/bin/bash
set -e

echo "=== ACP Relay Container Pre-flight Check ==="
echo

# Check Docker
echo -n "Docker installed: "
if command -v docker &> /dev/null; then
    echo "✓ ($(docker --version))"
else
    echo "✗ Docker not found"
    echo "  Install: https://docs.docker.com/engine/install/"
    exit 1
fi

echo -n "Docker running: "
if docker ps &> /dev/null; then
    echo "✓"
else
    echo "✗ Cannot connect to Docker"
    echo "  Start Docker: sudo systemctl start docker"
    exit 1
fi

echo -n "Docker permissions: "
if docker ps &> /dev/null; then
    echo "✓"
else
    echo "✗ Permission denied"
    echo "  Add to group: sudo usermod -aG docker $USER"
    exit 1
fi

# Check image
IMAGE_NAME="acp-agent:latest"
echo -n "Agent image exists: "
if docker image inspect "$IMAGE_NAME" &> /dev/null; then
    echo "✓"
else
    echo "✗ Image not found"
    echo "  Build image: ./docker/build.sh"
fi

# Check resources
echo -n "Available memory: "
FREE_MEM=$(free -h | grep Mem | awk '{print $7}')
echo "$FREE_MEM"

echo -n "Available disk: "
FREE_DISK=$(df -h /var/lib/docker | tail -1 | awk '{print $4}')
echo "$FREE_DISK"

# Check workspace directory
WORKSPACE_DIR="/var/lib/acp-relay/workspaces"
echo -n "Workspace directory: "
if [ -d "$WORKSPACE_DIR" ]; then
    echo "✓ exists"
else
    echo "✗ missing"
    echo "  Create: sudo mkdir -p $WORKSPACE_DIR"
fi

echo
echo "=== Pre-flight Check Complete ==="
```

### 8.2 Docker Compose Setup

Create `docker-compose.yaml`:

```yaml
version: '3.8'

services:
  relay:
    build:
      context: .
      dockerfile: Dockerfile.relay
    ports:
      - "8080:8080"  # HTTP API
      - "8081:8081"  # WebSocket
      - "8082:8082"  # Management
    volumes:
      # Docker socket for container management
      - /var/run/docker.sock:/var/run/docker.sock
      # Workspace directory
      - ./workspaces:/var/lib/acp-relay/workspaces
      # Configuration
      - ./config.yaml:/app/config.yaml
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    networks:
      - acp-network
    restart: unless-stopped

networks:
  acp-network:
    driver: bridge
```

Create `Dockerfile.relay`:

```dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o acp-relay ./cmd/relay

FROM alpine:3.19
RUN apk add --no-cache ca-certificates docker-cli
WORKDIR /app

COPY --from=builder /build/acp-relay /app/
COPY config.yaml /app/

EXPOSE 8080 8081 8082
CMD ["./acp-relay", "--config", "config.yaml"]
```

---

## 9. Troubleshooting Guide

### 9.1 Common Issues

#### Container fails to start

**Symptoms:**
```
Error: container_start_failed
```

**Debug steps:**
1. Check Docker daemon:
   ```bash
   systemctl status docker
   docker version
   ```

2. Check image exists:
   ```bash
   docker images | grep acp-agent
   ```

3. Check Docker logs:
   ```bash
   journalctl -u docker -n 100
   ```

4. Try manual container start:
   ```bash
   docker run -it acp-agent:latest /bin/sh
   ```

#### Memory limit exceeded

**Symptoms:**
```
Container killed with exit code 137 (OOMKilled)
```

**Solution:**
Increase memory limit in config.yaml:
```yaml
container:
  memory_limit: "4G"  # Increase from 2G
```

#### Permission denied

**Symptoms:**
```
Cannot connect to Docker daemon
```

**Solution:**
```bash
sudo usermod -aG docker $USER
newgrp docker
# OR run relay with sudo (not recommended)
```

#### Container can't access files

**Symptoms:**
```
Agent reports file not found in /workspace
```

**Debug:**
1. Check volume mount:
   ```bash
   docker inspect <container_id> | grep -A5 Mounts
   ```

2. Check permissions:
   ```bash
   ls -la /var/lib/acp-relay/workspaces/
   ```

3. Check SELinux (if applicable):
   ```bash
   getenforce
   # If enforcing, add :Z to volume mount
   ```

### 9.2 Debug Mode

Add debug configuration:

```yaml
# config.debug.yaml
container:
  enabled: true
  image: "acp-agent:debug"
  log_driver: "json-file"
  log_opts:
    max-size: "100m"
    max-file: "10"
  # Keep container after stop for inspection
  auto_remove: false
  # Allow network for debugging
  network_mode: "bridge"
```

### 9.3 Health Checks

Create `scripts/health.sh`:

```bash
#!/bin/bash

echo "=== Container Health Check ==="

# List agent containers
echo "Active agent containers:"
docker ps --filter "label=acp.relay=true" \
    --format "table {{.Names}}\t{{.Status}}\t{{.RunningFor}}"

echo
echo "Container resource usage:"
docker stats --no-stream \
    --filter "label=acp.relay=true" \
    --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}"

echo
echo "Recent container events:"
docker events \
    --filter "label=acp.relay=true" \
    --since "1h" \
    --until "now" \
    --format "{{.Time}} {{.Action}} {{.Actor.Attributes.name}}"
```

---

## Implementation Checklist

### Phase 1: Foundation
- [ ] Install Docker and verify access
- [ ] Add Docker SDK dependencies
- [ ] Create container package structure
- [ ] Build agent Docker image
- [ ] Write container manager

### Phase 2: Integration
- [ ] Update session manager for containers
- [ ] Add container configuration
- [ ] Update error handling
- [ ] Modify main.go to initialize container manager
- [ ] Test container creation/destruction

### Phase 3: Security & Limits
- [ ] Implement resource limits
- [ ] Add security options
- [ ] Configure volume mounts
- [ ] Test isolation

### Phase 4: Operations
- [ ] Create pre-flight check script
- [ ] Write health monitoring
- [ ] Document troubleshooting
- [ ] Create Docker Compose setup
- [ ] Test in production-like environment

### Phase 5: Polish
- [ ] Add metrics collection
- [ ] Implement container reuse pool (optional)
- [ ] Add container image versioning
- [ ] Create upgrade procedures
- [ ] Performance testing

