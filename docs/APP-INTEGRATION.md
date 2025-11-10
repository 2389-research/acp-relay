# Building Apps with ACP-Relay

A practical guide for developers building applications that interact with acp-relay.

## Table of Contents

- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
- [HTTP API Integration](#http-api-integration)
- [WebSocket Integration](#websocket-integration)
- [Common Patterns](#common-patterns)
- [Error Handling](#error-handling)
- [Best Practices](#best-practices)
- [Example Applications](#example-applications)

---

## Quick Start

**What is acp-relay?**

ACP-relay is a relay server that translates HTTP/WebSocket requests into Agent Client Protocol (ACP) messages. It manages AI agent sessions and provides three APIs:

- **HTTP API** (`:8080`) - Simple request/response
- **WebSocket API** (`:8081`) - Real-time bidirectional streaming
- **Management API** (`:8082`) - Health checks and monitoring

**5-Minute Integration:**

```javascript
// 1. Create a session
const response = await fetch('http://localhost:8080/session/new', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    jsonrpc: '2.0',
    method: 'session/new',
    params: { workingDirectory: '/tmp/my-workspace' },
    id: 1
  })
});

const { result } = await response.json();
const sessionId = result.sessionId;

// 2. Send a prompt
const promptResponse = await fetch('http://localhost:8080/session/prompt', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    jsonrpc: '2.0',
    method: 'session/prompt',
    params: {
      sessionId,
      content: [{ type: 'text', text: 'Hello, agent!' }]
    },
    id: 2
  })
});

const agentResponse = await promptResponse.json();
console.log(agentResponse.result);
```

---

## Core Concepts

### Sessions

A **session** represents a persistent connection to an AI agent:

- Each session has a unique ID (e.g., `sess_abc12345`)
- Sessions have isolated working directories
- Sessions persist across client disconnects (for resumption)
- One agent process per session

### JSON-RPC 2.0

All communication uses JSON-RPC 2.0 format:

```json
{
  "jsonrpc": "2.0",
  "method": "method/name",
  "params": { /* method parameters */ },
  "id": 1
}
```

### Message Flow

```
Your App → HTTP/WebSocket → ACP-Relay → Agent Process
Your App ← HTTP/WebSocket ← ACP-Relay ← Agent Process
```

---

## HTTP API Integration

**Use HTTP when:**
- You need simple request/response
- Your app is stateless
- You're building REST-style integrations
- You want easier load balancing

### Creating Sessions

```python
import requests

def create_session(workspace_dir):
    response = requests.post(
        'http://localhost:8080/session/new',
        json={
            'jsonrpc': '2.0',
            'method': 'session/new',
            'params': {'workingDirectory': workspace_dir},
            'id': 1
        }
    )
    result = response.json()
    return result['result']['sessionId']

# Usage
session_id = create_session('/tmp/my-project')
print(f"Created session: {session_id}")
```

### Sending Prompts

```python
def send_prompt(session_id, message):
    response = requests.post(
        'http://localhost:8080/session/prompt',
        json={
            'jsonrpc': '2.0',
            'method': 'session/prompt',
            'params': {
                'sessionId': session_id,
                'content': [{'type': 'text', 'text': message}]
            },
            'id': 2
        }
    )
    return response.json()['result']

# Usage
result = send_prompt(session_id, "List files in current directory")
print(result['content'])
```

### Complete HTTP Example (Python)

```python
import requests
import json

class ACPRelayClient:
    def __init__(self, base_url='http://localhost:8080'):
        self.base_url = base_url
        self.session_id = None
        self.request_id = 0

    def _next_id(self):
        self.request_id += 1
        return self.request_id

    def create_session(self, working_dir):
        """Create a new agent session."""
        response = requests.post(
            f'{self.base_url}/session/new',
            json={
                'jsonrpc': '2.0',
                'method': 'session/new',
                'params': {'workingDirectory': working_dir},
                'id': self._next_id()
            }
        )
        data = response.json()
        if 'error' in data:
            raise Exception(f"Session creation failed: {data['error']}")

        self.session_id = data['result']['sessionId']
        return self.session_id

    def send_message(self, text):
        """Send a message to the agent."""
        if not self.session_id:
            raise Exception("No active session. Call create_session() first.")

        response = requests.post(
            f'{self.base_url}/session/prompt',
            json={
                'jsonrpc': '2.0',
                'method': 'session/prompt',
                'params': {
                    'sessionId': self.session_id,
                    'content': [{'type': 'text', 'text': text}]
                },
                'id': self._next_id()
            }
        )

        data = response.json()
        if 'error' in data:
            raise Exception(f"Prompt failed: {data['error']}")

        return data['result']

# Usage
client = ACPRelayClient()
client.create_session('/tmp/workspace')

response = client.send_message("What files are in the current directory?")
print(response['content'])
```

---

## WebSocket Integration

**Use WebSocket when:**
- You need real-time bidirectional communication
- You're building interactive chat interfaces
- You need streaming responses
- You want to resume sessions after disconnect

### Connecting and Creating Session

```javascript
// JavaScript/Node.js with 'ws' library
const WebSocket = require('ws');

const ws = new WebSocket('ws://localhost:8081');

ws.on('open', () => {
  // Create session on connect
  ws.send(JSON.stringify({
    jsonrpc: '2.0',
    method: 'session/new',
    params: { workingDirectory: '/tmp/workspace' },
    id: 1
  }));
});

ws.on('message', (data) => {
  const message = JSON.parse(data);

  if (message.id === 1) {
    // Session created
    const sessionId = message.result.sessionId;
    const clientId = message.result.clientId;
    console.log(`Session: ${sessionId}, Client: ${clientId}`);

    // Now send a prompt
    ws.send(JSON.stringify({
      jsonrpc: '2.0',
      method: 'session/prompt',
      params: {
        sessionId,
        content: [{ type: 'text', text: 'Hello!' }]
      },
      id: 2
    }));
  } else {
    // Agent response
    console.log('Agent:', message.result);
  }
});

ws.on('error', (error) => {
  console.error('WebSocket error:', error);
});

ws.on('close', () => {
  console.log('Connection closed');
});
```

### Session Resumption

```javascript
// Resuming an existing session after disconnect
const ws = new WebSocket('ws://localhost:8081');

ws.on('open', () => {
  // Resume existing session instead of creating new one
  ws.send(JSON.stringify({
    jsonrpc: '2.0',
    method: 'session/resume',
    params: { sessionId: 'sess_abc12345' }, // Your existing session ID
    id: 1
  }));
});

ws.on('message', (data) => {
  const message = JSON.parse(data);

  if (message.id === 1) {
    console.log('Session resumed:', message.result);
    // Continue sending prompts...
  }
});
```

### Complete WebSocket Example (Python)

```python
import asyncio
import json
import websockets

class ACPWebSocketClient:
    def __init__(self, url='ws://localhost:8081'):
        self.url = url
        self.ws = None
        self.session_id = None
        self.request_id = 0
        self.pending_responses = {}

    def _next_id(self):
        self.request_id += 1
        return self.request_id

    async def connect(self):
        """Connect to WebSocket server."""
        self.ws = await websockets.connect(self.url)

    async def create_session(self, working_dir):
        """Create a new session."""
        request_id = self._next_id()

        await self.ws.send(json.dumps({
            'jsonrpc': '2.0',
            'method': 'session/new',
            'params': {'workingDirectory': working_dir},
            'id': request_id
        }))

        # Wait for response
        response = json.loads(await self.ws.recv())

        if 'error' in response:
            raise Exception(f"Session creation failed: {response['error']}")

        self.session_id = response['result']['sessionId']
        return self.session_id

    async def send_message(self, text):
        """Send a message to the agent."""
        if not self.session_id:
            raise Exception("No active session")

        request_id = self._next_id()

        await self.ws.send(json.dumps({
            'jsonrpc': '2.0',
            'method': 'session/prompt',
            'params': {
                'sessionId': self.session_id,
                'content': [{'type': 'text', 'text': text}]
            },
            'id': request_id
        }))

        # Wait for response
        response = json.loads(await self.ws.recv())

        if 'error' in response:
            raise Exception(f"Prompt failed: {response['error']}")

        return response['result']

    async def close(self):
        """Close the WebSocket connection."""
        if self.ws:
            await self.ws.close()

# Usage
async def main():
    client = ACPWebSocketClient()

    try:
        await client.connect()

        session_id = await client.create_session('/tmp/workspace')
        print(f"Created session: {session_id}")

        # Interactive loop
        while True:
            user_input = input("You: ")
            if user_input.lower() == 'quit':
                break

            response = await client.send_message(user_input)
            print(f"Agent: {response['content']}")

    finally:
        await client.close()

# Run
asyncio.run(main())
```

---

## Common Patterns

### Pattern 1: Stateless HTTP Bot

For simple bots that don't need session persistence:

```javascript
async function askAgent(question) {
  // Create session
  const session = await fetch('http://localhost:8080/session/new', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      jsonrpc: '2.0',
      method: 'session/new',
      params: { workingDirectory: '/tmp' },
      id: 1
    })
  });

  const { result: { sessionId } } = await session.json();

  // Ask question
  const response = await fetch('http://localhost:8080/session/prompt', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      jsonrpc: '2.0',
      method: 'session/prompt',
      params: {
        sessionId,
        content: [{ type: 'text', text: question }]
      },
      id: 2
    })
  });

  const { result } = await response.json();
  return result.content[0].text;
}

// Usage
const answer = await askAgent("What is 2+2?");
console.log(answer);
```

### Pattern 2: Multi-Client WebSocket Chat

Multiple clients can connect to the same session:

```python
# Client 1 - Creates session
client1 = ACPWebSocketClient()
await client1.connect()
session_id = await client1.create_session('/tmp/shared')

# Client 2 - Resumes same session
client2 = ACPWebSocketClient()
await client2.connect()
await client2.resume_session(session_id)

# Both clients can send/receive messages
await client1.send_message("Client 1 message")
await client2.send_message("Client 2 message")
```

### Pattern 3: Session Pool

Maintain a pool of sessions for load balancing:

```python
class SessionPool:
    def __init__(self, relay_url, pool_size=5):
        self.relay_url = relay_url
        self.sessions = []
        self.current_index = 0

        # Pre-create sessions
        for _ in range(pool_size):
            session_id = self.create_session()
            self.sessions.append(session_id)

    def create_session(self):
        response = requests.post(
            f'{self.relay_url}/session/new',
            json={
                'jsonrpc': '2.0',
                'method': 'session/new',
                'params': {'workingDirectory': '/tmp'},
                'id': 1
            }
        )
        return response.json()['result']['sessionId']

    def get_session(self):
        """Round-robin session selection."""
        session_id = self.sessions[self.current_index]
        self.current_index = (self.current_index + 1) % len(self.sessions)
        return session_id

    def send_prompt(self, text):
        session_id = self.get_session()
        # Send prompt to selected session...
```

### Pattern 4: Streaming Responses

Handle streaming agent responses:

```javascript
const ws = new WebSocket('ws://localhost:8081');
let responseBuffer = '';

ws.on('message', (data) => {
  const message = JSON.parse(data);

  // Check if this is a streaming response
  if (message.result && message.result.content) {
    for (const item of message.result.content) {
      if (item.type === 'text') {
        // Append to buffer
        responseBuffer += item.text;

        // Update UI in real-time
        updateUI(responseBuffer);
      }
    }

    // Check if stream is complete
    if (message.result.stopReason === 'endTurn') {
      console.log('Complete response:', responseBuffer);
      responseBuffer = '';
    }
  }
});
```

---

## Error Handling

ACP-relay provides LLM-optimized error messages:

```javascript
async function handleResponse(response) {
  const data = await response.json();

  if (data.error) {
    const error = data.error.data;

    console.error('Error:', data.error.message);
    console.error('Type:', error.error_type);
    console.error('Explanation:', error.explanation);
    console.error('Possible causes:', error.possible_causes);
    console.error('Suggested actions:', error.suggested_actions);

    // Check if recoverable
    if (error.recoverable) {
      // Implement retry logic
      return await retryRequest();
    } else {
      throw new Error(data.error.message);
    }
  }

  return data.result;
}
```

### Common Errors

**session_not_found:**
```json
{
  "error_type": "session_not_found",
  "explanation": "The session doesn't exist",
  "possible_causes": ["Session expired", "Invalid session ID"],
  "suggested_actions": ["Create new session", "Check session ID"],
  "recoverable": true
}
```

**agent_connection_timeout:**
```json
{
  "error_type": "agent_connection_timeout",
  "explanation": "Agent failed to start",
  "possible_causes": ["Agent binary missing", "Environment variables not set"],
  "suggested_actions": ["Check agent path", "Verify environment"],
  "recoverable": true
}
```

---

## Best Practices

### 1. Session Management

**Do:**
- Store session IDs securely (they grant access to agent)
- Reuse sessions for multi-turn conversations
- Handle session expiration gracefully
- Close sessions when done

**Don't:**
- Log session IDs in plain text
- Share session IDs between users
- Create new sessions for every message

### 2. Error Handling

**Do:**
- Parse the `error.data` field for actionable information
- Check `error.recoverable` before retrying
- Implement exponential backoff for retries
- Log relevant error state for debugging

**Don't:**
- Ignore error details
- Retry indefinitely on non-recoverable errors
- Show raw error messages to end users

### 3. Connection Management

**HTTP:**
- Use connection pooling
- Set appropriate timeouts (30s default)
- Handle network errors gracefully

**WebSocket:**
- Implement reconnection logic
- Resume sessions after disconnect
- Handle connection state changes
- Use heartbeat/ping to detect dead connections

### 4. Security

**Do:**
- Use HTTPS/WSS in production
- Validate working directory paths
- Implement authentication (relay doesn't include it)
- Rate limit requests per user
- Sanitize user inputs

**Don't:**
- Expose management API (`:8082`) externally
- Trust user-provided session IDs without validation
- Allow arbitrary file system access

### 5. Performance

**Do:**
- Use WebSocket for real-time interactions
- Use HTTP for simple request/response
- Pool sessions for high-throughput applications
- Monitor session count and resource usage

**Don't:**
- Create new session for every request
- Keep idle sessions open indefinitely
- Exceed configured `max_concurrent_sessions`

---

## Example Applications

### 1. CLI Chat Bot

```bash
#!/bin/bash
# Simple chat bot using curl

SESSION_ID=$(curl -s -X POST http://localhost:8080/session/new \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"session/new","params":{"workingDirectory":"/tmp"},"id":1}' \
  | jq -r '.result.sessionId')

echo "Session: $SESSION_ID"
echo "Type 'quit' to exit"

while true; do
  read -p "You: " input

  if [ "$input" = "quit" ]; then
    break
  fi

  response=$(curl -s -X POST http://localhost:8080/session/prompt \
    -H "Content-Type: application/json" \
    -d "{\"jsonrpc\":\"2.0\",\"method\":\"session/prompt\",\"params\":{\"sessionId\":\"$SESSION_ID\",\"content\":[{\"type\":\"text\",\"text\":\"$input\"}]},\"id\":2}")

  echo "Agent:" $(echo $response | jq -r '.result.content[0].text')
done
```

### 2. Web Chat Interface (React)

See `examples/web/` for complete implementation with:
- Real-time WebSocket connection
- Message history
- Session management
- Error handling
- Responsive UI

### 3. Slack Bot Integration

```javascript
const { App } = require('@slack/bolt');
const ACPRelayClient = require('./acp-client');

const app = new App({
  token: process.env.SLACK_BOT_TOKEN,
  signingSecret: process.env.SLACK_SIGNING_SECRET
});

const relay = new ACPRelayClient('http://localhost:8080');

// Map Slack users to sessions
const userSessions = new Map();

app.message(async ({ message, say }) => {
  const userId = message.user;

  // Get or create session for user
  if (!userSessions.has(userId)) {
    const sessionId = await relay.create_session('/tmp/slack-' + userId);
    userSessions.set(userId, sessionId);
  }

  // Send message to agent
  const response = await relay.send_prompt(
    userSessions.get(userId),
    message.text
  );

  // Reply in Slack
  await say(response.content[0].text);
});

app.start(process.env.PORT || 3000);
```

---

## Testing Your Integration

### Health Check

Verify relay is running:

```bash
curl http://localhost:8082/api/health
# Expected: {"status":"healthy","agent_command":"..."}
```

### Session Lifecycle Test

```bash
# 1. Create session
SESSION_ID=$(curl -s -X POST http://localhost:8080/session/new \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"session/new","params":{"workingDirectory":"/tmp"},"id":1}' \
  | jq -r '.result.sessionId')

echo "Created: $SESSION_ID"

# 2. Send prompt
curl -X POST http://localhost:8080/session/prompt \
  -H "Content-Type: application/json" \
  -d "{\"jsonrpc\":\"2.0\",\"method\":\"session/prompt\",\"params\":{\"sessionId\":\"$SESSION_ID\",\"content\":[{\"type\":\"text\",\"text\":\"hello\"}]},\"id\":2}"

# 3. Send another prompt (reusing session)
curl -X POST http://localhost:8080/session/prompt \
  -H "Content-Type: application/json" \
  -d "{\"jsonrpc\":\"2.0\",\"method\":\"session/prompt\",\"params\":{\"sessionId\":\"$SESSION_ID\",\"content\":[{\"type\":\"text\",\"text\":\"goodbye\"}]},\"id\":3}"
```

---

## Next Steps

- **API Reference**: See [api.md](api.md) for complete API specification
- **Deployment**: See [DEPLOYMENT.md](DEPLOYMENT.md) for production setup
- **Examples**: Check `examples/` directory for complete applications
- **Troubleshooting**: See [README.md](../README.md#troubleshooting)

## Support

- GitHub Issues: https://github.com/harper/acp-relay/issues
- Documentation: https://github.com/harper/acp-relay/tree/main/docs
