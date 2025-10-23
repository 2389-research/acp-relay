// ABOUTME: Database package for logging all ACP relay messages to SQLite
// ABOUTME: Provides message logging, session tracking, and query capabilities

package db

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schemaSQL string

type DB struct {
	conn *sql.DB
}

type MessageDirection string

const (
	DirectionClientToRelay MessageDirection = "client_to_relay"
	DirectionRelayToAgent  MessageDirection = "relay_to_agent"
	DirectionAgentToRelay  MessageDirection = "agent_to_relay"
	DirectionRelayToClient MessageDirection = "relay_to_client"
)

// Open opens or creates the SQLite database
func Open(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Create tables
	if _, err := conn.Exec(schemaSQL); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	log.Printf("Database initialized at %s", dbPath)
	return &DB{conn: conn}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// CreateSession logs a new session
func (db *DB) CreateSession(sessionID, workingDir string) error {
	_, err := db.conn.Exec(
		"INSERT INTO sessions (id, working_directory) VALUES (?, ?)",
		sessionID, workingDir,
	)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

// UpdateSessionAgentID updates the agent session ID after it's received
func (db *DB) UpdateSessionAgentID(sessionID, agentSessionID string) error {
	_, err := db.conn.Exec(
		"UPDATE sessions SET agent_session_id = ? WHERE id = ?",
		agentSessionID, sessionID,
	)
	if err != nil {
		return fmt.Errorf("failed to update agent session ID: %w", err)
	}
	return nil
}

// CloseSession marks a session as closed
func (db *DB) CloseSession(sessionID string) error {
	_, err := db.conn.Exec(
		"UPDATE sessions SET closed_at = CURRENT_TIMESTAMP WHERE id = ?",
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("failed to close session: %w", err)
	}
	return nil
}

// LogMessage logs a message with direction and parsed details
func (db *DB) LogMessage(sessionID string, direction MessageDirection, rawMessage []byte) error {
	// Parse message to extract useful fields
	var msg map[string]interface{}
	var messageType, method string
	var jsonrpcID *int64

	if err := json.Unmarshal(rawMessage, &msg); err == nil {
		// Determine message type
		if _, hasMethod := msg["method"]; hasMethod {
			if _, hasID := msg["id"]; hasID {
				messageType = "request"
			} else {
				messageType = "notification"
			}
			if m, ok := msg["method"].(string); ok {
				method = m
			}
		} else if _, hasResult := msg["result"]; hasResult {
			messageType = "response"
		} else if _, hasError := msg["error"]; hasError {
			messageType = "response"
		}

		// Extract ID if present
		if id, ok := msg["id"]; ok {
			switch v := id.(type) {
			case float64:
				idVal := int64(v)
				jsonrpcID = &idVal
			case int64:
				jsonrpcID = &v
			case int:
				idVal := int64(v)
				jsonrpcID = &idVal
			}
		}
	}

	_, err := db.conn.Exec(
		`INSERT INTO messages (session_id, direction, message_type, method, jsonrpc_id, raw_message)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID, direction, messageType, method, jsonrpcID, string(rawMessage),
	)
	if err != nil {
		return fmt.Errorf("failed to log message: %w", err)
	}
	return nil
}

// GetSessionMessages retrieves all messages for a session
func (db *DB) GetSessionMessages(sessionID string) ([]Message, error) {
	rows, err := db.conn.Query(
		`SELECT id, session_id, direction, message_type, method, jsonrpc_id, raw_message, timestamp
		 FROM messages WHERE session_id = ? ORDER BY timestamp ASC`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		var jsonrpcID sql.NullInt64
		var method sql.NullString
		var messageType sql.NullString

		err := rows.Scan(&m.ID, &m.SessionID, &m.Direction, &messageType, &method, &jsonrpcID, &m.RawMessage, &m.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		if jsonrpcID.Valid {
			m.JSONRPCId = &jsonrpcID.Int64
		}
		if method.Valid {
			m.Method = method.String
		}
		if messageType.Valid {
			m.MessageType = messageType.String
		}

		messages = append(messages, m)
	}

	return messages, nil
}

// Message represents a logged message
type Message struct {
	ID          int64
	SessionID   string
	Direction   MessageDirection
	MessageType string
	Method      string
	JSONRPCId   *int64
	RawMessage  string
	Timestamp   time.Time
}

// GetAllSessions retrieves all sessions
func (db *DB) GetAllSessions() ([]Session, error) {
	rows, err := db.conn.Query(
		`SELECT id, agent_session_id, working_directory, created_at, closed_at
		 FROM sessions ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		var agentSessionID sql.NullString
		var closedAt sql.NullTime

		err := rows.Scan(&s.ID, &agentSessionID, &s.WorkingDirectory, &s.CreatedAt, &closedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		if agentSessionID.Valid {
			s.AgentSessionID = agentSessionID.String
		}
		if closedAt.Valid {
			s.ClosedAt = &closedAt.Time
		}

		sessions = append(sessions, s)
	}

	return sessions, nil
}

// Session represents a logged session
type Session struct {
	ID               string
	AgentSessionID   string
	WorkingDirectory string
	CreatedAt        time.Time
	ClosedAt         *time.Time
}
