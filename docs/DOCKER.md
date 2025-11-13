# Docker Deployment Guide

This guide covers deploying acp-relay using Docker and Docker Compose.

## Architecture

The Docker setup consists of:

1. **Relay Container**: Runs the acp-relay server (Go application)
2. **Agent Containers**: Spawned dynamically by the relay in container mode
3. **Shared Volumes**: Database persistence and workspace directories

## Quick Start

### 1. Setup Environment

```bash
# Copy environment template
cp .env.example .env

# Edit .env and add your API key
vim .env
```

### 2. Build and Run

```bash
# Build and start the relay
docker compose up -d

# View logs
docker compose logs -f relay

# Check status
docker compose ps
```

### 3. Access the Relay

- HTTP API: http://localhost:23890
- WebSocket: ws://localhost:23891
- Management API: http://localhost:23892

## Volume Mounts

### Working Directory

Your project directory is mounted at `/app/workspace` inside the container:

```yaml
volumes:
  - ./:/app/workspace:rw
```

This allows you to:
- Access your code from within agent containers
- Make changes that persist across container restarts
- Develop locally while testing in Docker

### ACP Agent Workspaces

Each ACP agent gets its own workspace directory. The setup uses a **bind mount** (not a Docker volume) because:

1. The relay spawns agent containers as **siblings** (Docker-in-Docker via socket)
2. Agent containers need to mount the **same host paths** as the relay
3. The relay passes **host paths** to the Docker API when creating agent containers

```yaml
volumes:
  - ${ACP_WORKSPACES_DIR:-./data/workspaces}:/data/workspaces:rw
```

**Default location**: `./data/workspaces` (relative to docker-compose.yml)

**Custom location**: Set `ACP_WORKSPACES_DIR` in your `.env`:

```bash
ACP_WORKSPACES_DIR=/absolute/path/to/workspaces
```

⚠️ **Important**: Must be an absolute path if customized, as it's used by both the relay and agent containers.

### Database Persistence

The SQLite database is stored in a named volume:

```yaml
volumes:
  - relay-data:/data
```

This persists across container restarts but is managed by Docker.

## Configuration Files

Configuration files are **mounted as volumes** (not baked into the image), allowing you to:
- Edit config without rebuilding the image
- Swap configs for different environments
- Test config changes with simple container restart

### config-docker.yaml

Special configuration for Docker deployment:

- **Container mode enabled**: Agents run in isolated containers
- **Workspace paths**: Configured for Docker-in-Docker
- **Network settings**: All interfaces (0.0.0.0) for container networking

```yaml
agent:
  mode: "container"

container:
  workspace_host_base: "${ACP_WORKSPACES_HOST_PATH}"
  workspace_container_path: "/workspace"
```

Mounted at: `/app/config/config-docker.yaml`

### Regular config.yaml

Used for non-Docker deployments (local development, direct binary execution).

Mounted at: `/app/config/config.yaml`

### Changing Configuration

To use a different config:

```bash
# Edit the config file
vim config-docker.yaml

# Restart the container (no rebuild needed!)
docker compose restart relay
```

Or use a different config file:

```yaml
# In docker-compose.yml
command: ["/app/acp-relay", "--config", "/app/config/config.yaml"]
volumes:
  - ./my-custom-config.yaml:/app/config/config.yaml:ro
```

## Common Operations

### Rebuild After Code Changes

```bash
# Only needed for Go code changes, not config changes
docker compose up -d --build
```

### Change Config Without Rebuild

```bash
# Edit config
vim config-docker.yaml

# Just restart (no rebuild!)
docker compose restart relay
```

### View Logs

```bash
# All logs
docker compose logs -f

# Just relay
docker compose logs -f relay

# Last 100 lines
docker compose logs --tail=100 relay
```

### Stop and Remove

```bash
# Stop but keep data
docker compose down

# Stop and remove volumes (⚠️ deletes database)
docker compose down -v
```

### Access Relay Shell

```bash
docker compose exec relay bash
```

