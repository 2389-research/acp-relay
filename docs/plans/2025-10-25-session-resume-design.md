# Session Resume Design - Multi-Client Connection Manager

**Date:** 2025-10-25
**Status:** Approved
**Priority:** Critical

## Problem Statement

Current session resume implementation has critical issues:
1. **Goroutine leak** - Each resume spawns new goroutine, never cleaned up
2. **Channel blocking** - Buffer size 10, agent deadlocks if client disconnects
3. **Race conditions** - Multiple clients can resume same session, messages corrupted
4. **No connection tracking** - Session has no awareness of attached clients
5. **Message loss** - No buffering during disconnect periods

## Requirements

### Core Requirements
- **Absolute correctness** - Zero message loss, zero corruption
- **Multi-client support** - Multiple WebSockets can attach to same session
- **Unlimited buffering** - Buffer all messages during client disconnects
- **Independent flow control** - One slow client doesn't block others
- **No timeouts** - Sessions never auto-expire (explicit close only)

### Success Criteria
- ✅ Multiple clients can resume same session simultaneously
- ✅ All clients receive all messages (broadcast)
- ✅ Slow clients drop messages, don't block fast clients
- ✅ Client disconnect/reconnect loses zero messages
- ✅ No goroutine leaks after repeated resumes
- ✅ Sessions survive indefinitely until explicit close

## Architecture

### Connection Manager Pattern

```
Session
  └─ ConnectionManager
       ├─ connections: map[clientID]*ClientConnection
       ├─ Broadcaster goroutine (single, per session)
       └─ Methods: AttachClient(), DetachClient(), Broadcast()

ClientConnection
  ├─ id: string
  ├─ conn: *websocket.Conn
  ├─ buffer: [][]byte (unlimited)
  ├─ deliveryChan: chan struct{} (buffered signal)
  └─ Delivery goroutine (one per client)
```

**Key insight:** Keep existing `Session.FromAgent` channel as single source of truth. ConnectionManager subscribes and fans out to all clients.

## Component Design

### 1. Data Structures

```go
// internal/session/connection_manager.go

type ConnectionManager struct {
    mu          sync.RWMutex
    connections map[string]*ClientConnection
    session     *Session  // back-reference
}

type ClientConnection struct {
    id           string
    conn         *websocket.Conn
    buffer       [][]byte           // unlimited buffering
    deliveryChan chan struct{}      // signal channel (buffered 1)
    attached     time.Time
}
```

**Changes to Session:**
```go
type Session struct {
    // ... existing fields ...

    // NEW: Connection manager
    connMgr *ConnectionManager

    // KEEP: FromAgent channel (unchanged)
    FromAgent chan []byte
}
```

### 2. Message Flow

#### Broadcaster (Single Goroutine per Session)
```go
func (cm *ConnectionManager) StartBroadcaster() {
    go func() {
        for msg := range cm.session.FromAgent {
            cm.mu.Lock()

            // Append to ALL client buffers
            for _, client := range cm.connections {
                client.buffer = append(client.buffer, msg)

                // Signal delivery goroutine (non-blocking)
                select {
                case client.deliveryChan <- struct{}{}:
                default:
                }
            }

            cm.mu.Unlock()
        }
    }()
}
```

#### Delivery (One Goroutine per Client)
```go
func (cm *ConnectionManager) deliverPendingMessages(client *ClientConnection) {
    cm.mu.Lock()
    pending := client.buffer
    client.buffer = nil  // Clear buffer
    cm.mu.Unlock()

    for i, msg := range pending {
        if err := client.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
            // Client write failed - DROP remaining messages
            dropped := len(pending) - i
            log.Printf("Client %s write failed, dropping %d messages",
                client.id, dropped)

            // Auto-detach dead client
            cm.DetachClient(client.id)
            return
        }
    }
}
```

**Flow:**
1. Agent writes to `FromAgent`
2. Broadcaster appends to all client buffers (under lock)
3. Signals each client's delivery goroutine
4. Delivery goroutine drains buffer to WebSocket
5. If write fails → drop messages for that client only

### 3. Lifecycle Management

#### Attach Client
```go
func (cm *ConnectionManager) AttachClient(conn *websocket.Conn) string {
    cm.mu.Lock()
    defer cm.mu.Unlock()

    clientID := uuid.New().String()[:8]

    client := &ClientConnection{
        id:           clientID,
        conn:         conn,
        buffer:       make([][]byte, 0, 100),
        deliveryChan: make(chan struct{}, 1),
        attached:     time.Now(),
    }

    cm.connections[clientID] = client
    cm.startClientDelivery(client)

    log.Printf("Client %s attached (%d total)", clientID, len(cm.connections))
    return clientID
}
```

