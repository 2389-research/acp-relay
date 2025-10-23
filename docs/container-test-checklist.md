# Container Mode Manual Test Checklist

This checklist documents the manual testing procedures for container mode. Since Docker might not be running during development, these tests should be performed when Docker is available.

## Prerequisites

- [ ] Docker is installed and running (`docker ps` succeeds)
- [ ] Docker image is built: `docker build -t acp-relay-agent:latest .`
- [ ] Relay binary is built: `go build -o acp-relay ./cmd/relay`
- [ ] `config-container-test.yaml` exists in repo root

## Test 1: Container Mode Initialization

**Purpose:** Verify relay starts successfully in container mode and connects to Docker daemon.

**Steps:**

1. Start the relay with container config:
   ```bash
   ./acp-relay --config config-container-test.yaml
   ```

2. Verify startup logs contain:
   ```
   Database initialized at ./relay-messages.db
   Startup maintenance: marked 0 crashed/orphaned sessions as closed
   Container manager initialized (image: acp-relay-agent:latest)
   Session manager initialized (mode: container)
   Starting management API on 127.0.0.1:8082
   Starting HTTP server on 0.0.0.0:8080
   Starting WebSocket server on 0.0.0.0:8081
   ```

**Expected Result:** Relay starts successfully with no errors. "mode: container" appears in logs.

**Failure Modes:**
- "Cannot connect to Docker daemon": Docker not running or socket path wrong
- "Docker image not found": Run `docker build -t acp-relay-agent:latest .`
- Any other startup error: Check logs for details

---

## Test 2: Container Session Creation

**Purpose:** Verify relay creates containers for new sessions.

**Steps:**

1. With relay running (from Test 1), open a new terminal

2. Create a session via HTTP API:
   ```bash
   curl -X POST http://localhost:8080/session/new \
     -H "Content-Type: application/json" \
     -d '{
       "jsonrpc": "2.0",
       "method": "session/new",
       "params": {
         "workingDirectory": "/tmp/test-workspace"
       },
       "id": 1
     }'
   ```

3. Check response contains:
   ```json
   {
     "jsonrpc": "2.0",
     "result": {
       "sessionId": "sess_XXXXXXXX"
     },
     "id": 1
   }
   ```

4. Check relay logs contain:
   ```
   [sess_XXXXXXXX] Creating container session (image: acp-relay-agent:latest)
   [sess_XXXXXXXX] Session ready (mode: container)
   ```

**Expected Result:** Session created successfully with container mode logs.

**Failure Modes:**
- No sessionId in response: Check relay logs for error
- Timeout: Container startup might be slow, check `docker logs <container-id>`
- Error in response: Check error message and relay logs

---

## Test 3: Container Verification

**Purpose:** Verify Docker container is actually created and running.

**Steps:**

1. After creating session (from Test 2), list running containers:
   ```bash
   docker ps
   ```

2. Find container with name matching `sess_*` pattern

3. Check container details:
   ```bash
   docker inspect <container-id> | grep -A 5 "Mounts"
   ```

4. Verify mount exists:
   - Source: `/tmp/acp-workspaces/sess_XXXXXXXX`
   - Destination: `/workspace`

5. Check container logs:
   ```bash
   docker logs <container-id>
   ```

**Expected Result:**
- Container is running with correct name
- Workspace mount is configured correctly
- Container logs show agent initialization

**Failure Modes:**
- Container not running: Check `docker ps -a` for stopped containers
- No mount: Configuration error, check `config-container-test.yaml`
- Container exited: Check logs with `docker logs <container-id>`

---

## Test 4: Workspace Directory Verification

**Purpose:** Verify workspace directories are created on host and persist.

**Steps:**

1. After creating session (from Test 2), check workspace exists:
   ```bash
   ls -la /tmp/acp-workspaces/
   ```

2. Verify directory named `sess_XXXXXXXX` exists

3. Check permissions:
   ```bash
   ls -ld /tmp/acp-workspaces/sess_*
   ```

4. Create a test file in workspace:
   ```bash
   echo "test" > /tmp/acp-workspaces/sess_*/test.txt
   ```

5. Verify file is visible inside container:
   ```bash
   docker exec <container-id> ls /workspace/test.txt
   ```

**Expected Result:**
- Workspace directory exists with correct permissions (755)
- Files created on host are visible in container
- Files created in container appear on host

