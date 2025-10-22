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
		writeLLMError(w, errors.NewInternalError("agent response timeout after 30 seconds"), req.ID)
	case <-r.Context().Done():
		writeLLMError(w, errors.NewInternalError("request cancelled by client"), req.ID)
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
