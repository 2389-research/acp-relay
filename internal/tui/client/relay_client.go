// ABOUTME: WebSocket client for communicating with acp-relay server
// ABOUTME: Manages connection lifecycle, message passing via channels, and auto-reconnection
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type pendingRequest struct {
	responseChan chan map[string]interface{}
	timeout      *time.Timer
}

type RelayClient struct {
	url             string
	conn            *websocket.Conn
	mu              sync.RWMutex
	incoming        chan []byte
	outgoing        chan []byte
	errors          chan error
	done            chan struct{}
	closed          bool
	messageID       uint64
	pendingRequests map[uint64]*pendingRequest
	pendingMu       sync.Mutex
}

func NewRelayClient(url string) *RelayClient {
	return &RelayClient{
		url:             url,
		incoming:        make(chan []byte, 100),
		outgoing:        make(chan []byte, 100),
		errors:          make(chan error, 10),
		done:            make(chan struct{}),
		pendingRequests: make(map[uint64]*pendingRequest),
	}
}

func (c *RelayClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Prevent double connection
	if c.conn != nil && !c.closed {
		return fmt.Errorf("already connected")
	}

	// Add 30-second connection timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.url, nil) //nolint:bodyclose // websocket connection, not HTTP response //nolint:bodyclose // websocket connection, not HTTP response
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	c.conn = conn
	c.closed = false

	// Start read/write goroutines
	go c.readLoop()
	go c.writeLoop()

	return nil
}

func (c *RelayClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil && !c.closed
}

func (c *RelayClient) Send(msg []byte) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	select {
	case c.outgoing <- msg:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("send timeout")
	}
}

func (c *RelayClient) Incoming() <-chan []byte {
	return c.incoming
}

func (c *RelayClient) Errors() <-chan error {
	return c.errors
}

func (c *RelayClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	close(c.done)

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

func (c *RelayClient) readLoop() {
	defer func() {
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()
	}()

	for {
		select {
		case <-c.done:
			return
		default:
		}

		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			select {
			case c.errors <- fmt.Errorf("read: %w", err):
			case <-c.done:
			}
			return
		}

		// Try to route message to pending request
		if c.routeToRequest(msg) {
			continue
		}

		// Otherwise, send to incoming channel for main loop
		select {
		case c.incoming <- msg:
		case <-c.done:
			return
		}
	}
}

// routeToRequest attempts to route a message to a pending request.
// Returns true if message was routed, false otherwise.
func (c *RelayClient) routeToRequest(msg []byte) bool {
	// Parse message to check for JSON-RPC response with ID
	var response map[string]interface{}
	if err := json.Unmarshal(msg, &response); err != nil {
		return false
	}

	// Check if this is a response (has an ID)
	responseID, hasID := response["id"]
	if !hasID {
		return false // This is a notification, not a response
	}

	// Convert ID to uint64
	var msgID uint64
	switch v := responseID.(type) {
	case float64:
		msgID = uint64(v)
	case int:
		if v < 0 {
			return false
		}
		msgID = uint64(v) //nolint:gosec // Negative check above prevents overflow
	case int64:
		if v < 0 {
			return false
		}
		msgID = uint64(v) //nolint:gosec // Negative check above prevents overflow
	case uint64:
		msgID = v
	default:
		return false
	}

	// Check if we have a pending request for this ID
	c.pendingMu.Lock()
	pending, exists := c.pendingRequests[msgID]
	if exists {
		delete(c.pendingRequests, msgID)
	}
	c.pendingMu.Unlock()

	if !exists {
		return false
	}

	// Stop the timeout timer
	pending.timeout.Stop()

	// Send response to waiting goroutine
	select {
	case pending.responseChan <- response:
	default:
		// Channel closed or receiver gone
	}

	return true
}

func (c *RelayClient) writeLoop() {
	for {
		select {
		case <-c.done:
			return
		case msg := <-c.outgoing:
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				select {
				case c.errors <- fmt.Errorf("write: %w", err):
				case <-c.done:
				}
				return
			}
		}
	}
}

// ResumeSession sends a session/resume JSON-RPC request and waits for response.
//
//nolint:gocognit,nestif,funlen // JSON-RPC request/response handling requires setup, send, cleanup logic
func (c *RelayClient) ResumeSession(sessionID string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	// Generate unique message ID
	msgID := atomic.AddUint64(&c.messageID, 1)

	// Create response channel and register pending request
	responseChan := make(chan map[string]interface{}, 1)
	timeoutTimer := time.NewTimer(5 * time.Second)

	pending := &pendingRequest{
		responseChan: responseChan,
		timeout:      timeoutTimer,
	}

	c.pendingMu.Lock()
	c.pendingRequests[msgID] = pending
	c.pendingMu.Unlock()

	// Cleanup function
	cleanup := func() {
		c.pendingMu.Lock()
		delete(c.pendingRequests, msgID)
		c.pendingMu.Unlock()
		timeoutTimer.Stop()
	}

	// Construct JSON-RPC request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/resume",
		"params": map[string]interface{}{
			"sessionId": sessionID,
		},
		"id": msgID,
	}

	jsonMsg, err := json.Marshal(request)
	if err != nil {
		cleanup()
		return fmt.Errorf("marshal request: %w", err)
	}

	// Send the request
	if err := c.Send(jsonMsg); err != nil {
		cleanup()
		return fmt.Errorf("send request: %w", err)
	}

	// Wait for response or timeout
	select {
	case response := <-responseChan:
		// Check for error
		if errorData, hasError := response["error"]; hasError {
			if errorMap, ok := errorData.(map[string]interface{}); ok {
				if message, ok := errorMap["message"].(string); ok {
					return fmt.Errorf("resume failed: %s", message)
				}
			}
			return fmt.Errorf("resume failed with unknown error")
		}

		// Check for success result
		if _, hasResult := response["result"]; hasResult {
			return nil // Success
		}

		return fmt.Errorf("invalid response format")

	case <-timeoutTimer.C:
		cleanup()
		return fmt.Errorf("timeout waiting for session/resume response")
	}
}
