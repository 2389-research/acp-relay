# Pack'n'Play Improvements Design

**Status**: Design Approved
**Created**: 2025-01-24
**Author**: Claude (with Doctor Biz)

## Overview

This design implements security, UX, and observability improvements inspired by obra/packnplay patterns. The goal is to make acp-relay container mode secure-by-default, trivial to set up, and easy to debug.

### Success Criteria

1. **Security**: Container isolation demonstrably better (no host environment leakage)
2. **Setup UX**: First-time setup completes in <5 minutes with zero Docker knowledge
3. **Observability**: Can debug container issues without SSH access

### Approach

Full Stack implementation - all improvements deployed together as cohesive system.

## Architecture

### 4-Layer Structure

```
┌─────────────────────────────────────────────────────────┐
│                  Interface Layer                         │
│  • HTTP/WebSocket servers (unchanged)                    │
│  • Management API (unchanged)                            │
└─────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────┐
│               User Experience Layer                      │
│  • Setup subcommand (cmd/relay/setup.go)                │
│  • Config enhancements (XDG path expansion)              │
└─────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────┐
│                  Security Layer                          │
│  • Environment variable allowlisting                     │
│  • Container labeling and reuse                          │
│  • Read-only credential mounts                           │
└─────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────┐
│                Foundation Layer                          │
│  • XDG Base Directory support (internal/xdg)            │
│  • Runtime detection (internal/runtime)                  │
│  • Structured logging (internal/logger)                  │
└─────────────────────────────────────────────────────────┘
```

## Components

### New Packages

#### 1. internal/xdg

**Purpose**: XDG Base Directory Specification support for Linux/Unix standards compliance

**Key Functions**:
```go
ConfigHome() string      // Returns ~/.config/acp-relay
DataHome() string        // Returns ~/.local/share/acp-relay
CacheHome() string       // Returns ~/.cache/acp-relay
ExpandPath(string) string // Expands $XDG_* variables in config
```

**Special Handling**:
- Graceful fallback if `HOME` environment variable missing (cwd → ".")
- Literal path passthrough for non-XDG paths
- Warning logs for unexpected scenarios

**Files**:
- `internal/xdg/xdg.go` - Core implementation
- `internal/xdg/xdg_test.go` - Unit tests including HOME missing regression

#### 2. internal/runtime

**Purpose**: Container runtime detection (Docker, Podman, Colima) with socket path discovery

**Key Functions**:
```go
DetectAll() ([]RuntimeInfo, error)    // Find all available runtimes
DetectBest() (*RuntimeInfo, error)    // Priority: Colima > Docker > Podman
```

**RuntimeInfo Structure**:
```go
type RuntimeInfo struct {
    Name       string  // "docker", "podman", "colima"
    Status     string  // "available", "cli-only", "unavailable", "running", "stopped"
    SocketPath string  // e.g., "/var/run/docker.sock"
    Version    string  // e.g., "24.0.7"
}
```

**Detection Strategy**:
- Check CLI presence (`docker version`, `colima version`, `podman version`)
- Test socket reachability (actual connection attempt)
- Return detailed status for UX feedback

**Files**:
- `internal/runtime/detect.go` - Core detection logic
- `internal/runtime/detect_test.go` - Unit tests for each runtime

#### 3. internal/logger

**Purpose**: Structured logging with verbosity control

**Levels**: DEBUG, INFO, WARN, ERROR

**Key Functions**:
```go
SetVerbose(bool)           // Controlled by --verbose flag
Debug(format, args...)     // Only shown in verbose mode
Info(format, args...)      // Always shown
Warn(format, args...)      // Always shown with prefix
Error(format, args...)     // Always shown with prefix
```

**Backward Compatibility**:
- Existing `log.Printf()` calls continue to work
- Wrapper functions preserve existing behavior
- No breaking changes to current logging

**Files**:
- `internal/logger/logger.go` - Core implementation
- `internal/logger/logger_test.go` - Unit tests including compat tests

### Enhanced Components

#### 4. internal/container/manager.go

**New Functions**:

