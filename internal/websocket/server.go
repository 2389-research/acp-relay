// ABOUTME: WebSocket server for bidirectional ACP communication
// ABOUTME: Handles persistent connections with streaming JSON-RPC messages

package websocket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

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

	s.handleConnection(conn)
}

func (s *Server) handleConnection(conn *websocket.Conn) {
	defer conn.Close()

	var currentSession *session.Session
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Goroutine: Read from agent and send to WebSocket
	fromAgent := make(chan []byte, 10)
	go func() {
		for {
			select {
			case msg := <-fromAgent:
				// Log relay->client message
				if currentSession != nil && currentSession.DB != nil {
					if err := currentSession.DB.LogMessage(currentSession.ID, db.DirectionRelayToClient, msg); err != nil {
						log.Printf("[WS:%s] failed to log relay->client message: %v", currentSession.ID[:8], err)
					}
				}

				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					log.Printf("websocket write error: %v", err)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

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
		json.Unmarshal(message, &msgType)

		// If no method field, this is a response from the client - forward to agent
		if msgType.Method == "" {
			if currentSession != nil {
				preview := string(message)
				if len(preview) > 100 {
					preview = preview[:100] + "..."
				}
				log.Printf("[WS:%s] Client response (id=%d) -> Agent: %s", currentSession.ID[:8], msgType.ID, preview)
				currentSession.ToAgent <- append(message, '\n')
			}
			continue
		}

		// Parse as request
		var req jsonrpc.Request
		if err := json.Unmarshal(message, &req); err != nil {
			s.sendLLMError(conn, errors.NewParseError(err.Error()), nil)
			continue
		}

		// Handle different methods
		switch req.Method {
		case "session/new":
			var params struct {
				WorkingDirectory string `json:"workingDirectory"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				s.sendLLMError(conn, errors.NewInvalidParamsError("workingDirectory", "string", "invalid or missing"), req.ID)
				continue
			}

			sess, err := s.sessionMgr.CreateSession(ctx, params.WorkingDirectory)
			if err != nil {
				s.sendLLMError(conn, errors.NewAgentConnectionError(params.WorkingDirectory, 1, 10000, err.Error()), req.ID)
				continue
			}

			currentSession = sess

			// Start forwarding agent messages to WebSocket
			go func() {
				for msg := range sess.FromAgent {
					fromAgent <- msg
				}
			}()

			// Send response
			result := map[string]interface{}{"sessionId": sess.ID}
			s.sendResponse(conn, result, req.ID)

		case "session/resume":
			var params struct {
				SessionID string `json:"sessionId"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				s.sendLLMError(conn, errors.NewInvalidParamsError("sessionId", "string", "invalid or missing"), req.ID)
				continue
			}

			// Try to get existing session
			sess, exists := s.sessionMgr.GetSession(params.SessionID)
			if !exists {
				s.sendLLMError(conn, errors.NewSessionNotFoundError(params.SessionID), req.ID)
				continue
			}

			currentSession = sess
			log.Printf("[WS:%s] Client resuming session", sess.ID[:8])

			// Start forwarding agent messages to WebSocket
			go func() {
				for msg := range sess.FromAgent {
					fromAgent <- msg
				}
			}()

			// Send response with session ID
			result := map[string]interface{}{"sessionId": sess.ID}
			s.sendResponse(conn, result, req.ID)

		case "session/prompt":
			// Forward to agent, translating "content" to "prompt" per ACP spec
			if currentSession == nil {
				s.sendLLMError(conn, errors.NewSessionNotFoundError("no session"), req.ID)
				continue
			}

			var params struct {
				SessionID string          `json:"sessionId"`
				Content   json.RawMessage `json:"content"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				s.sendLLMError(conn, errors.NewInvalidParamsError("sessionId or content", "object", "invalid or missing"), req.ID)
				continue
			}

			// Translate params from "content" to "prompt" and use agent's session ID
			agentParams := map[string]interface{}{
				"sessionId": currentSession.AgentSessionID, // Use agent's session ID, not relay's
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
			currentSession.ToAgent <- reqData

		default:
			// Forward other methods to agent as-is
			if currentSession == nil {
				s.sendLLMError(conn, errors.NewSessionNotFoundError("no session"), req.ID)
				continue
			}

			reqData, _ := json.Marshal(req)
			reqData = append(reqData, '\n')
			currentSession.ToAgent <- reqData
		}
	}

	// Cleanup: Don't close the session, just detach from it
	// This allows the session to be resumed later
	if currentSession != nil {
		log.Printf("[WS:%s] Client disconnected, session remains active for resumption", currentSession.ID[:8])
	}
}

func (s *Server) sendResponse(conn *websocket.Conn, result interface{}, id *json.RawMessage) {
	resultData, _ := json.Marshal(result)
	resp := jsonrpc.Response{
		JSONRPC: "2.0",
		Result:  resultData,
		ID:      id,
	}

	data, _ := json.Marshal(resp)
	conn.WriteMessage(websocket.TextMessage, data)
}

func (s *Server) sendError(conn *websocket.Conn, code int, message string, id *json.RawMessage) {
	resp := jsonrpc.Response{
		JSONRPC: "2.0",
		Error: &jsonrpc.Error{
			Code:    code,
			Message: message,
		},
		ID: id,
	}

	data, _ := json.Marshal(resp)
	conn.WriteMessage(websocket.TextMessage, data)
}

func (s *Server) sendLLMError(conn *websocket.Conn, err *jsonrpc.Error, id *json.RawMessage) {
	resp := jsonrpc.Response{
		JSONRPC: "2.0",
		Error:   err,
		ID:      id,
	}

	data, _ := json.Marshal(resp)
	conn.WriteMessage(websocket.TextMessage, data)
}
