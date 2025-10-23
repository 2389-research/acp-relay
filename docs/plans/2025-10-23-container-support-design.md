# Container-Based Agent Execution Design

**Date:** 2025-10-23
**Status:** Approved
**Target:** Local/friendly infrastructure deployment

## Overview

Add Docker container-based agent execution alongside existing process-based execution. Both modes will coexist, selectable via configuration. This provides isolation, reproducibility, and prepares for future multi-tenancy.

## Requirements

### Functional
- Support both process and container execution modes via config flag
- Container mode provides identical API to process mode (transparent to clients)
- Host directory mounts for workspace persistence
- Proper stdout/stderr separation (fixes Docker stream demuxing bug)
- Clean session lifecycle (create, attach, stop, cleanup)

### Non-Functional
- Backward compatible: existing configs continue working in process mode
- Fail-fast with helpful errors (Docker not running, image missing)
- Prepare architecture for future dynamic Dockerfile generation

### Constraints
- Local/friendly infrastructure (not paranoid multi-tenant security)
- Must not break existing process-based functionality
- Start with static Dockerfile, plan for dynamic generation later

## Architecture

### Component Structure

```
internal/
├── container/              # NEW - Docker container management
│   ├── manager.go         # ContainerManager - creates/manages containers
│   ├── session.go         # Container-specific session state
│   ├── docker.go          # Docker client wrapper and helpers
│   ├── stream.go          # Stream demuxing (fixes stdout/stderr bug)
│   └── errors.go          # Container-specific error types
│
├── session/               # MODIFIED - Add container delegation
│   ├── manager.go         # Routes to process or container based on config
│   ├── session.go         # Common session interface (works for both modes)
│   └── bridge.go          # Stdio bridge (unchanged)
│
└── config/                # MODIFIED - Add container config
    └── config.go          # Add agent.mode, agent.container settings
```

### Design Pattern: Adapter Pattern

**Choice:** Separate `container.Manager` that implements same interface as process-based session creation.

**Rationale:**
- Clean separation of concerns (Docker logic isolated)
- Session Manager delegates based on config mode
- Both paths return same `*session.Session` type
- Easy to add future execution modes (Kubernetes, containerd)
- Container-specific features (pooling, image caching) have clear home

**Alternatives considered:**
- Minimal if/else in existing Manager: Would work but gets messy as complexity grows
- Strategy pattern with Executor interface: More abstraction than needed for two modes

## Configuration

### Schema Changes

```yaml
agent:
  mode: "process"  # NEW: "process" or "container" (default: "process")

  # Existing process-mode config (unchanged)
  command: "npx"
  args: ["@zed-industries/claude-code-acp"]
  env:
    ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"

  # NEW: Container-mode config
  container:
    image: "acp-relay-agent:latest"
    docker_host: "unix:///var/run/docker.sock"
    network_mode: "bridge"

    # Resource limits
    memory_limit: "512m"
    cpu_limit: 1.0

    # Workspace mounting
    workspace_host_base: "/tmp/acp-workspaces"
    workspace_container_path: "/workspace"

    # Lifecycle
    auto_remove: true
    startup_timeout_seconds: 10
```

### Backward Compatibility

- If `agent.mode` unset → defaults to "process"
- Existing configs work without modification
- Container config only validated when `mode: "container"`

## Implementation Details

### 1. Container Manager (internal/container/manager.go)

**Responsibilities:**
- Initialize Docker client and verify connectivity
- Create containers with proper config (mounts, limits, env)
- Attach to container stdio
- Demux stdout/stderr streams
- Monitor container lifecycle
- Stop and cleanup containers

**Key Methods:**

```go
type Manager struct {
    config       config.ContainerConfig
    agentEnv     map[string]string
    dockerClient *client.Client
    sessions     map[string]*ContainerSession
    mu           sync.RWMutex
    db           *db.DB
}

func NewManager(cfg, agentEnv, db) (*Manager, error)
func (m *Manager) CreateSession(ctx, sessionID, workingDir) (*session.Session, error)
func (m *Manager) StopContainer(sessionID string) error
func (m *Manager) monitorContainer(ctx, containerID, sessionID)
```