```go
filterAllowedEnvVars(env map[string]string) map[string]string
// Allowlist: TERM, LANG, LC_*, COLORTERM only

buildContainerLabels(sessionID string) map[string]string
// Labels: managed-by=acp-relay, session-id=<id>, created-at=<timestamp>

findExistingContainer(ctx context.Context, sessionID string) (string, error)
// Query Docker labels, return container ID if found & running

sanitizeContainerName(sessionID string) string
// Produce valid Docker name: acp-relay-<session-id>
```

**Modified Functions**:

`CreateSession()` - Enhanced flow:
1. Check for existing container via labels
2. If found & running → reuse (update state, start monitor)
3. If found & stopped → remove & create new
4. Filter environment variables through allowlist
5. Add container labels
6. Set sanitized container name

**Critical Bug Fixes** (from previous implementation):
- Container reuse MUST update manager's containers map
- Container reuse MUST start monitor goroutine
- HOME fallback MUST use correct chain (not just empty string)
- String prefix checks MUST use `strings.HasPrefix` not `filepath.HasPrefix`

#### 5. cmd/relay/setup.go (NEW SUBCOMMAND)

**Purpose**: Interactive first-time setup wizard

**Flow**:
1. Runtime detection and status display
2. Runtime selection (auto-select if only one available)
3. Path configuration (data, config, cache dirs)
4. Verbosity preference
5. Runtime connection validation
6. Image verification (offer to pull if missing)
7. Config file generation
8. Dry-run session creation test

**Key Design Decision**: This is a SUBCOMMAND (`acp-relay setup`), NOT a separate binary (`acp-relay-setup`).

**Files**:
- `cmd/relay/setup.go` - Subcommand implementation
- `tests/integration/setup_test.go` - Integration tests

#### 6. internal/config/config.go

**Enhanced Behavior**:
- `Load()` now expands `$XDG_*` variables in paths using `xdg.ExpandPath()`
- Example: `database.path: "$XDG_DATA_HOME/db.sqlite"` → `~/.local/share/acp-relay/db.sqlite`
- Backward compatible: non-XDG paths passed through unchanged

## Data Flow

### Flow 1: First-Time Setup

```
User runs: acp-relay setup
  ↓
Runtime Detection
  • Detect all available: Docker/Podman/Colima
  • Show status: installed/running/socket-found
  ↓
Interactive Q&A
  • Which runtime to use? (auto-select if only one)
  • Where to store data? (default: ~/.local/share/acp-relay)
  • Where to put config? (default: ~/.config/acp-relay/config.yaml)
  • Verbose logging? (default: no)
  ↓
Validation
  • Test runtime connection
  • Verify image exists or offer to pull
  • Create directory structure
  ↓
Config Generation
  • Write config with detected settings
  • Set appropriate docker_host path
  • Configure XDG paths
  ↓
Verification
  • Test session creation (dry-run)
  • Report success or issues
```

### Flow 2: Enhanced Session Creation

```
HTTP POST /session/new or WebSocket session/new
  ↓
Session Manager
  ↓
Container Manager.CreateSession()
  ↓
[NEW] Check for existing container
  • Query Docker labels: managed-by=acp-relay, session-id=<id>
  • If found & running → reuse
  • If found & stopped → remove & create new
  ↓
[ENHANCED] Environment Preparation
  • Start with agentEnv from config
  • Filter through allowlist (TERM, LANG, LC_*, COLORTERM)
  • Expand $XDG_* variables using xdg package
  • Log warnings for empty ANTHROPIC_API_KEY
  ↓
[ENHANCED] Container Creation
  • Add labels: managed-by, session-id, created-at
  • Set sanitized name: acp-relay-<session-id>
  • Mount ~/.claude as read-only
  • Mount workspace read-write
  ↓
[ENHANCED] State Tracking
  • Update containers map
  • Start monitor goroutine
  • Return session components
```

### Flow 3: Runtime Detection

