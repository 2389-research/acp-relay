# Permission Handling in ACP

## Overview

When the agent needs to perform an action that requires permission (like writing a file), it sends a `session/request_permission` JSON-RPC request to the client.

## Request Format

The agent sends:

```json
{
  "jsonrpc": "2.0",
  "id": 0,
  "method": "session/request_permission",
  "params": {
    "sessionId": "...",
    "toolCall": {
      "toolCallId": "toolu_...",
      "rawInput": {
        "file_path": "/tmp/test.txt",
        "content": "..."
      }
    },
    "options": [
      {
        "kind": "allow_always",
        "name": "Always Allow",
        "optionId": "allow_always"
      },
      {
        "kind": "allow_once",
        "name": "Allow",
        "optionId": "allow"
      },
      {
        "kind": "reject_once",
        "name": "Reject",
        "optionId": "reject"
      }
    ]
  }
}
```

## Response Format

The client must respond with:

```json
{
  "jsonrpc": "2.0",
  "id": 0,
  "result": {
    "outcome": {
      "outcome": "selected",
      "optionId": "allow"
    }
  }
}
```

### Key Points

1. **Nested `outcome` structure**: The response MUST have an `outcome` object containing two fields
2. **`outcome: "selected"`**: Always set to `"selected"` to indicate an option was chosen
3. **`optionId`**: The ID of the selected option from the request (`"allow"`, `"allow_always"`, or `"reject"`)

## Common Mistakes

❌ **Wrong**: Just sending the optionId directly
```json
{
  "result": {
    "optionId": "allow"
  }
}
```

❌ **Wrong**: Using `behavior` and `updatedInput` (that's for internal SDK use)
```json
{
  "result": {
    "behavior": "allow",
    "updatedInput": {...}
  }
}
```

✅ **Correct**: Nested outcome structure
```json
{
  "result": {
    "outcome": {
      "outcome": "selected",
      "optionId": "allow"
    }
  }
}
```

## Implementation Example

See `examples/permission_test.py` for a working example:

```python
response = {
    "jsonrpc": "2.0",
    "id": msg_id,
    "result": {
        "outcome": {
            "outcome": "selected",
            "optionId": "allow"  # or "allow_always" or "reject"
        }
    }
}
await websocket.send(json.dumps(response))
```

## References

- [ACP TypeScript SDK Example](https://github.com/agentclientprotocol/typescript-sdk/blob/main/src/examples/client.ts)
- [ACP Rust SDK](https://github.com/agentclientprotocol/rust-sdk)
