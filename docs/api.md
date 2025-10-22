# ACP Relay API Documentation

Complete API reference for the ACP Relay Server, covering all three APIs: HTTP, WebSocket, and Management.

## Table of Contents

- [HTTP API (Port 8080)](#http-api-port-8080)
- [WebSocket API (Port 8081)](#websocket-api-port-8081)
- [Management API (Port 8082)](#management-api-port-8082)
- [Error Handling](#error-handling)
- [Examples](#examples)

## HTTP API (Port 8080)

The HTTP API provides request/response endpoints for creating sessions and sending prompts to agents.

### Base URL

```
http://localhost:8080
```

### Content Type

All requests and responses use `application/json`.

### Endpoints

#### POST /session/new

Create a new agent session with an isolated working directory.

**Request:**

```json
{
  "jsonrpc": "2.0",
  "method": "session/new",
  "params": {
    "workingDirectory": "/path/to/workspace"
  },
  "id": 1
}
```

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| workingDirectory | string | Yes | Absolute path to the working directory for the agent process |

**Success Response:**

```json
{
  "jsonrpc": "2.0",
  "result": {
    "sessionId": "sess_abc12345"
  },
  "id": 1
}
```

**Error Response:**

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32000,
    "message": "I attempted to spawn the ACP agent process but it failed...",
    "data": {
      "error_type": "agent_connection_timeout",
      "explanation": "The relay server tried to start the agent subprocess...",
      "possible_causes": [
        "The agent command path is incorrect or the binary doesn't exist",
        "The agent requires environment variables that aren't set"
      ],
      "suggested_actions": [
        "Check that the agent command exists: ls -l /path/to/agent",
        "Verify the agent can run manually: /path/to/agent --help"
      ],
      "relevant_state": {
        "agent_url": "/path/to/workspace",
        "attempts": 1,
        "timeout_ms": 10000
      },
      "recoverable": true
    }
  },
  "id": 1
}
```

**cURL Example:**

```bash
curl -X POST http://localhost:8080/session/new \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "session/new",
    "params": {
      "workingDirectory": "/tmp/my-workspace"
    },
    "id": 1
  }'
```

---

#### POST /session/prompt

Send a prompt to an existing agent session and receive a response.

**Request:**

```json
{
  "jsonrpc": "2.0",
  "method": "session/prompt",
  "params": {
    "sessionId": "sess_abc12345",
    "content": [
      {
        "type": "text",
        "text": "List all files in the current directory"
      }
    ]
  },
  "id": 2
}
```

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| sessionId | string | Yes | Session ID returned from session/new |
| content | array | Yes | Array of content items (text, image, etc.) per ACP protocol |

**Success Response:**

The response is forwarded directly from the agent and follows the ACP protocol format:

```json
{
  "jsonrpc": "2.0",
  "result": {
    "content": [
      {
        "type": "text",
        "text": "file1.txt\nfile2.txt\nREADME.md"
      }
    ],
    "stopReason": "endTurn"
  },
  "id": 2
}
```

**Error Response (Session Not Found):**

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32000,
    "message": "The session 'sess_abc12345' does not exist...",
    "data": {
      "error_type": "session_not_found",
      "explanation": "The relay server doesn't have an active session with this ID.",
      "possible_causes": [
        "The session was never created (missing session/new call)",
        "The session ID was mistyped or corrupted",
        "The session expired due to inactivity timeout",
        "The agent process crashed and the session was cleaned up"
      ],
      "suggested_actions": [
        "Create a new session using session/new",
        "Verify you're using the correct session ID from the session/new response",
        "Check if the agent process is still running"
      ],
      "relevant_state": {
        "session_id": "sess_abc12345"
      },
      "recoverable": true
    }
  },
  "id": 2
}
```

**Timeout Behavior:**

- The relay waits up to 30 seconds for the agent to respond
- If the agent doesn't respond within 30 seconds, an error is returned
- The session remains active after a timeout

**cURL Example:**

```bash
curl -X POST http://localhost:8080/session/prompt \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "session/prompt",
    "params": {
      "sessionId": "sess_abc12345",
      "content": [
        {
          "type": "text",
          "text": "What is the current directory?"
        }
      ]
    },
    "id": 2
  }'
```

---

## WebSocket API (Port 8081)

The WebSocket API provides bidirectional, streaming communication for real-time agent interactions.

### Connection URL

```
ws://localhost:8081
```

### Protocol

The WebSocket connection uses JSON-RPC 2.0 messages, sent as text frames (not binary).

### Connection Flow

1. Client connects to `ws://localhost:8081`
2. Client sends `session/new` message
3. Server responds with session ID
4. Client sends additional JSON-RPC messages
5. Server forwards messages to agent and streams responses back
6. When connection closes, the session is automatically terminated

### Messages

#### session/new

Create a session over the WebSocket connection.

**Client → Server:**

```json
{
  "jsonrpc": "2.0",
  "method": "session/new",
  "params": {
    "workingDirectory": "/tmp/workspace"
  },
  "id": 1
}
```

**Server → Client:**

```json
{
  "jsonrpc": "2.0",
  "result": {
    "sessionId": "sess_abc12345"
  },
  "id": 1
}
```

#### Other Methods

After creating a session, all other JSON-RPC messages are forwarded directly to the agent.

**Client → Server:**

```json
{
  "jsonrpc": "2.0",
  "method": "session/prompt",
  "params": {
    "sessionId": "sess_abc12345",
    "content": [
      {"type": "text", "text": "Hello"}
    ]
  },
  "id": 2
}
```

**Server → Client (Agent Response):**

```json
{
  "jsonrpc": "2.0",
  "result": {
    "content": [
      {"type": "text", "text": "Hello! How can I help you?"}
    ],
    "stopReason": "endTurn"
  },
  "id": 2
}
```

### Streaming Responses

If the agent sends multiple messages (e.g., streaming tokens), each message is forwarded to the client as a separate WebSocket frame.

### Error Handling

Errors are sent as JSON-RPC error responses:

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32700,
    "message": "Invalid JSON in request",
    "data": {
      "error_type": "parse_error",
      "explanation": "The message could not be parsed as valid JSON",
      "details": "unexpected token at position 15"
    }
  },
  "id": null
}
```

### JavaScript Example

```javascript
const ws = new WebSocket('ws://localhost:8081');

