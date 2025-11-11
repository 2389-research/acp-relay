// ABOUTME: Unit tests for WebSocket relay client
// ABOUTME: Tests connection, message sending/receiving, and reconnection logic
package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func mockRelayHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	// Echo messages back
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func TestRelayClient_Connect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockRelayHandler))
	defer server.Close()

	wsURL := "ws" + server.URL[4:] // Replace http with ws

	client := NewRelayClient(wsURL)
	err := client.Connect()
	require.NoError(t, err)

	defer func() { _ = client.Close() }()
	assert.True(t, client.IsConnected())
}

func TestRelayClient_SendReceive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockRelayHandler))
	defer server.Close()

	wsURL := "ws" + server.URL[4:]

	client := NewRelayClient(wsURL)
	require.NoError(t, client.Connect())
	defer func() { _ = client.Close() }()

	// Send message
	testMsg := []byte(`{"jsonrpc":"2.0","method":"test","id":1}`)
	err := client.Send(testMsg)
	require.NoError(t, err)

	// Receive echo
	select {
	case msg := <-client.Incoming():
		assert.Equal(t, testMsg, msg)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestRelayClient_ErrorChannel(t *testing.T) {
	// Connect to invalid URL
	client := NewRelayClient("ws://localhost:99999")
	err := client.Connect()

	assert.Error(t, err)
	assert.False(t, client.IsConnected())
}

// Mock handler that responds to session/resume requests.
func mockResumeHandler(_ *testing.T, shouldSucceed bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}

			// Parse incoming message to check if it's session/resume
			var req map[string]interface{}
			if err := json.Unmarshal(msg, &req); err != nil {
				continue
			}

			method, _ := req["method"].(string)
			id, _ := req["id"].(float64)

			if method == "session/resume" {
				var response []byte
				if shouldSucceed {
					// Send success response
					params := req["params"].(map[string]interface{})
					sessionID := params["sessionId"].(string)
					response = []byte(`{"jsonrpc":"2.0","id":` + fmt.Sprintf("%.0f", id) + `,"result":{"sessionId":"` + sessionID + `"}}`)
				} else {
					// Send error response
					response = []byte(`{"jsonrpc":"2.0","id":` + fmt.Sprintf("%.0f", id) + `,"error":{"code":-32000,"message":"session not found"}}`)
				}

				if err := conn.WriteMessage(websocket.TextMessage, response); err != nil {
					return
				}
			}
		}
	}
}

func TestRelayClient_ResumeSession_Success(t *testing.T) {
	server := httptest.NewServer(mockResumeHandler(t, true))
	defer server.Close()

	wsURL := "ws" + server.URL[4:]

	client := NewRelayClient(wsURL)
	require.NoError(t, client.Connect())
	defer func() { _ = client.Close() }()

	// Give connection time to stabilize
	time.Sleep(50 * time.Millisecond)

	// Test successful resume
	err := client.ResumeSession("test-session-123")
	assert.NoError(t, err)
}

func TestRelayClient_ResumeSession_Failure(t *testing.T) {
	server := httptest.NewServer(mockResumeHandler(t, false))
	defer server.Close()

	wsURL := "ws" + server.URL[4:]

	client := NewRelayClient(wsURL)
	require.NoError(t, client.Connect())
	defer func() { _ = client.Close() }()

	// Give connection time to stabilize
	time.Sleep(50 * time.Millisecond)

	// Test failed resume (session not found)
	err := client.ResumeSession("nonexistent-session")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestRelayClient_ResumeSession_Timeout(t *testing.T) {
	// Handler that never responds
	timeoutHandler := func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		// Read messages but never respond
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
			// Don't send response, causing timeout
		}
	}

	server := httptest.NewServer(http.HandlerFunc(timeoutHandler))
	defer server.Close()

	wsURL := "ws" + server.URL[4:]

	client := NewRelayClient(wsURL)
	require.NoError(t, client.Connect())
	defer func() { _ = client.Close() }()

	// Give connection time to stabilize
	time.Sleep(50 * time.Millisecond)

	// Test timeout - should fail within 5 seconds
	start := time.Now()
	err := client.ResumeSession("test-session")
	duration := time.Since(start)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
	assert.LessOrEqual(t, duration, 6*time.Second) // Allow slight overhead
}
