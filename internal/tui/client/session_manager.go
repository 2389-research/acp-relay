// ABOUTME: Session manager for CRUD operations on agent sessions
// ABOUTME: Maintains session list, status tracking, and message history
package client

import (
	"fmt"
	"sync"
	"time"
)

type SessionStatus int

const (
	StatusActive SessionStatus = iota
	StatusIdle
	StatusDead
)

func (s SessionStatus) String() string {
	switch s {
	case StatusActive:
		return "Active"
	case StatusIdle:
		return "Idle"
	case StatusDead:
		return "Dead"
	default:
		return "Unknown"
	}
}

func (s SessionStatus) Icon() string {
	switch s {
	case StatusActive:
		return "⚡"
	case StatusIdle:
		return "💤"
	case StatusDead:
		return "💀"
	default:
		return "❓"
	}
}

type Session struct {
	ID          string
	WorkingDir  string
	DisplayName string
	Status      SessionStatus
	CreatedAt   time.Time
	LastActive  time.Time
}

type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

func (sm *SessionManager) Create(id, workingDir, displayName string) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.sessions[id]; exists {
		return nil, fmt.Errorf("session %s already exists", id)
	}

	now := time.Now()
	sess := &Session{
		ID:          id,
		WorkingDir:  workingDir,
		DisplayName: displayName,
		Status:      StatusActive,
		CreatedAt:   now,
		LastActive:  now,
	}

	sm.sessions[id] = sess
	return sess, nil
}

func (sm *SessionManager) Get(id string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sess, exists := sm.sessions[id]
	return sess, exists
}

func (sm *SessionManager) List() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*Session, 0, len(sm.sessions))
	for _, sess := range sm.sessions {
		sessions = append(sessions, sess)
	}
	return sessions
}

func (sm *SessionManager) Delete(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.sessions[id]; !exists {
		return fmt.Errorf("session %s not found", id)
	}

	delete(sm.sessions, id)
	return nil
}

func (sm *SessionManager) UpdateStatus(id string, status SessionStatus) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sess, exists := sm.sessions[id]
	if !exists {
		return fmt.Errorf("session %s not found", id)
	}

	sess.Status = status
	if status == StatusActive {
		sess.LastActive = time.Now()
	}

	return nil
}

func (sm *SessionManager) Rename(id, newName string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sess, exists := sm.sessions[id]
	if !exists {
		return fmt.Errorf("session %s not found", id)
	}

	sess.DisplayName = newName
	return nil
}
