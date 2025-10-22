// ABOUTME: HTTP handlers for ACP JSON-RPC endpoints
// ABOUTME: Translates HTTP requests to JSON-RPC messages

package http

import (
	"encoding/json"
	"fmt"
	"io"
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
	sess, err := s.sessionMgr.CreateSession(r.Context(), params.WorkingDirectory)
	if err != nil {
		writeLLMError(w, errors.NewAgentConnectionError(params.WorkingDirectory, 1, 10000, err.Error()), req.ID)
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

	sess.ToAgent <- reqData

	// Collect all messages until we get the response to our request
	// Per ACP spec: agent may send multiple session/update notifications,
	// then finally responds to the session/prompt request with a stopReason
	var messages []json.RawMessage
	requestID := string(*req.ID)

	timeout := time.After(30 * time.Second)
	for {
		select {
		case respData := <-sess.FromAgent:
			messages = append(messages, respData)

			// Check if this is the final response (has matching ID)
			var resp map[string]interface{}
			if err := json.Unmarshal(respData, &resp); err == nil {
				if idField, ok := resp["id"]; ok {
					// This is a response (not a notification)
					idBytes, _ := json.Marshal(idField)
					if string(idBytes) == requestID {
						// This is the response to our request - turn is complete
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

		case <-timeout:
			writeLLMError(w, errors.NewInternalError("agent response timeout after 30 seconds"), req.ID)
			return
		case <-r.Context().Done():
			writeLLMError(w, errors.NewInternalError("request cancelled by client"), req.ID)
			return
		}
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