```
runtime.DetectAll()
  ↓
Check Docker
  • Is CLI installed? → docker version
  • Is daemon reachable? → try /var/run/docker.sock
  • Status: "available", "cli-only", or "unavailable"
  ↓
Check Colima
  • Is CLI installed? → colima version
  • Is running? → colima status
  • Find socket: ~/.colima/default/docker.sock
  • Status: "running", "stopped", or "unavailable"
  ↓
Check Podman
  • Is CLI installed? → podman version
  • Is daemon reachable? → try /var/run/podman/podman.sock
  • Status: "available", "cli-only", or "unavailable"
  ↓
Return []RuntimeInfo
  • Each with: name, status, socket_path, version
  • Sorted by priority: Colima > Docker > Podman
```

## Error Handling

### New Error Types

#### RuntimeNotFoundError
```go
type RuntimeNotFoundError struct {
    RequestedRuntime string
    AvailableRuntimes []string
}
```
- **When**: User config specifies unavailable runtime
- **Suggested Actions**: List available runtimes, offer to run `acp-relay setup`

#### ContainerReuseError
```go
type ContainerReuseError struct {
    ContainerID string
    SessionID string
    Reason string
}
```
- **When**: Found existing container but can't reuse (permission issue, corrupt state)
- **Suggested Actions**: Remove stale container, check Docker permissions

#### XDGPathError
```go
type XDGPathError struct {
    Variable string
    AttemptedPath string
}
```
- **When**: Can't create XDG directories (permissions, disk full)
- **Suggested Actions**: Check permissions, verify disk space, try manual creation

#### SetupRequiredError
```go
type SetupRequiredError struct {
    MissingConfig bool
    InvalidRuntime bool
    NoRuntimeFound bool
}
```
- **When**: First-time user or broken config
- **Suggested Actions**: Run `acp-relay setup`, check runtime installation

### Error Recovery Strategies

**Container Reuse Path**:
```
Try reuse existing container
  ↓ FAIL
Check if container is stopped
  ↓ YES
Remove stale container, create new
  ↓ FAIL
Fall back to fresh creation
```

**Runtime Detection Path**:
```
Try configured runtime
  ↓ FAIL
Try auto-detect best available
  ↓ FAIL
Return SetupRequiredError with diagnostics
```

**Environment Variable Expansion**:
```
Expand $XDG_* variables
  ↓ FAIL (bad syntax)
Log warning, use literal value
Continue (non-fatal)
```

### Logging Strategy

**Existing Behavior**: Standard log package with session prefixes
**Enhancement**: Add verbosity levels without breaking existing logs

```go
// New verbose mode (opt-in via --verbose flag)
logger.Debug("[%s] Checking for existing container with labels: %v", sessionID, labels)
logger.Info("[%s] Reusing existing container: %s", sessionID, containerID)

// Existing logs continue to work (compatibility)
log.Printf("[%s] Container started: %s", sessionID, containerID)
```

**Level Usage Guidelines**:
- **DEBUG**: Container label queries, environment filtering, runtime detection details
- **INFO**: Session lifecycle, container reuse decisions, setup completion
- **WARN**: Empty API keys, fallback to non-XDG paths, missing HOME variable
- **ERROR**: Container creation failures, runtime connection issues

### Backward Compatibility

**If old config lacks XDG paths**:
- Detect missing `database.path` with `$XDG_*` prefix
- Log INFO: "Using legacy config format, consider running 'acp-relay setup' to migrate"
- Continue with existing behavior

**If container mode disabled**:
- All new features still work (XDG, logging, setup command)
- Container enhancements simply don't apply

## Testing Strategy

### Unit Tests

**internal/xdg/xdg_test.go**:
- `TestConfigHome()` - Verify ~/.config/acp-relay path
- `TestDataHome()` - Verify ~/.local/share/acp-relay path
- `TestCacheHome()` - Verify ~/.cache/acp-relay path
- `TestExpandPath()` - Verify $XDG_* expansion
- `TestExpandPath_MissingHOME()` - **Regression test for Error #2**
- `TestExpandPath_RelativePaths()` - Verify non-XDG paths pass through

