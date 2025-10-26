# ACP Relay Web Interface Example

A simple web-based chat interface demonstrating how to interact with the ACP relay via WebSocket.

**Note:** This is a standalone example application, not part of the relay server itself. The relay only provides the HTTP and WebSocket APIs.

## Features

- **Create new sessions** - Start a new ACP agent session with optional working directory
- **Real-time communication** - WebSocket-based streaming for instant updates
- **Event display** - Shows all JSON-RPC messages (requests, responses, notifications, errors)
- **User input** - Send prompts to the active session
- **Auto-reconnect** - Automatically reconnects if the WebSocket connection drops

## Usage

### Starting the Relay

```bash
# From the project root
./relay serve
```

This will start:
- HTTP API on `http://localhost:23890` (REST-style session endpoints)
- WebSocket server on `ws://localhost:23891` (real-time communication)
- Management API on `http://127.0.0.1:23892` (health checks)

### Accessing the Web Interface

1. Open `examples/web/index.html` directly in your browser (as a `file://` URL)
   - Simply double-click the file, or
   - Run: `open examples/web/index.html` (macOS) or `xdg-open examples/web/index.html` (Linux)
2. Click "New Session" to create a session
3. Optionally specify a working directory before creating the session
4. Once connected, type your prompts in the text area and click "Send"

### Interface Elements

- **Status Indicator** - Green dot indicates WebSocket connection is active
- **Session ID** - Displays the current active session ID
- **Working Directory** - Optional path for the agent's working directory
- **Messages Area** - Shows all communication with color-coded message types:
  - **Teal** - Requests you send
  - **Blue** - Responses from the agent
  - **Yellow** - Notifications from the agent
  - **Red** - Errors
- **Input Area** - Text area to type your prompts

### Message Types

The interface displays all JSON-RPC 2.0 messages:

- **Requests** - Messages you send to the relay (method + params + id)
- **Responses** - Final responses from the agent (result + id)
- **Notifications** - Streaming updates from the agent (method + params, no id)
- **Errors** - Error messages with details (error + id)

## Technical Details

- Built with vanilla HTML/CSS/JavaScript (no dependencies)
- Uses WebSocket API for bidirectional communication
- Follows JSON-RPC 2.0 protocol
- Auto-scrolls to show latest messages
- Automatically increments request IDs

## Troubleshooting

### "Disconnected from relay"
- Ensure the relay is running (`./relay serve`)
- Check that port 23891 is not blocked by firewall
- WebSocket will auto-reconnect every 2 seconds

### "No active session"
- Click "New Session" before sending prompts
- Check the Session ID field to confirm a session is active
