// ABOUTME: Unit tests for WebSocket relay client
// ABOUTME: Tests connection, message sending/receiving, and reconnection logic
package client

import (
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
	defer conn.Close()

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

	defer client.Close()
	assert.True(t, client.IsConnected())
}

func TestRelayClient_SendReceive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockRelayHandler))
	defer server.Close()

	wsURL := "ws" + server.URL[4:]

	client := NewRelayClient(wsURL)
	require.NoError(t, client.Connect())
	defer client.Close()

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
