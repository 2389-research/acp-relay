// ABOUTME: HTTP server for handling REST-style ACP requests
// ABOUTME: Routes requests to appropriate handlers

package http

import (
	"net/http"

	"github.com/harper/acp-relay/internal/session"
)

type Server struct {
	sessionMgr *session.Manager
	mux        *http.ServeMux
}

func NewServer(mgr *session.Manager) *Server {
	s := &Server{
		sessionMgr: mgr,
		mux:        http.NewServeMux(),
	}

	s.mux.HandleFunc("/session/new", s.handleSessionNew)
	s.mux.HandleFunc("/session/prompt", s.handleSessionPrompt)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
