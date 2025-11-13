// ABOUTME: WebSocket server for bidirectional ACP communication
// ABOUTME: Handles persistent connections with streaming JSON-RPC messages

package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/harper/acp-relay/internal/db"
	"github.com/harper/acp-relay/internal/errors"
	"github.com/harper/acp-relay/internal/jsonrpc"
	"github.com/harper/acp-relay/internal/session"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // TODO: Add proper origin checking
	},
}

type Server struct {
	sessionMgr *session.Manager
}

func NewServer(mgr *session.Manager) *Server {
	return &Server{sessionMgr: mgr}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}

	//nolint:contextcheck // websocket connection outlives HTTP request context
	s.handleConnection(conn)
}

//nolint:gocognit,gocyclo,funlen // complex websocket protocol handling requiring many protocol branches
func (s *Server) handleConnection(conn *websocket.Conn) {
	defer func() { _ = conn.Close() }()

	var currentSession *session.Session
	var currentClientID string

	// Main loop: Read from WebSocket
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("websocket read error: %v", err)
			break
		}

		// Log client->relay message
		if currentSession != nil && currentSession.DB != nil {
			if err := currentSession.DB.LogMessage(currentSession.ID, db.DirectionClientToRelay, message); err != nil {
				log.Printf("[WS:%s] failed to log client->relay message: %v", currentSession.ID[:8], err)
			}
		}

		// Check if this is a request (has "method") or response (no "method")
		var msgType struct {
			Method string `json:"method"`
			ID     int    `json:"id"`
		}
		if err := json.Unmarshal(message, &msgType); err != nil {
			// Log but don't fail - we'll parse properly below
			log.Printf("[WS] failed to detect message type: %v", err)
		}

		// If no method field, this is a response from the client - forward to agent
		if msgType.Method == "" {
			if currentSession != nil {
				preview := string(message)
				if len(preview) > 100 {
					preview = preview[:100] + "..."
				}
				log.Printf("[WS:%s] Client response (id=%d) -> Agent: %s", currentSession.ID[:8], msgType.ID, preview)
				select {
				case currentSession.ToAgent <- append(message, '\n'):
					// Successfully sent
				case <-currentSession.Context.Done():
					log.Printf("[WS:%s] Session context done, cannot forward client response", currentSession.ID[:8])
				}
			}
			continue
		}

		// Parse as request
		var req jsonrpc.Request
		if err := json.Unmarshal(message, &req); err != nil {
			s.sendLLMError(conn, currentSession, currentClientID, errors.NewParseError(err.Error()), nil)
			continue
		}

		// Handle different methods
		switch req.Method {
		case "session/new":
			var params struct {
				WorkingDirectory string `json:"workingDirectory"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewInvalidParamsError("workingDirectory", "string", "invalid or missing"), req.ID)
				continue
			}

			// Use context.Background() so session outlives this WebSocket connection
			// Sessions should persist even after client disconnects (for resume)
			sess, err := s.sessionMgr.CreateSession(context.Background(), params.WorkingDirectory)
			if err != nil {
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewAgentConnectionError(params.WorkingDirectory, 1, 10000, err.Error()), req.ID)
				continue
			}

			currentSession = sess

			// Attach this client to the session FIRST
			// This ensures the client is ready to receive messages when broadcaster starts
			currentClientID = sess.AttachClient(conn)

			// THEN start broadcaster (will consume buffered messages from FromAgent)
			sess.StartBroadcaster()

			// Send response with both session ID and client ID
			result := map[string]interface{}{
				"sessionId": sess.ID,
				"clientId":  currentClientID,
			}
			s.sendResponseSafe(conn, sess, currentClientID, result, req.ID)

		case "session/resume":
			var params struct {
				SessionID string `json:"sessionId"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewInvalidParamsError("sessionId", "string", "invalid or missing"), req.ID)
				continue
			}

			// Try to get existing session
			sess, exists := s.sessionMgr.GetSession(params.SessionID)
			if !exists {
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewSessionNotFoundError(params.SessionID), req.ID)
				continue
			}

			currentSession = sess

			// Attach this client to the session
			currentClientID = sess.AttachClient(conn)
			log.Printf("[WS:%s] Client %s resuming session", sess.ID[:8], currentClientID)

			// Fetch and replay recent messages to avoid blank screen
			// Only replay agent->client messages (responses, notifications, events)
			s.replayRecentMessages(conn, sess, currentClientID, params.SessionID)

			// Send response with both session ID and client ID
			result := map[string]interface{}{
				"sessionId": sess.ID,
				"clientId":  currentClientID,
			}
			s.sendResponseSafe(conn, sess, currentClientID, result, req.ID)

		case "session/list":
			// List all sessions from database
			// TODO(security): Add authorization - currently any client can list all sessions
			sessions, err := s.sessionMgr.ListSessions()
			if err != nil {
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewInternalError(fmt.Sprintf("failed to get sessions: %v", err)), req.ID)
				continue
			}

			// Convert to JSON-friendly format
			sessionList := make([]map[string]interface{}, len(sessions))
			for i, sess := range sessions {
				sessionList[i] = map[string]interface{}{
					"id":               sess.ID,
					"agentSessionId":   sess.AgentSessionID,
					"workingDirectory": sess.WorkingDirectory,
					"createdAt":        sess.CreatedAt.Format(time.RFC3339),
					"closedAt":         nil,
					"isActive":         sess.ClosedAt == nil,
				}
				if sess.ClosedAt != nil {
					sessionList[i]["closedAt"] = sess.ClosedAt.Format(time.RFC3339)
					sessionList[i]["isActive"] = false
				}
			}

			result := map[string]interface{}{
				"sessions": sessionList,
			}
			s.sendResponseSafe(conn, currentSession, currentClientID, result, req.ID)

		case "session/history":
			// Get message history for a session
			// TODO(security): Add authorization - currently any client can read any session's history
			var params struct {
				SessionID string `json:"sessionId"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewInvalidParamsError("sessionId", "string", "invalid or missing"), req.ID)
				continue
			}

			messages, err := s.sessionMgr.GetSessionHistory(params.SessionID)
			if err != nil {
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewInternalError(fmt.Sprintf("failed to get session messages: %v", err)), req.ID)
				continue
			}

			// Convert to JSON-friendly format
			messageList := make([]map[string]interface{}, len(messages))
			for i, msg := range messages {
				messageList[i] = map[string]interface{}{
					"id":          msg.ID,
					"direction":   string(msg.Direction),
					"messageType": msg.MessageType,
					"method":      msg.Method,
					"rawMessage":  msg.RawMessage,
					"timestamp":   msg.Timestamp.Format(time.RFC3339),
				}
				if msg.JSONRPCId != nil {
					messageList[i]["jsonrpcId"] = *msg.JSONRPCId
				}
			}

			result := map[string]interface{}{
				"sessionId": params.SessionID,
				"messages":  messageList,
			}
			s.sendResponseSafe(conn, currentSession, currentClientID, result, req.ID)

		case "session/prompt":
			// Forward to agent, translating "content" to "prompt" per ACP spec
			if currentSession == nil {
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewSessionNotFoundError("no session"), req.ID)
				continue
			}

			var params struct {
				SessionID string          `json:"sessionId"`
				Content   json.RawMessage `json:"content"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewInvalidParamsError("sessionId or content", "object", "invalid or missing"), req.ID)
				continue
			}

			// Translate params from "content" to "prompt" and use agent's session ID
			agentParams := map[string]interface{}{
				"sessionId": currentSession.AgentSessionID, // Use agent's session ID, not relay's
				"prompt":    params.Content,
			}
			agentParamsJSON, err := json.Marshal(agentParams)
			if err != nil {
				log.Printf("[WS:%s] failed to marshal agent params: %v", currentSession.ID[:8], err)
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewParseError("internal error marshaling params"), req.ID)
				continue
			}

			agentReq := jsonrpc.Request{
				JSONRPC: "2.0",
				Method:  "session/prompt",
				Params:  json.RawMessage(agentParamsJSON),
				ID:      req.ID,
			}

			reqData, err := json.Marshal(agentReq)
			if err != nil {
				log.Printf("[WS:%s] failed to marshal agent request: %v", currentSession.ID[:8], err)
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewParseError("internal error marshaling request"), req.ID)
				continue
			}
			reqData = append(reqData, '\n')
			select {
			case currentSession.ToAgent <- reqData:
				// Successfully sent
			case <-currentSession.Context.Done():
				log.Printf("[WS:%s] Session context done, cannot send prompt to agent", currentSession.ID[:8])
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewAgentConnectionError("", 0, 0, "session closed"), req.ID)
			}

		default:
			// Forward other methods to agent as-is
			if currentSession == nil {
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewSessionNotFoundError("no session"), req.ID)
				continue
			}

			reqData, err := json.Marshal(req)
			if err != nil {
				log.Printf("[WS:%s] failed to marshal request: %v", currentSession.ID[:8], err)
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewParseError("internal error marshaling request"), req.ID)
				continue
			}
			reqData = append(reqData, '\n')
			select {
			case currentSession.ToAgent <- reqData:
				// Successfully sent
			case <-currentSession.Context.Done():
				log.Printf("[WS:%s] Session context done, cannot forward request to agent", currentSession.ID[:8])
				s.sendLLMError(conn, currentSession, currentClientID, errors.NewAgentConnectionError("", 0, 0, "session closed"), req.ID)
			}
		}
	}

	// Cleanup: Detach client from session (session remains active for resumption)
	if currentSession != nil && currentClientID != "" {
		currentSession.DetachClient(currentClientID)
		log.Printf("[WS:%s] Client %s disconnected, session remains active for resumption", currentSession.ID[:8], currentClientID)
	}
}