**Failure Modes:**
- Directory doesn't exist: Session creation failed
- Permission denied: Check `workspace_host_base` permissions
- File not visible in container: Mount not working

---

## Test 5: Process Mode Compatibility

**Purpose:** Verify process mode still works and modes don't interfere.

**Steps:**

1. Stop container relay (Ctrl+C or kill process)

2. Verify containers are cleaned up (if auto_remove: true):
   ```bash
   docker ps | grep sess_
   ```

3. Start relay in process mode:
   ```bash
   ./acp-relay --config config.yaml
   ```

4. Verify startup logs contain:
   ```
   Session manager initialized (mode: process)
   ```

5. Create a session via HTTP:
   ```bash
   curl -X POST http://localhost:8080/session/new \
     -H "Content-Type: application/json" \
     -d '{
       "jsonrpc": "2.0",
       "method": "session/new",
       "params": {
         "workingDirectory": "/tmp/test-workspace"
       },
       "id": 1
     }'
   ```

6. Verify no Docker containers created:
   ```bash
   docker ps | grep sess_
   ```

**Expected Result:**
- Process mode starts successfully
- Sessions create without Docker containers
- No interference between modes

**Failure Modes:**
- Process mode doesn't start: Check config.yaml syntax
- Containers still being created: Mode not switching properly
- Session creation fails: Unrelated issue, check logs

---

## Test 6: Resource Limits

**Purpose:** Verify container resource limits are applied.

**Steps:**

1. With container relay running, create a session

2. Check container resource limits:
   ```bash
   docker inspect <container-id> | grep -A 10 "Memory\|NanoCpus"
   ```

3. Verify values match config:
   - Memory: 536870912 (512MB in bytes)
   - NanoCPUs: 1000000000 (1.0 CPU)

4. Check live resource usage:
   ```bash
   docker stats --no-stream <container-id>
   ```

**Expected Result:**
- Memory limit set to 512MB
- CPU limit set to 1.0
- Stats show current usage within limits

**Failure Modes:**
- Limits not set: Configuration parsing issue
- Container using more than limit: Docker not enforcing limits
- Stats command fails: Container might have stopped

---

## Test 7: Session Cleanup

**Purpose:** Verify containers are properly stopped and cleaned up.

**Steps:**

1. With container relay running, create a session

2. Note the container ID:
   ```bash
   CONTAINER_ID=$(docker ps | grep sess_ | awk '{print $1}')
   echo $CONTAINER_ID
   ```

3. Close the session (via API or by stopping relay)

4. If relay is stopped with Ctrl+C, check container status:
   ```bash
   docker ps | grep $CONTAINER_ID
   ```

5. If auto_remove is true, container should be gone:
   ```bash
   docker ps -a | grep $CONTAINER_ID
   ```

6. Check workspace directory still exists:
   ```bash
   ls /tmp/acp-workspaces/sess_*
   ```

**Expected Result:**
- Container is stopped when session closes
- If auto_remove: true, container is removed
- Workspace directory persists on host

**Failure Modes:**
- Container still running: Session not properly closed
- Container exists but stopped: auto_remove not working
- Workspace deleted: Shouldn't happen, workspaces persist

---

## Test 8: Error Handling

**Purpose:** Verify helpful error messages when Docker issues occur.

**Steps:**

1. Stop Docker daemon

2. Try starting relay in container mode:
   ```bash
   ./acp-relay --config config-container-test.yaml
   ```

3. Verify error message is helpful:
   ```
   Cannot connect to Docker daemon. Is Docker running? Check: docker ps
   ```

4. Start Docker, but delete the image:
   ```bash
   docker rmi acp-relay-agent:latest
   ```

5. Try starting relay:
   ```bash
   ./acp-relay --config config-container-test.yaml
   ```

6. Verify error message suggests building:
   ```
   Docker image 'acp-relay-agent:latest' not found. Build it with:
     docker build -t acp-relay-agent:latest .
   ```

**Expected Result:**
- Clear, actionable error messages
- No cryptic Docker SDK errors
- Helpful suggestions for resolution

**Failure Modes:**
- Generic error messages: Error types not working
- Relay crashes: Should fail gracefully
- No suggestion: Error handling incomplete

---

## Test 9: Multiple Concurrent Sessions

**Purpose:** Verify multiple containers can run simultaneously.

**Steps:**

1. Start relay in container mode

