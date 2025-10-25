package session

import (
	"testing"
)

func TestNewConnectionManager(t *testing.T) {
	// Create a mock session
	sess := &Session{
		ID: "sess_test123",
	}

	cm := NewConnectionManager(sess)

	if cm == nil {
		t.Fatal("NewConnectionManager returned nil")
	}

	if cm.session != sess {
		t.Error("ConnectionManager.session not set correctly")
	}

	if cm.connections == nil {
		t.Error("ConnectionManager.connections map not initialized")
	}

	if len(cm.connections) != 0 {
		t.Errorf("Expected 0 connections, got %d", len(cm.connections))
	}
}
