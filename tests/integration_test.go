// ABOUTME: Integration tests for ACP relay server end-to-end functionality
// ABOUTME: Tests full HTTP flow with real server and test agent

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/harper/acp-relay/internal/config"
	"github.com/harper/acp-relay/internal/db"
	httpserver "github.com/harper/acp-relay/internal/http"
	mgmtserver "github.com/harper/acp-relay/internal/management"
	"github.com/harper/acp-relay/internal/session"
	wsserver "github.com/harper/acp-relay/internal/websocket"
)

func startTestServer(t *testing.T, cfg *config.Config) (cleanup func()) {
	// Open database
	database, err := db.Open(cfg.Database.Path)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Create session manager
	sessionMgr := session.NewManager(session.ManagerConfig{
		Mode:            cfg.Agent.Mode,
		AgentCommand:    cfg.Agent.Command,
		AgentArgs:       cfg.Agent.Args,
		AgentEnv:        cfg.Agent.Env,
		ContainerConfig: cfg.Agent.Container,
	}, database)

	// Create servers
	httpSrv := httpserver.NewServer(sessionMgr)
	wsSrv := wsserver.NewServer(sessionMgr)
	mgmtSrv := mgmtserver.NewServer(cfg, sessionMgr, database)

	// Start servers in goroutines
	httpListener, _ := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Server.HTTPHost, cfg.Server.HTTPPort))
	wsListener, _ := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Server.WebSocketHost, cfg.Server.WebSocketPort))
	mgmtListener, _ := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Server.ManagementHost, cfg.Server.ManagementPort))

	go http.Serve(httpListener, httpSrv)
	go http.Serve(wsListener, wsSrv)
	go http.Serve(mgmtListener, mgmtSrv)

	// Wait for servers to be ready
	time.Sleep(100 * time.Millisecond)

	return func() {
		httpListener.Close()
		wsListener.Close()
		mgmtListener.Close()
		database.Close()
	}
}

func TestFullHTTPFlow(t *testing.T) {
	// Load test config
	cfg, err := config.Load("test_config.yaml")
	if err != nil {
		t.Fatalf("failed to load test config: %v", err)
	}

	// Start test server
	cleanup := startTestServer(t, cfg)
	defer cleanup()

	// Wait a bit for server to be ready
	time.Sleep(200 * time.Millisecond)

	// Test 1: Health check endpoint
	t.Run("health_check", func(t *testing.T) {
		resp, err := http.Get("http://127.0.0.1:18082/api/health")
		if err != nil {
			t.Fatalf("failed to check health: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var health map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
			t.Fatalf("failed to decode health response: %v", err)
		}

		if health["status"] != "healthy" {
			t.Errorf("expected status healthy, got %v", health["status"])
		}

		t.Logf("Health check passed: %+v", health)
	})

	// Test 2: Create session via HTTP
	t.Run("create_session", func(t *testing.T) {
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
		resp, err := http.Post("http://127.0.0.1:18080/session/new", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var sessionResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
			t.Fatalf("failed to decode session response: %v", err)
		}

		if sessionResp["jsonrpc"] != "2.0" {
			t.Errorf("expected jsonrpc 2.0, got %v", sessionResp["jsonrpc"])
		}

		result, ok := sessionResp["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected result object, got %v", sessionResp)
		}

		sessionID, ok := result["sessionId"].(string)
		if !ok || sessionID == "" {
			t.Errorf("expected sessionId in result, got %v", result)
		}

		t.Logf("Created session: %s", sessionID)
	})

	// Test 3: Send prompt to session
	t.Run("send_prompt", func(t *testing.T) {
		t.Skip("Skipping prompt test - requires more complex agent interaction")
		// NOTE: The test agent echoes back JSON-RPC messages, but the session/prompt
		// endpoint expects specific response formats. This test needs a more sophisticated
		// test agent that understands the ACP protocol properly.
	})

	// Test 4: Error handling - invalid session ID
	t.Run("invalid_session", func(t *testing.T) {
		promptReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "session/prompt",
			"params": map[string]interface{}{
				"sessionId": "invalid_session_id",
				"content": []map[string]interface{}{
					{"type": "text", "text": "Hello"},
				},
			},
			"id": 3,
		}

		body, _ := json.Marshal(promptReq)
		resp, err := http.Post("http://127.0.0.1:18080/session/prompt", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("failed to send prompt: %v", err)
		}
		defer resp.Body.Close()

		var promptResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&promptResp)

		// Should get an error response
		if promptResp["error"] == nil {
			t.Error("expected error in response")
		}

		errorObj := promptResp["error"].(map[string]interface{})
		if errorObj["message"] == nil {
			t.Error("expected error message")
		}

		t.Logf("Error response: %+v", errorObj)
	})
}