**Initialization checks:**
1. Create Docker client
2. Ping Docker daemon (fail fast if not running)
3. Inspect image (fail fast if not built)

### 2. Stream Demuxing (internal/container/stream.go)

**Problem:** Docker multiplexes stdout/stderr with 8-byte headers when `Tty: false`

**Solution:** Use `stdcopy.StdCopy` to demux into separate pipes

```go
func demuxStreams(multiplexed io.Reader) (stdout, stderr io.Reader) {
    stdoutPipe, stdoutWriter := io.Pipe()
    stderrPipe, stderrWriter := io.Pipe()

    go func() {
        defer stdoutWriter.Close()
        defer stderrWriter.Close()
        stdcopy.StdCopy(stdoutWriter, stderrWriter, multiplexed)
    }()

    return stdoutPipe, stderrPipe
}
```

**Critical:** Must set `Tty: false` in container config for demuxing to work.

**Alternative rejected:** `Tty: true` would avoid demuxing but loses stderr entirely.

### 3. Session Manager Integration (internal/session/manager.go)

**Changes:**

```go
type Manager struct {
    // ... existing fields
    containerManager *container.Manager  // NEW: optional, nil in process mode
}

func NewManager(cfg ManagerConfig, database *db.DB) *Manager {
    m := &Manager{...}

    // Initialize container manager if mode is "container"
    if cfg.Mode == "container" {
        m.containerManager, err = container.NewManager(...)
    }

    return m
}

func (m *Manager) CreateSession(ctx, workingDir) (*Session, error) {
    sessionID := generateID()

    // Route based on mode
    if m.config.Mode == "container" {
        sess, err = m.containerManager.CreateSession(ctx, sessionID, workingDir)
    } else {
        sess, err = m.createProcessSession(ctx, sessionID, workingDir)
    }

    // Common initialization (same for both modes)
    m.sessions[sessionID] = sess
    go sess.StartStdioBridge()
    sess.SendInitialize()
    sess.SendSessionNew(workingDir)

    return sess, nil
}

func (m *Manager) createProcessSession(...) (*Session, error) {
    // Existing exec.Command code moved here (unchanged)
}
```

**Key insight:** Both paths converge to same session initialization. Stdio bridge and ACP protocol handling work identically.

### 4. Session Struct Changes (internal/session/session.go)

**Modified fields (some become optional):**

```go
type Session struct {
    ID          string
    WorkingDir  string

    // Process mode fields (nil in container mode)
    AgentCmd    *exec.Cmd

    // Container mode fields (empty in process mode)
    ContainerID string

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

**No interface changes:** Existing session methods (SendInitialize, SendSessionNew, StartStdioBridge) work unchanged.

### 5. Dockerfile

**Location:** Repo root: `Dockerfile`

```dockerfile
FROM node:20-slim

RUN apt-get update && apt-get install -y \
    git \
    curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

RUN npm install -g @zed-industries/claude-code-acp

