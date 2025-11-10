# ACP-Relay Architecture

This document provides a visual representation of the acp-relay system architecture.

## Viewing the Architecture

The architecture is defined in `architecture.dot` using GraphViz DOT notation.

### Generate Diagram

To generate the architecture diagram from the source:

```bash
# PNG format (for viewing)
dot -Tpng docs/architecture.dot -o docs/architecture.png

# SVG format (scalable, better for web)
dot -Tsvg docs/architecture.dot -o docs/architecture.svg

# PDF format (for documentation)
dot -Tpdf docs/architecture.dot -o docs/architecture.pdf
```

### Prerequisites

Install GraphViz:

```bash
# macOS
brew install graphviz

# Ubuntu/Debian
sudo apt-get install graphviz

# Windows
choco install graphviz
```

## Architecture Overview

The acp-relay system uses a **layered architecture** with the following major components:

### 1. Frontend Layer (Interface Layer)
- **HTTP Server** (`:8080`) - REST-style request/response API
- **WebSocket Server** (`:8081`) - Bidirectional streaming communication
- **Management API** (`:8082`) - Health checks and monitoring (localhost only)

### 2. Translation Layer
- **JSON-RPC 2.0** - Protocol conversion between HTTP/WebSocket and ACP

### 3. Backend Layer (Session Management)
- **Session Manager** - Process lifecycle, stdio bridging, mode selection
- **Process Mode** - Direct subprocess execution
- **Container Mode** - Docker-isolated execution with enhanced security

### 4. Container Infrastructure (Pack'n'Play)
- **Container Manager** - Docker client, lifecycle management
- **Environment Filtering** - Allowlist-based env var security
- **Container Reuse** - Label-based container tracking and reuse
- **Runtime Detection** - Auto-detect Docker/Podman/Colima

### 5. Support Infrastructure
- **XDG Paths** - Standard Linux/Unix directory structure
- **Structured Logger** - Level-based logging with --verbose flag
- **Config Loader** - YAML parsing with XDG expansion

### 6. Storage Layer
- **SQLite Database** - Message logging and session tracking
- **Config Files** - YAML configuration and .env secrets

### 7. Error Handling
- **LLM-Optimized Errors** - Detailed error messages with:
  - Explanation of what happened
  - Possible causes
  - Suggested remediation actions
  - Relevant system state
  - Recoverability flag

## Key Data Flows

### 1. Client Request Flow
```
Client → Server (HTTP/WS) → JSON-RPC → Session Manager →
Container Manager → Docker/Colima → Agent Container → Response
```

### 2. Setup Flow
```
acp-relay setup → Runtime Detection → Interactive Q&A →
Config Generation → XDG Directory Creation
```

### 3. Container Creation Flow
```
Session Manager → Container Manager →
Check Existing (labels) → Environment Filtering →
Docker API → Agent Container (with labels)
```

## Color Coding

The architecture diagram uses color coding to identify component types:

- **Light Blue** - Interface Layer (servers, CLI)
- **Light Orange** - Core Business Logic (session management, translation)
- **Light Green** - Infrastructure (container runtime, agents)
- **Light Purple** - Storage (database, config files)
- **Light Red** - Error Handling
- **Light Gray** - External Clients

## Pack'n'Play Features

The diagram highlights the following Pack'n'Play enhancements:

1. **Environment Isolation** - Only safe variables (TERM, LANG, LC_*) passed to containers
2. **Container Reuse** - Existing containers tracked via labels and reused when possible
3. **Runtime Auto-Detection** - Automatically finds Docker, Podman, or Colima
4. **Interactive Setup** - `acp-relay setup` wizard for first-time configuration
5. **XDG Compliance** - Standard directory structure (`~/.config`, `~/.local/share`)
6. **Structured Logging** - Debug, Info, Warn, Error levels with --verbose control

## Security Model

The architecture enforces the following security boundaries:

- **Environment Variable Filtering** - Only allowlisted vars (TERM, LANG, LC_*, COLORTERM) pass to containers
- **User Config Passthrough** - Variables explicitly configured in config.yaml always pass through
- **Container Labels** - All managed containers tagged with `managed-by=acp-relay`
- **Read-Only Mounts** - `~/.claude` directory mounted read-only for credentials
- **Workspace Isolation** - Each session gets isolated workspace directory

## Execution Modes

### Process Mode (Simple)
- Direct subprocess execution
- Host filesystem access
- Full environment variable access
- Simple setup, less isolation

### Container Mode (Secure)
- Docker-isolated execution
- Environment variable filtering
- Container reuse for performance
- Requires Docker/Colima/Podman

## Component Details

### Session Manager
- **Responsibility**: Lifecycle management of agent processes/containers
- **Key Functions**:
  - `CreateSession()` - Spawn new agent
  - `SendMessage()` - Forward requests to agent
  - `CloseSession()` - Clean shutdown
- **Stdio Bridging**: Go channels ↔ process pipes

### Container Manager
- **Responsibility**: Docker container management
- **Key Functions**:
  - `CreateSession()` - Create or reuse container
  - `findExistingContainer()` - Label-based lookup
  - `filterAllowedEnvVars()` - Security filtering
  - `StopContainer()` - Graceful shutdown

### Runtime Detection
- **Responsibility**: Auto-discover available container runtimes
- **Detection Priority**: Colima > Docker > Podman
- **Checks**:
  - CLI availability
  - Socket path discovery
  - Daemon reachability
  - Version information

## Regenerating the Diagram

The source `.dot` file should be kept in version control. Generated images (PNG/SVG) are in `.gitignore`.

To regenerate after modifying `architecture.dot`:

```bash
make architecture   # If Makefile target exists
# or
dot -Tpng docs/architecture.dot -o docs/architecture.png
dot -Tsvg docs/architecture.dot -o docs/architecture.svg
```

## Related Documentation

- [Pack'n'Play Improvements](packnplay-improvements.md) - Detailed design document
- [API Documentation](api.md) - HTTP/WebSocket/Management API reference
- [Container Specification](container-spec.md) - Container mode details
- [README](../README.md) - Project overview and quick start

## Production Considerations

### Performance

- Connection pooling: Up to 100 concurrent sessions (configurable)
- Database: SQLite with WAL mode for concurrent writes
- Memory: ~512MB per container (configurable)

### Scaling

- Horizontal: Run multiple relay instances behind load balancer
- Vertical: Increase max_concurrent_sessions in config
- Database: Consider PostgreSQL for high-volume deployments

### Security

- Management API bound to localhost only
- Container mode isolates agent processes
- Environment variable filtering prevents leakage
- Regular security audits recommended

### Monitoring

- Health endpoint for load balancer checks
- Database logs all messages for debugging
- Structured logging with --verbose flag
- Consider Prometheus exporter for metrics

## Contributing

When modifying the architecture:

1. Update `architecture.dot` with your changes
2. Regenerate images: `dot -Tpng docs/architecture.dot -o docs/architecture.png`
3. Update this document if adding new components or flows
4. Commit only the `.dot` file (images are generated)
