// ABOUTME: WebSocket client for communicating with acp-relay server
// ABOUTME: Manages connection lifecycle, message passing via channels, and auto-reconnection
package client

import (
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
)

type RelayClient struct {
	url      string
	conn     *websocket.Conn
	mu       sync.RWMutex
	incoming chan []byte
	outgoing chan []byte
	errors   chan error
	done     chan struct{}
	closed   bool
}

func NewRelayClient(url string) *RelayClient {
	return &RelayClient{
		url:      url,
		incoming: make(chan []byte, 100),
		outgoing: make(chan []byte, 100),
		errors:   make(chan error, 10),
		done:     make(chan struct{}),
	}
}

func (c *RelayClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, _, err := websocket.DefaultDialer.Dial(c.url, nil)
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

		select {
		case c.incoming <- msg:
		case <-c.done:
			return
		}
	}
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

// Bubbletea message types for async communication

type RelayMessageMsg struct {
	Data []byte
}

type RelayErrorMsg struct {
	Err error
}

type RelayDisconnectedMsg struct{}

// WaitForMessage returns a Cmd that waits for the next message
func (c *RelayClient) WaitForMessage() func() tea.Msg {
	return func() tea.Msg {
		select {
		case msg := <-c.incoming:
			return RelayMessageMsg{Data: msg}
		case err := <-c.errors:
			return RelayErrorMsg{Err: err}
		}
	}
}
