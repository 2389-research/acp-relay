// ABOUTME: Management API for runtime config and health monitoring
// ABOUTME: Provides endpoints for health checks and configuration updates

package management

import (
	"encoding/json"
	"net/http"

	"github.com/harper/acp-relay/internal/config"
	"github.com/harper/acp-relay/internal/db"
	"github.com/harper/acp-relay/internal/session"
)

type Server struct {
	config     *config.Config
	sessionMgr *session.Manager
	db         *db.DB
	mux        *http.ServeMux
}

func NewServer(cfg *config.Config, mgr *session.Manager, database *db.DB) *Server {
	s := &Server{
		config:     cfg,
		sessionMgr: mgr,
		db:         database,
		mux:        http.NewServeMux(),
	}

	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/config", s.handleConfig)
	s.mux.HandleFunc("/api/sessions", s.handleSessions)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":        "healthy",
		"agent_command": s.config.Agent.Command,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.config)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Enable CORS for web interface
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	sessions, err := s.db.GetAllSessions()
	if err != nil {
		http.Error(w, "failed to get sessions", http.StatusInternalServerError)
		return
	}

	// Convert to JSON-friendly format with isActive flag
	type SessionResponse struct {
		ID               string  `json:"id"`
		AgentSessionID   string  `json:"agentSessionId,omitempty"`
		WorkingDirectory string  `json:"workingDirectory"`
		CreatedAt        string  `json:"createdAt"`
		ClosedAt         *string `json:"closedAt,omitempty"`
		IsActive         bool    `json:"isActive"`
	}

	response := make([]SessionResponse, 0, len(sessions))
	for _, s := range sessions {
		closedAt := (*string)(nil)
		if s.ClosedAt != nil {
			closedAtStr := s.ClosedAt.Format("2006-01-02 15:04:05")
			closedAt = &closedAtStr
		}

		response = append(response, SessionResponse{
			ID:               s.ID,
			AgentSessionID:   s.AgentSessionID,
			WorkingDirectory: s.WorkingDirectory,
			CreatedAt:        s.CreatedAt.Format("2006-01-02 15:04:05"),
			ClosedAt:         closedAt,
			IsActive:         s.ClosedAt == nil,
		})
	}

	json.NewEncoder(w).Encode(response)
}