ws.onopen = () => {
  // Create a session
  ws.send(JSON.stringify({
    jsonrpc: "2.0",
    method: "session/new",
    params: {
      workingDirectory: "/tmp/workspace"
    },
    id: 1
  }));
};

ws.onmessage = (event) => {
  const response = JSON.parse(event.data);
  console.log('Received:', response);

  if (response.id === 1 && response.result) {
    // Got session ID, now send a prompt
    const sessionId = response.result.sessionId;

    ws.send(JSON.stringify({
      jsonrpc: "2.0",
      method: "session/prompt",
      params: {
        sessionId: sessionId,
        content: [
          { type: "text", text: "Hello, agent!" }
        ]
      },
      id: 2
    }));
  }
};

ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};

ws.onclose = () => {
  console.log('WebSocket connection closed');
};
```

### Python Example (using websockets library)

```python
import asyncio
import json
import websockets

async def interact_with_agent():
    uri = "ws://localhost:8081"

    async with websockets.connect(uri) as websocket:
        # Create session
        await websocket.send(json.dumps({
            "jsonrpc": "2.0",
            "method": "session/new",
            "params": {
                "workingDirectory": "/tmp/workspace"
            },
            "id": 1
        }))

        # Receive session ID
        response = json.loads(await websocket.recv())
        session_id = response["result"]["sessionId"]
        print(f"Session created: {session_id}")

        # Send prompt
        await websocket.send(json.dumps({
            "jsonrpc": "2.0",
            "method": "session/prompt",
            "params": {
                "sessionId": session_id,
                "content": [
                    {"type": "text", "text": "Hello, agent!"}
                ]
            },
            "id": 2
        }))

        # Receive response
        response = json.loads(await websocket.recv())
        print("Agent response:", response)

