# Container Mode Setup Guide

This guide helps you set up and run the ACP relay in container mode.

## Prerequisites

1. **Docker or Colima** must be running
2. **ANTHROPIC_API_KEY** must be set in your environment
3. **Container image** must be built

## Quick Start

### 1. Set your API key

```bash
export ANTHROPIC_API_KEY="your-api-key-here"
```

**Important:** The relay uses `os.ExpandEnv()` to expand `${ANTHROPIC_API_KEY}` from the config, so the environment variable MUST be set before starting the relay.

To make it permanent, add to your `~/.bashrc`, `~/.zshrc`, or equivalent:

```bash
# Add to your shell config
export ANTHROPIC_API_KEY="sk-ant-..."
```

### 2. Build the runtime image

```bash
docker build -t acp-relay-runtime:latest .
```

This creates a **generic runtime image** with Node.js, Python, git, and common tools. The specific agent command is configured at runtime via config.yaml, not baked into the image.

**One image, multiple agents!** The same runtime image can run:
- Claude Code: `npx @zed-industries/claude-code-acp`
- Codex agent: `python -m codex_agent`
- Custom agents: `/usr/local/bin/my-agent`

### 3. Configure the relay

Edit `config-container-test.yaml` or create your own config:

```yaml
agent:
  mode: "container"
  command: "npx"  # Runtime command (not baked into image)
  args:
    - "@zed-industries/claude-code-acp"
  env:
    ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"  # Expands from environment
    HOME: "${HOME}"
    PATH: "${PATH}"

  container:
    image: "acp-relay-runtime:latest"  # Generic runtime image
    docker_host: "unix:///var/run/docker.sock"  # Docker Desktop
    # OR for Colima:
    # docker_host: "unix:///Users/YOUR_USERNAME/.config/colima/docker.sock"
    network_mode: "bridge"
    memory_limit: "512m"
    cpu_limit: 1.0
    workspace_host_base: "/tmp/acp-workspaces"
    workspace_container_path: "/workspace"
    auto_remove: true
```

### 4. Run the relay

```bash
./acp-relay -config config-container-test.yaml
```

You should see:

```
Container manager initialized (image: acp-relay-agent:latest)
Session manager initialized (mode: container)
Starting HTTP server on 0.0.0.0:8080
Starting WebSocket server on 0.0.0.0:8081
```

## Troubleshooting

### "WARNING: ANTHROPIC_API_KEY is empty"

When creating a session, if you see:

```
[sess_123] WARNING: ANTHROPIC_API_KEY is empty after expansion (template: ${ANTHROPIC_API_KEY})
[sess_123] Make sure ANTHROPIC_API_KEY is set in your environment before starting the relay
```

**Solution:** Set the environment variable before starting the relay:

```bash
export ANTHROPIC_API_KEY="sk-ant-your-key-here"
./acp-relay -config config-container-test.yaml
```

### "Cannot connect to Docker daemon"

**Error:** `docker_unavailable: Cannot connect to Docker daemon`

**Solutions:**

1. **Check Docker is running:**
   ```bash
   docker ps
   ```

2. **For Docker Desktop:** Use `unix:///var/run/docker.sock`

3. **For Colima:**
   ```bash
   # Check Colima is running
   colima status

   # Start if needed
   colima start

   # Find socket path
   ls -la ~/.config/colima/docker.sock
   # OR
   ls -la ~/.colima/default/docker.sock

   # Update config to use the correct path
   docker_host: "unix:///Users/YOUR_USERNAME/.config/colima/docker.sock"
   ```

### "Image not found"

**Error:** `image_not_found: Docker image 'acp-relay-agent:latest' not found`

**Solution:** Build the image:

```bash
docker build -t acp-relay-agent:latest .
```

Verify it exists:

```bash
docker images | grep acp-relay-agent
```

## Features

### Session Resumption

Sessions now persist when clients disconnect, allowing you to resume work after network interruptions or client crashes.

**How it works:**

1. **Create a session:**
   ```json
   {"jsonrpc":"2.0","method":"session/new","params":{"workingDirectory":"/tmp"},"id":1}
   ```
   Response: `{"result":{"sessionId":"sess_abc12345"}}`

2. **Client disconnects** (crash, network issue, etc.)
   - Session stays alive
   - Container keeps running
   - Agent state is preserved

3. **Resume the session:**
   ```json
   {"jsonrpc":"2.0","method":"session/resume","params":{"sessionId":"sess_abc12345"},"id":2}
   ```
   Response: `{"result":{"sessionId":"sess_abc12345"}}`

4. **Continue working** - pick up exactly where you left off!

**Logs you'll see:**

```
[WS:sess_abc] Client disconnected, session remains active for resumption
[WS:sess_abc] Client resuming session
```

**Important:** Sessions don't timeout automatically yet, so you may want to manually clean up old sessions using the management API.

### Automatic ~/.claude Mounting

The relay automatically mounts your `~/.claude` directory as **read-only** at `/home/.claude` inside the container. This gives agents access to:

- `~/.claude/CLAUDE.md` - Your global Claude instructions
- `~/.claude/skills/` - Custom skills
- `~/.claude/docs/` - Documentation files

You'll see a log message when this happens:

```
[sess_123] Mounting ~/.claude directory as read-only
```

If the directory doesn't exist, it's silently skipped:

```
[sess_123] ~/.claude directory not found, skipping mount
```

### Environment Variable Logging

When a session starts, you'll see detailed logging of environment variables:

```
[sess_abc123] Setting env: ANTHROPIC_API_KEY=sk-ant-... (from template: ${ANTHROPIC_API_KEY})
[sess_abc123] Setting env: HOME=/Users/harper (from template: ${HOME})
[sess_abc123] Setting env: PATH=... (from template: ${PATH})
[sess_abc123] Mounting ~/.claude directory as read-only
```

This helps verify that environment variables are being set correctly.

## Testing

To verify everything is working:

1. **Check Docker connectivity:**
   ```bash
   docker ps
   ```

2. **Verify image exists:**
   ```bash
   docker images | grep acp-relay-agent
   ```

3. **Test environment variable:**
   ```bash
   echo $ANTHROPIC_API_KEY | cut -c1-20
   # Should output: sk-ant-api03-...
   ```

4. **Start relay and watch logs:**
   ```bash
   ./acp-relay -config config-container-test.yaml
   ```

5. **Create a test session** via WebSocket or HTTP to see the environment variable logging

## Environment Variable Template Expansion

The relay uses Go's `os.ExpandEnv()` which expands shell-style variables:

- `${VAR}` → value of `VAR` environment variable
- `$VAR` → also works
- Missing variables expand to empty string (triggers WARNING for API keys)

Example:

```yaml
env:
  ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"  # ✅ Expands from environment
  STATIC_VALUE: "hardcoded-value"            # ✅ No expansion
  MISSING_VAR: "${DOES_NOT_EXIST}"           # ⚠️  Expands to empty string
```

## Next Steps

- See `docs/container-mode.md` for detailed architecture
- See `docs/container-test-checklist.md` for manual testing procedures
- Check GitHub Issues for known limitations (e.g., #2 - runtime-configurable agent)