**internal/runtime/detect_test.go**:
- `TestDetectDocker()` - Docker detection logic
- `TestDetectColima()` - Colima detection logic
- `TestDetectPodman()` - Podman detection logic
- `TestDetectAll_Priority()` - Verify Colima > Docker > Podman
- `TestDetectBest_NoRuntimeAvailable()` - Error handling
- `TestSocketPathDiscovery()` - Socket location logic

**internal/logger/logger_test.go**:
- `TestDebugLevel()` - Debug only shows in verbose mode
- `TestInfoLevel()` - Info always shows
- `TestWarnLevel()` - Warn always shows with prefix
- `TestErrorLevel()` - Error always shows with prefix
- `TestSetVerbose()` - Toggle functionality
- `TestBackwardCompatibility()` - Existing log.Printf works

**internal/container/manager_test.go** (enhanced):
- Existing tests: `TestCreateSession()`, `TestStopContainer()`, `TestMemoryLimitParsing()`
- `TestFilterAllowedEnvVars()` - Allowlist enforcement
- `TestBuildContainerLabels()` - Label generation
- `TestFindExistingContainer_Found()` - Label query when container exists
- `TestFindExistingContainer_NotFound()` - Label query when no container
- `TestFindExistingContainer_Stopped()` - Stopped container handling
- `TestReuseContainer_UpdatesState()` - **Regression test for Error #1**
- `TestSanitizeContainerName()` - Name sanitization
- `TestContainerReuse_FullFlow()` - End-to-end reuse

### Integration Tests

**tests/integration/setup_test.go** (new):
- `TestSetupCommand_InteractiveFlow()` - Full Q&A flow
- `TestSetupCommand_ConfigGeneration()` - Config file output
- `TestSetupCommand_RuntimeDetection()` - Runtime detection
- `TestSetupCommand_XDGPaths()` - XDG path handling

**tests/integration/container_lifecycle_test.go** (enhanced):
- `TestSessionCreation_FreshContainer()` - Normal creation
- `TestSessionCreation_ReuseExisting()` - Reuse scenario
- `TestSessionCreation_CleanupStale()` - Stopped container cleanup
- `TestContainerLabels_Present()` - Label verification
- `TestEnvironmentFiltering()` - Allowlist verification
- `TestXDGVariableExpansion()` - Config expansion

**tests/integration/runtime_test.go** (new):
- `TestRuntimeDetection_Docker()` - Docker detection
- `TestRuntimeDetection_Colima()` - Colima detection
- `TestRuntimeDetection_Priority()` - Selection priority
- `TestInvalidRuntime_FallsBackToDetection()` - Error recovery

### End-to-End Tests

**tests/e2e/first_time_user_test.go** (new):
```bash
# Scenario: Brand new user with Docker installed
1. Run: acp-relay setup (with stdin automation)
2. Verify: config file created at XDG location
3. Verify: runtime detection found Docker
4. Run: acp-relay (start server)
5. POST: /session/new
6. Verify: session created in <5 minutes
7. Verify: container has correct labels
```

**tests/e2e/container_reuse_test.go** (new):
```bash
# Scenario: Same session ID requested twice
1. Create session A with ID "test-session"
2. Verify container created with name acp-relay-test-session
3. Create session B with same ID "test-session"
4. Verify: same container reused (no new container)
5. Verify: docker ps shows only 1 container for this session
```

**tests/e2e/security_test.go** (new):
```bash
# Scenario: Environment variable isolation
1. Start relay with HOST_SECRET=sensitive in environment
2. Create session
3. Exec into container: echo $HOST_SECRET
4. Verify: variable NOT present
5. Verify: only TERM, LANG, LC_*, COLORTERM present
```

### Manual Testing Checklist

**Setup UX**:
- [ ] Run `acp-relay setup` and verify questions make sense
- [ ] Check that config file is human-readable
- [ ] Verify runtime detection works on macOS (Docker Desktop vs Colima)
- [ ] Confirm setup completes in <5 minutes

**Debugging Experience**:
- [ ] Run with `--verbose` and verify useful debug output
- [ ] Check that container labels visible in `docker ps`
- [ ] Verify error messages are actionable

