package session

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestCreateSession(t *testing.T) {
	tmpDir := t.TempDir()

	// Get absolute path to mock agent
	_, filename, _, _ := runtime.Caller(0)
	mockAgentPath := filepath.Join(filepath.Dir(filename), "testdata", "mock_agent.py")

	mgr := NewManager(ManagerConfig{
		AgentCommand: "python3",
		AgentArgs:    []string{mockAgentPath},
		AgentEnv:     map[string]string{},
	}, nil) // nil db for test

	sess, err := mgr.CreateSession(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer mgr.CloseSession(sess.ID)

	if sess.ID == "" {
		t.Error("expected session ID to be set")
	}

	if sess.WorkingDir != tmpDir {
		t.Errorf("expected working dir %s, got %s", tmpDir, sess.WorkingDir)
	}

	if sess.AgentStdin == nil {
		t.Error("expected stdin to be set")
	}
}

func TestCloseSession(t *testing.T) {
	tmpDir := t.TempDir()

	// Get absolute path to mock agent
	_, filename, _, _ := runtime.Caller(0)
	mockAgentPath := filepath.Join(filepath.Dir(filename), "testdata", "mock_agent.py")

	mgr := NewManager(ManagerConfig{
		AgentCommand: "python3",
		AgentArgs:    []string{mockAgentPath},
		AgentEnv:     map[string]string{},
	}, nil) // nil db for test

	sess, err := mgr.CreateSession(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	err = mgr.CloseSession(sess.ID)
	if err != nil {
		t.Errorf("failed to close session: %v", err)
	}

	// Verify process is killed
	time.Sleep(100 * time.Millisecond)
	if sess.AgentCmd.ProcessState == nil {
		t.Error("expected process to be terminated")
	}
}
