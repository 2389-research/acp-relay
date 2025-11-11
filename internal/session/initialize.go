// ABOUTME: ACP protocol initialize handshake implementation
// ABOUTME: Sends required initialize message to agent subprocess on session creation

package session

import (
	"encoding/json"
	"fmt"
	"time"
)

// SendInitialize sends the required ACP initialize message to the agent
// This must be called before any other messages per the ACP protocol spec.
func (s *Session) SendInitialize() error {
	// Build initialize request per ACP spec
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": 1,
			"clientInfo": map[string]interface{}{
				"name":    "acp-relay",
				"version": "0.1.0",
			},
			"capabilities": map[string]interface{}{
				// Basic capabilities - no file system or terminal access for now
			},
		},
		"id": 0, // Use ID 0 for initialize
	}

	// Marshal to JSON
	data, err := json.Marshal(initReq)
	if err != nil {
		return fmt.Errorf("failed to marshal initialize request: %w", err)
	}

	// Add newline for line-delimited JSON
	data = append(data, '\n')

	// Send to agent with timeout
	select {
	case s.ToAgent <- data:
		// Successfully sent
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout sending initialize to agent")
	case <-s.Context.Done():
		return fmt.Errorf("session cancelled while sending initialize")
	}

	// Wait for initialize response with timeout
	select {
	case respData := <-s.FromAgent:
		var resp map[string]interface{}
		if err := json.Unmarshal(respData, &resp); err != nil {
			return fmt.Errorf("failed to parse initialize response: %w", err)
		}

		// Check for error in response
		if errObj, ok := resp["error"]; ok {
			return fmt.Errorf("agent returned error on initialize: %v", errObj)
		}

		// Verify we got a result
		if _, ok := resp["result"]; !ok {
			return fmt.Errorf("initialize response missing result field")
		}

		return nil

	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for initialize response from agent")
	case <-s.Context.Done():
		return fmt.Errorf("session cancelled while waiting for initialize response")
	}
}