2. Create 3 sessions in quick succession:
   ```bash
   for i in 1 2 3; do
     curl -X POST http://localhost:8080/session/new \
       -H "Content-Type: application/json" \
       -d "{
         \"jsonrpc\": \"2.0\",
         \"method\": \"session/new\",
         \"params\": {
           \"workingDirectory\": \"/tmp/test-workspace-$i\"
         },
         \"id\": $i
       }" &
   done
   wait
   ```

3. Check all containers are running:
   ```bash
   docker ps | grep sess_ | wc -l
   ```

4. Verify 3 workspace directories exist:
   ```bash
   ls /tmp/acp-workspaces/ | wc -l
   ```

5. Check each container has unique mount:
   ```bash
   docker ps --format "{{.Names}}" | grep sess_ | while read name; do
     docker inspect $name | grep -A 5 "Mounts"
   done
   ```

**Expected Result:**
- 3 containers running simultaneously
- 3 workspace directories created
- Each container has unique workspace mount
- No resource conflicts

**Failure Modes:**
- Fewer than 3 containers: Some creations failed
- Shared workspaces: Session ID collision
- Resource exhaustion: System limits reached

---

## Cleanup Procedures

### Clean Up Orphaned Containers

If tests fail and leave containers running:

```bash
# List all session containers
docker ps -a | grep sess_

# Stop all session containers
docker ps -a | grep sess_ | awk '{print $1}' | xargs docker stop

# Remove all session containers (if not auto-removed)
docker ps -a | grep sess_ | awk '{print $1}' | xargs docker rm -f
```

### Clean Up Workspace Directories

Workspaces persist after containers stop:

```bash
# View workspace directories
ls -la /tmp/acp-workspaces/

# Remove all workspaces
rm -rf /tmp/acp-workspaces/sess_*

# Remove entire workspace directory
rm -rf /tmp/acp-workspaces/
```

### Clean Up Test Database

If test runs create database entries:

```bash
# Remove test database
rm -f relay-messages.db relay-messages.db-shm relay-messages.db-wal
```

### Clean Up Test Config

After testing, remove the test config:

```bash
rm config-container-test.yaml
```

---

## Debugging Tips

### View Container Logs

```bash
# Get container ID from relay logs
# Then view container logs
docker logs <container-id>

# Follow logs in real-time
docker logs -f <container-id>

# Show last 50 lines
docker logs --tail 50 <container-id>
```

### Exec Into Container

```bash
# Open shell in running container
docker exec -it <container-id> /bin/sh

# Run specific command
docker exec <container-id> ls /workspace
docker exec <container-id> env
```

### Check Container Resources

```bash
# Real-time stats
docker stats <container-id>

# Detailed inspection
docker inspect <container-id>

# Check network
docker inspect <container-id> | grep -A 10 "NetworkSettings"
```

### Check Relay Logs

```bash
# If relay is running in background
tail -f /path/to/relay.log

# Or view stdout if running in foreground
# Relay logs show:
# - Session creation: [sess_*] Creating container session
# - Errors: Failed to create container: ...
# - Container exit: [sess_*] Container exited with code X
```

---

## Success Criteria

All tests should pass with these outcomes:

- [ ] Relay starts in container mode without errors
- [ ] Containers are created for each session
- [ ] Workspace directories exist and are mounted correctly
- [ ] Process mode still works (backward compatible)
- [ ] Resource limits are applied to containers
- [ ] Containers clean up properly (if auto_remove: true)
- [ ] Error messages are helpful and actionable
- [ ] Multiple concurrent sessions work without conflicts
- [ ] Workspaces persist after container stops

---

## Test Results Template

Record test results here:

```
Test Date: ____________________
Tester: _______________________
Docker Version: _______________
Go Version: ___________________

Test 1 (Initialization):        [ ] Pass  [ ] Fail
Test 2 (Session Creation):      [ ] Pass  [ ] Fail
Test 3 (Container Verify):      [ ] Pass  [ ] Fail
Test 4 (Workspace Verify):      [ ] Pass  [ ] Fail
Test 5 (Process Mode):          [ ] Pass  [ ] Fail
Test 6 (Resource Limits):       [ ] Pass  [ ] Fail
Test 7 (Cleanup):               [ ] Pass  [ ] Fail
Test 8 (Error Handling):        [ ] Pass  [ ] Fail
Test 9 (Concurrent Sessions):   [ ] Pass  [ ] Fail

Notes:
_________________________________________________________________
_________________________________________________________________
_________________________________________________________________
```