func (s *Server) sendResponseSafe(conn *websocket.Conn, sess *session.Session, clientID string, result interface{}, id *json.RawMessage) {
	resultData, err := json.Marshal(result)
	if err != nil {
		log.Printf("Failed to marshal result: %v", err)
		return
	}
	resp := jsonrpc.Response{
		JSONRPC: "2.0",
		Result:  resultData,
		ID:      id,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		return
	}

	// Use SafeWriteMessage if we have session and client ID (to avoid race with delivery goroutine)
	if sess != nil && clientID != "" {
		if err := sess.SafeWriteMessage(clientID, websocket.TextMessage, data); err != nil {
			log.Printf("Failed to send response to client %s: %v", clientID, err)
		}
	} else {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Failed to write response to websocket: %v", err)
		}
	}
}

func (s *Server) sendLLMError(conn *websocket.Conn, sess *session.Session, clientID string, err *jsonrpc.Error, id *json.RawMessage) {
	resp := jsonrpc.Response{
		JSONRPC: "2.0",
		Error:   err,
		ID:      id,
	}

	data, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		log.Printf("Failed to marshal LLM error response: %v", marshalErr)
		return
	}

	// Use SafeWriteMessage if we have session and client ID (to avoid race with delivery goroutine)
	if sess != nil && clientID != "" {
		if writeErr := sess.SafeWriteMessage(clientID, websocket.TextMessage, data); writeErr != nil {
			log.Printf("Failed to send LLM error to client %s: %v", clientID, writeErr)
		}
	} else {
		if writeErr := conn.WriteMessage(websocket.TextMessage, data); writeErr != nil {
			log.Printf("Failed to write LLM error response: %v", writeErr)
		}
	}
}

func (s *Server) replayRecentMessages(conn *websocket.Conn, sess *session.Session, clientID, sessionID string) {
	const replayMessageCount = 50
	messages, err := s.sessionMgr.GetSessionHistory(sessionID)
	if err != nil || len(messages) == 0 {
		return
	}

	// Get last N messages
	startIdx := 0
	if len(messages) > replayMessageCount {
		startIdx = len(messages) - replayMessageCount
	}
	recentMessages := messages[startIdx:]

	// Replay agent->relay and relay->client messages
	replayCount := 0
	for _, msg := range recentMessages {
		if msg.Direction == db.DirectionAgentToRelay || msg.Direction == db.DirectionRelayToClient {
			if len(msg.RawMessage) > 0 {
				_ = conn.WriteMessage(websocket.TextMessage, []byte(msg.RawMessage))
				replayCount++
			}
		}
	}

	if replayCount > 0 {
		log.Printf("[WS:%s] Replayed %d recent messages to client %s", sess.ID[:8], replayCount, clientID)
	}
}
