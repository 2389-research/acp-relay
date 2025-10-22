# ACP Relay Server Implementation Plan

> **For Claude:** Use `${SUPERPOWERS_SKILLS_ROOT}/skills/collaboration/executing-plans/SKILL.md` to implement this plan task-by-task.

**Goal:** Build a Go-based relay server that translates HTTP/WebSocket requests into ACP JSON-RPC messages, spawning agent subprocesses per session with isolated working directories.

**Architecture:** Three-layer design: Frontend (HTTP/WebSocket handlers) → Translation (protocol conversion) → Backend (stdio subprocess management). Each session spawns an isolated agent process with its own working directory.

**Tech Stack:** Go 1.23+, gorilla/websocket, standard library JSON-RPC, viper for config

---

## Task 1: Project Setup & Configuration

**Files:**
- Create: `go.mod`
- Create: `cmd/relay/main.go`
- Create: `internal/config/config.go`
- Create: `config.yaml`
- Test: `internal/config/config_test.go`

**Step 1: Initialize Go module**

```bash
go mod init github.com/yourusername/acp-relay
```

**Step 2: Write test for config loading**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create test config file
	content := `
server:
  http_port: 8080
  websocket_port: 8081
agent:
  command: "/usr/local/bin/test-agent"
`
	err := os.WriteFile("test_config.yaml", []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("test_config.yaml")

	cfg, err := Load("test_config.yaml")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Server.HTTPPort != 8080 {
		t.Errorf("expected http_port 8080, got %d", cfg.Server.HTTPPort)
	}

	if cfg.Agent.Command != "/usr/local/bin/test-agent" {
		t.Errorf("expected agent command '/usr/local/bin/test-agent', got %s", cfg.Agent.Command)
	}
}
```

**Step 3: Run test to verify it fails**

```bash
go test ./internal/config -v
```

Expected: FAIL with "undefined: Load" or "package config is not in GOROOT"

**Step 4: Write minimal config implementation**

Create `internal/config/config.go`:

```go
// ABOUTME: Configuration loading and management for ACP relay server
// ABOUTME: Supports YAML files and environment variable overrides

package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	Agent  AgentConfig  `mapstructure:"agent"`
}

type ServerConfig struct {
	HTTPPort      int    `mapstructure:"http_port"`
	HTTPHost      string `mapstructure:"http_host"`
	WebSocketPort int    `mapstructure:"websocket_port"`
	WebSocketHost string `mapstructure:"websocket_host"`
	ManagementPort int   `mapstructure:"management_port"`
	ManagementHost string `mapstructure:"management_host"`
}

type AgentConfig struct {
	Command               string            `mapstructure:"command"`
	Args                  []string          `mapstructure:"args"`
	Env                   map[string]string `mapstructure:"env"`
	StartupTimeoutSeconds int               `mapstructure:"startup_timeout_seconds"`
	MaxConcurrentSessions int               `mapstructure:"max_concurrent_sessions"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
```

**Step 5: Install dependencies**

```bash
go get github.com/spf13/viper
go mod tidy
```

**Step 6: Run test to verify it passes**

```bash
go test ./internal/config -v
```

Expected: PASS

**Step 7: Create default config.yaml**

Create `config.yaml`:

```yaml
server:
  http_port: 8080
  http_host: "0.0.0.0"
  websocket_port: 8081
  websocket_host: "0.0.0.0"
  management_port: 8082
  management_host: "127.0.0.1"

agent:
  command: "/usr/local/bin/acp-agent"
  args: []
  env: {}
  startup_timeout_seconds: 10
  max_concurrent_sessions: 100

logging:
  level: "info"
  format: "json"
```

**Step 8: Create minimal main.go**

Create `cmd/relay/main.go`:

```go
// ABOUTME: Main entry point for ACP relay server
// ABOUTME: Loads configuration and starts HTTP/WebSocket servers

package main

import (
	"flag"
	"log"

	"github.com/yourusername/acp-relay/internal/config"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Printf("loaded config: http_port=%d, ws_port=%d",
		cfg.Server.HTTPPort, cfg.Server.WebSocketPort)
}
```

**Step 9: Test running the binary**

```bash
go run cmd/relay/main.go --config config.yaml
```

Expected: Logs "loaded config: http_port=8080, ws_port=8081"

**Step 10: Commit**

```bash
git init
git add .
git commit -m "feat: initial project setup with config loading"
```

---

## Task 2: JSON-RPC Message Types

**Files:**
- Create: `internal/jsonrpc/types.go`
- Test: `internal/jsonrpc/types_test.go`

**Step 1: Write test for JSON-RPC request parsing**

Create `internal/jsonrpc/types_test.go`:

```go
package jsonrpc

import (
	"encoding/json"
	"testing"
)

func TestParseRequest(t *testing.T) {
	data := []byte(`{
		"jsonrpc": "2.0",
		"method": "session/new",
		"params": {"workingDirectory": "/tmp/test"},
		"id": 1
	}`)

	var req Request
	err := json.Unmarshal(data, &req)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", req.JSONRPC)
	}

	if req.Method != "session/new" {
		t.Errorf("expected method session/new, got %s", req.Method)
	}

	if req.ID == nil {
		t.Error("expected id to be set")
	}
}

func TestParseResponse(t *testing.T) {
	data := []byte(`{
		"jsonrpc": "2.0",
		"result": {"sessionId": "sess_123"},
		"id": 1
	}`)

	var resp Response
	err := json.Unmarshal(data, &resp)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.Result == nil {
		t.Error("expected result to be set")
	}
}

func TestParseError(t *testing.T) {
	data := []byte(`{
		"jsonrpc": "2.0",
		"error": {
			"code": -32600,
			"message": "Invalid request",
			"data": {"detail": "test"}
		},
		"id": 1
	}`)

	var resp Response
	err := json.Unmarshal(data, &resp)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error to be set")
	}

	if resp.Error.Code != -32600 {
		t.Errorf("expected code -32600, got %d", resp.Error.Code)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/jsonrpc -v
```

Expected: FAIL with "package jsonrpc is not in GOROOT"

**Step 3: Write JSON-RPC types**

Create `internal/jsonrpc/types.go`:

```go
// ABOUTME: JSON-RPC 2.0 message types for ACP protocol
// ABOUTME: Implements request, response, and error structures

package jsonrpc

import "encoding/json"

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      *json.RawMessage `json:"id,omitempty"`
}

type Response struct {
	JSONRPC string           `json:"jsonrpc"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *Error           `json:"error,omitempty"`
	ID      *json.RawMessage `json:"id,omitempty"`
}

