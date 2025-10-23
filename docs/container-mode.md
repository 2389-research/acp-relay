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
