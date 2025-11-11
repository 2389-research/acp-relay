// ABOUTME: ACP protocol session/new implementation
// ABOUTME: Forwards session creation to agent and captures their session ID

package session

import (
	"encoding/json"
	"fmt"
	"time"
)

// SendSessionNew sends session/new to the agent and captures their session ID
// This must be called after SendInitialize per the ACP protocol spec.
func (s *Session) SendSessionNew(workingDir string) error {
	// Build session/new request per ACP spec
	sessionNewReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/new",
		"params": map[string]interface{}{
			"cwd":        workingDir,
			"mcpServers": []interface{}{},
		},
		"id": 1, // Use ID 1 for session/new
	}

	// Marshal to JSON
	data, err := json.Marshal(sessionNewReq)
	if err != nil {
		return fmt.Errorf("failed to marshal session/new request: %w", err)
	}

	// Add newline for line-delimited JSON
	data = append(data, '\n')

	// Send to agent with timeout
	select {
	case s.ToAgent <- data:
		// Successfully sent
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout sending session/new to agent")
	case <-s.Context.Done():
		return fmt.Errorf("session cancelled while sending session/new")
	}

	// Wait for session/new response with timeout
	select {
	case respData := <-s.FromAgent:
		var resp map[string]interface{}
		if err := json.Unmarshal(respData, &resp); err != nil {
			return fmt.Errorf("failed to parse session/new response: %w", err)
		}

		// Check for error in response
		if errObj, ok := resp["error"]; ok {
			return fmt.Errorf("agent returned error on session/new: %v", errObj)
		}

		// Parse result to get sessionId
		result, ok := resp["result"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("session/new response missing result field")
		}

		agentSessionID, ok := result["sessionId"].(string)
		if !ok {
			return fmt.Errorf("session/new response missing sessionId in result")
		}

		// Store the agent's session ID
		s.AgentSessionID = agentSessionID

		return nil

	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for session/new response from agent")
	case <-s.Context.Done():
		return fmt.Errorf("session cancelled while waiting for session/new response")
	}
}