#### Detach Client
```go
func (cm *ConnectionManager) DetachClient(clientID string) {
    cm.mu.Lock()
    client, exists := cm.connections[clientID]
    if !exists {
        cm.mu.Unlock()
        return
    }

    delete(cm.connections, clientID)
    bufferedCount := len(client.buffer)
    cm.mu.Unlock()

    // Stop delivery goroutine
    close(client.deliveryChan)

    log.Printf("Client %s detached, dropped %d buffered messages",
        clientID, bufferedCount)

    // NO timeout! Session persists even with 0 clients
}
```

### 4. WebSocket Handler Changes

**Remove goroutine leak:**
```go
// OLD (DELETE):
fromAgent := make(chan []byte, 10)
go func() {
    for msg := range fromAgent {
        conn.WriteMessage(websocket.TextMessage, msg)
    }
}()

// NEW (REPLACE):
var currentClientID string
// ConnectionManager handles all delivery
```

**session/new:**
```go
case "session/new":
    sess, err := s.sessionMgr.CreateSession(ctx, params.WorkingDirectory)

    currentSession = sess
    currentClientID = sess.connMgr.AttachClient(conn)

    result := map[string]interface{}{
        "sessionId": sess.ID,
        "clientId":  currentClientID,
    }
    s.sendResponse(conn, result, req.ID)
```

**session/resume:**
```go
case "session/resume":
    sess, exists := s.sessionMgr.GetSession(params.SessionID)

    currentSession = sess
    currentClientID = sess.connMgr.AttachClient(conn)

    log.Printf("Client %s resumed session %s", currentClientID, sess.ID[:8])

    result := map[string]interface{}{
        "sessionId": sess.ID,
        "clientId":  currentClientID,
    }
    s.sendResponse(conn, result, req.ID)
```

**Cleanup on disconnect:**
```go
// At end of handleConnection():
if currentSession != nil && currentClientID != "" {
    currentSession.connMgr.DetachClient(currentClientID)
    log.Printf("Client %s disconnected, session remains active", currentClientID)
}
```

## Error Handling

### Slow/Dead Clients
- Write fails → auto-detach client, drop buffered messages
- Other clients unaffected (independent delivery)

### Agent Process Death
```go
// In session.StartStdioBridge() stdout reader:
// When scanner.Scan() returns (process exited):
s.connMgr.BroadcastError("Agent process terminated")
s.manager.CloseSession(s.ID)
```

### Buffer Growth Monitoring
```go
if len(client.buffer) > 10000 {
    log.Printf("[WARN] Client %s buffer at %d messages (slow?)",
        clientID, len(client.buffer))
}
```

## Session Lifecycle

Sessions close ONLY when:
1. Client explicitly calls `session/close`
2. Relay restarts/shuts down
3. Agent process crashes/exits

**No automatic timeout** - sessions persist indefinitely with 0 clients.

**Optional future config:**
```yaml
session:
  max_idle_time: "24h"  # optional, defaults to never
```

## Testing Strategy

### Unit Tests
- `TestAttachMultipleClients` - Multiple clients, unique IDs
- `TestBroadcastToAllClients` - All clients buffer all messages
- `TestSlowClientDropsMessages` - Fast clients unaffected by slow
- `TestDetachCleansUpGoroutine` - No goroutine leak
- `TestUnlimitedBuffering` - Large message counts don't crash

### Integration Tests
- `TestResumeAfterDisconnect` - Buffered messages delivered on resume
- `TestMultipleClientsReceiveAllMessages` - Broadcast works
- `TestNoGoroutineLeakOnRepeatedResume` - 100 connect/disconnect cycles
- `TestAgentDeathNotifiesClients` - Process exit handled gracefully

### Load Tests
- Client disconnects, agent sends 100K messages, client resumes
- Verify memory usage, all messages delivered

## Implementation Phases

1. **Phase 1:** Add ConnectionManager structs + basic methods (attach/detach)
2. **Phase 2:** Implement broadcaster + delivery goroutines
3. **Phase 3:** Update WebSocket handlers (remove old goroutine)
4. **Phase 4:** Add error handling + monitoring
5. **Phase 5:** Write comprehensive tests
6. **Phase 6:** Verify no regressions in existing functionality

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Unbounded memory growth | Log warnings at 10K messages, document as design trade-off |
| Goroutine proliferation | One per client (bounded by connections), monitor in tests |
| Lock contention | RWMutex, minimal critical sections, benchmark if needed |
| Multiple clients race | Mutex protects connections map, each client has own buffer |

## Success Metrics

- ✅ Zero goroutine leaks (verified via tests)
- ✅ Zero message loss (verified via integration tests)
- ✅ Multi-client support (verified via tests)
- ✅ Sessions survive disconnect/reconnect (verified via tests)
- ✅ No deadlocks under load (verified via load tests)

## Alternatives Considered

1. **Per-Client Goroutines** - More goroutines, cleaner isolation, rejected for more complexity
2. **Hybrid Smart Channels** - Less invasive, rejected for trickier edge cases
3. **Session timeout** - Rejected in favor of explicit close only
4. **Database persistence** - Rejected because can't serialize running process

**Selected:** Connection Manager Pattern for centralized control and easier correctness verification.
