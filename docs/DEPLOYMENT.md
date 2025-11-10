# Deployment Guide

## Prerequisites

- Go 1.24.1 or higher
- Docker (for container mode)
- SQLite3

## Build for Production

```bash
# Build optimized binary
CGO_ENABLED=1 go build -ldflags="-s -w" -o acp-relay ./cmd/relay

# Verify binary
./acp-relay --help
```

## Configuration

1. Run setup:
   ```bash
   ./acp-relay setup
   ```

2. Edit config at `~/.config/acp-relay/config.yaml`

3. Set environment variables:
   ```bash
   export ANTHROPIC_API_KEY=sk-ant-...
   ```

## Running

### Development

```bash
./acp-relay serve --config config.yaml --verbose
```

### Production (with systemd)

Create `/etc/systemd/system/acp-relay.service`:

```ini
[Unit]
Description=ACP Relay Server
After=network.target docker.service

[Service]
Type=simple
User=acp-relay
ExecStart=/usr/local/bin/acp-relay serve --config /etc/acp-relay/config.yaml
Restart=on-failure
RestartSec=5s
Environment="ANTHROPIC_API_KEY=sk-ant-..."

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable acp-relay
sudo systemctl start acp-relay
sudo systemctl status acp-relay
```

## Monitoring

- Health check: `curl http://localhost:8082/api/health`
- Logs: `journalctl -u acp-relay -f`
- Database: `sqlite3 ~/.local/share/acp-relay/relay-messages.db`

## Security

- Management API is localhost-only by default
- Use firewall to restrict HTTP/WebSocket APIs
- Rotate API keys regularly
- Monitor database size and rotate logs

## Troubleshooting

See [README.md Troubleshooting](../README.md#troubleshooting) section.