**Backward Compatibility**:
- [ ] Old config files still work without migration
- [ ] Process mode unaffected by container enhancements

### Test Coverage Goals

- **Unit tests**: 80% coverage minimum for new packages (xdg, runtime, logger)
- **Integration tests**: All major flows covered (setup, reuse, runtime detection)
- **E2E tests**: Critical user journeys (<5min setup, container reuse, security)
- **Regression tests**: All three bugs from previous implementation

## Critical Bug Fixes from Previous Implementation

### Error #1: Container Reuse State Management Bug
- **Problem**: When reusing existing container, didn't update manager's containers map or start monitoring goroutine
- **Impact**: `StopContainer()` would fail to find reused containers
- **Fix**: Must update state and start monitor in reuse path
- **Test**: `TestReuseContainer_UpdatesState()`

### Error #2: Missing HOME Environment Variable Handling
- **Problem**: XDG functions created paths at root if HOME not set
- **Impact**: Incorrect path construction, potential permission errors
- **Fix**: `getHome()` helper with fallback chain: HOME → cwd → "."
- **Test**: `TestExpandPath_MissingHOME()`

### Error #3: Wrong String Prefix Checking
- **Problem**: Used `filepath.HasPrefix()` (OS-specific) for literal `$XDG_*` pattern matching
- **Impact**: Incorrect behavior on case-insensitive filesystems
- **Fix**: Use `strings.HasPrefix()` for string literal matching
- **Test**: `TestExpandPath()` with various inputs

## Implementation Notes

### File Structure
```
acp-relay/
├── cmd/relay/
│   ├── main.go         # Enhanced with --verbose flag, setup command
│   └── setup.go        # NEW: setup subcommand
├── internal/
│   ├── xdg/            # NEW PACKAGE
│   │   ├── xdg.go
│   │   └── xdg_test.go
│   ├── runtime/        # NEW PACKAGE
│   │   ├── detect.go
│   │   └── detect_test.go
│   ├── logger/         # NEW PACKAGE
│   │   ├── logger.go
│   │   └── logger_test.go
│   ├── container/
│   │   └── manager.go  # ENHANCED
│   └── config/
│       └── config.go   # ENHANCED
├── tests/
│   ├── integration/
│   │   ├── setup_test.go          # NEW
│   │   ├── container_lifecycle_test.go  # ENHANCED
│   │   └── runtime_test.go        # NEW
│   └── e2e/
│       ├── first_time_user_test.go  # NEW
│       ├── container_reuse_test.go  # NEW
│       └── security_test.go         # NEW
└── docs/
    └── design/
        └── packnplay-improvements.md  # THIS FILE
```

### Dependencies
- No new external dependencies required
- Uses existing Docker client library
- Standard library for everything else

### Backward Compatibility
- All existing configs continue to work
- Process mode unaffected
- Optional `--verbose` flag (defaults to off)
- Optional `acp-relay setup` (not required for operation)

## Success Metrics

### Security (Criterion 1)
- ✅ Host environment variables NOT leaked (only allowlist passes)
- ✅ Container labels enable tracking and management
- ✅ Read-only mounts for credentials

### Setup UX (Criterion 2)
- ✅ First-time setup in <5 minutes (manual test)
- ✅ Zero Docker knowledge required (runtime detection)
- ✅ Interactive Q&A guides user through config

### Observability (Criterion 3)
- ✅ `--verbose` flag provides debug output
- ✅ Container labels visible in `docker ps`
- ✅ LLM-optimized error messages with suggested actions

## Next Steps

1. **Phase 5**: Set up git worktree for isolated development
2. **Phase 6**: Create detailed implementation plan with exact code snippets
3. **Implementation**: TDD approach with test-first development
4. **Code Review**: Use superpowers:code-reviewer between major sections
5. **Verification**: Manual testing against success criteria
6. **Merge**: PR with comprehensive description

---

**Design Status**: ✅ Complete and validated with Doctor Biz
**Ready for**: Worktree setup and implementation planning
