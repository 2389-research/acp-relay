// ABOUTME: Management API for runtime config and health monitoring
// ABOUTME: Provides endpoints for health checks and configuration updates

package management

import (
	"encoding/json"
	"net/http"

	"github.com/harper/acp-relay/internal/config"
	"github.com/harper/acp-relay/internal/session"
)

type Server struct {
	config     *config.Config
	sessionMgr *session.Manager
	mux        *http.ServeMux
}

func NewServer(cfg *config.Config, mgr *session.Manager) *Server {
	s := &Server{
		config:     cfg,
		sessionMgr: mgr,
		mux:        http.NewServeMux(),
	}

	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/config", s.handleConfig)

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