type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Standard JSON-RPC error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
	ServerError    = -32000
)
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/jsonrpc -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/jsonrpc/
git commit -m "feat: add JSON-RPC message types"
```

---

## Task 3: Session Manager (Process Lifecycle)

**Files:**
- Create: `internal/session/manager.go`
- Create: `internal/session/session.go`
- Test: `internal/session/manager_test.go`

**Step 1: Write test for session creation**

Create `internal/session/manager_test.go`:

```go
package session

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestCreateSession(t *testing.T) {
	tmpDir := t.TempDir()

	mgr := NewManager(ManagerConfig{
		AgentCommand: "cat", // Use 'cat' as a test subprocess
		AgentArgs:    []string{},
		AgentEnv:     map[string]string{},
	})

	sess, err := mgr.CreateSession(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer mgr.CloseSession(sess.ID)

	if sess.ID == "" {
		t.Error("expected session ID to be set")
	}

	if sess.WorkingDir != tmpDir {
		t.Errorf("expected working dir %s, got %s", tmpDir, sess.WorkingDir)
	}

	if sess.AgentStdin == nil {
		t.Error("expected stdin to be set")
	}
}

func TestCloseSession(t *testing.T) {
	tmpDir := t.TempDir()

	mgr := NewManager(ManagerConfig{
		AgentCommand: "cat",
		AgentArgs:    []string{},
		AgentEnv:     map[string]string{},
	})

	sess, err := mgr.CreateSession(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	err = mgr.CloseSession(sess.ID)
	if err != nil {
		t.Errorf("failed to close session: %v", err)
	}

	// Verify process is killed
	time.Sleep(100 * time.Millisecond)
	if sess.AgentCmd.ProcessState == nil {
		t.Error("expected process to be terminated")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/session -v
```

Expected: FAIL with "undefined: NewManager"

**Step 3: Write session types**

Create `internal/session/session.go`:

```go
// ABOUTME: Session data structure representing a client-agent connection
// ABOUTME: Each session has its own agent subprocess and working directory

package session

import (
	"context"
	"io"
	"os/exec"
)

type Session struct {
	ID           string
	WorkingDir   string
	AgentCmd     *exec.Cmd
	AgentStdin   io.WriteCloser
	AgentStdout  io.ReadCloser
	AgentStderr  io.ReadCloser
	ToAgent      chan []byte
	FromAgent    chan []byte
	Context      context.Context
	Cancel       context.CancelFunc
}
```

**Step 4: Write minimal manager implementation**

Create `internal/session/manager.go`:

```go
// ABOUTME: Session manager for creating and managing agent subprocesses
// ABOUTME: Handles process lifecycle, stdio piping, and cleanup

package session

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/google/uuid"
)

type ManagerConfig struct {
	AgentCommand string
	AgentArgs    []string
	AgentEnv     map[string]string
}

type Manager struct {
	config   ManagerConfig
	sessions map[string]*Session
	mu       sync.RWMutex
}

func NewManager(cfg ManagerConfig) *Manager {
	return &Manager{
		config:   cfg,
		sessions: make(map[string]*Session),
	}
}

func (m *Manager) CreateSession(ctx context.Context, workingDir string) (*Session, error) {
	sessionID := "sess_" + uuid.New().String()[:8]

	sessionCtx, cancel := context.WithCancel(ctx)

	// Create agent command
	cmd := exec.CommandContext(sessionCtx, m.config.AgentCommand, m.config.AgentArgs...)
	cmd.Dir = workingDir

	// Set up environment
	cmd.Env = append(cmd.Env, "PWD="+workingDir)
	for k, v := range m.config.AgentEnv {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Create pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start agent: %w", err)
	}

	sess := &Session{
		ID:          sessionID,
		WorkingDir:  workingDir,
		AgentCmd:    cmd,
		AgentStdin:  stdin,
		AgentStdout: stdout,
		AgentStderr: stderr,
		ToAgent:     make(chan []byte, 10),
		FromAgent:   make(chan []byte, 10),
		Context:     sessionCtx,
		Cancel:      cancel,
	}

	m.mu.Lock()
	m.sessions[sessionID] = sess
	m.mu.Unlock()

	return sess, nil
}

func (m *Manager) CloseSession(sessionID string) error {
	m.mu.Lock()
	sess, exists := m.sessions[sessionID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	// Cancel context (kills process)
	sess.Cancel()

	// Close channels
	close(sess.ToAgent)
	close(sess.FromAgent)

	return nil
}

func (m *Manager) GetSession(sessionID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sess, exists := m.sessions[sessionID]
	return sess, exists
}
```

**Step 5: Install UUID dependency**

```bash
go get github.com/google/uuid
go mod tidy
```

**Step 6: Run test to verify it passes**

```bash
go test ./internal/session -v
```

Expected: PASS

**Step 7: Commit**

```bash
git add internal/session/
git commit -m "feat: add session manager with process lifecycle"
```

---

## Task 4: Stdio Bridge (Goroutines for Agent I/O)

**Files:**
- Modify: `internal/session/manager.go`
- Modify: `internal/session/session.go`
- Test: `internal/session/bridge_test.go`

**Step 1: Write test for stdio bridging**

Create `internal/session/bridge_test.go`:

```go
package session

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestStdioBridge(t *testing.T) {
	tmpDir := t.TempDir()

	// Use 'cat' which echoes stdin to stdout
	mgr := NewManager(ManagerConfig{
		AgentCommand: "cat",
		AgentArgs:    []string{},
		AgentEnv:     map[string]string{},
	})

	sess, err := mgr.CreateSession(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer mgr.CloseSession(sess.ID)

	// Start stdio bridge
	go sess.StartStdioBridge()

	// Send a JSON-RPC message
	msg := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "test",
		"id":      1,
	}
	data, _ := json.Marshal(msg)
	data = append(data, '\n')

	sess.ToAgent <- data

	// Read response
	select {
	case response := <-sess.FromAgent:
		var parsed map[string]interface{}
		if err := json.Unmarshal(response, &parsed); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if parsed["method"] != "test" {
			t.Errorf("expected method 'test', got %v", parsed["method"])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for response")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/session -v
```

Expected: FAIL with "sess.StartStdioBridge undefined"

**Step 3: Add StartStdioBridge method**

Add to `internal/session/session.go`:

```go
import (
	"bufio"
	"encoding/json"
	"log"
)

// StartStdioBridge starts goroutines to bridge channels and stdio
func (s *Session) StartStdioBridge() {
	// Goroutine: ToAgent channel -> AgentStdin
	go func() {
		for msg := range s.ToAgent {
			if _, err := s.AgentStdin.Write(msg); err != nil {
				log.Printf("error writing to agent stdin: %v", err)
				return
			}
		}
	}()

	// Goroutine: AgentStdout -> FromAgent channel
	go func() {
		scanner := bufio.NewScanner(s.AgentStdout)
		for scanner.Scan() {
			line := scanner.Bytes()
			// Make a copy since scanner reuses the buffer
			msg := make([]byte, len(line))
			copy(msg, line)

			select {
			case s.FromAgent <- msg:
			case <-s.Context.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("error reading agent stdout: %v", err)
		}
	}()

	// Goroutine: AgentStderr -> log
	go func() {
		scanner := bufio.NewScanner(s.AgentStderr)
		for scanner.Scan() {
			log.Printf("agent stderr [%s]: %s", s.ID, scanner.Text())
		}
	}()
}
```

**Step 4: Update CreateSession to start bridge**

Modify `internal/session/manager.go`, add to end of `CreateSession`:

```go
	// Start stdio bridge
	go sess.StartStdioBridge()

	return sess, nil
```

**Step 5: Run test to verify it passes**

```bash
go test ./internal/session -v -run TestStdioBridge
```

Expected: PASS

**Step 6: Commit**

```bash
git add internal/session/
git commit -m "feat: add stdio bridge for agent communication"
```

---

## Task 5: HTTP Server (session/new and session/prompt)

**Files:**
- Create: `internal/http/server.go`
- Create: `internal/http/handlers.go`
- Test: `internal/http/handlers_test.go`
- Modify: `cmd/relay/main.go`

**Step 1: Write test for HTTP session/new**

Create `internal/http/handlers_test.go`:

```go
package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourusername/acp-relay/internal/session"
)

func TestSessionNew(t *testing.T) {
	mgr := session.NewManager(session.ManagerConfig{
		AgentCommand: "cat",
		AgentArgs:    []string{},
		AgentEnv:     map[string]string{},
	})

	srv := NewServer(mgr)

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/new",
		"params": map[string]interface{}{
			"workingDirectory": t.TempDir(),
		},
		"id": 1,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/session/new", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatal("expected result object")
	}

	if result["sessionId"] == "" {
		t.Error("expected sessionId in result")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/http -v
```

Expected: FAIL with "undefined: NewServer"

**Step 3: Write HTTP server implementation**

Create `internal/http/server.go`:

```go
// ABOUTME: HTTP server for handling REST-style ACP requests
// ABOUTME: Routes requests to appropriate handlers

package http

import (
	"net/http"

	"github.com/yourusername/acp-relay/internal/session"
)

type Server struct {
	sessionMgr *session.Manager
	mux        *http.ServeMux
}

func NewServer(mgr *session.Manager) *Server {
	s := &Server{
		sessionMgr: mgr,
		mux:        http.NewServeMux(),
	}

	s.mux.HandleFunc("/session/new", s.handleSessionNew)
	s.mux.HandleFunc("/session/prompt", s.handleSessionPrompt)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
```

Create `internal/http/handlers.go`:

```go
// ABOUTME: HTTP handlers for ACP JSON-RPC endpoints
// ABOUTME: Translates HTTP requests to JSON-RPC messages

package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yourusername/acp-relay/internal/jsonrpc"
)

func (s *Server) handleSessionNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, jsonrpc.InvalidRequest, "failed to read body", nil)
		return
	}
	defer r.Body.Close()

	var req jsonrpc.Request
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, jsonrpc.ParseError, "invalid JSON", nil)
		return
	}

	// Parse params
	var params struct {
		WorkingDirectory string `json:"workingDirectory"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeError(w, jsonrpc.InvalidParams, "invalid params", nil)
		return
	}

	// Create session
	sess, err := s.sessionMgr.CreateSession(r.Context(), params.WorkingDirectory)
	if err != nil {
		writeError(w, jsonrpc.ServerError, fmt.Sprintf("failed to create session: %v", err), nil)
		return
	}

	// Return response
	result := map[string]interface{}{
		"sessionId": sess.ID,
	}

	writeResponse(w, result, req.ID)
}

func (s *Server) handleSessionPrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, jsonrpc.InvalidRequest, "failed to read body", nil)
		return
	}
	defer r.Body.Close()

	var req jsonrpc.Request
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, jsonrpc.ParseError, "invalid JSON", nil)
		return
	}

	// Parse params
	var params struct {
		SessionID string          `json:"sessionId"`
		Content   json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeError(w, jsonrpc.InvalidParams, "invalid params", nil)
		return
	}

	// Get session
	sess, exists := s.sessionMgr.GetSession(params.SessionID)
	if !exists {
		writeError(w, jsonrpc.ServerError, "session not found", nil)
		return
	}

	// Forward request to agent
	agentReq := jsonrpc.Request{
		JSONRPC: "2.0",
		Method:  "session/prompt",
		Params:  req.Params,
		ID:      req.ID,
	}

	reqData, _ := json.Marshal(agentReq)
	reqData = append(reqData, '\n')

	sess.ToAgent <- reqData

	// Wait for response (with timeout)
	select {
	case respData := <-sess.FromAgent:
		w.Header().Set("Content-Type", "application/json")
		w.Write(respData)
	case <-time.After(30 * time.Second):
		writeError(w, jsonrpc.ServerError, "agent response timeout", nil)
	case <-r.Context().Done():
		writeError(w, jsonrpc.ServerError, "request cancelled", nil)
	}
}

func writeResponse(w http.ResponseWriter, result interface{}, id *json.RawMessage) {
	resultData, _ := json.Marshal(result)

	resp := jsonrpc.Response{
		JSONRPC: "2.0",
		Result:  resultData,
		ID:      id,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, code int, message string, id *json.RawMessage) {
	resp := jsonrpc.Response{
		JSONRPC: "2.0",
		Error: &jsonrpc.Error{
			Code:    code,
			Message: message,
		},
		ID: id,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors still return 200
	json.NewEncoder(w).Encode(resp)
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/http -v
```

Expected: PASS

**Step 5: Wire up HTTP server in main.go**

Modify `cmd/relay/main.go`:

```go
package main

import (
	"flag"
	"log"
	"net/http"
	"fmt"

	"github.com/yourusername/acp-relay/internal/config"
	httpserver "github.com/yourusername/acp-relay/internal/http"
	"github.com/yourusername/acp-relay/internal/session"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Create session manager
	sessionMgr := session.NewManager(session.ManagerConfig{
		AgentCommand: cfg.Agent.Command,
		AgentArgs:    cfg.Agent.Args,
		AgentEnv:     cfg.Agent.Env,
	})

	// Create HTTP server
	httpSrv := httpserver.NewServer(sessionMgr)

	addr := fmt.Sprintf("%s:%d", cfg.Server.HTTPHost, cfg.Server.HTTPPort)
	log.Printf("Starting HTTP server on %s", addr)

	if err := http.ListenAndServe(addr, httpSrv); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
```

**Step 6: Test manually**

```bash
go run cmd/relay/main.go
```

Expected: Server starts on port 8080

**Step 7: Commit**

```bash
git add .
git commit -m "feat: add HTTP server with session/new and session/prompt handlers"
```

---

## Task 6: WebSocket Server

**Files:**
- Create: `internal/websocket/server.go`
- Test: `internal/websocket/server_test.go`
- Modify: `cmd/relay/main.go`

**Step 1: Write test for WebSocket connection**

Create `internal/websocket/server_test.go`:

```go
package websocket

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yourusername/acp-relay/internal/session"
)

func TestWebSocketConnection(t *testing.T) {
	mgr := session.NewManager(session.ManagerConfig{
		AgentCommand: "cat",
		AgentArgs:    []string{},
		AgentEnv:     map[string]string{},
	})

	srv := NewServer(mgr)

	// Create test server
	httpSrv := httptest.NewServer(srv)
	defer httpSrv.Close()

	// Connect via WebSocket
	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer ws.Close()

	// Send session/new
	tmpDir := t.TempDir()
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/new",
		"params": map[string]interface{}{
			"workingDirectory": tmpDir,
		},
		"id": 1,
	}

	if err := ws.WriteJSON(req); err != nil {
		t.Fatalf("failed to send: %v", err)
	}

	// Read response
	var resp map[string]interface{}
	if err := ws.ReadJSON(&resp); err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatal("expected result object")
	}

	if result["sessionId"] == "" {
		t.Error("expected sessionId in result")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/websocket -v
```

Expected: FAIL with "package websocket is not in GOROOT"

**Step 3: Install gorilla/websocket**

```bash
go get github.com/gorilla/websocket
go mod tidy
```

**Step 4: Write WebSocket server**

Create `internal/websocket/server.go`:

```go
// ABOUTME: WebSocket server for bidirectional ACP communication
// ABOUTME: Handles persistent connections with streaming JSON-RPC messages

package websocket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yourusername/acp-relay/internal/jsonrpc"
	"github.com/yourusername/acp-relay/internal/session"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // TODO: Add proper origin checking
	},
}

type Server struct {
	sessionMgr *session.Manager
}

func NewServer(mgr *session.Manager) *Server {
	return &Server{sessionMgr: mgr}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}

	s.handleConnection(conn)
}

func (s *Server) handleConnection(conn *websocket.Conn) {
	defer conn.Close()

	var currentSession *session.Session
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Goroutine: Read from agent and send to WebSocket
	fromAgent := make(chan []byte, 10)
	go func() {
		for {
			select {
			case msg := <-fromAgent:
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					log.Printf("websocket write error: %v", err)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Main loop: Read from WebSocket
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("websocket read error: %v", err)
			break
		}

		var req jsonrpc.Request
		if err := json.Unmarshal(message, &req); err != nil {
			s.sendError(conn, jsonrpc.ParseError, "invalid JSON", nil)
			continue
		}

		// Handle different methods
		switch req.Method {
		case "session/new":
			var params struct {
				WorkingDirectory string `json:"workingDirectory"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				s.sendError(conn, jsonrpc.InvalidParams, "invalid params", req.ID)
				continue
			}

			sess, err := s.sessionMgr.CreateSession(ctx, params.WorkingDirectory)
			if err != nil {
				s.sendError(conn, jsonrpc.ServerError, err.Error(), req.ID)
				continue
			}

			currentSession = sess

			// Start forwarding agent messages to WebSocket
			go func() {
				for msg := range sess.FromAgent {
					fromAgent <- msg
				}
			}()

			// Send response
			result := map[string]interface{}{"sessionId": sess.ID}
			s.sendResponse(conn, result, req.ID)

		default:
			// Forward to agent
			if currentSession == nil {
				s.sendError(conn, jsonrpc.ServerError, "no session created", req.ID)
				continue
			}

			reqData, _ := json.Marshal(req)
			reqData = append(reqData, '\n')
			currentSession.ToAgent <- reqData
		}
	}

	// Cleanup
	if currentSession != nil {
		s.sessionMgr.CloseSession(currentSession.ID)
	}
}

func (s *Server) sendResponse(conn *websocket.Conn, result interface{}, id *json.RawMessage) {
	resultData, _ := json.Marshal(result)
	resp := jsonrpc.Response{
		JSONRPC: "2.0",
		Result:  resultData,
		ID:      id,
	}

	data, _ := json.Marshal(resp)
	conn.WriteMessage(websocket.TextMessage, data)
}

func (s *Server) sendError(conn *websocket.Conn, code int, message string, id *json.RawMessage) {
	resp := jsonrpc.Response{
		JSONRPC: "2.0",
		Error: &jsonrpc.Error{
			Code:    code,
			Message: message,
		},
		ID: id,
	}

	data, _ := json.Marshal(resp)
	conn.WriteMessage(websocket.TextMessage, data)
}
```

**Step 5: Run test to verify it passes**

```bash
go test ./internal/websocket -v
```

Expected: PASS

**Step 6: Wire up WebSocket server in main.go**

Modify `cmd/relay/main.go`:

```go
import (
	// ... existing imports
	wsserver "github.com/yourusername/acp-relay/internal/websocket"
)

func main() {
	// ... existing code

	// Create WebSocket server
	wsSrv := wsserver.NewServer(sessionMgr)

	// Start HTTP server
	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.Server.HTTPHost, cfg.Server.HTTPPort)
		log.Printf("Starting HTTP server on %s", addr)
		if err := http.ListenAndServe(addr, httpSrv); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Start WebSocket server
	wsAddr := fmt.Sprintf("%s:%d", cfg.Server.WebSocketHost, cfg.Server.WebSocketPort)
	log.Printf("Starting WebSocket server on %s", wsAddr)
	if err := http.ListenAndServe(wsAddr, wsSrv); err != nil {
		log.Fatalf("WebSocket server failed: %v", err)
	}
}
```

**Step 7: Test manually**

```bash
go run cmd/relay/main.go
```

Expected: Both HTTP (8080) and WebSocket (8081) servers start

**Step 8: Commit**

```bash
git add .
git commit -m "feat: add WebSocket server with bidirectional communication"
```

---

## Task 7: LLM-Optimized Error Handling

**Files:**
- Create: `internal/errors/llm_errors.go`
- Test: `internal/errors/llm_errors_test.go`
- Modify: `internal/http/handlers.go`
- Modify: `internal/websocket/server.go`

**Step 1: Write test for LLM error formatting**

Create `internal/errors/llm_errors_test.go`:

```go
package errors

