// ABOUTME: Tests for DatabaseClient SQLite integration
// ABOUTME: Tests session and message retrieval from relay server database

//nolint:goconst // test file uses repeated SQL schemas and test strings for clarity
package client

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDatabase creates an in-memory database with schema and test data.
func setupTestDatabase(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Create schema
	schema := `
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			agent_session_id TEXT,
			working_directory TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			closed_at DATETIME
		);

		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			direction TEXT NOT NULL CHECK(direction IN ('client_to_relay', 'relay_to_agent', 'agent_to_relay', 'relay_to_client')),
			message_type TEXT,
			method TEXT,
			jsonrpc_id INTEGER,
			raw_message TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		);
	`
	_, err = db.Exec(schema)
	require.NoError(t, err)

	return db
}

// insertTestSession inserts a test session into the database.
func insertTestSession(t *testing.T, db *sql.DB, id, workingDir string, closedAt *time.Time) {
	insertTestSessionWithTime(t, db, id, workingDir, time.Now(), closedAt)
}

// insertTestSessionWithTime inserts a test session with a specific created_at time.
func insertTestSessionWithTime(t *testing.T, db *sql.DB, id, workingDir string, createdAt time.Time, closedAt *time.Time) {
	query := "INSERT INTO sessions (id, working_directory, created_at, closed_at) VALUES (?, ?, ?, ?)"
	var closedAtStr interface{}
	if closedAt != nil {
		closedAtStr = closedAt.Format("2006-01-02 15:04:05")
	}
	_, err := db.Exec(query, id, workingDir, createdAt.Format("2006-01-02 15:04:05"), closedAtStr)
	require.NoError(t, err)
}

// insertTestMessage inserts a test message into the database.
func insertTestMessage(t *testing.T, db *sql.DB, sessionID, direction, messageType, method string, rawMsg map[string]interface{}) {
	rawJSON, err := json.Marshal(rawMsg)
	require.NoError(t, err)

	query := "INSERT INTO messages (session_id, direction, message_type, method, raw_message, timestamp) VALUES (?, ?, ?, ?, ?, datetime('now'))"
	_, err = db.Exec(query, sessionID, direction, messageType, method, string(rawJSON))
	require.NoError(t, err)
}

func TestNewDatabaseClient_Success(t *testing.T) {
	// Create a temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create the database with schema
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)

	schema := `
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			agent_session_id TEXT,
			working_directory TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			closed_at DATETIME
		);
		CREATE TABLE messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			direction TEXT NOT NULL,
			message_type TEXT,
			method TEXT,
			jsonrpc_id INTEGER,
			raw_message TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`
	_, err = db.Exec(schema)
	require.NoError(t, err)
	_ = db.Close()

	// Test creating DatabaseClient
	client, err := NewDatabaseClient(dbPath)
	assert.NoError(t, err)
	assert.NotNil(t, client)
	defer func() { _ = client.Close() }()
}

func TestNewDatabaseClient_DefaultPath(t *testing.T) {
	// Test with empty path (should use default)
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	defaultPath := filepath.Join(homeDir, ".local", "share", "acp-relay", "db.sqlite")

	// Create the directory structure
	err = os.MkdirAll(filepath.Dir(defaultPath), 0750)
	require.NoError(t, err)

	// Create database with schema
	db, err := sql.Open("sqlite3", defaultPath)
	require.NoError(t, err)

	schema := `
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			agent_session_id TEXT,
			working_directory TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			closed_at DATETIME
		);
		CREATE TABLE messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			direction TEXT NOT NULL,
			message_type TEXT,
			method TEXT,
			jsonrpc_id INTEGER,
			raw_message TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`
	_, err = db.Exec(schema)
	require.NoError(t, err)
	_ = db.Close()

	// Cleanup after test
	defer func() { _ = os.Remove(defaultPath) }()

	client, err := NewDatabaseClient("")
	assert.NoError(t, err)
	assert.NotNil(t, client)
	defer func() { _ = client.Close() }()
}

func TestNewDatabaseClient_InvalidPath(t *testing.T) {
	// Test with invalid path
	client, err := NewDatabaseClient("/nonexistent/path/to/database.db")
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestGetAllSessions_WithActiveSessions(t *testing.T) {
	db := setupTestDatabase(t)
	defer func() { _ = db.Close() }()

	// Insert test sessions
	insertTestSession(t, db, "session-1", "/path/to/project1", nil)
	insertTestSession(t, db, "session-2", "/path/to/project2", nil)
	closedTime := time.Now().Add(-1 * time.Hour)
	insertTestSession(t, db, "session-3", "/path/to/project3", &closedTime)

	// Create a temporary file for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Copy in-memory db to file (workaround for testing)
	backupDB, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer func() { _ = backupDB.Close() }()

	// Recreate schema and data in file
	schema := `
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			agent_session_id TEXT,
			working_directory TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			closed_at DATETIME
		);
	`
	_, err = backupDB.Exec(schema)
	require.NoError(t, err)

	// Insert with different timestamps to test ordering
	baseTime := time.Now().Add(-3 * time.Hour)
	insertTestSessionWithTime(t, backupDB, "session-1", "/path/to/project1", baseTime, nil)
	insertTestSessionWithTime(t, backupDB, "session-2", "/path/to/project2", baseTime.Add(1*time.Hour), nil)
	insertTestSessionWithTime(t, backupDB, "session-3", "/path/to/project3", baseTime.Add(2*time.Hour), &closedTime)

	// Test GetAllSessions
	client, err := NewDatabaseClient(dbPath)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	sessions, err := client.GetAllSessions()
	assert.NoError(t, err)
	assert.Len(t, sessions, 3)

	// Verify sessions are ordered by created_at DESC
	assert.Equal(t, "session-3", sessions[0].ID)
	assert.Equal(t, "session-2", sessions[1].ID)
	assert.Equal(t, "session-1", sessions[2].ID)

	// Verify active status
	assert.True(t, sessions[2].IsActive)  // session-1
	assert.True(t, sessions[1].IsActive)  // session-2
	assert.False(t, sessions[0].IsActive) // session-3 (closed)
}

func TestGetAllSessions_EmptyDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	schema := `
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			agent_session_id TEXT,
			working_directory TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			closed_at DATETIME
		);
	`
	_, err = db.Exec(schema)
	require.NoError(t, err)

	client, err := NewDatabaseClient(dbPath)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	sessions, err := client.GetAllSessions()
	assert.NoError(t, err)
	assert.Empty(t, sessions)
}