asyncio.run(interact_with_agent())
```

---

## Management API (Port 8082)

The Management API provides health checks and runtime configuration. **This API is bound to localhost (127.0.0.1) only** for security.

### Base URL

```
http://localhost:8082
```

### Endpoints

#### GET /api/health

Get server health status and basic information.

**Request:**

```bash
curl http://localhost:8082/api/health
```

**Response:**

```json
{
  "status": "healthy",
  "agent_command": "/usr/local/bin/acp-agent"
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| status | string | Health status (always "healthy" if server is responding) |
| agent_command | string | Path to the agent binary configured |

**Use Cases:**

- Load balancer health checks
- Monitoring systems
- Startup verification

---

#### GET /api/config

Get the current server configuration (read-only).

**Request:**

```bash
curl http://localhost:8082/api/config
```

**Response:**

```json
{
  "server": {
    "http_port": 8080,
    "http_host": "0.0.0.0",
    "websocket_port": 8081,
    "websocket_host": "0.0.0.0",
    "management_port": 8082,
    "management_host": "127.0.0.1"
  },
  "agent": {
    "command": "/usr/local/bin/acp-agent",
    "args": [],
    "env": {},
    "startup_timeout_seconds": 10,
    "max_concurrent_sessions": 100
  }
}
```

**Use Cases:**

- Debugging configuration issues
- Verifying configuration changes
- Auditing server settings

---

## Error Handling

The ACP Relay Server uses JSON-RPC 2.0 error format with LLM-optimized extensions.

### Standard JSON-RPC Error Codes

| Code | Message | Description |
|------|---------|-------------|
| -32700 | Parse error | Invalid JSON received |
| -32600 | Invalid request | Request does not conform to JSON-RPC spec |
| -32601 | Method not found | Requested method does not exist |
| -32602 | Invalid params | Invalid method parameters |
| -32603 | Internal error | Internal JSON-RPC error |
| -32000 | Server error | Generic server error |

### LLM-Optimized Error Format

Errors include an additional `data` field with detailed information:

```json
{
  "error_type": "agent_connection_timeout",
  "explanation": "Human-readable explanation of the error",
  "possible_causes": [
    "Reason 1",
    "Reason 2"
  ],
  "suggested_actions": [
    "Action 1",
    "Action 2"
  ],
  "relevant_state": {
    "key": "value"
  },
  "recoverable": true,
  "details": "Additional technical details"
}
```

### Common Error Types

#### agent_connection_timeout

The agent process failed to start within the configured timeout.

**Possible Causes:**
- Agent binary doesn't exist or is not executable
- Agent requires missing environment variables
- Agent crashes immediately on startup

**Suggested Actions:**
- Verify agent binary exists: `ls -l /path/to/agent`
- Test agent manually: `/path/to/agent --help`
- Check stderr logs for agent errors
- Verify environment variables in config.yaml

#### session_not_found

The requested session ID does not exist.

**Possible Causes:**
- Session was never created
- Session ID was mistyped
- Session expired or was closed
- Agent process crashed

**Suggested Actions:**
- Create a new session with `session/new`
- Verify the session ID is correct
- Check agent process status

#### parse_error

The JSON in the request could not be parsed.

**Possible Causes:**
- Malformed JSON
- Missing quotes or brackets
- Invalid escape sequences

**Suggested Actions:**
- Validate JSON with a linter
- Check for special characters that need escaping
- Verify Content-Type header is set correctly

#### invalid_params

The request parameters are invalid or missing.

**Possible Causes:**
- Required parameter is missing
- Parameter has wrong type
- Parameter value is invalid

**Suggested Actions:**
- Review API documentation for required parameters
- Check parameter types match specification
- Validate parameter values are within acceptable ranges

---

## Examples

### Complete HTTP Flow

```bash
# 1. Create a session
SESSION_RESPONSE=$(curl -X POST http://localhost:8080/session/new \
  -H "Content-Type: application/json" \
  -s -d '{
    "jsonrpc": "2.0",
    "method": "session/new",
    "params": {
      "workingDirectory": "/tmp/workspace"
    },
    "id": 1
  }')

echo "Session response: $SESSION_RESPONSE"

# 2. Extract session ID (using jq)
SESSION_ID=$(echo $SESSION_RESPONSE | jq -r '.result.sessionId')
echo "Session ID: $SESSION_ID"

# 3. Send a prompt
curl -X POST http://localhost:8080/session/prompt \
  -H "Content-Type: application/json" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"method\": \"session/prompt\",
    \"params\": {
      \"sessionId\": \"$SESSION_ID\",
      \"content\": [
        {
          \"type\": \"text\",
          \"text\": \"List files in the current directory\"
        }
      ]
    },
    \"id\": 2
  }"
```

### Complete WebSocket Flow (Go)

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"

    "github.com/gorilla/websocket"
)