import (
	"encoding/json"
	"testing"
)

func TestAgentConnectionError(t *testing.T) {
	err := NewAgentConnectionError("ws://localhost:9000", 3, 5000, "connection timeout")

	data, jsonErr := json.Marshal(err.Data)
	if jsonErr != nil {
		t.Fatalf("failed to marshal: %v", jsonErr)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if parsed["error_type"] != "agent_connection_timeout" {
		t.Errorf("expected error_type agent_connection_timeout")
	}

	explanation, ok := parsed["explanation"].(string)
	if !ok || explanation == "" {
		t.Error("expected explanation to be set")
	}

	suggestions, ok := parsed["suggested_actions"].([]interface{})
	if !ok || len(suggestions) == 0 {
		t.Error("expected suggested_actions to be set")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/errors -v
```

Expected: FAIL

**Step 3: Write LLM error helpers**

Create `internal/errors/llm_errors.go`:

```go
// ABOUTME: LLM-optimized error messages with explanations and suggested actions
// ABOUTME: Provides verbose, actionable error context for AI agents

package errors

import (
	"encoding/json"
	"fmt"

	"github.com/yourusername/acp-relay/internal/jsonrpc"
)

type LLMErrorData struct {
	ErrorType        string                 `json:"error_type"`
	Explanation      string                 `json:"explanation"`
	PossibleCauses   []string               `json:"possible_causes,omitempty"`
	SuggestedActions []string               `json:"suggested_actions,omitempty"`
	RelevantState    map[string]interface{} `json:"relevant_state,omitempty"`
	Recoverable      bool                   `json:"recoverable"`
	Details          string                 `json:"details,omitempty"`
}

func NewAgentConnectionError(agentURL string, attempts int, durationMs int, details string) *jsonrpc.Error {
	message := fmt.Sprintf(
		"I attempted to spawn the ACP agent process but it failed to start within %dms. "+
			"This typically means the agent command is incorrect, the agent binary is missing, "+
			"or the agent encountered an error during startup.",
		durationMs,
	)

	data := LLMErrorData{
		ErrorType:   "agent_connection_timeout",
		Explanation: "The relay server tried to start the agent subprocess but it did not become ready within the configured timeout.",
		PossibleCauses: []string{
			"The agent command path is incorrect or the binary doesn't exist",
			"The agent requires environment variables that aren't set",
			"The agent crashed immediately on startup",
			"The agent is waiting for input but the relay hasn't sent initialization",
		},
		SuggestedActions: []string{
			"Check that the agent command exists: ls -l /path/to/agent",
			"Verify the agent can run manually: /path/to/agent --help",
			"Check the relay's stderr logs for agent error messages",
			"Ensure required environment variables are set in config.yaml",
		},
		RelevantState: map[string]interface{}{
			"agent_url":  agentURL,
			"attempts":   attempts,
			"timeout_ms": durationMs,
		},
		Recoverable: true,
		Details:     details,
	}

	dataBytes, _ := json.Marshal(data)

	return &jsonrpc.Error{
		Code:    jsonrpc.ServerError,
		Message: message,
		Data:    dataBytes,
	}
}

func NewSessionNotFoundError(sessionID string) *jsonrpc.Error {
	message := fmt.Sprintf(
		"The session '%s' does not exist. This means the session was never created, "+
			"it expired due to inactivity, or the agent process crashed and cleaned up.",
		sessionID,
	)

	data := LLMErrorData{
		ErrorType:   "session_not_found",
		Explanation: "The relay server doesn't have an active session with this ID.",
		PossibleCauses: []string{
			"The session was never created (missing session/new call)",
			"The session ID was mistyped or corrupted",
			"The session expired due to inactivity timeout",
			"The agent process crashed and the session was cleaned up",
		},
		SuggestedActions: []string{
			"Create a new session using session/new",
			"Verify you're using the correct session ID from the session/new response",
			"Check if the agent process is still running",
		},
		RelevantState: map[string]interface{}{
			"session_id": sessionID,
		},
		Recoverable: true,
	}

	dataBytes, _ := json.Marshal(data)

	return &jsonrpc.Error{
		Code:    jsonrpc.ServerError,
		Message: message,
		Data:    dataBytes,
	}
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/errors -v
```

Expected: PASS

**Step 5: Use LLM errors in HTTP handlers**

Modify `internal/http/handlers.go`, update error calls:

```go
import (
	"github.com/yourusername/acp-relay/internal/errors"
)

// In handleSessionPrompt, replace session not found error:
	sess, exists := s.sessionMgr.GetSession(params.SessionID)
	if !exists {
		err := errors.NewSessionNotFoundError(params.SessionID)
		writeLLMError(w, err, req.ID)
		return
	}

// Add new helper function:
func writeLLMError(w http.ResponseWriter, err *jsonrpc.Error, id *json.RawMessage) {
	resp := jsonrpc.Response{
		JSONRPC: "2.0",
		Error:   err,
		ID:      id,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
```

**Step 6: Use LLM errors in WebSocket server**

Similarly update `internal/websocket/server.go` to use `errors.NewSessionNotFoundError()` and add `sendLLMError()` helper.

**Step 7: Commit**

```bash
git add .
git commit -m "feat: add LLM-optimized error messages with explanations"
```

---

## Task 8: Management API & Health Endpoint

**Files:**
- Create: `internal/management/server.go`
- Test: `internal/management/server_test.go`
- Modify: `cmd/relay/main.go`

**Step 1: Write test for health endpoint**

Create `internal/management/server_test.go`:

```go
package management

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/yourusername/acp-relay/internal/config"
	"github.com/yourusername/acp-relay/internal/session"
)

func TestHealthEndpoint(t *testing.T) {
	cfg := &config.Config{
		Agent: config.AgentConfig{
			Command: "cat",
		},
	}

	mgr := session.NewManager(session.ManagerConfig{
		AgentCommand: cfg.Agent.Command,
	})

	srv := NewServer(cfg, mgr)

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var health map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &health)

	if health["status"] != "healthy" {
		t.Error("expected status healthy")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/management -v
```

Expected: FAIL

**Step 3: Write management server**

Create `internal/management/server.go`:

```go
// ABOUTME: Management API for runtime config and health monitoring
// ABOUTME: Provides endpoints for health checks and configuration updates

package management

import (
	"encoding/json"
	"net/http"

	"github.com/yourusername/acp-relay/internal/config"
	"github.com/yourusername/acp-relay/internal/session"
)

type Server struct {
	config     *config.Config
	sessionMgr *session.Manager
	mux        *http.ServeMux
}

func NewServer(cfg *config.Config, mgr *session.Manager) *Server {
	s := &Server{
		config:     cfg,
		sessionMgr: mgr,
		mux:        http.NewServeMux(),
	}

	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/config", s.handleConfig)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	// TODO: Add actual health checks
	health := map[string]interface{}{
		"status":          "healthy",
		"agent_command":   s.config.Agent.Command,
		"active_sessions": 0, // TODO: Track session count
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.config)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/management -v
```

Expected: PASS

**Step 5: Wire up management server in main.go**

Modify `cmd/relay/main.go`:

```go
import (
	mgmtserver "github.com/yourusername/acp-relay/internal/management"
)

func main() {
	// ... existing code

	// Create management server
	mgmtSrv := mgmtserver.NewServer(cfg, sessionMgr)

	// Start management server
	go func() {
		mgmtAddr := fmt.Sprintf("%s:%d", cfg.Server.ManagementHost, cfg.Server.ManagementPort)
		log.Printf("Starting management API on %s", mgmtAddr)
		if err := http.ListenAndServe(mgmtAddr, mgmtSrv); err != nil {
			log.Fatalf("Management server failed: %v", err)
		}
	}()

	// ... rest of code
}
```

**Step 6: Test manually**

```bash
go run cmd/relay/main.go
curl http://localhost:8082/api/health
```

Expected: Returns health JSON

**Step 7: Commit**

```bash
git add .
git commit -m "feat: add management API with health and config endpoints"
```

---

## Task 9: README & Documentation

**Files:**
- Create: `README.md`
- Create: `docs/api.md`

**Step 1: Write README**

Create `README.md`:

```markdown
# ACP Relay Server

A Go-based relay server that translates HTTP/WebSocket requests into ACP (Agent Client Protocol) JSON-RPC messages, spawning isolated agent subprocesses per session.

## Features

- **HTTP API**: REST-style request/response for simple use cases
- **WebSocket API**: Bidirectional streaming for real-time communication
- **Process Isolation**: One agent subprocess per session with isolated working directories
- **LLM-Optimized Errors**: Verbose error messages with explanations and suggested actions
- **Management API**: Health checks and runtime configuration

## Quick Start

### Installation

```bash
go build -o acp-relay ./cmd/relay
```

### Configuration

Create `config.yaml`:

```yaml
server:
  http_port: 8080
  websocket_port: 8081

agent:
  command: "/path/to/your/acp-agent"
  args: []
```

### Run

```bash
./acp-relay --config config.yaml
```

## API Documentation

See [docs/api.md](docs/api.md) for detailed API documentation.

## Architecture

- **Frontend Layer**: HTTP and WebSocket handlers
- **Translation Layer**: Protocol conversion (HTTP/WS ↔ JSON-RPC)
- **Backend Layer**: Agent subprocess management via stdio

## License

MIT
```

**Step 2: Write API documentation**

Create `docs/api.md`:

```markdown
# ACP Relay API Documentation

## HTTP API (Port 8080)

### POST /session/new

Create a new agent session.

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

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "sessionId": "sess_abc123"
  },
  "id": 1
}
```

### POST /session/prompt

Send a prompt to an existing session.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "session/prompt",
  "params": {
    "sessionId": "sess_abc123",
    "content": [
      {"type": "text", "text": "Hello, agent!"}
    ]
  },
  "id": 2
}
```

## WebSocket API (Port 8081)

Connect to `ws://localhost:8081` and send JSON-RPC messages.

Same message format as HTTP API, but bidirectional and streaming.

## Management API (Port 8082)

### GET /api/health

Get server health status.

### GET /api/config

Get current configuration.
```

**Step 3: Commit**

```bash
git add README.md docs/api.md
git commit -m "docs: add README and API documentation"
```

---

## Task 10: Integration Tests & Final Testing

**Files:**
- Create: `tests/integration_test.go`
- Create: `tests/test_agent.sh`

**Step 1: Create test agent script**

Create `tests/test_agent.sh`:

```bash
#!/bin/bash
# Simple test agent that echoes JSON-RPC messages

while IFS= read -r line; do
  echo "$line"
done
```

```bash
chmod +x tests/test_agent.sh
```

**Step 2: Write integration test**

Create `tests/integration_test.go`:

```go
package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestFullHTTPFlow(t *testing.T) {
	// Start relay server
	cmd := exec.Command("go", "run", "../cmd/relay/main.go", "--config", "test_config.yaml")
	cmd.Dir = ".."

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start relay: %v", err)
	}
	defer cmd.Process.Kill()

	// Wait for server to start
	time.Sleep(2 * time.Second)

	// Create session
	tmpDir := t.TempDir()
	sessionReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/new",
		"params": map[string]interface{}{
			"workingDirectory": tmpDir,
		},
		"id": 1,
	}

	body, _ := json.Marshal(sessionReq)
	resp, err := http.Post("http://localhost:8080/session/new", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var sessionResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&sessionResp)

	result := sessionResp["result"].(map[string]interface{})
	sessionID := result["sessionId"].(string)

	if sessionID == "" {
		t.Error("expected sessionId")
	}

	t.Logf("Created session: %s", sessionID)
}
```

Create `tests/test_config.yaml`:

```yaml
server:
  http_port: 8080
  websocket_port: 8081
  management_port: 8082

agent:
  command: "./tests/test_agent.sh"
  args: []
```

**Step 3: Run integration tests**

```bash
cd tests
go test -v
```

Expected: PASS

**Step 4: Manual end-to-end testing**

Test with curl:

```bash
# Terminal 1: Start relay
go run cmd/relay/main.go

# Terminal 2: Test API
curl -X POST http://localhost:8080/session/new \
  -d '{"jsonrpc":"2.0","method":"session/new","params":{"workingDirectory":"/tmp"},"id":1}'
```

**Step 5: Commit**

```bash
git add tests/
git commit -m "test: add integration tests and test agent"
```

---

## Summary

This plan implements a complete ACP relay server with:

✅ Configuration management (YAML + env vars)
✅ Session management (process-per-session with stdio)
✅ HTTP API (session/new, session/prompt)
✅ WebSocket API (bidirectional streaming)
✅ LLM-optimized error handling
✅ Management API (health, config)
✅ Documentation
✅ Tests (unit + integration)

**TODO (Future):**
- Idle timeout handling for long-running tasks
- Authentication/authorization
- Multi-agent routing
- Metrics and observability
- Graceful shutdown with cleanup

**Execution Notes:**
- Follow TDD: Write test → Run (fail) → Implement → Run (pass) → Commit
- Replace `github.com/yourusername/acp-relay` with actual module path
- Test frequently with real ACP agents
- Each task should take 15-30 minutes