//nolint:funlen // test function with extensive setup
func TestGetSessionMessages_WithMessages(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Create schema
	schema := `
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			agent_session_id TEXT,
			working_directory TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			closed_at DATETIME
		);
		CREATE TABLE messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			direction TEXT NOT NULL,
			message_type TEXT,
			method TEXT,
			jsonrpc_id INTEGER,
			raw_message TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`
	_, err = db.Exec(schema)
	require.NoError(t, err)

	// Insert test session
	insertTestSession(t, db, "session-1", "/path/to/project", nil)

	// Insert test messages (client_to_relay: user messages)
	userMsg := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/prompt",
		"params": map[string]interface{}{
			"prompt": "Hello, Agent!",
		},
		"id": 1,
	}
	insertTestMessage(t, db, "session-1", "client_to_relay", "request", "session/prompt", userMsg)

	// Insert agent response (relay_to_client: agent chunks)
	agentChunk := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "session/chunk",
		"params": map[string]interface{}{
			"chunk": map[string]interface{}{
				"type": "text",
				"text": "Hello, Doctor Biz!",
			},
		},
	}
	insertTestMessage(t, db, "session-1", "relay_to_client", "notification", "session/chunk", agentChunk)

	// Test GetSessionMessages
	client, err := NewDatabaseClient(dbPath)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	messages, err := client.GetSessionMessages("session-1")
	assert.NoError(t, err)
	assert.Len(t, messages, 2)

	// Verify first message (user)
	assert.Equal(t, MessageTypeUser, messages[0].Type)
	assert.Contains(t, messages[0].Content, "Hello, Agent!")

	// Verify second message (agent)
	assert.Equal(t, MessageTypeAgent, messages[1].Type)
	assert.Contains(t, messages[1].Content, "Hello, Doctor Biz!")
}

func TestGetSessionMessages_EmptySession(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	schema := `
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			agent_session_id TEXT,
			working_directory TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			closed_at DATETIME
		);
		CREATE TABLE messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			direction TEXT NOT NULL,
			message_type TEXT,
			method TEXT,
			jsonrpc_id INTEGER,
			raw_message TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`
	_, err = db.Exec(schema)
	require.NoError(t, err)

	insertTestSession(t, db, "session-1", "/path/to/project", nil)

	client, err := NewDatabaseClient(dbPath)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	messages, err := client.GetSessionMessages("session-1")
	assert.NoError(t, err)
	assert.Empty(t, messages)
}

func TestMarkSessionClosed(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	schema := `
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			agent_session_id TEXT,
			working_directory TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			closed_at DATETIME
		);
	`
	_, err = db.Exec(schema)
	require.NoError(t, err)

	insertTestSession(t, db, "session-1", "/path/to/project", nil)

	// Verify session is active
	var closedAt sql.NullTime
	err = db.QueryRow("SELECT closed_at FROM sessions WHERE id = ?", "session-1").Scan(&closedAt)
	require.NoError(t, err)
	assert.False(t, closedAt.Valid)

	// Mark session as closed
	client, err := NewDatabaseClient(dbPath)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	err = client.MarkSessionClosed("session-1")
	assert.NoError(t, err)

	// Verify session is now closed
	err = db.QueryRow("SELECT closed_at FROM sessions WHERE id = ?", "session-1").Scan(&closedAt)
	require.NoError(t, err)
	assert.True(t, closedAt.Valid)
	assert.False(t, closedAt.Time.IsZero())
}

func TestMarkSessionClosed_NonexistentSession(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	schema := `
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			agent_session_id TEXT,
			working_directory TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			closed_at DATETIME
		);
	`
	_, err = db.Exec(schema)
	require.NoError(t, err)

	client, err := NewDatabaseClient(dbPath)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// Should not error, just update 0 rows
	err = client.MarkSessionClosed("nonexistent-session")
	assert.NoError(t, err)
}

func TestDatabaseClient_Close(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	schema := `
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			agent_session_id TEXT,
			working_directory TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			closed_at DATETIME
		);
	`
	_, err = db.Exec(schema)
	require.NoError(t, err)

	client, err := NewDatabaseClient(dbPath)
	require.NoError(t, err)

	err = client.Close()
	assert.NoError(t, err)

	// Verify database is closed by attempting another operation
	_, err = client.GetAllSessions()
	assert.Error(t, err)
}
