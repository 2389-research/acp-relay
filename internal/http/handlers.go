// ABOUTME: HTTP handlers for ACP JSON-RPC endpoints
// ABOUTME: Translates HTTP requests to JSON-RPC messages

package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/harper/acp-relay/internal/errors"
	"github.com/harper/acp-relay/internal/jsonrpc"
)

func (s *Server) handleSessionNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeLLMError(w, errors.NewInvalidRequestError(fmt.Sprintf("failed to read body: %v", err)), nil)
		return
	}
	defer r.Body.Close()

	var req jsonrpc.Request
	if err := json.Unmarshal(body, &req); err != nil {
		writeLLMError(w, errors.NewParseError(err.Error()), nil)
		return
	}

	// Parse params
	var params struct {
		WorkingDirectory string `json:"workingDirectory"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeLLMError(w, errors.NewInvalidParamsError("workingDirectory", "string", "invalid or missing"), req.ID)
		return
	}

	// Create session
	// Use context.Background() because session should outlive this HTTP request
	sess, err := s.sessionMgr.CreateSession(context.Background(), params.WorkingDirectory)
	if err != nil {
		writeLLMError(w, errors.NewAgentConnectionError(params.WorkingDirectory, 1, 10000, err.Error()), req.ID)
		return
	}

	// Start draining FromAgent into MessageBuffer to prevent channel blocking
	// HTTP is stateless so we can't stream messages like WebSocket does
	go func() {
		log.Printf("[HTTP:%s] Starting drain goroutine", sess.ID[:8])
		drainCount := 0
		for {
			select {
			case msg := <-sess.FromAgent:
				drainCount++
				preview := string(msg)
				if len(preview) > 100 {
					preview = preview[:100] + "..."
				}
				log.Printf("[HTTP:%s] Drain #%d received from FromAgent: %s", sess.ID[:8], drainCount, preview)

				sess.BufferMutex.Lock()
				sess.MessageBuffer = append(sess.MessageBuffer, msg)
				bufLen := len(sess.MessageBuffer)
				sess.BufferMutex.Unlock()

				log.Printf("[HTTP:%s] Drain #%d added to buffer (total buffered: %d)", sess.ID[:8], drainCount, bufLen)

			case <-sess.Context.Done():
				log.Printf("[HTTP:%s] Drain goroutine stopping, drained %d messages", sess.ID[:8], drainCount)
				return
			}
		}
	}()

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
		writeLLMError(w, errors.NewInvalidRequestError(fmt.Sprintf("failed to read body: %v", err)), nil)
		return
	}
	defer r.Body.Close()

	var req jsonrpc.Request
	if err := json.Unmarshal(body, &req); err != nil {
		writeLLMError(w, errors.NewParseError(err.Error()), nil)
		return
	}

	// Parse params
	var params struct {
		SessionID string          `json:"sessionId"`
		Content   json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeLLMError(w, errors.NewInvalidParamsError("sessionId or content", "object", "invalid or missing"), req.ID)
		return
	}

	// Get session
	sess, exists := s.sessionMgr.GetSession(params.SessionID)
	if !exists {
		writeLLMError(w, errors.NewSessionNotFoundError(params.SessionID), req.ID)
		return
	}

	// Forward request to agent, translating "content" to "prompt" and using agent's session ID
	agentParams := map[string]interface{}{
		"sessionId": sess.AgentSessionID, // Use agent's session ID, not relay's
		"prompt":    json.RawMessage(params.Content),
	}
	agentParamsJSON, _ := json.Marshal(agentParams)

	agentReq := jsonrpc.Request{
		JSONRPC: "2.0",
		Method:  "session/prompt",
		Params:  json.RawMessage(agentParamsJSON),
		ID:      req.ID,
	}

	reqData, _ := json.Marshal(agentReq)
	reqData = append(reqData, '\n')

	log.Printf("[HTTP:%s] Sending prompt to agent (reqID=%s)", params.SessionID[:8], string(*req.ID))
	sess.ToAgent <- reqData
	log.Printf("[HTTP:%s] Prompt sent to ToAgent channel", params.SessionID[:8])

	// Collect all messages until we get the response to our request
	// Per ACP spec: agent may send multiple session/update notifications,
	// then finally responds to the session/prompt request with a stopReason
	//
	// Note: Messages are buffered in sess.MessageBuffer by a background goroutine
	// to prevent channel blocking since HTTP is stateless
	requestID := string(*req.ID)
	startTime := time.Now()
	timeout := 30 * time.Second
	pollCount := 0

	log.Printf("[HTTP:%s] Starting poll loop, looking for reqID=%s", params.SessionID[:8], requestID)

	for {
		pollCount++

		// Check for timeout
		elapsed := time.Since(startTime)
		if elapsed > timeout {
			sess.BufferMutex.Lock()
			bufSize := len(sess.MessageBuffer)
			sess.BufferMutex.Unlock()
			log.Printf("[HTTP:%s] TIMEOUT after %v, %d polls, buffer size: %d", params.SessionID[:8], elapsed, pollCount, bufSize)
			writeLLMError(w, errors.NewInternalError("agent response timeout after 30 seconds"), req.ID)
			return
		}

		// Check if request was cancelled
		select {
		case <-r.Context().Done():
			writeLLMError(w, errors.NewInternalError("request cancelled by client"), req.ID)
			return
		default:
		}

		// Check message buffer for response
		sess.BufferMutex.Lock()
		bufSize := len(sess.MessageBuffer)
		messages := make([]json.RawMessage, bufSize)
		for i, msg := range sess.MessageBuffer {
			messages[i] = json.RawMessage(msg)
		}
		sess.BufferMutex.Unlock()

		if pollCount == 1 || pollCount%100 == 0 {
			log.Printf("[HTTP:%s] Poll #%d: buffer=%d messages, elapsed=%v", params.SessionID[:8], pollCount, bufSize, elapsed)
		}

		// Look for the final response with matching ID
		for _, respData := range messages {
			var resp map[string]interface{}
			if err := json.Unmarshal(respData, &resp); err == nil {
				if idField, ok := resp["id"]; ok {
					// This is a response (not a notification)
					idBytes, _ := json.Marshal(idField)
					msgID := string(idBytes)
					if msgID == requestID {
						// This is the response to our request - turn is complete
						log.Printf("[HTTP:%s] ✓ Found matching response! Poll #%d, returning %d messages", params.SessionID[:8], pollCount, len(messages))
						w.Header().Set("Content-Type", "application/json")
						// Return all messages as a JSON array
						w.Write([]byte("["))
						for i, msg := range messages {
							if i > 0 {
								w.Write([]byte(","))
							}
							w.Write(msg)
						}
						w.Write([]byte("]"))
						return
					}
				}
			}
		}

		// Sleep briefly before checking again
		time.Sleep(100 * time.Millisecond)
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

func writeLLMError(w http.ResponseWriter, err *jsonrpc.Error, id *json.RawMessage) {
	resp := jsonrpc.Response{
		JSONRPC: "2.0",
		Error:   err,
		ID:      id,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors still return 200
	json.NewEncoder(w).Encode(resp)
}
