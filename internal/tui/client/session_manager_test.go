// ABOUTME: Unit tests for session manager (CRUD operations)
// ABOUTME: Tests session creation, listing, deletion, and state management
package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSessionManager(t *testing.T) {
	sm := NewSessionManager()

	assert.NotNil(t, sm)
	assert.Empty(t, sm.List())
}

func TestSessionManager_Create(t *testing.T) {
	sm := NewSessionManager()

	sess, err := sm.Create("sess-123", "/tmp/workspace", "Test Session")
	require.NoError(t, err)

	assert.Equal(t, "sess-123", sess.ID)
	assert.Equal(t, "/tmp/workspace", sess.WorkingDir)
	assert.Equal(t, "Test Session", sess.DisplayName)
	assert.Equal(t, StatusActive, sess.Status)
}

func TestSessionManager_Get(t *testing.T) {
	sm := NewSessionManager()
	sm.Create("sess-123", "/tmp/workspace", "Test")

	sess, exists := sm.Get("sess-123")
	assert.True(t, exists)
	assert.Equal(t, "sess-123", sess.ID)

	_, exists = sm.Get("nonexistent")
	assert.False(t, exists)
}

func TestSessionManager_Delete(t *testing.T) {
	sm := NewSessionManager()
	sm.Create("sess-123", "/tmp/workspace", "Test")

	err := sm.Delete("sess-123")
	require.NoError(t, err)

	_, exists := sm.Get("sess-123")
	assert.False(t, exists)

	// Delete nonexistent should error
	err = sm.Delete("nonexistent")
	assert.Error(t, err)
}

func TestSessionManager_List(t *testing.T) {
	sm := NewSessionManager()
	sm.Create("sess-1", "/tmp/1", "One")
	sm.Create("sess-2", "/tmp/2", "Two")

	sessions := sm.List()
	assert.Len(t, sessions, 2)
}

func TestSessionManager_UpdateStatus(t *testing.T) {
	sm := NewSessionManager()
	sess, _ := sm.Create("sess-123", "/tmp", "Test")

	assert.Equal(t, StatusActive, sess.Status)

	err := sm.UpdateStatus("sess-123", StatusIdle)
	require.NoError(t, err)

	updated, _ := sm.Get("sess-123")
	assert.Equal(t, StatusIdle, updated.Status)
}

func TestSessionManager_Rename(t *testing.T) {
	sm := NewSessionManager()
	sm.Create("sess-123", "/tmp", "Original Name")

	// Test successful rename
	err := sm.Rename("sess-123", "New Name")
	require.NoError(t, err)

	sess, _ := sm.Get("sess-123")
	assert.Equal(t, "New Name", sess.DisplayName)

	// Test rename non-existent session
	err = sm.Rename("nonexistent", "Foo")
	assert.Error(t, err)
}