## Troubleshooting

### Agent Containers Not Starting

**Symptom**: Agents fail to start with volume mount errors

**Solution**: Check that `ACP_WORKSPACES_DIR` is an absolute path:

```bash
# In .env
ACP_WORKSPACES_DIR=/Users/harper/Public/src/2389/acp-relay/data/workspaces
```

### Docker Socket Permission Denied

**Symptom**: `Failed to initialize container manager: permission denied while trying to connect to the Docker daemon socket`

**Cause**: The relay container needs access to the Docker socket to spawn agent containers.

**Solution**: The docker-compose.yml runs the relay as `root` to access the Docker socket. This is safe because:
- The relay container is isolated
- Agent containers can still run as non-root
- It's required for Docker-in-Docker functionality

If you want to avoid root, use `group_add` to match the host's docker GID:

```yaml
# In docker-compose.yml (alternative to user: root)
services:
  relay:
    user: "1000:999"  # uid:gid where gid is docker group on host
    # Or:
    group_add:
      - "999"  # Add host's docker group GID
```

Find your docker group GID: `getent group docker | cut -d: -f3`

### Workspace Permission Errors

**Symptom**: Database or workspace errors with "permission denied"

**Solution**: Ensure directories are writable by the container:

```bash
# Create workspace directory with correct permissions
mkdir -p ./data/workspaces
chmod 755 ./data/workspaces
```

### Port Already in Use

**Symptom**: `Error starting userland proxy: bind: address already in use`

**Solution**: Change ports in docker-compose.yml:

```yaml
ports:
  - "23890:23890"  # Change left side: "24890:23890"
```

### Docker Socket Permission Denied

**Symptom**: Relay can't spawn agent containers

**Solution**: Ensure your user is in the `docker` group:

```bash
sudo usermod -aG docker $USER
# Log out and back in
```

## Health Checks

The relay includes a health check endpoint:

```bash
curl http://localhost:23890/health
```

Docker Compose monitors this automatically:

```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:23890/health"]
  interval: 30s
  timeout: 10s
  retries: 3
```

## Production Considerations

### Security

1. **Docker Socket Access**: The relay runs as root to access the Docker socket for spawning agent containers. This is required for Docker-in-Docker and is safe within the isolated container context.
2. **Restrict Management API**: Change `management_host` to `127.0.0.1` in production
3. **Use Secrets**: Don't commit `.env` with real API keys
4. **Update Base Images**: Regularly rebuild with latest security patches

### Performance

1. **Resource Limits**: Adjust container limits in config-docker.yaml:
   ```yaml
   container:
     memory_limit: "1g"
     cpu_limit: 2.0
   ```

2. **Database Backup**: Regular backups of the relay-data volume:
   ```bash
   docker run --rm -v acp-relay_relay-data:/data -v $(pwd):/backup \
     alpine tar czf /backup/relay-data-backup.tar.gz -C /data .
   ```

### Monitoring

1. **Logs**: Use a log aggregation service (e.g., Loki, ELK)
2. **Metrics**: Export Prometheus metrics (future enhancement)
3. **Alerts**: Monitor health check failures

## Development Workflow

1. **Edit code locally** (auto-synced via volume mount)
2. **Rebuild** when dependencies change: `docker compose up -d --build`
3. **Test** agent containers spawn correctly
4. **Debug** using logs: `docker compose logs -f`

## Agent Container Image

The `Dockerfile` (not `Dockerfile.relay`) builds the agent container image:

- Node.js 20 runtime
- Python 3 support
- Pre-installed ACP agents
- Common development tools

To build manually:

```bash
docker build -t acp-relay-agent:latest -f Dockerfile .
```

## Environment Variables Reference

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ANTHROPIC_API_KEY` | Yes | - | Claude API key |
| `ACP_WORKSPACES_DIR` | No | `./data/workspaces` | Host path for agent workspaces |
| `ACP_WORKSPACES_HOST_PATH` | Auto | Same as above | Internal variable for config |
