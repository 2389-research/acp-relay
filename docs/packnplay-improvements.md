# Pack'n'Play Improvements

Security, UX, and observability enhancements for acp-relay container mode.

## Overview

These improvements implement patterns from obra/packnplay to make acp-relay:

1. **Secure by default**: No host environment variable leakage
2. **Trivial to set up**: <5 minute first-time setup with zero Docker knowledge
3. **Easy to debug**: Container labels and verbose logging

## Features

### Security

**Environment Variable Allowlisting**

Only safe terminal and locale variables pass to containers:
- `TERM`, `COLORTERM` - Terminal capabilities
- `LANG`, `LC_*` - Locale settings

Sensitive variables like `HOME`, `PATH`, API keys are blocked.

**Container Labels**

Every managed container has labels:
- `managed-by=acp-relay` - Identifies relay-managed containers
- `session-id=<id>` - Links container to session
- `created-at=<timestamp>` - Creation time

Use these for debugging: `docker ps --filter label=managed-by=acp-relay`

### UX

**Interactive Setup**

```bash
acp-relay setup
```

Guides you through:
1. Runtime detection (Docker/Colima/Podman)
2. Path configuration
3. Config generation

Completes in <5 minutes, no Docker knowledge required.

**Container Reuse**

Existing containers are reused when the same session ID is requested, reducing startup time and resource usage.

**XDG Directory Support**

Follows Linux/Unix standards:
- Config: `~/.config/acp-relay/`
- Data: `~/.local/share/acp-relay/`
- Cache: `~/.cache/acp-relay/`

Config paths can use `$XDG_*` variables:

```yaml
database:
  path: "$XDG_DATA_HOME/db.sqlite"
```

### Observability

**Verbose Logging**

```bash
acp-relay --verbose
```

Shows DEBUG-level output:
- Container label queries
- Environment variable filtering
- Runtime detection details

**Container Labels**

List managed containers:
```bash
docker ps --filter label=managed-by=acp-relay
```

Find specific session:
```bash
docker ps --filter label=session-id=my-session
```

**Structured Logs**

All logs have level prefixes:
- `[DEBUG]` - Verbose only
- `[INFO]` - Always shown
- `[WARN]` - Issues that don't stop operation
- `[ERROR]` - Failures

## Usage

### First-Time Setup

1. Run setup command:
   ```bash
   acp-relay setup
   ```

2. Follow prompts:
   - Select runtime (auto-selected if only one)
   - Accept default paths or customize
   - Choose verbosity

3. Review generated config:
   ```bash
   cat ~/.config/acp-relay/config.yaml
   ```

4. Update agent command path in config

5. Start server:
   ```bash
   acp-relay --config ~/.config/acp-relay/config.yaml
   ```

### Debugging

**View managed containers:**
```bash
docker ps --filter label=managed-by=acp-relay
```

**Check container details:**
```bash
docker inspect <container-id>
```

**View container logs:**
```bash
docker logs <container-id>
```

**Run with verbose logging:**
```bash
acp-relay --verbose
```

## Configuration

### XDG Variables

Use in `config.yaml`:

```yaml
database:
  path: "$XDG_DATA_HOME/db.sqlite"  # Expands to ~/.local/share/acp-relay/db.sqlite

agent:
  container:
    workspace_host_base: "$XDG_DATA_HOME/workspaces"
```

### Environment Variables

Set before starting relay:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
acp-relay
```

Config references:
```yaml
agent:
  env:
    ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
```

## Success Criteria Validation

✅ **Criterion #1: Security**
- Environment allowlist blocks host variables
- Only TERM, LANG, LC_*, COLORTERM pass through
- Validated by E2E security test

✅ **Criterion #2: Setup UX**
- Interactive setup completes in <5 minutes
- No Docker knowledge required
- Validated by E2E first-time user test

✅ **Criterion #3: Observability**
- Container labels visible in `docker ps`
- Verbose logging shows debug details
- LLM-optimized error messages with actions

## Migration

### From Old Config

Old configs continue to work unchanged. To use new features:

1. Run `acp-relay setup` to generate new config
2. Copy your agent command and env vars
3. Use new config with XDG paths

### Environment Variables

If you previously set environment variables for agents, update your config:

```yaml
agent:
  env:
    ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"  # Reference host var
```

Note: Only allowlisted variables will reach containers. Others must be in config.

## Troubleshooting

**Setup says "no runtime found"**
- Install Docker: https://docs.docker.com/get-docker/
- Or install Colima: `brew install colima && colima start`

**Container exits immediately**
- Check logs: `docker logs <container-id>`
- Verify agent command in config
- Ensure ANTHROPIC_API_KEY set

**Environment variables missing in container**
- Check allowlist (only TERM, LANG, LC_*, COLORTERM allowed)
- Add other vars explicitly in config `agent.env`

**Can't find managed containers**
- List all: `docker ps -a --filter label=managed-by=acp-relay`
- Check session ID: `docker ps --filter label=session-id=<your-id>`

## Design Documentation

See [docs/design/packnplay-improvements.md](design/packnplay-improvements.md) for:
- Architecture details
- Component design
- Error handling strategy
- Testing approach