ENTRYPOINT ["npx", "@zed-industries/claude-code-acp"]
```

**Build:** `docker build -t acp-relay-agent:latest .`

**Future:** Dynamic Dockerfile generation based on agent config (separate feature)

### 6. Error Handling

**New error type (internal/container/errors.go):**

```go
type ContainerError struct {
    Type    string  // "image_not_found", "docker_unavailable", "attach_failed"
    Message string  // Human-readable with suggested action
    Cause   error
}
```

**Error scenarios:**

| Error | Detection | Message |
|-------|-----------|---------|
| Docker daemon not running | NewManager → Ping fails | "Cannot connect to Docker daemon. Is Docker running? Check: docker ps" |
| Image not found | NewManager → ImageInspect fails | "Docker image 'X' not found. Build it with: docker build -t X ." |
| Container exits early | monitorContainer → non-zero exit | "Container exited with code N. Last 50 log lines: ..." |
| Attach fails | CreateSession → ContainerAttach fails | "Failed to attach to container stdio: <reason>" |

**Principle:** Fail fast with actionable error messages.

### 7. Container Lifecycle

**Create flow:**
1. Create host workspace directory (`/tmp/acp-workspaces/<sessionID>`)
2. Create container with mounts, env, resource limits
3. Start container
4. Attach to stdin/stdout/stderr
5. Demux stdout/stderr into separate readers
6. Start background monitoring goroutine
7. Return Session to caller

**Monitor flow (background goroutine):**
1. ContainerWait for exit status
2. If non-zero exit: read last 50 log lines and log them
3. Signal session closure (future enhancement)

**Stop flow:**
1. ContainerStop with 10-second timeout
2. AutoRemove (if configured) deletes container automatically
3. Close database session
4. Host workspace persists for inspection

## Data Flow

### Container Session Creation

```
Client Request
    ↓
HTTP/WebSocket Handler
    ↓
SessionManager.CreateSession()
    ↓
[mode == "container"]
    ↓
ContainerManager.CreateSession()
    ↓
1. Create host workspace dir
2. Docker: ContainerCreate (with mounts, limits, env)
3. Docker: ContainerStart
4. Docker: ContainerAttach
5. Demux stdout/stderr
6. Start monitor goroutine
    ↓
Return *session.Session
    ↓
SessionManager: StartStdioBridge()
SessionManager: SendInitialize()
SessionManager: SendSessionNew()
    ↓
Session Ready (identical to process mode from here)
```

### Workspace Mounting

```
Host: /tmp/acp-workspaces/sess_abc123/
        ↕ (bind mount)
Container: /workspace/

Files created by agent in /workspace appear on host immediately.
Files persist after container stops (for inspection/debugging).
```

## Testing Strategy

### Unit Tests
- Mock Docker client using interface
- Test CreateSession logic (config → container.Config translation)
- Test error handling (image missing, daemon down)
- Test stream demuxing with sample Docker streams

### Integration Tests
- Require Docker daemon running
- Create real container, attach, send JSON-RPC, receive response
- Verify workspace files appear on host
- Test session lifecycle: create → attach → stop → cleanup

### Manual Testing
- Run relay in process mode: verify existing behavior unchanged
- Run relay in container mode: verify identical client experience
- Test error scenarios: no Docker, no image, container crash

## Deployment

### Prerequisites
- Docker daemon running
- Image built: `docker build -t acp-relay-agent:latest .`
- Config set to `agent.mode: "container"`

### Migration Path
1. Build Docker image
2. Update config.yaml with container settings
3. Change `agent.mode: "process"` → `"container"`
4. Restart relay
5. Monitor logs for container creation/attachment

### Rollback
- Change `agent.mode: "container"` → `"process"`
- Restart relay
- Falls back to existing process-based execution

## Future Enhancements

### Phase 2: Security Hardening (for production)
- Rootless Docker support
- Docker API proxy with allowlist
- User namespaces and capability dropping
- Secrets management (env injection without disk writes)
- Network isolation policies

### Phase 3: Production Polish
- Container pooling (warm containers for faster startup)
- Metrics collection (startup latency, OOM rate, resource usage)
- Health checks and liveness probes
- Orphan cleanup on relay startup
- Structured logging with trace IDs

### Dynamic Dockerfile Generation
- Generate Dockerfile based on agent config
- Support multiple agent types (Claude Code, Codex, custom)
- Cache generated images
- Version management

## References

- Original container spec: `docs/container-spec.md`
- Security gaps analysis: `docs/container-spec-gaps.md`
- Docker SDK docs: https://pkg.go.dev/github.com/docker/docker/client
- Stream demuxing: https://pkg.go.dev/github.com/docker/docker/pkg/stdcopy

## Approval

**Design approved:** 2025-10-23
**Approved by:** Doctor Biz
**Ready for implementation:** Yes