func main() {
    // Connect to WebSocket server
    conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8081", nil)
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    // Send session/new
    sessionReq := map[string]interface{}{
        "jsonrpc": "2.0",
        "method":  "session/new",
        "params": map[string]interface{}{
            "workingDirectory": "/tmp/workspace",
        },
        "id": 1,
    }

    if err := conn.WriteJSON(sessionReq); err != nil {
        log.Fatal(err)
    }

    // Read session response
    var sessionResp map[string]interface{}
    if err := conn.ReadJSON(&sessionResp); err != nil {
        log.Fatal(err)
    }

    result := sessionResp["result"].(map[string]interface{})
    sessionID := result["sessionId"].(string)
    fmt.Printf("Session ID: %s\n", sessionID)

    // Send prompt
    promptReq := map[string]interface{}{
        "jsonrpc": "2.0",
        "method":  "session/prompt",
        "params": map[string]interface{}{
            "sessionId": sessionID,
            "content": []interface{}{
                map[string]interface{}{
                    "type": "text",
                    "text": "Hello, agent!",
                },
            },
        },
        "id": 2,
    }

    if err := conn.WriteJSON(promptReq); err != nil {
        log.Fatal(err)
    }

    // Read agent response
    var agentResp map[string]interface{}
    if err := conn.ReadJSON(&agentResp); err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Agent response: %v\n", agentResp)
}
```

### Monitoring Script

```bash
#!/bin/bash
# monitor-relay.sh - Check relay server health

check_health() {
    response=$(curl -s http://localhost:8082/api/health)
    status=$(echo $response | jq -r '.status')

    if [ "$status" = "healthy" ]; then
        echo "✓ Relay server is healthy"
        return 0
    else
        echo "✗ Relay server is unhealthy"
        return 1
    fi
}

check_http() {
    response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/session/new)

    if [ "$response" = "405" ]; then
        echo "✓ HTTP API is responding (405 = GET not allowed, expected)"
        return 0
    else
        echo "✗ HTTP API is not responding properly"
        return 1
    fi
}

check_websocket() {
    # Note: Requires websocat (https://github.com/vi/websocat)
    echo '{"jsonrpc":"2.0","method":"session/new","params":{"workingDirectory":"/tmp"},"id":1}' | \
        timeout 2s websocat ws://localhost:8081 > /dev/null 2>&1

    if [ $? -eq 0 ]; then
        echo "✓ WebSocket API is responding"
        return 0
    else
        echo "✗ WebSocket API is not responding"
        return 1
    fi
}

echo "Checking ACP Relay Server..."
check_health
check_http

echo ""
echo "All checks complete!"
```

---

## Best Practices

### Session Management

1. **Always create a session before sending prompts**
   - The relay does not maintain session state across connections
   - Each WebSocket connection requires a `session/new` call

2. **Store session IDs securely**
   - Session IDs are sensitive and grant access to the agent process
   - Don't log session IDs in plain text

3. **Close sessions when done**
   - WebSocket: close the connection
   - HTTP: sessions are eventually cleaned up by timeout (future feature)

### Error Handling

1. **Parse the `data` field in errors**
   - Contains actionable information for debugging
   - `suggested_actions` provides concrete steps to resolve issues

2. **Check `recoverable` flag**
   - If `true`, you can retry the operation
   - If `false`, the error requires manual intervention

3. **Log `relevant_state` for debugging**
   - Contains contextual information about the error
   - Helps trace issues across distributed systems

### Performance

1. **Use WebSocket for interactive sessions**
   - WebSocket has lower latency than HTTP
   - Streaming responses are more efficient

2. **Use HTTP for stateless, one-off requests**
   - HTTP is simpler for automation and scripts
   - Better for load balancing and horizontal scaling

3. **Configure appropriate timeouts**
   - Default 30s timeout may be too short for complex tasks
   - Consider the agent's expected response time

### Security

1. **Never expose the Management API externally**
   - It's bound to localhost by default for a reason
   - Contains sensitive configuration information

2. **Validate working directories**
   - Ensure agents can't access sensitive file system areas
   - Consider chroot/containerization for isolation

3. **Implement authentication**
   - The relay does not include built-in authentication
   - Add authentication middleware for production use

---

## Changelog

### Version 1.0.0

- Initial release
- HTTP API with session/new and session/prompt
- WebSocket API with bidirectional streaming
- Management API with health and config endpoints
- LLM-optimized error messages
- Process-per-session isolation
